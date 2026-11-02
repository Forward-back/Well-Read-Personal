package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	werrors "gaze/internal/errors"
	"gaze/internal/models/chat"
	"gaze/internal/models/embedding"
	"gaze/internal/types"
)

const (
	readingStructuredStateVersion = 2
	readingQuestionCap            = 4
	readingSearchQueryCap         = 3
	readingForcedSearchCap        = 4
	readingSearchEvidenceCap      = 5
	readingSearchIterations       = 3
	readingConclusionCap          = 8
	readingContradictionCap       = 6
	readingTermCap                = 8
	readingPrefilterThreshold     = 12
	readingPrefilterLimit         = 10
	readingEmbeddingCacheMax      = 2000
	readingEmbeddingTimeout       = 6 * time.Second
)

const defaultReadingGoal = "审出文档中的含糊表述、缺失参数、跨章节/跨文档矛盾、版本失效风险，并形成可确认的阅读结论"

// readingEmbeddingCache avoids re-embedding unchanged conclusion text across
// rounds. It is intentionally process-local: a wrong/stale entry only changes
// which conclusions are shown in the prompt, never persisted facts.
var readingEmbeddingCache = struct {
	sync.Mutex
	values map[string][]float32
}{values: map[string][]float32{}}

func readingWindowRunes(depth string) int {
	switch normalizeReadingDepth(depth) {
	case types.ReadingDepthQuick:
		return 8000
	case types.ReadingDepthDeep:
		return 1500
	default:
		return 4000
	}
}

func readingStructuredStepLimit(depth string) int {
	switch normalizeReadingDepth(depth) {
	case types.ReadingDepthQuick:
		return 40
	case types.ReadingDepthDeep:
		return 160
	default:
		return 80
	}
}

func readingStructuredTokenBudget(depth string) int {
	switch normalizeReadingDepth(depth) {
	case types.ReadingDepthQuick:
		return 90000
	case types.ReadingDepthDeep:
		return 360000
	default:
		return 180000
	}
}

func readingRoundMaxTokens(depth string) int {
	switch normalizeReadingDepth(depth) {
	case types.ReadingDepthQuick:
		return 3000
	case types.ReadingDepthDeep:
		return 6000
	default:
		return 4000
	}
}

func readingChatOptions(depth string, maxTokens int) *chat.ChatOptions {
	thinking := true
	return &chat.ChatOptions{
		Temperature: 0.2,
		MaxTokens:   maxTokens,
		Thinking:    &thinking,
	}
}

// ensureReadingModelSupportsThinking enforces the product rule that the
// reading model must be configured for thinking mode. Models whose effective
// thinking control is "none" are rejected instead of silently returning a
// chain-of-thought-free reading.
func (s *readingRunService) ensureReadingModelSupportsThinking(
	ctx context.Context, modelID string,
) error {
	model, err := s.modelService.GetModelByID(ctx, modelID)
	if err != nil || model == nil {
		return werrors.NewValidationError("精读模型不存在或不可用")
	}
	control := chat.EffectiveThinkingControl(chat.ConfigFromModel(model))
	if control == "none" {
		return werrors.NewValidationError(
			"精读模型未开启思考模式，请在运维台为精读模型配置支持思考的模型",
		)
	}
	return nil
}

// ensureReadingRunModel applies a lazy model migration to structured runs
// created before the tenant switched to a thinking-capable reading model. The
// run keeps working with the tenant's current reading model instead of dying
// on the mandatory-thinking check.
func (s *readingRunService) ensureReadingRunModel(
	ctx context.Context, run *types.ReadingRun,
) error {
	if err := s.ensureReadingModelSupportsThinking(ctx, run.ReadingModelID); err == nil {
		return nil
	}
	tenant, err := s.tenantService.GetTenantByID(ctx, run.TenantID)
	if err != nil || tenant == nil || strings.TrimSpace(tenant.ReadingModelID) == "" {
		return s.ensureReadingModelSupportsThinking(ctx, run.ReadingModelID)
	}
	if tenant.ReadingModelID == run.ReadingModelID {
		return s.ensureReadingModelSupportsThinking(ctx, run.ReadingModelID)
	}
	if retryErr := s.ensureReadingModelSupportsThinking(ctx, tenant.ReadingModelID); retryErr != nil {
		return s.ensureReadingModelSupportsThinking(ctx, run.ReadingModelID)
	}
	run.ReadingModelID = strings.TrimSpace(tenant.ReadingModelID)
	run.UpdatedAt = time.Now()
	return nil
}

type readingDocument struct {
	Knowledge *types.Knowledge
	Chunks    []*types.Chunk
	Title     string
}

type readingChunkInfo struct {
	Chunk         *types.Chunk
	KnowledgeID   string
	DocumentTitle string
	Heading       string
}

type readingRoundState struct {
	Conclusions    []*types.ReadingConclusion
	Terms          []*types.ReadingTerm
	Contradictions []*types.ReadingContradiction
	Pending        []*types.ReadingQuestionNode
}

type readingEvidenceRef struct {
	ChunkID string `json:"chunk_id"`
	Quote   string `json:"quote"`
}

type readingLLMQuestion struct {
	Text              string `json:"text"`
	Kind              string `json:"kind"`
	GapType           string `json:"gap_type"`
	Why               string `json:"why"`
	RecommendedAnswer string `json:"recommended_answer"`
	SearchQuery       string `json:"search_query"`
}

type readingLLMAnswer struct {
	QuestionIndex int                  `json:"question_index"`
	QuestionText  string               `json:"question_text"`
	Answer        string               `json:"answer"`
	Supported     bool                 `json:"supported"`
	Evidence      []readingEvidenceRef `json:"evidence"`
}

type readingLLMConclusion struct {
	Text     string               `json:"text"`
	Evidence []readingEvidenceRef `json:"evidence"`
}

type readingLLMTerm struct {
	Term       string               `json:"term"`
	Definition string               `json:"definition"`
	Source     []readingEvidenceRef `json:"source"`
}

type readingLLMContradiction struct {
	Description string               `json:"description"`
	EvidenceA   []readingEvidenceRef `json:"evidence_a"`
	EvidenceB   []readingEvidenceRef `json:"evidence_b"`
}

type readingRoundResponse struct {
	Understanding  string                    `json:"understanding"`
	SearchQueries  []string                  `json:"search_queries"`
	Questions      []readingLLMQuestion      `json:"questions"`
	Answers        []readingLLMAnswer        `json:"answers"`
	Conclusions    []readingLLMConclusion    `json:"conclusions"`
	Terms          []readingLLMTerm          `json:"terms"`
	Contradictions []readingLLMContradiction `json:"contradictions"`
}

type readingSearchTrace struct {
	Query    string   `json:"query"`
	HitCount int      `json:"hit_count"`
	ChunkIDs []string `json:"chunk_ids,omitempty"`
}

type readingRoundOutcome struct {
	Understanding  string
	Questions      []*types.ReadingQuestionNode
	Conclusions    []*types.ReadingConclusion
	Terms          []*types.ReadingTerm
	Contradictions []*types.ReadingContradiction
	TokenUsage     int
	DurationMS     int
	Reasoning      string
	SearchTrace    []readingSearchTrace
}

func (o *readingRoundOutcome) pendingCount() int {
	count := 0
	for _, question := range o.Questions {
		if question.Status == types.ReadingQuestionPending {
			count++
		}
	}
	return count
}

const readingStructuredRoundRules = `## 结构化精读输出纪律
你每轮必须完成五拍：理解本轮窗口、提出 2-4 个关键问题、能自答的先自答、报告矛盾、把结果写成结构化 JSON。
思考过程（reasoning）请使用中文。
你是在审问文档，不是替用户总结文档；问题必须指向文档缺口，不得提“主要内容是什么”这类泛问。
每个问题必须给 kind 和 gap_type：
- kind: fact（文档内可查，引擎会强制检索自答）或 decision（必须用户裁定）
- gap_type: ambiguity / missing_param / contradiction / scope / version
每个问题必须有 recommended_answer；没有推荐解读的问题不要提出。
只允许引用本轮窗口或引擎检索结果中出现的 chunk_id；quote 必须是该 chunk 中逐字存在的连续片段。
能从给定材料回答的问题必须给出 supported=true 的 answer，不要抛给用户；只有检索不落的问题才保留为待确认。
结论必须有证据；没有证据不要写结论。矛盾必须给冲突双方出处。不要重复已确认结论和术语。
一次只向用户抛一个问题，引擎会负责逐个提问。`

// structuredStep advances one structured reading round. One round reads one
// window, generates understanding plus questions, auto-answers what it can,
// detects contradictions, and persists all four state tables atomically.
func (s *readingRunService) structuredStep(
	ctx context.Context, caller types.Caller, run *types.ReadingRun,
) (*types.ReadingRun, error) {
	depth := readingRunDepth(run)
	if run.StepCount >= readingStructuredStepLimit(depth) {
		run.Status = types.ReadingRunStopped
		run.UpdatedAt = time.Now()
		_ = s.repo.UpdateRun(ctx, run)
		return nil, werrors.NewValidationError("已达到最大步数，精读已停止")
	}
	if run.TokenUsage >= readingStructuredTokenBudget(depth) {
		run.Status = types.ReadingRunStopped
		run.UpdatedAt = time.Now()
		_ = s.repo.UpdateRun(ctx, run)
		return nil, werrors.NewValidationError("已达到 token 预算，精读已停止")
	}
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return nil, werrors.NewInternalServerError("阅读计划损坏")
	}
	if err := s.ensureReadingRunModel(ctx, run); err != nil {
		run.Status = types.ReadingRunStopped
		run.UpdatedAt = time.Now()
		_ = s.repo.UpdateRun(ctx, run)
		return nil, err
	}
	sectionIndex := -1
	for i := range plan.Sections {
		if !plan.Sections[i].Read {
			sectionIndex = i
			break
		}
	}
	if sectionIndex == -1 {
		return s.finishStructured(ctx, caller, run, &plan)
	}
	section := &plan.Sections[sectionIndex]
	// Every round re-checks ACL before any window content reaches the model.
	if _, err := s.ensureDocumentReadable(ctx, caller, section.KnowledgeID); err != nil {
		run.Status = types.ReadingRunStopped
		run.UpdatedAt = time.Now()
		_ = s.repo.UpdateRun(ctx, run)
		return nil, err
	}
	state, err := s.loadReadingRoundState(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	docs, chunkInfo, chunkSection, err := s.loadReadingDocuments(ctx, caller, run)
	if err != nil {
		return nil, err
	}
	outcome, err := s.runReadingRound(ctx, caller, run, &plan, section, state, docs, chunkInfo, chunkSection)
	if err != nil {
		run.Status = types.ReadingRunFailed
		run.UpdatedAt = time.Now()
		_ = s.repo.UpdateRun(ctx, run)
		return nil, err
	}
	section.Read = true
	planJSON, _ := json.Marshal(plan)
	run.Plan = types.JSON(planJSON)
	run.CurrentSectionID = section.ID
	run.StepCount++
	run.TokenUsage += outcome.TokenUsage
	run.UpdatedAt = time.Now()
	if outcome.pendingCount() > 0 {
		run.Status = types.ReadingRunWaitingUser
	} else {
		run.Status = types.ReadingRunReading
	}
	var firstPending *types.ReadingQuestionNode
	for _, question := range outcome.Questions {
		if question.Status == types.ReadingQuestionPending {
			firstPending = question
			break
		}
	}
	questionJSON := types.JSON("{}")
	if firstPending != nil {
		raw, _ := json.Marshal(firstPending)
		questionJSON = types.JSON(raw)
	}
	roundLog, _ := json.Marshal(map[string]any{
		"understanding":       outcome.Understanding,
		"question_count":      len(outcome.Questions),
		"pending_count":       outcome.pendingCount(),
		"conclusion_count":    len(outcome.Conclusions),
		"term_count":          len(outcome.Terms),
		"contradiction_count": len(outcome.Contradictions),
		"duration_ms":         outcome.DurationMS,
		"reasoning":           readingTruncateRunes(outcome.Reasoning, 30000),
		"search_trace":        outcome.SearchTrace,
	})
	step := &types.ReadingRunStep{
		ID: uuid.NewString(), RunID: run.ID, Seq: run.StepCount,
		Action: "round", SectionID: section.ID,
		Question: questionJSON, NotesPatch: types.JSON(roundLog),
		Citations: evidenceJSON(outcome.roundEvidence()), Status: "done", CreatedAt: time.Now(),
	}
	if run.Status == types.ReadingRunWaitingUser {
		step.Action = "ask_user"
	}
	if err := s.repo.SaveRound(
		ctx, run, outcome.Questions, outcome.Conclusions, outcome.Terms, outcome.Contradictions, step,
	); err != nil {
		return nil, err
	}
	return s.Get(ctx, run.ID)
}

func (o *readingRoundOutcome) roundEvidence() []types.ReadingEvidence {
	seen := map[string]struct{}{}
	out := make([]types.ReadingEvidence, 0)
	add := func(values []types.ReadingEvidence) {
		for _, value := range values {
			key := value.ChunkID + "|" + value.Quote
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, value)
		}
	}
	for _, conclusion := range o.Conclusions {
		add(evidenceFromJSON(conclusion.Evidence))
	}
	for _, question := range o.Questions {
		add(evidenceFromJSON(question.Evidence))
	}
	return out
}

func (s *readingRunService) loadReadingRoundState(
	ctx context.Context, runID string,
) (*readingRoundState, error) {
	conclusions, err := s.repo.ListConclusions(ctx, runID)
	if err != nil {
		return nil, err
	}
	terms, err := s.repo.ListTerms(ctx, runID)
	if err != nil {
		return nil, err
	}
	contradictions, err := s.repo.ListContradictions(ctx, runID)
	if err != nil {
		return nil, err
	}
	questions, err := s.repo.ListQuestions(ctx, runID)
	if err != nil {
		return nil, err
	}
	state := &readingRoundState{Conclusions: conclusions, Terms: terms, Contradictions: contradictions}
	for _, question := range questions {
		if question.Status == types.ReadingQuestionPending {
			state.Pending = append(state.Pending, question)
		}
	}
	return state, nil
}

func (s *readingRunService) loadReadingDocuments(
	ctx context.Context, caller types.Caller, run *types.ReadingRun,
) ([]*readingDocument, map[string]*readingChunkInfo, map[string]string, error) {
	var documentIDs []string
	_ = json.Unmarshal(run.DocumentIDs, &documentIDs)
	docs := make([]*readingDocument, 0, len(documentIDs))
	chunkInfo := map[string]*readingChunkInfo{}
	chunkSection := map[string]string{}
	for _, docID := range documentIDs {
		knowledge, err := s.ensureDocumentReadable(ctx, caller, docID)
		if err != nil {
			return nil, nil, nil, err
		}
		chunks, err := s.chunkRepo.ListChunksByKnowledgeID(ctx, caller.TenantID, knowledge.ID)
		if err != nil {
			return nil, nil, nil, err
		}
		sort.SliceStable(chunks, func(i, j int) bool { return chunks[i].ChunkIndex < chunks[j].ChunkIndex })
		title := knowledge.Title
		if strings.TrimSpace(title) == "" {
			title = knowledge.FileName
		}
		docs = append(docs, &readingDocument{Knowledge: knowledge, Chunks: chunks, Title: title})
		for _, chunk := range chunks {
			if chunk == nil || strings.TrimSpace(chunk.Content) == "" {
				continue
			}
			chunkInfo[chunk.ID] = &readingChunkInfo{
				Chunk: chunk, KnowledgeID: knowledge.ID, DocumentTitle: title,
				Heading: strings.TrimSpace(chunk.ContextHeader),
			}
		}
	}
	var plan types.ReadingPlan
	_ = json.Unmarshal(run.Plan, &plan)
	for _, section := range plan.Sections {
		for _, segment := range section.Segments {
			if _, ok := chunkSection[segment.ID]; !ok {
				chunkSection[segment.ID] = section.ID
			}
		}
		for _, chunkID := range section.ChunkIDs {
			if _, ok := chunkSection[chunkID]; !ok {
				chunkSection[chunkID] = section.ID
			}
		}
	}
	return docs, chunkInfo, chunkSection, nil
}

func (s *readingRunService) runReadingRound(
	ctx context.Context, caller types.Caller, run *types.ReadingRun,
	plan *types.ReadingPlan, section *types.ReadingSection, state *readingRoundState,
	docs []*readingDocument, chunkInfo map[string]*readingChunkInfo, chunkSection map[string]string,
) (*readingRoundOutcome, error) {
	model, err := s.modelService.GetChatModel(ctx, run.ReadingModelID)
	if err != nil {
		return nil, fmt.Errorf("加载精读模型失败: %w", err)
	}
	roundStart := time.Now()
	depth := readingRunDepth(run)
	round := run.StepCount + 1
	stateSummary := s.buildReadingStateSummary(ctx, plan, state, section)
	messages := []chat.Message{
		{Role: "system", Content: s.readingInstructions(ctx, run) + "\n\n" + readingStructuredRoundRules},
		{Role: "user", Content: buildReadingRoundPrompt(run, plan, section, stateSummary, round)},
	}
	var merged readingRoundResponse
	questionOrder := make([]readingLLMQuestion, 0)
	answers := make([]readingLLMAnswer, 0)
	searchEvidence := map[string][]types.ReadingEvidence{}
	totalUsage := 0
	searchUsed := 0
	reasoningParts := make([]string, 0, 2)
	searchTraces := make([]readingSearchTrace, 0, readingSearchQueryCap)
	semanticScope := s.readingSemanticScope(ctx, plan)
	for iteration := 0; iteration < readingSearchIterations; iteration++ {
		parsed, raw, reasoning, usage, err := s.callReadingRoundModel(ctx, model, messages, depth)
		totalUsage += usage
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(reasoning) != "" {
			reasoningParts = append(reasoningParts, strings.TrimSpace(reasoning))
		}
		mergeReadingRoundResponse(&merged, parsed, &questionOrder, &answers)
		queries := uniqueReadingStrings(parsed.SearchQueries)
		if len(queries) == 0 {
			break
		}
		remaining := readingSearchQueryCap - searchUsed
		if remaining <= 0 {
			break
		}
		if len(queries) > remaining {
			queries = queries[:remaining]
		}
		// Search queries are independent read-only lookups; run them together
		// so a round with multiple queries does not serialize retrieval latency.
		resultsByQuery := make([][]types.ReadingEvidence, len(queries))
		var waitGroup sync.WaitGroup
		for index, query := range queries {
			waitGroup.Add(1)
			go func(index int, query string) {
				defer waitGroup.Done()
				resultsByQuery[index] = s.searchReadingEvidenceMerged(
					ctx, query, semanticScope, run, plan, docs, chunkInfo, chunkSection,
				)
			}(index, query)
		}
		waitGroup.Wait()
		var builder strings.Builder
		found := 0
		for index, query := range queries {
			results := resultsByQuery[index]
			searchEvidence[query] = results
			trace := readingSearchTrace{Query: query, HitCount: len(results)}
			for _, evidence := range results {
				trace.ChunkIDs = append(trace.ChunkIDs, evidence.ChunkID)
			}
			searchTraces = append(searchTraces, trace)
			found += len(results)
			if len(results) == 0 {
				fmt.Fprintf(&builder, "检索「%s」：未找到匹配内容。\n\n", query)
				continue
			}
			fmt.Fprintf(&builder, "检索「%s」的结果：\n", query)
			for _, evidence := range results {
				fmt.Fprintf(&builder, "- [chunk %s] %s § %s：%s\n",
					evidence.ChunkID, evidence.DocumentTitle, evidence.Heading, evidence.Quote)
			}
			builder.WriteString("\n")
		}
		searchUsed += len(queries)
		remainingAfter := readingSearchQueryCap - searchUsed
		var instruction string
		if found == 0 {
			instruction = "以上检索均未命中。不要编造证据；请基于已给出的材料返回最终 JSON，search_queries 必须为空。"
		} else {
			instruction = fmt.Sprintf(
				"以上是引擎检索结果。请回答前面的问题（question_index 对应问题顺序），"+
					"并返回最终 JSON；如仍需检索，search_queries 最多再给 %d 个，否则为空数组。",
				remainingAfter,
			)
		}
		messages = append(messages,
			chat.Message{Role: "assistant", Content: raw},
			chat.Message{Role: "user", Content: builder.String() + instruction},
		)
	}
	// Facts must be answered by the engine, not pushed to the user. The model
	// may omit answers/search queries, so force one retrieval pass for every
	// unanswered fact question, then ask the model to answer only those with
	// the retrieved evidence. Decisions stay with the user.
	answerByText := map[string]readingLLMAnswer{}
	answerByIndex := map[int]readingLLMAnswer{}
	for _, answer := range answers {
		if strings.TrimSpace(answer.QuestionText) != "" {
			answerByText[normalizeReadingText(answer.QuestionText)] = answer
		}
		answerByIndex[answer.QuestionIndex] = answer
	}
	type forcedFact struct {
		index    int
		question readingLLMQuestion
		evidence []types.ReadingEvidence
	}
	forced := make([]forcedFact, 0, readingForcedSearchCap)
	for i, question := range questionOrder {
		if i >= readingQuestionCap || len(forced) >= readingForcedSearchCap {
			break
		}
		text := strings.TrimSpace(question.Text)
		if text == "" || readingQuestionIsGeneric(text) ||
			strings.TrimSpace(question.RecommendedAnswer) == "" {
			continue
		}
		answer, ok := answerByText[normalizeReadingText(text)]
		if !ok {
			answer, ok = answerByIndex[i]
		}
		evidence := s.resolveReadingEvidence(answer.Evidence, chunkInfo, chunkSection)
		if ok && answer.Supported && len(evidence) > 0 && strings.TrimSpace(answer.Answer) != "" {
			continue
		}
		kind := normalizeReadingQuestionKind(question.Kind)
		query := strings.TrimSpace(question.SearchQuery)
		if kind != "fact" && query == "" {
			continue
		}
		if query == "" {
			query = text
		}
		results := s.searchReadingEvidenceMerged(
			ctx, query, semanticScope, run, plan, docs, chunkInfo, chunkSection,
		)
		searchEvidence[query] = results
		questionOrder[i].SearchQuery = query
		trace := readingSearchTrace{Query: query, HitCount: len(results)}
		for _, item := range results {
			trace.ChunkIDs = append(trace.ChunkIDs, item.ChunkID)
		}
		searchTraces = append(searchTraces, trace)
		if len(results) == 0 {
			continue
		}
		forced = append(forced, forcedFact{index: i, question: questionOrder[i], evidence: results})
	}
	if len(forced) > 0 {
		var builder strings.Builder
		builder.WriteString("以下是引擎在选中文档内检索到的证据。请只回答标记为 fact 的问题：" +
			"证据充分时给 supported=true 的 answer + evidence（chunk_id 和逐字 quote）；" +
			"证据不足时 supported=false。不要回答 decision 问题，不要新增事实。\n" +
			"返回严格 JSON：{\"answers\":[{\"question_index\":0,\"question_text\":\"问题原文\"," +
			"\"answer\":\"回答\",\"supported\":true,\"evidence\":[{\"chunk_id\":\"chunk id\"," +
			"\"quote\":\"原文连续片段\"}]}],\"conclusions\":[],\"terms\":[],\"contradictions\":[]}。\n\n")
		for index, item := range forced {
			builder.WriteString(fmt.Sprintf(
				"问题 %d：%s\n检索词：%s\n证据：\n",
				index, item.question.Text, item.question.SearchQuery,
			))
			for _, evidence := range item.evidence {
				builder.WriteString(fmt.Sprintf(
					"- [chunk %s] %s § %s：%s\n",
					evidence.ChunkID, evidence.DocumentTitle, evidence.Heading, evidence.Quote,
				))
			}
			builder.WriteString("\n")
		}
		autoMessages := []chat.Message{
			{Role: "system", Content: s.readingInstructions(ctx, run) + "\n\n" + readingStructuredRoundRules},
			{Role: "user", Content: builder.String()},
		}
		parsed, _, reasoning, usage, autoErr := s.callReadingRoundModel(ctx, model, autoMessages, depth)
		totalUsage += usage
		if autoErr == nil {
			if strings.TrimSpace(reasoning) != "" {
				reasoningParts = append(reasoningParts, strings.TrimSpace(reasoning))
			}
			for _, answer := range parsed.Answers {
				if answer.QuestionIndex >= 0 && answer.QuestionIndex < len(forced) {
					answer.QuestionText = forced[answer.QuestionIndex].question.Text
					answers = append(answers, answer)
				} else if strings.TrimSpace(answer.QuestionText) != "" {
					answers = append(answers, answer)
				}
			}
			merged.Conclusions = append(merged.Conclusions, parsed.Conclusions...)
			merged.Terms = append(merged.Terms, parsed.Terms...)
			merged.Contradictions = append(merged.Contradictions, parsed.Contradictions...)
		}
	}

	outcome := s.buildReadingRoundOutcome(
		run, section, round, &merged, questionOrder, answers, searchEvidence,
		state, chunkInfo, chunkSection,
	)
	outcome.TokenUsage = totalUsage
	outcome.DurationMS = int(time.Since(roundStart).Milliseconds())
	outcome.Reasoning = strings.Join(reasoningParts, "\n\n")
	outcome.SearchTrace = searchTraces
	return outcome, nil
}

func (s *readingRunService) callReadingRoundModel(
	ctx context.Context, model chat.Chat, messages []chat.Message, depth string,
) (*readingRoundResponse, string, string, int, error) {
	totalUsage := 0
	var lastErr error
	reasoningParts := make([]string, 0, 2)
	joinReasoning := func() string { return strings.Join(reasoningParts, "\n\n") }
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := model.Chat(ctx, messages, readingChatOptions(depth, readingRoundMaxTokens(depth)))
		if err != nil {
			return nil, "", joinReasoning(), totalUsage, fmt.Errorf("精读模型调用失败: %w", err)
		}
		totalUsage += resp.Usage.TotalTokens
		if strings.TrimSpace(resp.ReasoningContent) != "" {
			reasoningParts = append(reasoningParts, strings.TrimSpace(resp.ReasoningContent))
		}
		parsed, err := parseReadingRound(resp.Content)
		if err == nil {
			return parsed, resp.Content, joinReasoning(), totalUsage, nil
		}
		lastErr = err
		messages = append(messages,
			chat.Message{Role: "assistant", Content: resp.Content},
			chat.Message{Role: "user", Content: "上次输出不是有效 JSON。请只返回严格 JSON，不要包含解释或 Markdown。"},
		)
	}
	return nil, "", joinReasoning(), totalUsage, fmt.Errorf("精读模型连续返回无效 JSON: %w", lastErr)
}

func mergeReadingRoundResponse(
	merged *readingRoundResponse, parsed *readingRoundResponse,
	questionOrder *[]readingLLMQuestion, answers *[]readingLLMAnswer,
) {
	if strings.TrimSpace(parsed.Understanding) != "" {
		merged.Understanding = strings.TrimSpace(parsed.Understanding)
	}
	seenQuestions := map[string]struct{}{}
	for _, question := range *questionOrder {
		seenQuestions[normalizeReadingText(question.Text)] = struct{}{}
	}
	for _, question := range parsed.Questions {
		text := strings.TrimSpace(question.Text)
		if text == "" {
			continue
		}
		key := normalizeReadingText(text)
		if _, ok := seenQuestions[key]; ok {
			continue
		}
		seenQuestions[key] = struct{}{}
		*questionOrder = append(*questionOrder, question)
	}
	*answers = append(*answers, parsed.Answers...)
	merged.Conclusions = append(merged.Conclusions, parsed.Conclusions...)
	merged.Terms = append(merged.Terms, parsed.Terms...)
	merged.Contradictions = append(merged.Contradictions, parsed.Contradictions...)
	merged.SearchQueries = parsed.SearchQueries
}

func buildReadingRoundPrompt(
	run *types.ReadingRun, plan *types.ReadingPlan, section *types.ReadingSection,
	stateSummary string, round int,
) string {
	var builder strings.Builder
	builder.WriteString("阅读目标：" + readingGoal(run) + "\n")
	builder.WriteString("阅读深度：" + readingDepthLabel(plan.Depth) + "\n")
	builder.WriteString(fmt.Sprintf("当前轮次：第 %d 轮；当前阅读单元：%s · %s\n\n", round, section.Title, section.Heading))
	builder.WriteString("本轮原文（chunk 标记仅用于引用，不要写进结论正文）：\n")
	if len(section.Segments) > 0 {
		for _, segment := range section.Segments {
			builder.WriteString(fmt.Sprintf("[chunk %s | %s]\n%s\n\n", segment.ID, segment.Heading, segment.Content))
		}
	} else {
		builder.WriteString(section.Content + "\n\n")
		if len(section.ChunkIDs) > 0 {
			builder.WriteString("可用 chunk_id：" + strings.Join(section.ChunkIDs, ", ") + "\n\n")
		}
	}
	builder.WriteString(stateSummary)
	builder.WriteString(`
返回严格 JSON（不要 Markdown、不要解释）：
{
  "understanding": "本轮理解，2-4 句",
  "search_queries": ["需要跨章节/跨文档证据时填写，最多 2 个；否则空数组"],
  "questions": [
    {"text": "关键问题", "kind": "fact|decision", "gap_type": "ambiguity|missing_param|contradiction|scope|version", "why": "为什么关键", "recommended_answer": "推荐解读", "search_query": "用于自动检索的关键词"}
  ],
  "answers": [
    {"question_index": 0, "question_text": "问题原文", "answer": "回答", "supported": true, "evidence": [{"chunk_id": "chunk id", "quote": "原文连续片段"}]}
  ],
  "conclusions": [
    {"text": "结论", "evidence": [{"chunk_id": "chunk id", "quote": "原文连续片段"}]}
  ],
  "terms": [
    {"term": "术语", "definition": "定义", "source": [{"chunk_id": "chunk id", "quote": "原文连续片段"}]}
  ],
  "contradictions": [
    {"description": "冲突说明", "evidence_a": [{"chunk_id": "chunk id", "quote": "原文"}], "evidence_b": [{"chunk_id": "chunk id", "quote": "原文"}]}
  ]
}

规则：
- questions 每轮 2-4 个；一次只向用户抛一个，引擎负责逐个提问。
- 能根据本轮原文或检索结果回答的问题，必须给 supported=true 的 answer，不要留在待确认。
- 仅当证据不足时才保留为待确认问题；推荐解读要短，理由要引用具体 chunk。
- conclusions 只能写有证据的结论；没有证据的推断不要写成结论。
- 同一事实在不同文档或章节口径不一致时，写入 contradictions，并给双方出处。
- 不要重复已确认结论和已收录术语；发现旧结论被新证据推翻时必须报告矛盾。
`)
	return builder.String()
}

func (s *readingRunService) buildReadingStateSummary(
	ctx context.Context, plan *types.ReadingPlan,
	state *readingRoundState, section *types.ReadingSection,
) string {
	conclusions := s.readingStateConclusions(ctx, plan, state, section)
	var builder strings.Builder
	builder.WriteString("已确认结论：\n")
	if len(conclusions) == 0 {
		builder.WriteString("（暂无）\n")
	} else {
		for _, conclusion := range conclusions {
			label := ""
			if conclusion.Source == "user" {
				label = "（用户裁定）"
			}
			builder.WriteString("- " + readingTruncateRunes(conclusion.Text, 300) + label + "\n")
		}
	}
	builder.WriteString("\n已收录术语：\n")
	written := 0
	for i := len(state.Terms) - 1; i >= 0 && written < 20; i-- {
		term := state.Terms[i]
		builder.WriteString("- " + term.Term + "：" + readingTruncateRunes(term.Definition, 160) + "\n")
		written++
	}
	if written == 0 {
		builder.WriteString("（暂无）\n")
	}
	builder.WriteString("\n未决问题：\n")
	if len(state.Pending) == 0 {
		builder.WriteString("（暂无）\n")
	} else {
		for _, question := range state.Pending {
			builder.WriteString("- " + readingTruncateRunes(question.Text, 200) + "\n")
		}
	}
	builder.WriteString("\n已有矛盾：\n")
	if len(state.Contradictions) == 0 {
		builder.WriteString("（暂无）\n")
	} else {
		for _, contradiction := range state.Contradictions {
			builder.WriteString("- " + readingTruncateRunes(contradiction.Description, 200) + "\n")
		}
	}
	builder.WriteString("\n")
	return builder.String()
}

// readingStateConclusions prefers semantically relevant confirmed conclusions
// once the run accumulates enough of them; otherwise it falls back to the most
// recent 20. Embedding failures are fail-open: the fallback keeps the round
// working and only costs prompt tokens.
func (s *readingRunService) readingStateConclusions(
	ctx context.Context, plan *types.ReadingPlan,
	state *readingRoundState, section *types.ReadingSection,
) []*types.ReadingConclusion {
	if selected := s.selectRelevantConclusions(ctx, plan, state, section); len(selected) > 0 {
		return selected
	}
	out := make([]*types.ReadingConclusion, 0, 20)
	for i := len(state.Conclusions) - 1; i >= 0 && len(out) < 20; i-- {
		if state.Conclusions[i].Status == types.ReadingConclusionConfirmed {
			out = append(out, state.Conclusions[i])
		}
	}
	return out
}

func (s *readingRunService) selectRelevantConclusions(
	ctx context.Context, plan *types.ReadingPlan,
	state *readingRoundState, section *types.ReadingSection,
) []*types.ReadingConclusion {
	confirmed := make([]*types.ReadingConclusion, 0)
	for _, conclusion := range state.Conclusions {
		if conclusion.Status == types.ReadingConclusionConfirmed {
			confirmed = append(confirmed, conclusion)
		}
	}
	if len(confirmed) <= readingPrefilterThreshold || plan == nil || strings.TrimSpace(plan.KnowledgeBaseID) == "" {
		return nil
	}
	knowledgeBase, err := s.kbService.GetKnowledgeBaseByID(ctx, plan.KnowledgeBaseID)
	if err != nil || knowledgeBase == nil || strings.TrimSpace(knowledgeBase.EmbeddingModelID) == "" {
		return nil
	}
	embedder, err := s.modelService.GetEmbeddingModel(ctx, knowledgeBase.EmbeddingModelID)
	if err != nil || embedder == nil {
		return nil
	}
	texts := make([]string, 0, len(confirmed)+1)
	texts = append(texts, readingTruncateRunes(section.Content, 2000))
	for _, conclusion := range confirmed {
		texts = append(texts, readingTruncateRunes(conclusion.Text, 500))
	}
	vectors := readingEmbedTexts(ctx, embedder, knowledgeBase.EmbeddingModelID, texts)
	if len(vectors) != len(texts) || len(vectors) == 0 {
		return nil
	}
	scores := make([]float32, len(confirmed))
	hasSignal := false
	for i := range confirmed {
		scores[i] = readingCosineSimilarity(vectors[0], vectors[i+1])
		if scores[i] > 0 {
			hasSignal = true
		}
	}
	if !hasSignal {
		return nil
	}
	indexes := readingTopIndexes(scores, readingPrefilterLimit)
	selected := make([]*types.ReadingConclusion, 0, len(indexes))
	for _, index := range indexes {
		selected = append(selected, confirmed[index])
	}
	return selected
}

func readingEmbedTexts(
	ctx context.Context, embedder embedding.Embedder, modelID string, texts []string,
) [][]float32 {
	out := make([][]float32, len(texts))
	missingTexts := make([]string, 0)
	missingIndexes := make([]int, 0)
	readingEmbeddingCache.Lock()
	for i, text := range texts {
		key := modelID + "|" + normalizeReadingText(text)
		if vector, ok := readingEmbeddingCache.values[key]; ok && len(vector) > 0 {
			out[i] = vector
			continue
		}
		missingTexts = append(missingTexts, text)
		missingIndexes = append(missingIndexes, i)
	}
	readingEmbeddingCache.Unlock()
	if len(missingTexts) == 0 {
		return out
	}
	embedCtx, cancel := context.WithTimeout(ctx, readingEmbeddingTimeout)
	defer cancel()
	vectors, err := embedder.BatchEmbed(embedCtx, missingTexts)
	if err != nil || len(vectors) != len(missingTexts) {
		return nil
	}
	readingEmbeddingCache.Lock()
	if len(readingEmbeddingCache.values)+len(vectors) > readingEmbeddingCacheMax {
		readingEmbeddingCache.values = map[string][]float32{}
	}
	for i, vector := range vectors {
		key := modelID + "|" + normalizeReadingText(missingTexts[i])
		readingEmbeddingCache.values[key] = vector
		out[missingIndexes[i]] = vector
	}
	readingEmbeddingCache.Unlock()
	return out
}

func readingCosineSimilarity(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

func readingTopIndexes(scores []float32, limit int) []int {
	indexes := make([]int, len(scores))
	for i := range indexes {
		indexes[i] = i
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		return scores[indexes[i]] > scores[indexes[j]]
	})
	if limit > 0 && len(indexes) > limit {
		indexes = indexes[:limit]
	}
	return indexes
}

func parseReadingRound(content string) (*readingRoundResponse, error) {
	raw := stripJSONFence(content)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("精读模型未返回 JSON")
	}
	var parsed readingRoundResponse
	if err := json.Unmarshal([]byte(raw[start:end+1]), &parsed); err != nil {
		return nil, fmt.Errorf("精读模型返回 JSON 无效: %w", err)
	}
	if strings.TrimSpace(parsed.Understanding) == "" &&
		len(parsed.Questions) == 0 && len(parsed.Answers) == 0 &&
		len(parsed.Conclusions) == 0 && len(parsed.Terms) == 0 &&
		len(parsed.Contradictions) == 0 && len(parsed.SearchQueries) == 0 {
		return nil, fmt.Errorf("精读模型返回空轮次")
	}
	return &parsed, nil
}

func (s *readingRunService) buildReadingRoundOutcome(
	run *types.ReadingRun, section *types.ReadingSection, round int,
	merged *readingRoundResponse, questionOrder []readingLLMQuestion, answers []readingLLMAnswer,
	searchEvidence map[string][]types.ReadingEvidence,
	state *readingRoundState, chunkInfo map[string]*readingChunkInfo, chunkSection map[string]string,
) *readingRoundOutcome {
	outcome := &readingRoundOutcome{Understanding: strings.TrimSpace(merged.Understanding)}
	now := time.Now()

	answerByText := map[string]readingLLMAnswer{}
	answerByIndex := map[int]readingLLMAnswer{}
	for _, answer := range answers {
		if strings.TrimSpace(answer.QuestionText) != "" {
			answerByText[normalizeReadingText(answer.QuestionText)] = answer
		}
		answerByIndex[answer.QuestionIndex] = answer
	}

	existingConclusions := map[string]struct{}{}
	for _, conclusion := range state.Conclusions {
		existingConclusions[normalizeReadingText(conclusion.Text)] = struct{}{}
	}
	seenConclusions := map[string]struct{}{}
	conclusionByText := map[string]*types.ReadingConclusion{}

	appendConclusion := func(text string, evidence []types.ReadingEvidence, source string) *types.ReadingConclusion {
		text = strings.TrimSpace(text)
		if text == "" || len(evidence) == 0 || len(outcome.Conclusions) >= readingConclusionCap {
			return nil
		}
		key := normalizeReadingText(text)
		if _, ok := existingConclusions[key]; ok {
			return nil
		}
		if _, ok := seenConclusions[key]; ok {
			return nil
		}
		seenConclusions[key] = struct{}{}
		gates := types.ReadingConclusionGates{ReferencesOK: true, ConflictFree: true, CrossDocConsistent: true}
		rawGates, _ := json.Marshal(gates)
		conclusion := &types.ReadingConclusion{
			ID: uuid.NewString(), RunID: run.ID, TenantID: run.TenantID,
			SectionID: section.ID, Round: round, Text: text,
			Evidence: evidenceJSON(evidence), Status: types.ReadingConclusionCandidate,
			Source: source, Gates: types.JSON(rawGates), CreatedAt: now, UpdatedAt: now,
		}
		outcome.Conclusions = append(outcome.Conclusions, conclusion)
		conclusionByText[key] = conclusion
		return conclusion
	}

	seenQuestions := map[string]struct{}{}
	questionCount := 0
	for i, candidate := range questionOrder {
		if questionCount >= readingQuestionCap {
			break
		}
		text := strings.TrimSpace(candidate.Text)
		if text == "" {
			continue
		}
		key := normalizeReadingText(text)
		if _, ok := seenQuestions[key]; ok {
			continue
		}
		seenQuestions[key] = struct{}{}
		if readingQuestionIsGeneric(text) {
			continue
		}
		if strings.TrimSpace(candidate.RecommendedAnswer) == "" {
			continue
		}
		questionCount++
		node := &types.ReadingQuestionNode{
			ID: uuid.NewString(), RunID: run.ID, TenantID: run.TenantID,
			SectionID: section.ID, Round: round, Text: text,
			Kind:              normalizeReadingQuestionKind(candidate.Kind),
			GapType:           normalizeReadingGapType(candidate.GapType),
			RecommendedAnswer: strings.TrimSpace(candidate.RecommendedAnswer),
			Why:               strings.TrimSpace(candidate.Why),
			SearchQuery:       strings.TrimSpace(candidate.SearchQuery),
			Evidence:          evidenceJSON(nil),
			AnswerSource:      "", Status: types.ReadingQuestionPending,
			CreatedAt: now, UpdatedAt: now,
		}
		answer, ok := answerByText[key]
		if !ok {
			answer, ok = answerByIndex[i]
			if ok && strings.TrimSpace(answer.QuestionText) != "" &&
				normalizeReadingText(answer.QuestionText) != key {
				ok = false
			}
		}
		evidence := s.resolveReadingEvidence(answer.Evidence, chunkInfo, chunkSection)
		if ok && answer.Supported && len(evidence) > 0 && strings.TrimSpace(answer.Answer) != "" {
			node.Status = types.ReadingQuestionAutoAnswered
			node.Answer = strings.TrimSpace(answer.Answer)
			node.AnswerSource = "auto"
			node.Evidence = evidenceJSON(evidence)
			appendConclusion(text+"："+node.Answer, evidence, "auto")
		} else if query := strings.TrimSpace(candidate.SearchQuery); query != "" {
			if found, exists := searchEvidence[query]; exists && len(found) > 0 {
				node.Evidence = evidenceJSON(found)
			}
		}
		outcome.Questions = append(outcome.Questions, node)
	}

	for _, candidate := range merged.Conclusions {
		evidence := s.resolveReadingEvidence(candidate.Evidence, chunkInfo, chunkSection)
		appendConclusion(candidate.Text, evidence, "model")
	}
	// An auto answer is itself a conclusion candidate; keep it even if the
	// model did not repeat it in the conclusions array.

	existingTerms := map[string]struct{}{}
	for _, term := range state.Terms {
		existingTerms[normalizeReadingText(term.Term)] = struct{}{}
	}
	seenTerms := map[string]struct{}{}
	for _, candidate := range merged.Terms {
		term := strings.TrimSpace(candidate.Term)
		definition := strings.TrimSpace(candidate.Definition)
		if term == "" || definition == "" || len(outcome.Terms) >= readingTermCap {
			continue
		}
		key := normalizeReadingText(term)
		if _, ok := existingTerms[key]; ok {
			continue
		}
		if _, ok := seenTerms[key]; ok {
			continue
		}
		seenTerms[key] = struct{}{}
		source := s.resolveReadingEvidence(candidate.Source, chunkInfo, chunkSection)
		status := types.ReadingTermCandidate
		canonical := false
		if len(source) > 0 {
			status = types.ReadingTermConfirmed
			canonical = true
		}
		outcome.Terms = append(outcome.Terms, &types.ReadingTerm{
			ID: uuid.NewString(), RunID: run.ID, TenantID: run.TenantID,
			Term: term, Definition: definition, Source: evidenceJSON(source),
			Canonical: canonical, Status: status, CreatedAt: now, UpdatedAt: now,
		})
	}

	seenContradictions := map[string]struct{}{}
	conflictChunks := map[string]struct{}{}
	crossDocChunks := map[string]struct{}{}
	for _, candidate := range merged.Contradictions {
		description := strings.TrimSpace(candidate.Description)
		if description == "" || len(outcome.Contradictions) >= readingContradictionCap {
			continue
		}
		key := normalizeReadingText(description)
		if _, ok := seenContradictions[key]; ok {
			continue
		}
		evidenceA := s.resolveReadingEvidence(candidate.EvidenceA, chunkInfo, chunkSection)
		evidenceB := s.resolveReadingEvidence(candidate.EvidenceB, chunkInfo, chunkSection)
		if len(evidenceA) == 0 || len(evidenceB) == 0 {
			continue
		}
		seenContradictions[key] = struct{}{}
		outcome.Contradictions = append(outcome.Contradictions, &types.ReadingContradiction{
			ID: uuid.NewString(), RunID: run.ID, TenantID: run.TenantID,
			Description: description, EvidenceA: evidenceJSON(evidenceA), EvidenceB: evidenceJSON(evidenceB),
			Status: types.ReadingContradictionOpen, CreatedAt: now, UpdatedAt: now,
		})
		for _, evidence := range append(evidenceA, evidenceB...) {
			conflictChunks[evidence.ChunkID] = struct{}{}
		}
		if evidenceA[0].KnowledgeID != evidenceB[0].KnowledgeID {
			for _, evidence := range append(evidenceA, evidenceB...) {
				crossDocChunks[evidence.ChunkID] = struct{}{}
			}
		}
	}

	for _, conclusion := range outcome.Conclusions {
		gates := types.ReadingConclusionGates{
			ReferencesOK: true, ConflictFree: true, CrossDocConsistent: true,
		}
		for _, evidence := range evidenceFromJSON(conclusion.Evidence) {
			if _, ok := conflictChunks[evidence.ChunkID]; ok {
				gates.ConflictFree = false
			}
			if _, ok := crossDocChunks[evidence.ChunkID]; ok {
				gates.CrossDocConsistent = false
			}
		}
		switch {
		case !gates.ConflictFree:
			conclusion.Status = types.ReadingConclusionContradicted
		case gates.ReferencesOK && gates.CrossDocConsistent:
			conclusion.Status = types.ReadingConclusionConfirmed
		default:
			conclusion.Status = types.ReadingConclusionCandidate
		}
		rawGates, _ := json.Marshal(gates)
		conclusion.Gates = types.JSON(rawGates)
	}
	// A question whose auto answer was contradicted must stay visible as an
	// unresolved item instead of pretending it was answered cleanly.
	for _, question := range outcome.Questions {
		if question.Status != types.ReadingQuestionAutoAnswered {
			continue
		}
		if conclusion, ok := conclusionByText[normalizeReadingText(question.Text+"："+question.Answer)]; ok &&
			conclusion.Status == types.ReadingConclusionContradicted {
			question.Status = types.ReadingQuestionContradicted
		}
	}
	return outcome
}

func (s *readingRunService) resolveReadingEvidence(
	refs []readingEvidenceRef, chunkInfo map[string]*readingChunkInfo, chunkSection map[string]string,
) []types.ReadingEvidence {
	out := make([]types.ReadingEvidence, 0, len(refs))
	seen := map[string]struct{}{}
	for _, ref := range refs {
		chunkID := strings.TrimSpace(ref.ChunkID)
		if chunkID == "" {
			continue
		}
		if _, ok := seen[chunkID]; ok {
			continue
		}
		info, ok := chunkInfo[chunkID]
		if !ok || info.Chunk == nil {
			continue
		}
		quote := strings.TrimSpace(ref.Quote)
		if quote == "" || !readingQuoteContained(quote, info.Chunk.Content) {
			quote = readingEvidenceSnippet(info.Chunk.Content, quote, 220)
		}
		if strings.TrimSpace(quote) == "" {
			continue
		}
		seen[chunkID] = struct{}{}
		sectionID := chunkSection[chunkID]
		out = append(out, types.ReadingEvidence{
			KnowledgeID: info.KnowledgeID, DocumentTitle: info.DocumentTitle,
			ChunkID: chunkID, SectionID: sectionID, Heading: info.Heading, Quote: quote,
		})
	}
	return out
}

func readingQuoteContained(quote, content string) bool {
	normalizedQuote := normalizeReadingText(quote)
	normalizedContent := normalizeReadingText(content)
	if utf8.RuneCountInString(normalizedQuote) < 4 {
		return false
	}
	return strings.Contains(normalizedContent, normalizedQuote)
}

func readingEvidenceSnippet(content, quote string, max int) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	needle := strings.TrimSpace(quote)
	if needle != "" {
		if index := strings.Index(content, needle); index >= 0 {
			return readingRuneWindow(content, index, max)
		}
		// Try the first meaningful run of the paraphrase so we still return a
		// real snippet from the chunk rather than the model's wording.
		for _, token := range readingQueryTokens(needle) {
			if index := strings.Index(content, token); index >= 0 {
				return readingRuneWindow(content, index, max)
			}
		}
	}
	return readingTruncateRunes(content, max)
}

func readingRuneWindow(content string, index, max int) string {
	runes := []rune(content)
	byteIndex := len([]rune(content[:index]))
	start := byteIndex - 40
	if start < 0 {
		start = 0
	}
	end := start + max
	if end > len(runes) {
		end = len(runes)
	}
	snippet := string(runes[start:end])
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(runes) {
		snippet += "…"
	}
	return strings.TrimSpace(snippet)
}

func (s *readingRunService) searchReadingEvidence(
	query string, docs []*readingDocument, chunkSection map[string]string,
) []types.ReadingEvidence {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	tokens := readingQueryTokens(query)
	normalizedQuery := normalizeReadingText(query)
	type scored struct {
		score    int
		evidence types.ReadingEvidence
	}
	results := make([]scored, 0)
	seen := map[string]struct{}{}
	for _, doc := range docs {
		for _, chunk := range doc.Chunks {
			if chunk == nil || strings.TrimSpace(chunk.Content) == "" {
				continue
			}
			if _, ok := seen[chunk.ID]; ok {
				continue
			}
			content := strings.ToLower(chunk.Content)
			heading := strings.ToLower(strings.TrimSpace(chunk.ContextHeader))
			title := strings.ToLower(doc.Title)
			score := 0
			if normalizedQuery != "" && strings.Contains(normalizeReadingText(chunk.Content), normalizedQuery) {
				score += 10
			}
			for _, token := range tokens {
				lowerToken := strings.ToLower(token)
				if strings.Contains(content, lowerToken) {
					score += 2
				}
				if heading != "" && strings.Contains(heading, lowerToken) {
					score += 3
				}
				if title != "" && strings.Contains(title, lowerToken) {
					score += 1
				}
			}
			if score == 0 {
				continue
			}
			seen[chunk.ID] = struct{}{}
			results = append(results, scored{
				score: score,
				evidence: types.ReadingEvidence{
					KnowledgeID: doc.Knowledge.ID, DocumentTitle: doc.Title,
					ChunkID: chunk.ID, SectionID: chunkSection[chunk.ID],
					Heading: strings.TrimSpace(chunk.ContextHeader),
					Quote:   readingEvidenceSnippet(chunk.Content, query, 220),
				},
			})
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > readingSearchEvidenceCap {
		results = results[:readingSearchEvidenceCap]
	}
	out := make([]types.ReadingEvidence, 0, len(results))
	for _, result := range results {
		out = append(out, result.evidence)
	}
	return out
}

func (s *readingRunService) readingSemanticScope(
	ctx context.Context, plan *types.ReadingPlan,
) *types.ACLScope {
	if s.ragEngine == nil || plan == nil || strings.TrimSpace(plan.KnowledgeBaseID) == "" {
		return nil
	}
	return types.TenantWideScope()
}

// searchReadingEvidenceMerged combines YLSW semantic retrieval with the local
// keyword fallback. Semantic hits are capped and deduped by app chunk ID; if
// YLSW is unavailable or returns chunks the app cannot map, keyword search
// still answers within the selected documents.
func (s *readingRunService) searchReadingEvidenceMerged(
	ctx context.Context, query string, scope *types.ACLScope,
	run *types.ReadingRun, plan *types.ReadingPlan,
	docs []*readingDocument, chunkInfo map[string]*readingChunkInfo, chunkSection map[string]string,
) []types.ReadingEvidence {
	semantic := s.semanticSearchSelectedDocs(ctx, query, scope, run, plan, chunkInfo, chunkSection)
	keyword := s.searchReadingEvidence(query, docs, chunkSection)
	out := make([]types.ReadingEvidence, 0, readingSearchEvidenceCap)
	seen := map[string]struct{}{}
	add := func(values []types.ReadingEvidence, limit int) {
		for _, evidence := range values {
			if len(out) >= readingSearchEvidenceCap || (limit > 0 && len(out) >= limit) {
				return
			}
			if evidence.ChunkID == "" {
				continue
			}
			if _, ok := seen[evidence.ChunkID]; ok {
				continue
			}
			seen[evidence.ChunkID] = struct{}{}
			out = append(out, evidence)
		}
	}
	add(semantic, 3)
	add(keyword, 0)
	add(semantic, 0)
	return out
}

func (s *readingRunService) semanticSearchSelectedDocs(
	ctx context.Context, query string, scope *types.ACLScope,
	run *types.ReadingRun, plan *types.ReadingPlan,
	chunkInfo map[string]*readingChunkInfo, chunkSection map[string]string,
) []types.ReadingEvidence {
	if s.ragEngine == nil || scope == nil || plan == nil || strings.TrimSpace(plan.KnowledgeBaseID) == "" {
		return nil
	}
	documentIDs := planDocumentIDs(run)
	if len(documentIDs) == 0 {
		return nil
	}
	searchCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	results, err := s.ragEngine.SearchDocuments(
		searchCtx, []string{query}, []string{plan.KnowledgeBaseID},
		documentIDs, scope, 8,
	)
	if err != nil {
		return nil
	}
	out := make([]types.ReadingEvidence, 0, len(results))
	seen := map[string]struct{}{}
	for _, result := range results {
		if result == nil {
			continue
		}
		chunkID := strings.TrimSpace(result.ID)
		if chunkID == "" {
			continue
		}
		info, ok := chunkInfo[chunkID]
		if !ok || info.Chunk == nil {
			continue
		}
		if _, ok := seen[chunkID]; ok {
			continue
		}
		seen[chunkID] = struct{}{}
		out = append(out, types.ReadingEvidence{
			KnowledgeID: info.KnowledgeID, DocumentTitle: info.DocumentTitle,
			ChunkID: chunkID, SectionID: chunkSection[chunkID],
			Heading: info.Heading,
			Quote:   readingEvidenceSnippet(info.Chunk.Content, query, 220),
		})
	}
	return out
}

func readingQueryTokens(query string) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	tokens := make([]string, 0, len(fields)+8)
	seen := map[string]struct{}{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if utf8.RuneCountInString(value) < 2 {
			return
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		tokens = append(tokens, value)
	}
	for _, field := range fields {
		add(field)
		hasHan := false
		for _, r := range field {
			if unicode.Is(unicode.Han, r) {
				hasHan = true
				break
			}
		}
		if !hasHan {
			continue
		}
		runes := []rune(field)
		for i := 0; i+2 <= len(runes); i++ {
			add(string(runes[i : i+2]))
			if len(tokens) >= 20 {
				return tokens
			}
		}
	}
	if len(tokens) > 20 {
		tokens = tokens[:20]
	}
	return tokens
}

func normalizeReadingText(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		builder.WriteRune(unicode.ToLower(r))
	}
	return builder.String()
}

func normalizeReadingQuestionKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "fact":
		return "fact"
	default:
		return "decision"
	}
}

func normalizeReadingGapType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ambiguity", "missing_param", "contradiction", "scope", "version":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unclassified"
	}
}

func readingQuestionIsGeneric(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}
	if len([]rune(text)) > 80 {
		return false
	}
	for _, phrase := range []string{
		"主要内容", "核心内容", "总结一下", "概述", "讲了什么",
		"有什么内容", "文档大意", "主要观点",
	} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func uniqueReadingStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := normalizeReadingText(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func evidenceJSON(values []types.ReadingEvidence) types.JSON {
	if values == nil {
		values = []types.ReadingEvidence{}
	}
	raw, _ := json.Marshal(values)
	return types.JSON(raw)
}

func evidenceFromJSON(raw types.JSON) []types.ReadingEvidence {
	var values []types.ReadingEvidence
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &values)
	}
	return values
}

func (s *readingRunService) finishStructured(
	ctx context.Context, caller types.Caller, run *types.ReadingRun, plan *types.ReadingPlan,
) (*types.ReadingRun, error) {
	state, err := s.loadReadingRoundState(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	summary, reasoning, usage := s.buildReadingFinalSummary(ctx, run, plan, state)
	run.TokenUsage += usage
	confirmed := make([]string, 0)
	openQuestions := make([]string, 0)
	contradictions := make([]string, 0)
	citations := make([]types.ReadingEvidence, 0)
	seenCitations := map[string]struct{}{}
	for _, conclusion := range state.Conclusions {
		if conclusion.Status == types.ReadingConclusionConfirmed {
			confirmed = append(confirmed, conclusion.Text)
		}
		for _, evidence := range evidenceFromJSON(conclusion.Evidence) {
			key := evidence.ChunkID + "|" + evidence.Quote
			if _, ok := seenCitations[key]; ok {
				continue
			}
			seenCitations[key] = struct{}{}
			citations = append(citations, evidence)
		}
	}
	for _, question := range state.Pending {
		openQuestions = append(openQuestions, question.Text)
	}
	for _, contradiction := range state.Contradictions {
		contradictions = append(contradictions, contradiction.Description)
	}
	readSections := 0
	for _, section := range plan.Sections {
		if section.Read {
			readSections++
		}
	}
	final := map[string]any{
		"summary": summary, "key_points": confirmed, "open_questions": openQuestions,
		"contradictions": contradictions, "citations": citations,
		"coverage": map[string]any{
			"documents": len(planDocumentIDs(run)), "total_sections": len(plan.Sections),
			"read_sections": readSections, "step_count": run.StepCount + 1,
			"token_usage": run.TokenUsage,
		},
		"state_version": readingStructuredStateVersion,
		"generated_at":  time.Now().Format(time.RFC3339),
	}
	finalJSON, _ := json.Marshal(final)
	run.Final = types.JSON(finalJSON)
	run.Status = types.ReadingRunFinished
	run.StepCount++
	run.UpdatedAt = time.Now()
	finishLog, _ := json.Marshal(map[string]any{
		"summary":   summary,
		"reasoning": readingTruncateRunes(reasoning, 30000),
	})
	step := &types.ReadingRunStep{
		ID: uuid.NewString(), RunID: run.ID, Seq: run.StepCount,
		Action: "finish", Question: types.JSON("{}"),
		NotesPatch: types.JSON(finishLog), Citations: types.JSON("[]"),
		Status: "done", CreatedAt: time.Now(),
	}
	if err := s.repo.SaveRound(ctx, run, nil, nil, nil, nil, step); err != nil {
		return nil, err
	}
	return s.Get(ctx, run.ID)
}

func (s *readingRunService) buildReadingFinalSummary(
	ctx context.Context, run *types.ReadingRun, plan *types.ReadingPlan, state *readingRoundState,
) (string, string, int) {
	confirmed := make([]string, 0)
	for _, conclusion := range state.Conclusions {
		if conclusion.Status == types.ReadingConclusionConfirmed {
			confirmed = append(confirmed, conclusion.Text)
		}
	}
	if len(confirmed) == 0 {
		return fmt.Sprintf(
			"本次精读覆盖 %d 个阅读单元，未形成通过三道门的已确认结论；请查看未决问题与矛盾。",
			len(plan.Sections),
		), "", 0
	}
	model, err := s.modelService.GetChatModel(ctx, run.ReadingModelID)
	if err != nil {
		return fmt.Sprintf("本次精读共确认 %d 条结论，报告为证据投影，正式结论仍需人工确认。", len(confirmed)), "", 0
	}
	var builder strings.Builder
	builder.WriteString("阅读目标：" + readingGoal(run) + "\n\n已确认结论：\n")
	for i, text := range confirmed {
		if i >= 12 {
			break
		}
		builder.WriteString("- " + readingTruncateRunes(text, 300) + "\n")
	}
	builder.WriteString("\n矛盾：\n")
	for i, contradiction := range state.Contradictions {
		if i >= 6 {
			break
		}
		builder.WriteString("- " + readingTruncateRunes(contradiction.Description, 200) + "\n")
	}
	builder.WriteString("\n未决问题：\n")
	for i, question := range state.Pending {
		if i >= 8 {
			break
		}
		builder.WriteString("- " + readingTruncateRunes(question.Text, 200) + "\n")
	}
	builder.WriteString("\n请基于以上已确认结论写 3-5 句报告摘要，不得新增事实，不得把未确认内容写成结论。" +
		`返回严格 JSON：{"summary":"..."}`)
	resp, err := model.Chat(ctx, []chat.Message{
		{Role: "system", Content: s.readingInstructions(ctx, run)},
		{Role: "user", Content: builder.String()},
	}, readingChatOptions(readingRunDepth(run), 1500))
	if err != nil {
		return fmt.Sprintf("本次精读共确认 %d 条结论，报告为证据投影，正式结论仍需人工确认。", len(confirmed)), "", 0
	}
	var parsed struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stripJSONFence(resp.Content)), &parsed); err != nil || strings.TrimSpace(parsed.Summary) == "" {
		return fmt.Sprintf("本次精读共确认 %d 条结论，报告为证据投影，正式结论仍需人工确认。", len(confirmed)),
			strings.TrimSpace(resp.ReasoningContent), resp.Usage.TotalTokens
	}
	return strings.TrimSpace(parsed.Summary), strings.TrimSpace(resp.ReasoningContent), resp.Usage.TotalTokens
}

func planDocumentIDs(run *types.ReadingRun) []string {
	var documentIDs []string
	_ = json.Unmarshal(run.DocumentIDs, &documentIDs)
	return documentIDs
}

func (s *readingRunService) structuredFeedback(
	ctx context.Context, caller types.Caller, run *types.ReadingRun, req *types.ReadingFeedbackRequest,
) (*types.ReadingRun, error) {
	if run.Status != types.ReadingRunWaitingUser {
		return nil, werrors.NewValidationError("当前没有待反馈的问题")
	}
	questions, err := s.repo.ListQuestions(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	var target *types.ReadingQuestionNode
	for _, question := range questions {
		if question.Status != types.ReadingQuestionPending {
			continue
		}
		if req.QuestionID != "" && question.ID != req.QuestionID {
			continue
		}
		target = question
		break
	}
	if target == nil {
		return nil, werrors.NewValidationError("待反馈问题不存在")
	}
	now := time.Now()
	var conclusion *types.ReadingConclusion
	feedback := strings.TrimSpace(req.Feedback)
	if req.Skip {
		target.Status = types.ReadingQuestionSkipped
		target.Answer = "（用户跳过）"
		target.AnswerSource = "user"
	} else {
		if feedback == "" {
			return nil, werrors.NewValidationError("反馈不能为空")
		}
		target.Status = types.ReadingQuestionAnswered
		target.Answer = feedback
		target.AnswerSource = "user"
		evidence := evidenceFromJSON(target.Evidence)
		gates := types.ReadingConclusionGates{
			ReferencesOK: len(evidence) > 0, ConflictFree: true,
			CrossDocConsistent: true, UserAdjudicated: true,
		}
		rawGates, _ := json.Marshal(gates)
		conclusion = &types.ReadingConclusion{
			ID: uuid.NewString(), RunID: run.ID, TenantID: run.TenantID,
			SectionID: target.SectionID, Round: target.Round, Text: feedback,
			Evidence: evidenceJSON(evidence), Status: types.ReadingConclusionConfirmed,
			Source: "user", Gates: types.JSON(rawGates), CreatedAt: now, UpdatedAt: now,
		}
	}
	target.UpdatedAt = now
	remaining := 0
	for _, question := range questions {
		if question.ID != target.ID && question.Status == types.ReadingQuestionPending {
			remaining++
		}
	}
	if remaining > 0 {
		run.Status = types.ReadingRunWaitingUser
	} else {
		run.Status = types.ReadingRunReading
	}
	run.UpdatedAt = now
	if err := s.repo.SaveFeedback(ctx, run, target, conclusion); err != nil {
		return nil, err
	}
	return s.Get(ctx, run.ID)
}

func buildStructuredReadingMarkdown(
	run *types.ReadingRun, plan types.ReadingPlan,
	questions []*types.ReadingQuestionNode, conclusions []*types.ReadingConclusion,
	terms []*types.ReadingTerm, contradictions []*types.ReadingContradiction,
	final map[string]any, docTitles []string, coverage types.ReadingCoverage, generatedAt time.Time,
) string {
	var builder strings.Builder
	writeLine := func(line string) { builder.WriteString(line + "\n") }
	writeEvidence := func(evidence []types.ReadingEvidence) {
		for _, item := range evidence {
			location := item.DocumentTitle
			if item.Heading != "" {
				location += " § " + item.Heading
			}
			writeLine(fmt.Sprintf("  - 出处：%s：「%s」", location, readingTruncateRunes(item.Quote, 160)))
		}
	}
	writeLine("# WellRead 精读报告")
	writeLine("")
	writeLine("- 阅读目标：" + strings.TrimSpace(plan.Goal))
	writeLine("- 阅读深度：" + readingDepthLabel(plan.Depth))
	writeLine("- 精读策略：" + plan.Policy)
	writeLine("- 文档数量：" + fmt.Sprint(coverage.Documents))
	writeLine(fmt.Sprintf("- 覆盖：%d/%d 个阅读单元", coverage.ReadSections, coverage.TotalSections))
	writeLine("- 生成时间：" + generatedAt.Format(time.RFC3339))
	writeLine("")
	writeLine("## 文档")
	writeLine("")
	if len(docTitles) == 0 {
		writeLine("—")
	} else {
		for _, title := range docTitles {
			writeLine("- " + title)
		}
	}
	writeLine("")
	writeLine("## 结论摘要")
	writeLine("")
	summary, _ := final["summary"].(string)
	if strings.TrimSpace(summary) == "" {
		writeLine("—")
	} else {
		writeLine(strings.TrimSpace(summary))
	}
	writeLine("")
	writeLine("## 已确认结论")
	writeLine("")
	confirmed := 0
	for i := len(conclusions) - 1; i >= 0; i-- {
		conclusion := conclusions[i]
		if conclusion.Status != types.ReadingConclusionConfirmed {
			continue
		}
		confirmed++
		writeLine("- " + conclusion.Text)
		writeEvidence(evidenceFromJSON(conclusion.Evidence))
	}
	if confirmed == 0 {
		writeLine("—")
	}
	writeLine("")
	writeLine("## 待确认结论")
	writeLine("")
	candidates := 0
	for i := len(conclusions) - 1; i >= 0; i-- {
		conclusion := conclusions[i]
		if conclusion.Status == types.ReadingConclusionConfirmed {
			continue
		}
		candidates++
		writeLine(fmt.Sprintf("- [%s] %s", readingConclusionStatusLabel(conclusion.Status), conclusion.Text))
		writeEvidence(evidenceFromJSON(conclusion.Evidence))
	}
	if candidates == 0 {
		writeLine("—")
	}
	writeLine("")
	writeLine("## 用户裁定")
	writeLine("")
	decisions := 0
	for i := len(conclusions) - 1; i >= 0; i-- {
		conclusion := conclusions[i]
		if conclusion.Source != "user" {
			continue
		}
		decisions++
		writeLine("- " + conclusion.Text)
		writeEvidence(evidenceFromJSON(conclusion.Evidence))
	}
	if decisions == 0 {
		writeLine("—")
	}
	writeLine("")
	writeLine("## 术语")
	writeLine("")
	if len(terms) == 0 {
		writeLine("—")
	} else {
		for _, term := range terms {
			writeLine(fmt.Sprintf("- %s：%s（%s）", term.Term, term.Definition, readingTermStatusLabel(term.Status)))
			writeEvidence(evidenceFromJSON(term.Source))
		}
	}
	writeLine("")
	writeLine("## 未决问题")
	writeLine("")
	pending := 0
	for _, question := range questions {
		if question.Status != types.ReadingQuestionPending && question.Status != types.ReadingQuestionSkipped {
			continue
		}
		pending++
		suffix := ""
		if question.Status == types.ReadingQuestionSkipped {
			suffix = "（用户已跳过）"
		}
		writeLine("- " + question.Text + suffix)
		if question.RecommendedAnswer != "" {
			writeLine("  - 推荐解读：" + question.RecommendedAnswer)
		}
		writeEvidence(evidenceFromJSON(question.Evidence))
	}
	if pending == 0 {
		writeLine("—")
	}
	writeLine("")
	writeLine("## 矛盾")
	writeLine("")
	if len(contradictions) == 0 {
		writeLine("—")
	} else {
		for _, contradiction := range contradictions {
			writeLine("- [" + contradiction.Status + "] " + contradiction.Description)
			writeLine("  - A 方：")
			writeEvidence(evidenceFromJSON(contradiction.EvidenceA))
			writeLine("  - B 方：")
			writeEvidence(evidenceFromJSON(contradiction.EvidenceB))
		}
	}
	writeLine("")
	writeLine("## 覆盖与局限")
	writeLine("")
	writeLine("- 仅基于用户选定的文档，不引入集合外资料。")
	writeLine(fmt.Sprintf("- 已读 %d/%d 个阅读单元；未覆盖部分未纳入结论。", coverage.ReadSections, coverage.TotalSections))
	writeLine("- 已确认结论均带可定位出处；未过三道门的结论不进入已确认列表。")
	hasLegacy := false
	for _, conclusion := range conclusions {
		if conclusion != nil && conclusion.Source == types.ReadingConclusionSourceLegacy {
			hasLegacy = true
			break
		}
	}
	if hasLegacy {
		writeLine("- 历史迁移的结论无结构化出处，仅供追溯。")
	}
	writeLine("- 报告为阅读结论，不是公司正式文件；对外使用前需人工确认。")
	writeLine("")
	writeLine("> run_id: " + run.ID)
	return builder.String()
}

func readingConclusionStatusLabel(status string) string {
	switch status {
	case types.ReadingConclusionContradicted:
		return "被矛盾推翻"
	case types.ReadingConclusionCandidate:
		return "待确认"
	default:
		return status
	}
}

func readingTermStatusLabel(status string) string {
	if status == types.ReadingTermConfirmed {
		return "已确认"
	}
	return "待确认"
}

func (s *readingRunService) populateStructuredReading(
	ctx context.Context, run *types.ReadingRun,
) error {
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil || plan.StateVersion < readingStructuredStateVersion {
		return nil
	}
	questions, err := s.repo.ListQuestions(ctx, run.ID)
	if err != nil {
		return err
	}
	conclusions, err := s.repo.ListConclusions(ctx, run.ID)
	if err != nil {
		return err
	}
	terms, err := s.repo.ListTerms(ctx, run.ID)
	if err != nil {
		return err
	}
	contradictions, err := s.repo.ListContradictions(ctx, run.ID)
	if err != nil {
		return err
	}
	run.Questions = questions
	run.Conclusions = conclusions
	run.Terms = terms
	run.Contradictions = contradictions
	return nil
}

func (s *readingRunService) countPendingReadingQuestions(ctx context.Context, runID string) int {
	questions, err := s.repo.ListQuestions(ctx, runID)
	if err != nil {
		return 0
	}
	count := 0
	for _, question := range questions {
		if question.Status == types.ReadingQuestionPending {
			count++
		}
	}
	return count
}

func stripReadingPlanContent(run *types.ReadingRun) {
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return
	}
	changed := false
	for i := range plan.Sections {
		if plan.Sections[i].Content != "" || len(plan.Sections[i].Segments) > 0 {
			plan.Sections[i].Content = ""
			plan.Sections[i].Segments = nil
			changed = true
		}
	}
	if !changed {
		return
	}
	raw, err := json.Marshal(plan)
	if err == nil {
		run.Plan = types.JSON(raw)
	}
}

func (s *readingRunService) exportStructuredReading(
	ctx context.Context, caller types.Caller, run *types.ReadingRun,
	plan *types.ReadingPlan, format string,
) (*types.ReadingExport, error) {
	documentIDs := planDocumentIDs(run)
	for _, docID := range documentIDs {
		if _, err := s.ensureDocumentReadable(ctx, caller, docID); err != nil {
			return nil, werrors.NewForbiddenError("文档权限已变更，无法导出；请移除无权文档后重试")
		}
	}
	generatedContent, coverage, docTitles := s.structuredReportContent(run, plan)
	final := map[string]any{}
	_ = json.Unmarshal(run.Final, &final)
	generatedAt := time.Now()
	draft, draftErr := s.repo.GetReportDraft(ctx, caller.TenantID, caller.UserID, run.ID)
	if draftErr != nil {
		draft = nil
	}
	edited := draft != nil && strings.TrimSpace(draft.ContentMD) != ""
	result := &types.ReadingExport{Format: format, Coverage: coverage, GeneratedAt: generatedAt}
	if format == "json" {
		payload := map[string]any{
			"run_id": run.ID, "goal": plan.Goal, "depth": plan.Depth, "policy": plan.Policy,
			"documents": docTitles, "coverage": coverage,
			"questions": run.Questions, "conclusions": run.Conclusions,
			"terms": run.Terms, "contradictions": run.Contradictions,
			"final": final, "state_version": plan.StateVersion,
			"report_edited": edited, "generated_at": generatedAt,
		}
		raw, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return nil, err
		}
		result.Content = string(raw)
		result.Filename = fmt.Sprintf("wellread-reading-%s.json", run.ID)
	} else {
		if edited {
			result.Content = draft.ContentMD
		} else {
			result.Content = generatedContent
		}
		result.Filename = fmt.Sprintf("wellread-reading-%s.md", run.ID)
	}
	return result, nil
}
