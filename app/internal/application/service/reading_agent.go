package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	werrors "gaze/internal/errors"
	"gaze/internal/logger"
	"gaze/internal/models/chat"
	"gaze/internal/types"
)

// state_version=3 engine: structure pre-read, per-chapter single interaction
// point with distilled memory, an explicit synthesis pass, and self-tests.
// state_version=2 runs keep using the structuredStep code path unchanged.

const (
	readingAgentStateVersion          = 3
	readingAgentQuestionCap           = 1
	readingAgentStructureExcerptRunes = 240
	readingAgentStructureMaxTokens    = 3000
	readingAgentSearchIterations      = 2
	readingAgentSelfTestsPerSection   = 4
	readingAgentSelfTestCap           = 20
	readingAgentSelfTestSectionCap    = 10
	readingAgentMemoryConclusionCap   = 30
	readingAgentMemoryTermCap         = 20
	readingAgentMemorySkipLimit       = 2
)

const readingAgentRoundRules = `## 精读 agent 逐章输出纪律
你每轮只读一个章节窗口，必须先给出理解陈述，再给出至多一个交互点。
思考过程（reasoning）请使用中文。
你是在审问文档，不是替用户总结文档；问题必须指向文档缺口，不得提“主要内容是什么”这类泛问。
本轮 questions 最多 1 个；quiet 或快速档时 questions 必须为空数组。
事实类问题（kind=fact）必须由你自己在 answers 中用证据回答，不得留给用户；无法回答就丢弃该问题。
只有必须由用户裁定的问题（kind=decision）才允许保留为待确认，且必须给 recommended_answer 与 3 个选项。
只允许引用本轮窗口或引擎检索结果中出现的 chunk_id；quote 必须是该 chunk 中逐字存在的连续片段。
结论必须有证据；没有证据不要写结论。矛盾必须给冲突双方出处。不要重复已确认结论和术语。
你在 reasoning 中发现的事实、规则、参数、流程与缺口必须完整落到 conclusions；在 reasoning 中看到的冲突必须完整落到 contradictions，不能只写在 reasoning 里。
材料足够时，本轮 conclusions 至少 2 条；每条都要有 chunk_id 与逐字 quote。术语必须带 source。`

type readingAgentStructureEntry struct {
	SectionID     string `json:"section_id"`
	Thesis        string `json:"thesis"`
	KnowledgeType string `json:"knowledge_type"`
}

type readingAgentStructureResponse struct {
	Sections []readingAgentStructureEntry `json:"sections"`
}

type readingAgentArgument struct {
	Claim    string               `json:"claim"`
	Evidence []readingEvidenceRef `json:"evidence"`
}

type readingAgentSynthesisResponse struct {
	Summary           string                    `json:"summary"`
	ArgumentChain     []readingAgentArgument    `json:"argument_chain"`
	Contradictions    []readingLLMContradiction `json:"contradictions"`
	TermFinalizations []readingLLMTerm          `json:"term_finalizations"`
	OpenQuestions     []string                  `json:"open_questions"`
}

type readingAgentSelfTestItem struct {
	SectionID   string               `json:"section_id"`
	Question    string               `json:"question"`
	Answer      string               `json:"answer"`
	Explanation string               `json:"explanation"`
	Evidence    []readingEvidenceRef `json:"evidence"`
}

type readingAgentSelfTestResponse struct {
	Tests []readingAgentSelfTestItem `json:"tests"`
}

type readingAgentSectionPoint struct {
	SectionID string `json:"section_id"`
	Thesis    string `json:"thesis"`
	OneLine   string `json:"one_line,omitempty"`
}

type readingAgentTermMemory struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

type readingAgentKnowledgeMemory struct {
	SectionPoints  []readingAgentSectionPoint `json:"section_points"`
	Conclusions    []string                   `json:"conclusions"`
	Terms          []readingAgentTermMemory   `json:"terms"`
	Contradictions []string                   `json:"contradictions"`
}

type readingAgentSectionResult struct {
	SectionID   string `json:"section_id"`
	Result      string `json:"result"`
	ErrorPoints string `json:"error_points,omitempty"`
}

type readingAgentUserMemory struct {
	Sections         []readingAgentSectionResult `json:"sections"`
	ConsecutiveSkips int                         `json:"consecutive_skips"`
	Quiet            bool                        `json:"quiet"`
	Rejected         []string                    `json:"rejected_conclusions,omitempty"`
}

type readingAgentTaskMemory struct {
	Goal            string `json:"goal"`
	Depth           string `json:"depth"`
	Phase           string `json:"phase"`
	CurrentSection  string `json:"current_section_id,omitempty"`
	RemainingSteps  int    `json:"remaining_steps"`
	RemainingTokens int    `json:"remaining_tokens"`
	Quiet           bool   `json:"quiet"`
}

type readingAgentMemoryState struct {
	Knowledge readingAgentKnowledgeMemory
	User      readingAgentUserMemory
	Task      readingAgentTaskMemory
}

func normalizeReadingKnowledgeType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case types.ReadingKnowledgeTypeConcept, types.ReadingKnowledgeTypeArgument,
		types.ReadingKnowledgeTypeProcess, types.ReadingKnowledgeTypeComparison,
		types.ReadingKnowledgeTypeExample, types.ReadingKnowledgeTypeReference:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return types.ReadingKnowledgeTypeUnknown
	}
}

func readingStructureFromPlan(plan *types.ReadingPlan) map[string]types.ReadingSectionStructure {
	out := map[string]types.ReadingSectionStructure{}
	for _, entry := range plan.Structure {
		out[entry.SectionID] = entry
	}
	return out
}

func readingAgentStructureFallback(plan *types.ReadingPlan) []types.ReadingSectionStructure {
	out := make([]types.ReadingSectionStructure, 0, len(plan.Sections))
	for _, section := range plan.Sections {
		thesis := strings.TrimSpace(section.Heading)
		if thesis == "" {
			thesis = strings.TrimSpace(section.Title)
		}
		out = append(out, types.ReadingSectionStructure{
			SectionID: section.ID, Thesis: readingTruncateRunes(thesis, readingAgentStructureExcerptRunes),
			KnowledgeType: types.ReadingKnowledgeTypeUnknown,
		})
	}
	return out
}

func buildReadingStructurePrompt(plan *types.ReadingPlan) string {
	var builder strings.Builder
	builder.WriteString("阅读目标：" + strings.TrimSpace(plan.Goal) + "\n")
	builder.WriteString("阅读深度：" + readingDepthLabel(plan.Depth) + "\n\n")
	builder.WriteString("以下是本次精读的分节骨架。请只做结构预读，不写结论，不生成复习问题。\n")
	builder.WriteString("为每一节给出一句话论点（thesis）与知识类型（knowledge_type）。\n")
	builder.WriteString("knowledge_type 只能取 concept/argument/process/comparison/example/reference/unknown。\n\n")
	for _, section := range plan.Sections {
		excerpt := readingTruncateRunes(strings.TrimSpace(section.Content), readingAgentStructureExcerptRunes)
		builder.WriteString(fmt.Sprintf(
			"- section_id=%s | 文档=%s | 标题=%s\n  开头：%s\n",
			section.ID, section.Title, strings.TrimSpace(section.Heading), excerpt,
		))
	}
	builder.WriteString(`

返回严格 JSON（不要 Markdown、不要解释）：
{"sections":[{"section_id":"原样返回","thesis":"一句话论点","knowledge_type":"concept|argument|process|comparison|example|reference|unknown"}]}
必须覆盖上面列出的每一个 section_id；无法判断时 thesis 用标题、knowledge_type 用 unknown。`)
	return builder.String()
}

// preReadReadingStructure runs the lightweight structure call. Failure never
// blocks run creation: the plan falls back to headings and records degraded.
func (s *readingRunService) preReadReadingStructure(
	ctx context.Context, run *types.ReadingRun, plan *types.ReadingPlan,
) (int, bool, string) {
	modelID := strings.TrimSpace(run.ExtractionModelID)
	if modelID == "" {
		modelID = strings.TrimSpace(run.ReadingModelID)
	}
	fallback := readingAgentStructureFallback(plan)
	plan.Structure = fallback
	model, err := s.modelService.GetChatModel(ctx, modelID)
	if err != nil {
		return 0, true, fmt.Sprintf("结构预读模型不可用: %v", err)
	}
	messages := []chat.Message{
		{Role: "system", Content: s.readingInstructions(ctx, run) + "\n\n你只负责结构预读，必须返回严格 JSON。"},
		{Role: "user", Content: buildReadingStructurePrompt(plan)},
	}
	resp, err := model.Chat(ctx, messages, readingAgentExtractionOptions(readingAgentStructureMaxTokens))
	if err != nil {
		return 0, true, fmt.Sprintf("结构预读调用失败: %v", err)
	}
	usage := resp.Usage.TotalTokens
	parsed, parseErr := parseReadingAgentStructure(resp.Content)
	if parseErr != nil {
		// One repair attempt keeps the create path cheap while giving the model
		// a chance to return strict JSON.
		repair := append(messages,
			chat.Message{Role: "assistant", Content: resp.Content, ReasoningContent: resp.ReasoningContent},
			chat.Message{Role: "user", Content: "上次输出不是有效结构 JSON。请只返回严格 JSON，不要解释或 Markdown。"},
		)
		retry, retryErr := model.Chat(ctx, repair, readingAgentExtractionOptions(readingAgentStructureMaxTokens))
		if retryErr == nil {
			usage += retry.Usage.TotalTokens
			parsed, parseErr = parseReadingAgentStructure(retry.Content)
		}
	}
	if parseErr != nil {
		plan.Structure = fallback
		return usage, true, parseErr.Error()
	}
	byID := map[string]readingAgentStructureEntry{}
	for _, entry := range parsed.Sections {
		id := strings.TrimSpace(entry.SectionID)
		if id == "" {
			continue
		}
		byID[id] = entry
	}
	structure, matched := mergeReadingStructureEntries(plan, byID)
	plan.Structure = structure
	if matched == 0 {
		return usage, true, "结构预读未返回任何可匹配的 section_id"
	}
	return usage, false, ""
}

// mergeReadingStructureEntries maps model output back onto the plan's sections.
// Unknown section IDs are dropped; missing sections and empty theses fall back
// to the heading (or document title) with knowledge_type=unknown.
func mergeReadingStructureEntries(
	plan *types.ReadingPlan, entries map[string]readingAgentStructureEntry,
) ([]types.ReadingSectionStructure, int) {
	structure := make([]types.ReadingSectionStructure, 0, len(plan.Sections))
	matched := 0
	for _, section := range plan.Sections {
		entry, ok := entries[section.ID]
		thesis := strings.TrimSpace(entry.Thesis)
		if !ok || thesis == "" {
			thesis = strings.TrimSpace(section.Heading)
			if thesis == "" {
				thesis = strings.TrimSpace(section.Title)
			}
			structure = append(structure, types.ReadingSectionStructure{
				SectionID: section.ID, Thesis: readingTruncateRunes(thesis, readingAgentStructureExcerptRunes),
				KnowledgeType: types.ReadingKnowledgeTypeUnknown,
			})
			continue
		}
		matched++
		structure = append(structure, types.ReadingSectionStructure{
			SectionID: section.ID, Thesis: readingTruncateRunes(thesis, readingAgentStructureExcerptRunes),
			KnowledgeType: normalizeReadingKnowledgeType(entry.KnowledgeType),
		})
	}
	return structure, matched
}

func parseReadingAgentStructure(content string) (*readingAgentStructureResponse, error) {
	raw := stripJSONFence(content)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("结构预读未返回 JSON")
	}
	var parsed readingAgentStructureResponse
	if err := json.Unmarshal([]byte(raw[start:end+1]), &parsed); err != nil {
		return nil, fmt.Errorf("结构预读 JSON 无效: %w", err)
	}
	if len(parsed.Sections) == 0 {
		return nil, fmt.Errorf("结构预读返回空 sections")
	}
	return &parsed, nil
}

func readingAgentExtractionOptions(maxTokens int) *chat.ChatOptions {
	// Extraction may run on a non-thinking model, so thinking stays at the
	// model/provider default instead of being forced on.
	return &chat.ChatOptions{Temperature: 0.2, MaxTokens: maxTokens}
}

// readingAgentRoundMaxTokens is larger than the v2 round budget: v3 requires
// conclusions and contradictions to be written into JSON (not only reasoning),
// and long windows otherwise truncate the JSON mid-object.
func readingAgentRoundMaxTokens(depth string) int {
	switch normalizeReadingDepth(depth) {
	case types.ReadingDepthQuick:
		return 8000
	case types.ReadingDepthDeep:
		return 12000
	default:
		return 12000
	}
}

func readingAgentSynthesisOptions(maxTokens int) *chat.ChatOptions {
	thinking := true
	return &chat.ChatOptions{Temperature: 0.2, MaxTokens: maxTokens, Thinking: &thinking}
}

// resolveReadingModelIDs applies the role rules: extraction and synthesis each
// prefer their own model and fall back to the single reading model. The
// synthesis role must support thinking; extraction may be non-thinking.
func (s *readingRunService) resolveReadingModelIDs(
	ctx context.Context, tenant *types.Tenant,
) (string, string, error) {
	if tenant == nil {
		return "", "", fmt.Errorf("空间配置缺失")
	}
	fallback := strings.TrimSpace(tenant.ReadingModelID)
	extraction := strings.TrimSpace(tenant.ReadingExtractionModelID)
	if extraction == "" {
		extraction = fallback
	}
	synthesis := strings.TrimSpace(tenant.ReadingSynthesisModelID)
	if synthesis == "" {
		synthesis = fallback
	}
	if extraction == "" || synthesis == "" {
		return "", "", fmt.Errorf("空间未配置精读模型")
	}
	if err := s.validateReadingExtractionModel(ctx, extraction); err != nil {
		return "", "", err
	}
	if err := s.ensureReadingModelSupportsThinking(ctx, synthesis); err != nil {
		return "", "", err
	}
	return extraction, synthesis, nil
}

func (s *readingRunService) validateReadingExtractionModel(ctx context.Context, modelID string) error {
	model, err := s.modelService.GetModelByID(ctx, modelID)
	if err != nil || model == nil {
		return fmt.Errorf("抽取模型不存在或不可用")
	}
	if model.Type != types.ModelTypeKnowledgeQA {
		return fmt.Errorf("抽取模型必须是 KnowledgeQA 类型")
	}
	return nil
}

func (s *readingRunService) readingAgentModel(
	ctx context.Context, run *types.ReadingRun, synthesis bool,
) (chat.Chat, string, error) {
	modelID := strings.TrimSpace(run.ExtractionModelID)
	if synthesis {
		modelID = strings.TrimSpace(run.SynthesisModelID)
		if modelID == "" {
			modelID = strings.TrimSpace(run.ReadingModelID)
		}
	}
	if modelID == "" {
		modelID = strings.TrimSpace(run.ReadingModelID)
	}
	model, err := s.modelService.GetChatModel(ctx, modelID)
	if err != nil {
		return nil, modelID, fmt.Errorf("加载精读模型失败: %w", err)
	}
	return model, modelID, nil
}

// callReadingAgentRoundModel mirrors callReadingRoundModel but lets extraction
// run without forced thinking.
func (s *readingRunService) callReadingAgentRoundModel(
	ctx context.Context, model chat.Chat, messages []chat.Message, depth string,
) (*readingRoundResponse, string, string, int, error) {
	totalUsage := 0
	var lastErr error
	reasoningParts := make([]string, 0, 2)
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := s.readingModelChat(ctx, model, messages, readingAgentExtractionOptions(readingAgentRoundMaxTokens(depth)))
		if err != nil {
			return nil, "", strings.Join(reasoningParts, "\n\n"), totalUsage, fmt.Errorf("精读模型调用失败: %w", err)
		}
		totalUsage += resp.Usage.TotalTokens
		if strings.TrimSpace(resp.ReasoningContent) != "" {
			reasoningParts = append(reasoningParts, strings.TrimSpace(resp.ReasoningContent))
		}
		parsed, err := parseReadingRound(resp.Content)
		if err == nil {
			return parsed, resp.Content, strings.Join(reasoningParts, "\n\n"), totalUsage, nil
		}
		lastErr = err
		if strings.TrimSpace(resp.Content) != "" || strings.TrimSpace(resp.ReasoningContent) != "" {
			messages = append(messages, chat.Message{
				Role: "assistant", Content: resp.Content, ReasoningContent: resp.ReasoningContent,
			})
		}
		messages = append(messages,
			chat.Message{Role: "user", Content: "上次输出不是有效 JSON。请只返回严格 JSON，不要包含解释或 Markdown。"},
		)
	}
	return nil, "", strings.Join(reasoningParts, "\n\n"), totalUsage,
		fmt.Errorf("精读模型连续返回无效 JSON: %w", lastErr)
}

func buildReadingAgentRoundPrompt(
	run *types.ReadingRun, plan *types.ReadingPlan, section *types.ReadingSection,
	memory *readingAgentMemoryState, quiet bool, round int,
	annotations []*types.ReadingAnnotation, feynman *types.ReadingFeynmanAttempt,
) string {
	var builder strings.Builder
	builder.WriteString("阅读目标：" + readingGoal(run) + "\n")
	builder.WriteString("阅读深度：" + readingDepthLabel(plan.Depth) + "\n")
	structure := readingStructureFromPlan(plan)[section.ID]
	thesis := strings.TrimSpace(structure.Thesis)
	if thesis == "" {
		thesis = strings.TrimSpace(section.Heading)
	}
	builder.WriteString(fmt.Sprintf("当前轮次：第 %d 轮；当前阅读单元：%s · %s\n", round, section.Title, section.Heading))
	builder.WriteString(fmt.Sprintf("章节一句话论点：%s（知识类型：%s）\n", thesis, structure.KnowledgeType))
	if quiet {
		builder.WriteString("本章处于安静模式：只产理解陈述与笔记，questions 必须为空数组，不向用户提问。\n")
	}
	builder.WriteString("\n本轮原文（chunk 标记仅用于引用，不要写进结论正文）：\n")
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
	if memory != nil {
		knowledgeJSON, _ := json.Marshal(memory.Knowledge)
		userJSON, _ := json.Marshal(memory.User)
		builder.WriteString("已蒸馏的知识记忆：\n" + string(knowledgeJSON) + "\n\n")
		builder.WriteString("用户理解档案：\n" + string(userJSON) + "\n\n")
	}
	if len(annotations) > 0 {
		builder.WriteString("用户在原文上的批注（必须纳入本轮理解，不得静默忽略）：\n")
		for _, annotation := range annotations {
			builder.WriteString(fmt.Sprintf("- 「%s」", readingTruncateRunes(annotation.Quote, 160)))
			if strings.TrimSpace(annotation.Comment) != "" {
				builder.WriteString(" → " + readingTruncateRunes(annotation.Comment, 200))
			}
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	if feynman != nil {
		builder.WriteString(fmt.Sprintf("用户最近一次费曼自述评分：%d/100（覆盖 %d、准确 %d、证据 %d）\n",
			feynman.Score, feynman.Coverage, feynman.Accuracy, feynman.EvidenceScore))
		if strings.TrimSpace(feynman.Summary) != "" {
			builder.WriteString("评估摘要：" + readingTruncateRunes(feynman.Summary, 300) + "\n")
		}
		omissions := readingFeynmanPointTexts(feynman.Omissions)
		errorsFound := readingFeynmanPointTexts(feynman.Errors)
		if len(omissions) > 0 {
			builder.WriteString("已知遗漏：" + strings.Join(omissions, "；") + "\n")
		}
		if len(errorsFound) > 0 {
			builder.WriteString("已知错误：" + strings.Join(errorsFound, "；") + "\n")
		}
		if weak := readingFeynmanWeakAngles(feynman); len(weak) > 0 {
			builder.WriteString("薄弱角度：" + strings.Join(weak, "；") + "\n")
		}
		builder.WriteString("下一轮请优先补齐遗漏、纠正错误。\n\n")
	}
	builder.WriteString(`返回严格 JSON（不要 Markdown、不要解释）：
{
  "understanding": "本轮理解，2-4 句；先陈述再提问",
  "search_queries": ["需要跨章节/跨文档证据时填写，最多 2 个；否则空数组"],
  "questions": [
    {"text": "至多一个关键问题", "kind": "fact|decision", "gap_type": "ambiguity|missing_param|contradiction|scope|version", "why": "为什么关键", "recommended_answer": "推荐解读", "options": [{"label": "A", "text": "候选答案", "recommended": true}, {"label": "B", "text": "另一个视角", "recommended": false}, {"label": "C", "text": "第三个视角", "recommended": false}], "search_query": "用于自动检索的关键词"}
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
- questions 最多 1 个；安静模式或快速档必须为空。
- 事实问题必须自己回答；只有必须由用户裁定的问题才保留 pending。
- 每个保留的问题必须给 3 个不同视角的 options，恰好一个 recommended=true，recommended_answer 与推荐项一致。
- conclusions 必填：把本轮读到的事实、规则、参数、流程与缺口逐条写出，每条带 evidence；材料足够时至少 2 条。reasoning 里的分析不能替代 conclusions。
- 发现口径冲突（同一参数两处不同、规则互相矛盾、版本不一致）必须写入 contradictions，并给双方出处；不能只写在 reasoning。
- terms 必须带 source；结论只能写有证据的结论；不要重复已确认结论和术语。
`)
	return builder.String()
}

func (s *readingRunService) runReadingAgentRound(
	ctx context.Context, run *types.ReadingRun, plan *types.ReadingPlan,
	section *types.ReadingSection, state *readingRoundState, memory *readingAgentMemoryState,
	quiet bool, docs []*readingDocument, chunkInfo map[string]*readingChunkInfo,
	chunkSection map[string]string,
) (*readingRoundOutcome, error) {
	model, _, err := s.readingAgentModel(ctx, run, false)
	if err != nil {
		return nil, err
	}
	roundStart := time.Now()
	depth := readingRunDepth(run)
	round := run.StepCount + 1
	var annotations []*types.ReadingAnnotation
	if all, listErr := s.repo.ListAnnotations(ctx, run.ID); listErr == nil {
		for _, annotation := range all {
			if annotation.SectionID == section.ID {
				annotations = append(annotations, annotation)
			}
		}
	}
	var feynman *types.ReadingFeynmanAttempt
	if attempts, listErr := s.repo.ListFeynmanAttempts(ctx, run.ID, section.ID); listErr == nil && len(attempts) > 0 {
		feynman = attempts[0]
	}
	messages := []chat.Message{
		{Role: "system", Content: s.readingInstructions(ctx, run) + "\n\n" + readingAgentRoundRules},
		{Role: "user", Content: buildReadingAgentRoundPrompt(run, plan, section, memory, quiet, round, annotations, feynman)},
	}
	var merged readingRoundResponse
	questionOrder := make([]readingLLMQuestion, 0, 1)
	answers := make([]readingLLMAnswer, 0)
	searchEvidence := map[string][]types.ReadingEvidence{}
	totalUsage := 0
	searchUsed := 0
	reasoningParts := make([]string, 0, 2)
	searchTraces := make([]readingSearchTrace, 0, 4)
	for iteration := 0; iteration < readingAgentSearchIterations; iteration++ {
		parsed, raw, reasoning, usage, err := s.callReadingAgentRoundModel(ctx, model, messages, depth)
		totalUsage += usage
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(reasoning) != "" {
			reasoningParts = append(reasoningParts, reasoning)
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
		resultsByQuery := make([][]types.ReadingEvidence, len(queries))
		var waitGroup sync.WaitGroup
		for index, query := range queries {
			waitGroup.Add(1)
			go func(index int, query string) {
				defer waitGroup.Done()
				resultsByQuery[index] = s.searchReadingEvidenceMerged(
					ctx, query, run, plan, docs, chunkInfo, chunkSection,
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
			types.EmitReadingProgress(ctx, types.ReadingProgressEvent{
				Type: "search", Phase: "reading", Query: query, HitCount: len(results),
				SectionID: section.ID, SectionTitle: section.Title, Round: round,
			})
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
	if len(questionOrder) > readingAgentQuestionCap {
		questionOrder = questionOrder[:readingAgentQuestionCap]
	}
	if quiet {
		questionOrder = nil
	}
	// Facts must be answered by the engine; an unanswered fact question is
	// dropped instead of being pushed to the user.
	if len(questionOrder) == 1 {
		question := questionOrder[0]
		if normalizeReadingQuestionKind(question.Kind) == "fact" {
			answerByIndex := map[int]readingLLMAnswer{}
			answerByText := map[string]readingLLMAnswer{}
			for _, answer := range answers {
				answerByIndex[answer.QuestionIndex] = answer
				if strings.TrimSpace(answer.QuestionText) != "" {
					answerByText[normalizeReadingText(answer.QuestionText)] = answer
				}
			}
			answer, ok := answerByText[normalizeReadingText(question.Text)]
			if !ok {
				answer, ok = answerByIndex[0]
			}
			evidence := s.resolveReadingEvidence(answer.Evidence, chunkInfo, chunkSection)
			if !ok || !answer.Supported || len(evidence) == 0 || strings.TrimSpace(answer.Answer) == "" {
				query := strings.TrimSpace(question.SearchQuery)
				if query == "" {
					query = strings.TrimSpace(question.Text)
				}
				results := s.searchReadingEvidenceMerged(ctx, query, run, plan, docs, chunkInfo, chunkSection)
				trace := readingSearchTrace{Query: query, HitCount: len(results)}
				for _, item := range results {
					trace.ChunkIDs = append(trace.ChunkIDs, item.ChunkID)
				}
				searchTraces = append(searchTraces, trace)
				if len(results) > 0 {
					var builder strings.Builder
					builder.WriteString("以下是引擎在选中文档内检索到的证据。请只回答这个问题：" +
						"证据充分时给 supported=true 的 answer + evidence（chunk_id 和逐字 quote）；" +
						"证据不足时 supported=false。不要新增事实。\n" +
						"返回严格 JSON：{\"answers\":[{\"question_index\":0,\"question_text\":\"问题原文\"," +
						"\"answer\":\"回答\",\"supported\":true,\"evidence\":[{\"chunk_id\":\"chunk id\"," +
						"\"quote\":\"原文连续片段\"}]}],\"conclusions\":[],\"terms\":[],\"contradictions\":[]}。\n\n")
					builder.WriteString("问题：" + question.Text + "\n检索词：" + query + "\n证据：\n")
					for _, item := range results {
						builder.WriteString(fmt.Sprintf(
							"- [chunk %s] %s § %s：%s\n",
							item.ChunkID, item.DocumentTitle, item.Heading, item.Quote,
						))
					}
					autoMessages := []chat.Message{
						{Role: "system", Content: s.readingInstructions(ctx, run) + "\n\n" + readingAgentRoundRules},
						{Role: "user", Content: builder.String()},
					}
					parsed, _, reasoning, usage, autoErr := s.callReadingAgentRoundModel(ctx, model, autoMessages, depth)
					totalUsage += usage
					if autoErr == nil {
						if strings.TrimSpace(reasoning) != "" {
							reasoningParts = append(reasoningParts, reasoning)
						}
						for _, item := range parsed.Answers {
							item.QuestionText = question.Text
							answers = append(answers, item)
						}
						merged.Conclusions = append(merged.Conclusions, parsed.Conclusions...)
						merged.Terms = append(merged.Terms, parsed.Terms...)
						merged.Contradictions = append(merged.Contradictions, parsed.Contradictions...)
					}
				}
			}
		}
	}
	outcome := s.buildReadingRoundOutcome(
		run, section, round, &merged, questionOrder, answers, searchEvidence,
		state, chunkInfo, chunkSection,
	)
	kept := outcome.Questions[:0]
	for _, question := range outcome.Questions {
		if question.Status == types.ReadingQuestionPending &&
			normalizeReadingQuestionKind(question.Kind) == "fact" {
			continue
		}
		kept = append(kept, question)
	}
	outcome.Questions = kept
	outcome.TokenUsage = totalUsage
	outcome.DurationMS = int(time.Since(roundStart).Milliseconds())
	outcome.Reasoning = strings.Join(reasoningParts, "\n\n")
	outcome.SearchTrace = searchTraces
	return outcome, nil
}

func (s *readingRunService) agentStep(
	ctx context.Context, caller types.Caller, run *types.ReadingRun,
) (*types.ReadingRun, error) {
	depth := readingRunDepth(run)
	if run.StepCount >= readingStructuredStepLimit(depth) {
		run.Status = types.ReadingRunStopped
		run.UpdatedAt = time.Now()
		_ = s.repo.UpdateRun(ctx, run)
		return nil, fmt.Errorf("已达到最大步数，精读已停止")
	}
	if run.TokenUsage >= readingStructuredTokenBudget(depth) {
		run.Status = types.ReadingRunStopped
		run.UpdatedAt = time.Now()
		_ = s.repo.UpdateRun(ctx, run)
		return nil, fmt.Errorf("已达到 token 预算，精读已停止")
	}
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return nil, fmt.Errorf("阅读计划损坏")
	}
	switch run.Phase {
	case types.ReadingPhaseSynthesis:
		return s.agentSynthesisStep(ctx, run, &plan)
	case types.ReadingPhaseSelfTest:
		return s.agentSelfTestStep(ctx, run, &plan)
	case types.ReadingPhaseDone:
		return s.Get(ctx, run.ID)
	case types.ReadingPhaseStructure:
		run.Phase = types.ReadingPhaseReading
		run.UpdatedAt = time.Now()
		if err := s.repo.UpdateRun(ctx, run); err != nil {
			return nil, err
		}
		return s.Get(ctx, run.ID)
	default:
		return s.agentReadingStep(ctx, caller, run, &plan)
	}
}

func (s *readingRunService) agentReadingStep(
	ctx context.Context, caller types.Caller, run *types.ReadingRun, plan *types.ReadingPlan,
) (*types.ReadingRun, error) {
	sectionIndex := -1
	for i := range plan.Sections {
		if !plan.Sections[i].Read {
			sectionIndex = i
			break
		}
	}
	if sectionIndex == -1 {
		run.Phase = types.ReadingPhaseSynthesis
		run.Status = types.ReadingRunReading
		run.UpdatedAt = time.Now()
		if err := s.repo.UpdateRun(ctx, run); err != nil {
			return nil, err
		}
		return s.Get(ctx, run.ID)
	}
	section := &plan.Sections[sectionIndex]
	round := run.StepCount + 1
	emitReadingPhase(ctx, "reading", "start", section, round)
	defer emitReadingPhase(ctx, "reading", "end", section, round)
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
	memory, err := s.loadReadingAgentMemory(ctx, run)
	if err != nil {
		return nil, err
	}
	quiet := memory.User.Quiet || readingRunDepth(run) == types.ReadingDepthQuick
	outcome, err := s.runReadingAgentRound(
		ctx, run, plan, section, state, memory, quiet, docs, chunkInfo, chunkSection,
	)
	if err != nil {
		run.Status = types.ReadingRunStopped
		run.UpdatedAt = time.Now()
		_ = s.repo.UpdateRun(ctx, run)
		degradedLog, _ := json.Marshal(map[string]any{
			"phase": "reading", "section_id": section.ID, "degraded": true, "error": readingTruncateRunes(err.Error(), 800),
		})
		_ = s.repo.CreateStep(ctx, &types.ReadingRunStep{
			ID: uuid.NewString(), RunID: run.ID, Seq: run.StepCount + 1,
			Action: "degraded", SectionID: section.ID, Question: types.JSON("{}"),
			NotesPatch: types.JSON(degradedLog), Citations: types.JSON("[]"),
			Status: "degraded", CreatedAt: time.Now(),
		})
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
		"phase": "reading", "understanding": outcome.Understanding,
		"question_count": len(outcome.Questions), "pending_count": outcome.pendingCount(),
		"conclusion_count": len(outcome.Conclusions), "term_count": len(outcome.Terms),
		"contradiction_count": len(outcome.Contradictions),
		"duration_ms":         outcome.DurationMS, "quiet": quiet,
		"reasoning":    readingTruncateRunes(outcome.Reasoning, 30000),
		"search_trace": outcome.SearchTrace,
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
	if strings.TrimSpace(outcome.Understanding) != "" {
		understanding := &types.ReadingUnderstanding{
			ID: uuid.NewString(), RunID: run.ID, TenantID: run.TenantID,
			SectionID: section.ID, Round: run.StepCount,
			Text: outcome.Understanding, Decision: types.ReadingUnderstandingPending,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := s.repo.SaveUnderstanding(ctx, understanding); err != nil {
			logger.Warnf(ctx, "reading: persist understanding %s failed: %v", run.ID, err)
		}
	}
	if err := s.refreshReadingAgentMemory(ctx, run); err != nil {
		logger.Warnf(ctx, "reading: refresh agent memory %s failed: %v", run.ID, err)
	}
	return s.Get(ctx, run.ID)
}

func buildReadingAgentSynthesisPrompt(
	run *types.ReadingRun, plan *types.ReadingPlan, memory *readingAgentMemoryState,
	state *readingRoundState,
) string {
	var builder strings.Builder
	builder.WriteString("阅读目标：" + readingGoal(run) + "\n\n")
	builder.WriteString("结构骨架：\n")
	for _, entry := range plan.Structure {
		builder.WriteString(fmt.Sprintf("- %s：%s（%s）\n", entry.SectionID, entry.Thesis, entry.KnowledgeType))
	}
	knowledgeJSON, _ := json.Marshal(memory.Knowledge)
	builder.WriteString("\n知识记忆：\n" + string(knowledgeJSON) + "\n\n已确认结论（含证据）：\n")
	written := 0
	for _, conclusion := range state.Conclusions {
		if conclusion.Status != types.ReadingConclusionConfirmed || conclusion.Source == types.ReadingConclusionSourceLegacy {
			continue
		}
		if written >= 30 {
			break
		}
		written++
		builder.WriteString("- " + readingTruncateRunes(conclusion.Text, 300) + "\n")
		for _, evidence := range evidenceFromJSON(conclusion.Evidence) {
			builder.WriteString(fmt.Sprintf("  - [%s] %s\n", evidence.ChunkID, readingTruncateRunes(evidence.Quote, 160)))
		}
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
	builder.WriteString(`

请做全局综合：重建论证链、裁决跨章/跨文档矛盾、定稿术语、列出仍未决的问题。
不得新增没有证据的事实；只能合并、裁决或标注未决。新矛盾必须给双方证据。
返回严格 JSON（不要 Markdown、不要解释）：
{
  "summary": "3-5 句综合摘要",
  "argument_chain": [{"claim": "论证链要点", "evidence": [{"chunk_id": "chunk id", "quote": "原文连续片段"}]}],
  "contradictions": [{"description": "冲突说明", "evidence_a": [{"chunk_id": "chunk id", "quote": "原文"}], "evidence_b": [{"chunk_id": "chunk id", "quote": "原文"}]}],
  "term_finalizations": [{"term": "术语", "definition": "定稿定义", "source": [{"chunk_id": "chunk id", "quote": "原文连续片段"}]}],
  "open_questions": ["仍未决的问题"]
}`)
	return builder.String()
}

func parseReadingAgentSynthesis(content string) (*readingAgentSynthesisResponse, error) {
	raw := stripJSONFence(content)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("综合遍未返回 JSON")
	}
	var parsed readingAgentSynthesisResponse
	if err := json.Unmarshal([]byte(raw[start:end+1]), &parsed); err != nil {
		return nil, fmt.Errorf("综合遍 JSON 无效: %w", err)
	}
	if strings.TrimSpace(parsed.Summary) == "" && len(parsed.ArgumentChain) == 0 &&
		len(parsed.TermFinalizations) == 0 && len(parsed.Contradictions) == 0 {
		return nil, fmt.Errorf("综合遍返回空结果")
	}
	return &parsed, nil
}

func readingStrictEvidence(
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
		if !readingQuoteContained(quote, info.Chunk.Content) {
			continue
		}
		seen[chunkID] = struct{}{}
		out = append(out, types.ReadingEvidence{
			KnowledgeID: info.KnowledgeID, DocumentTitle: info.DocumentTitle,
			ChunkID: chunkID, SectionID: chunkSection[chunkID],
			Heading: info.Heading, Quote: quote,
		})
	}
	return out
}

func (s *readingRunService) agentSynthesisStep(
	ctx context.Context, run *types.ReadingRun, plan *types.ReadingPlan,
) (*types.ReadingRun, error) {
	state, err := s.loadReadingRoundState(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	memory, err := s.loadReadingAgentMemory(ctx, run)
	if err != nil {
		return nil, err
	}
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	docs, chunkInfo, chunkSection, err := s.loadReadingDocuments(ctx, caller, run)
	if err != nil {
		return nil, err
	}
	_ = docs
	model, _, err := s.readingAgentModel(ctx, run, true)
	if err != nil {
		return nil, err
	}
	emitReadingPhase(ctx, "synthesis", "start", nil, 0)
	defer emitReadingPhase(ctx, "synthesis", "end", nil, 0)
	depth := readingRunDepth(run)
	synthesisStart := time.Now()
	totalUsage := 0
	messages := []chat.Message{
		{Role: "system", Content: s.readingInstructions(ctx, run) + "\n\n你负责全局综合，必须返回严格 JSON。"},
		{Role: "user", Content: buildReadingAgentSynthesisPrompt(run, plan, memory, state)},
	}
	var parsed *readingAgentSynthesisResponse
	var parseErr error
	var reasoning string
	for attempt := 0; attempt < 2; attempt++ {
		resp, callErr := s.readingModelChat(ctx, model, messages, readingAgentSynthesisOptions(readingRoundMaxTokens(depth)))
		if callErr != nil {
			totalUsage += 0
			parseErr = fmt.Errorf("综合遍调用失败: %w", callErr)
			break
		}
		totalUsage += resp.Usage.TotalTokens
		if strings.TrimSpace(resp.ReasoningContent) != "" {
			reasoning += resp.ReasoningContent + "\n\n"
		}
		parsed, parseErr = parseReadingAgentSynthesis(resp.Content)
		if parseErr == nil {
			break
		}
		messages = append(messages,
			chat.Message{Role: "assistant", Content: resp.Content, ReasoningContent: resp.ReasoningContent},
			chat.Message{Role: "user", Content: "上次输出不是有效 JSON。请只返回严格 JSON，不要解释或 Markdown。"},
		)
	}
	degraded := parseErr != nil
	summary := ""
	argumentChain := []readingAgentArgument{}
	openQuestions := []string{}
	newTerms := []*types.ReadingTerm{}
	newContradictions := []*types.ReadingContradiction{}
	termFinalizations := []readingLLMTerm{}
	if parsed != nil {
		summary = strings.TrimSpace(parsed.Summary)
		for _, argument := range parsed.ArgumentChain {
			claim := strings.TrimSpace(argument.Claim)
			if claim == "" {
				continue
			}
			evidence := readingStrictEvidence(argument.Evidence, chunkInfo, chunkSection)
			if len(evidence) == 0 {
				continue
			}
			validRefs := make([]readingEvidenceRef, 0, len(evidence))
			for _, item := range evidence {
				validRefs = append(validRefs, readingEvidenceRef{ChunkID: item.ChunkID, Quote: item.Quote})
			}
			argumentChain = append(argumentChain, readingAgentArgument{Claim: claim, Evidence: validRefs})
		}
		openQuestions = uniqueReadingStrings(parsed.OpenQuestions)
		now := time.Now()
		existing := map[string]struct{}{}
		for _, contradiction := range state.Contradictions {
			existing[normalizeReadingText(contradiction.Description)] = struct{}{}
		}
		for _, candidate := range parsed.Contradictions {
			description := strings.TrimSpace(candidate.Description)
			if description == "" {
				continue
			}
			key := normalizeReadingText(description)
			if _, ok := existing[key]; ok {
				continue
			}
			evidenceA := readingStrictEvidence(candidate.EvidenceA, chunkInfo, chunkSection)
			evidenceB := readingStrictEvidence(candidate.EvidenceB, chunkInfo, chunkSection)
			if len(evidenceA) == 0 || len(evidenceB) == 0 {
				continue
			}
			existing[key] = struct{}{}
			newContradictions = append(newContradictions, &types.ReadingContradiction{
				ID: uuid.NewString(), RunID: run.ID, TenantID: run.TenantID,
				Description: description, EvidenceA: evidenceJSON(evidenceA), EvidenceB: evidenceJSON(evidenceB),
				Status: types.ReadingContradictionOpen, CreatedAt: now, UpdatedAt: now,
			})
		}
		terms, listErr := s.repo.ListTerms(ctx, run.ID)
		if listErr != nil {
			return nil, listErr
		}
		byTerm := map[string]*types.ReadingTerm{}
		for _, term := range terms {
			byTerm[normalizeReadingText(term.Term)] = term
		}
		for _, candidate := range parsed.TermFinalizations {
			termText := strings.TrimSpace(candidate.Term)
			definition := strings.TrimSpace(candidate.Definition)
			if termText == "" || definition == "" {
				continue
			}
			source := readingStrictEvidence(candidate.Source, chunkInfo, chunkSection)
			if len(source) == 0 {
				continue
			}
			termFinalizations = append(termFinalizations, candidate)
			if existingTerm, ok := byTerm[normalizeReadingText(termText)]; ok {
				existingTerm.Definition = definition
				existingTerm.Status = types.ReadingTermConfirmed
				existingTerm.Canonical = true
				existingTerm.UpdatedAt = now
				if updateErr := s.repo.UpdateTerm(ctx, existingTerm); updateErr != nil {
					return nil, updateErr
				}
				continue
			}
			newTerms = append(newTerms, &types.ReadingTerm{
				ID: uuid.NewString(), RunID: run.ID, TenantID: run.TenantID,
				Round: run.StepCount + 1, Term: termText, Definition: definition,
				Source: evidenceJSON(source), Canonical: true, Status: types.ReadingTermConfirmed,
				CreatedAt: now, UpdatedAt: now,
			})
		}
	}
	if summary == "" {
		degraded = true
		confirmed := 0
		for _, conclusion := range state.Conclusions {
			if conclusion.Status == types.ReadingConclusionConfirmed {
				confirmed++
			}
		}
		summary = fmt.Sprintf("本次精读覆盖 %d 个阅读单元，共确认 %d 条结论；综合遍未产出摘要，请查看结论与矛盾。",
			len(plan.Sections), confirmed)
	}
	run.StepCount++
	run.TokenUsage += totalUsage
	run.Phase = types.ReadingPhaseSelfTest
	run.Status = types.ReadingRunReading
	run.UpdatedAt = time.Now()
	existingFinal := map[string]any{}
	_ = json.Unmarshal(run.Final, &existingFinal)
	existingFinal["summary"] = summary
	existingFinal["argument_chain"] = argumentChain
	existingFinal["open_questions"] = openQuestions
	existingFinal["term_finalizations"] = termFinalizations
	existingFinal["synthesis_degraded"] = degraded
	existingFinal["state_version"] = readingAgentStateVersion
	finalJSON, _ := json.Marshal(existingFinal)
	run.Final = types.JSON(finalJSON)
	stepLog, _ := json.Marshal(map[string]any{
		"phase": "synthesis", "degraded": degraded, "duration_ms": int(time.Since(synthesisStart).Milliseconds()),
		"summary": summary, "argument_count": len(argumentChain),
		"contradiction_count": len(newContradictions), "term_count": len(termFinalizations),
		"reasoning": readingTruncateRunes(reasoning, 30000),
	})
	if parseErr != nil {
		stepLog, _ = json.Marshal(map[string]any{
			"phase": "synthesis", "degraded": true, "error": readingTruncateRunes(parseErr.Error(), 800),
			"summary": summary, "reasoning": readingTruncateRunes(reasoning, 30000),
		})
	}
	step := &types.ReadingRunStep{
		ID: uuid.NewString(), RunID: run.ID, Seq: run.StepCount,
		Action: "synthesis", Question: types.JSON("{}"),
		NotesPatch: types.JSON(stepLog), Citations: types.JSON("[]"),
		Status: "done", CreatedAt: time.Now(),
	}
	if err := s.repo.SaveRound(ctx, run, nil, nil, newTerms, newContradictions, step); err != nil {
		return nil, err
	}
	if err := s.refreshReadingAgentMemory(ctx, run); err != nil {
		logger.Warnf(ctx, "reading: refresh agent memory %s failed: %v", run.ID, err)
	}
	return s.Get(ctx, run.ID)
}

func buildReadingAgentSelfTestPrompt(
	section types.ReadingSection, structure types.ReadingSectionStructure,
	conclusions []*types.ReadingConclusion, terms []*types.ReadingTerm,
) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("章节：%s · %s\n一句话论点：%s（%s）\n\n",
		section.Title, section.Heading, structure.Thesis, structure.KnowledgeType))
	builder.WriteString("已确认结论（含证据）：\n")
	if len(conclusions) == 0 {
		builder.WriteString("（暂无）\n")
	}
	for _, conclusion := range conclusions {
		builder.WriteString("- " + readingTruncateRunes(conclusion.Text, 300) + "\n")
		for _, evidence := range evidenceFromJSON(conclusion.Evidence) {
			builder.WriteString(fmt.Sprintf("  - [%s] %s\n", evidence.ChunkID, readingTruncateRunes(evidence.Quote, 160)))
		}
	}
	builder.WriteString("\n已确认术语：\n")
	if len(terms) == 0 {
		builder.WriteString("（暂无）\n")
	}
	for _, term := range terms {
		builder.WriteString("- " + term.Term + "：" + readingTruncateRunes(term.Definition, 200) + "\n")
		for _, evidence := range evidenceFromJSON(term.Source) {
			builder.WriteString(fmt.Sprintf("  - [%s] %s\n", evidence.ChunkID, readingTruncateRunes(evidence.Quote, 160)))
		}
	}
	builder.WriteString(`

请为本章生成 2-4 道自测题。题目必须可以独立回答；答案必须能由证据支持；禁止使用被驳回的内容。
每题必须给出至少一条有效证据（chunk_id 与逐字 quote）。
返回严格 JSON（不要 Markdown、不要解释）：
{"tests":[{"section_id":"` + section.ID + `","question":"问题","answer":"答案","explanation":"解析","evidence":[{"chunk_id":"chunk id","quote":"原文连续片段"}]}]}`)
	return builder.String()
}

func (s *readingRunService) agentSelfTestStep(
	ctx context.Context, run *types.ReadingRun, plan *types.ReadingPlan,
) (*types.ReadingRun, error) {
	emitReadingPhase(ctx, "self_test", "start", nil, 0)
	defer emitReadingPhase(ctx, "self_test", "end", nil, 0)
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	_, chunkInfo, chunkSection, err := s.loadReadingDocuments(ctx, caller, run)
	if err != nil {
		return nil, err
	}
	state, err := s.loadReadingRoundState(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	model, _, err := s.readingAgentModel(ctx, run, true)
	if err != nil {
		return nil, err
	}
	depth := readingRunDepth(run)
	structure := readingStructureFromPlan(plan)
	confirmedBySection := map[string][]*types.ReadingConclusion{}
	for _, conclusion := range state.Conclusions {
		if conclusion.Status != types.ReadingConclusionConfirmed ||
			conclusion.Source == types.ReadingConclusionSourceLegacy {
			continue
		}
		if len(evidenceFromJSON(conclusion.Evidence)) == 0 {
			continue
		}
		confirmedBySection[conclusion.SectionID] = append(confirmedBySection[conclusion.SectionID], conclusion)
	}
	termsBySection := map[string][]*types.ReadingTerm{}
	for _, term := range state.Terms {
		if term.Status != types.ReadingTermConfirmed {
			continue
		}
		for _, evidence := range evidenceFromJSON(term.Source) {
			if evidence.SectionID != "" {
				termsBySection[evidence.SectionID] = append(termsBySection[evidence.SectionID], term)
				break
			}
		}
	}
	generated := make([]*types.ReadingSelfTest, 0, readingAgentSelfTestCap)
	seenQuestions := map[string]struct{}{}
	dropped := make([]string, 0)
	totalUsage := 0
	reasoningParts := make([]string, 0, 2)
	degraded := false
	sectionCalls := 0
	now := time.Now()
	for _, section := range plan.Sections {
		if len(generated) >= readingAgentSelfTestCap || sectionCalls >= readingAgentSelfTestSectionCap {
			break
		}
		conclusions := confirmedBySection[section.ID]
		terms := termsBySection[section.ID]
		if len(conclusions) == 0 && len(terms) == 0 {
			continue
		}
		sectionCalls++
		entry := structure[section.ID]
		if strings.TrimSpace(entry.Thesis) == "" {
			entry.Thesis = section.Heading
			entry.KnowledgeType = types.ReadingKnowledgeTypeUnknown
		}
		messages := []chat.Message{
			{Role: "system", Content: s.readingInstructions(ctx, run) + "\n\n你负责自测题生成，必须返回严格 JSON。"},
			{Role: "user", Content: buildReadingAgentSelfTestPrompt(section, entry, conclusions, terms)},
		}
		parsed, usage, reasoning, callErr := s.callReadingAgentSelfTests(ctx, model, messages, depth, section.ID)
		totalUsage += usage
		if reasoning != "" {
			reasoningParts = append(reasoningParts, reasoning)
		}
		if callErr != nil {
			degraded = true
			dropped = append(dropped, fmt.Sprintf("%s: %v", section.ID, callErr))
			continue
		}
		sectionGenerated := 0
		for _, item := range parsed.Tests {
			if len(generated) >= readingAgentSelfTestCap {
				break
			}
			if sectionGenerated >= readingAgentSelfTestsPerSection {
				break
			}
			if strings.TrimSpace(item.SectionID) != "" && strings.TrimSpace(item.SectionID) != section.ID {
				dropped = append(dropped, "section mismatch: "+readingTruncateRunes(item.Question, 60))
				continue
			}
			question := strings.TrimSpace(item.Question)
			answer := strings.TrimSpace(item.Answer)
			if question == "" || answer == "" {
				dropped = append(dropped, "empty question/answer")
				continue
			}
			key := normalizeReadingText(question)
			if _, ok := seenQuestions[key]; ok {
				continue
			}
			evidence := readingStrictEvidence(item.Evidence, chunkInfo, chunkSection)
			if len(evidence) == 0 {
				dropped = append(dropped, "no verbatim evidence: "+readingTruncateRunes(question, 60))
				continue
			}
			seenQuestions[key] = struct{}{}
			sectionGenerated++
			generated = append(generated, &types.ReadingSelfTest{
				ID:    uuid.NewSHA1(uuid.NameSpaceOID, []byte(run.ID+"|"+key)).String(),
				RunID: run.ID, TenantID: run.TenantID, SectionID: section.ID,
				Question: question, Answer: answer, Explanation: strings.TrimSpace(item.Explanation),
				Evidence: evidenceJSON(evidence), Status: types.ReadingSelfTestCandidate,
				CreatedAt: now, UpdatedAt: now,
			})
		}
	}
	inserted, err := s.repo.CreateSelfTests(ctx, generated)
	if err != nil {
		return nil, err
	}
	run.StepCount++
	run.TokenUsage += totalUsage
	run.Phase = types.ReadingPhaseDone
	run.UpdatedAt = time.Now()
	stepLog, _ := json.Marshal(map[string]any{
		"phase": "self_test", "generated": len(generated), "inserted": inserted,
		"dropped": dropped, "degraded": degraded,
		"reasoning": readingTruncateRunes(strings.Join(reasoningParts, "\n\n"), 30000),
	})
	step := &types.ReadingRunStep{
		ID: uuid.NewString(), RunID: run.ID, Seq: run.StepCount,
		Action: "self_test", Question: types.JSON("{}"),
		NotesPatch: types.JSON(stepLog), Citations: types.JSON("[]"),
		Status: "done", CreatedAt: time.Now(),
	}
	if err := s.repo.CreateStep(ctx, step); err != nil {
		return nil, err
	}
	return s.finishReadingAgent(ctx, run, plan, state, generated, degraded)
}

func (s *readingRunService) callReadingAgentSelfTests(
	ctx context.Context, model chat.Chat, messages []chat.Message, depth, sectionID string,
) (*readingAgentSelfTestResponse, int, string, error) {
	totalUsage := 0
	var lastErr error
	reasoningParts := make([]string, 0, 2)
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := s.readingModelChat(ctx, model, messages, readingAgentSynthesisOptions(readingRoundMaxTokens(depth)))
		if err != nil {
			return nil, totalUsage, strings.Join(reasoningParts, "\n\n"), fmt.Errorf("自测题生成失败: %w", err)
		}
		totalUsage += resp.Usage.TotalTokens
		if strings.TrimSpace(resp.ReasoningContent) != "" {
			reasoningParts = append(reasoningParts, strings.TrimSpace(resp.ReasoningContent))
		}
		parsed, parseErr := parseReadingAgentSelfTests(resp.Content)
		if parseErr == nil {
			return parsed, totalUsage, strings.Join(reasoningParts, "\n\n"), nil
		}
		lastErr = parseErr
		messages = append(messages,
			chat.Message{Role: "assistant", Content: resp.Content, ReasoningContent: resp.ReasoningContent},
			chat.Message{Role: "user", Content: "上次输出不是有效 JSON。请只返回严格 JSON，不要解释或 Markdown。"},
		)
	}
	return nil, totalUsage, strings.Join(reasoningParts, "\n\n"), fmt.Errorf("自测题 JSON 无效: %w", lastErr)
}

func parseReadingAgentSelfTests(content string) (*readingAgentSelfTestResponse, error) {
	raw := stripJSONFence(content)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("自测遍未返回 JSON")
	}
	var parsed readingAgentSelfTestResponse
	if err := json.Unmarshal([]byte(raw[start:end+1]), &parsed); err != nil {
		return nil, fmt.Errorf("自测遍 JSON 无效: %w", err)
	}
	return &parsed, nil
}

func (s *readingRunService) finishReadingAgent(
	ctx context.Context, run *types.ReadingRun, plan *types.ReadingPlan,
	state *readingRoundState, selfTests []*types.ReadingSelfTest, degraded bool,
) (*types.ReadingRun, error) {
	confirmed := make([]string, 0)
	openQuestions := make([]string, 0)
	contradictions := make([]string, 0)
	citations := make([]types.ReadingEvidence, 0)
	seenCitations := map[string]struct{}{}
	for _, conclusion := range state.Conclusions {
		if conclusion.Status == types.ReadingConclusionRejected {
			continue
		}
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
	final := map[string]any{}
	_ = json.Unmarshal(run.Final, &final)
	if len(openQuestions) > 0 {
		existingOpen, _ := final["open_questions"].([]any)
		for _, item := range existingOpen {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				openQuestions = append(openQuestions, text)
			}
		}
		openQuestions = uniqueReadingStrings(openQuestions)
	}
	final["key_points"] = confirmed
	if _, ok := final["open_questions"]; !ok {
		final["open_questions"] = openQuestions
	}
	final["contradictions"] = contradictions
	final["citations"] = citations
	final["structure"] = plan.Structure
	final["self_tests"] = map[string]any{
		"generated": len(selfTests), "candidate": len(selfTests), "degraded": degraded,
	}
	readSections := 0
	for _, section := range plan.Sections {
		if section.Read {
			readSections++
		}
	}
	final["coverage"] = map[string]any{
		"documents": len(planDocumentIDs(run)), "total_sections": len(plan.Sections),
		"read_sections": readSections, "step_count": run.StepCount + 1,
		"token_usage": run.TokenUsage,
	}
	final["state_version"] = readingAgentStateVersion
	final["generated_at"] = time.Now().Format(time.RFC3339)
	finalJSON, _ := json.Marshal(final)
	run.Final = types.JSON(finalJSON)
	run.Status = types.ReadingRunFinished
	run.Phase = types.ReadingPhaseDone
	run.StepCount++
	run.UpdatedAt = time.Now()
	finishLog, _ := json.Marshal(map[string]any{
		"phase": "self_test", "degraded": degraded, "self_test_count": len(selfTests),
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
	if err := s.refreshReadingAgentMemory(ctx, run); err != nil {
		logger.Warnf(ctx, "reading: refresh agent memory %s failed: %v", run.ID, err)
	}
	return s.Get(ctx, run.ID)
}

func readingAgentTrailingSkips(results []readingAgentSectionResult) int {
	trailing := 0
	for _, item := range results {
		if item.Result == "skipped" {
			trailing++
		} else {
			trailing = 0
		}
	}
	return trailing
}

func (s *readingRunService) loadReadingAgentMemory(
	ctx context.Context, run *types.ReadingRun,
) (*readingAgentMemoryState, error) {
	row, err := s.repo.GetMemory(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	state := &readingAgentMemoryState{}
	if row == nil {
		return state, nil
	}
	if len(row.Knowledge) > 0 {
		_ = json.Unmarshal(row.Knowledge, &state.Knowledge)
	}
	if len(row.UserModel) > 0 {
		_ = json.Unmarshal(row.UserModel, &state.User)
	}
	if len(row.Task) > 0 {
		_ = json.Unmarshal(row.Task, &state.Task)
	}
	return state, nil
}

func (s *readingRunService) refreshReadingAgentMemory(ctx context.Context, run *types.ReadingRun) error {
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return err
	}
	previous, err := s.loadReadingAgentMemory(ctx, run)
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
	understandings, err := s.repo.ListUnderstandings(ctx, run.ID)
	if err != nil {
		return err
	}
	questions, err := s.repo.ListQuestions(ctx, run.ID)
	if err != nil {
		return err
	}
	structure := readingStructureFromPlan(&plan)
	latestUnderstanding := map[string]*types.ReadingUnderstanding{}
	for _, item := range understandings {
		latestUnderstanding[item.SectionID] = item
	}
	questionBySection := map[string][]*types.ReadingQuestionNode{}
	for _, question := range questions {
		questionBySection[question.SectionID] = append(questionBySection[question.SectionID], question)
	}
	knowledge := readingAgentKnowledgeMemory{
		SectionPoints:  make([]readingAgentSectionPoint, 0, len(plan.Sections)),
		Conclusions:    make([]string, 0, 8),
		Terms:          make([]readingAgentTermMemory, 0, 8),
		Contradictions: make([]string, 0, 4),
	}
	user := readingAgentUserMemory{
		Sections: make([]readingAgentSectionResult, 0, len(plan.Sections)),
		Rejected: make([]string, 0, 4),
	}
	for _, section := range plan.Sections {
		entry := structure[section.ID]
		thesis := strings.TrimSpace(entry.Thesis)
		if thesis == "" {
			thesis = strings.TrimSpace(section.Heading)
		}
		point := readingAgentSectionPoint{SectionID: section.ID, Thesis: thesis}
		if understanding := latestUnderstanding[section.ID]; understanding != nil {
			point.OneLine = readingTruncateRunes(understanding.Text, 160)
		}
		knowledge.SectionPoints = append(knowledge.SectionPoints, point)
		if !section.Read {
			continue
		}
		result := "pending"
		errorPoints := make([]string, 0, 2)
		if understanding := latestUnderstanding[section.ID]; understanding != nil {
			switch understanding.Decision {
			case types.ReadingUnderstandingConfirmed:
				result = "confirmed"
			case types.ReadingUnderstandingCorrected:
				result = "corrected"
				if strings.TrimSpace(understanding.Correction) != "" {
					errorPoints = append(errorPoints, readingTruncateRunes(understanding.Correction, 160))
				}
			case types.ReadingUnderstandingSkipped:
				result = "skipped"
			}
		}
		for _, question := range questionBySection[section.ID] {
			switch question.Status {
			case types.ReadingQuestionSkipped:
				if result == "pending" {
					result = "skipped"
				}
			case types.ReadingQuestionAnswered, types.ReadingQuestionAutoAnswered, types.ReadingQuestionContradicted:
				if result == "pending" || result == "skipped" {
					result = "answered"
				}
			}
			if question.Judgment == types.ReadingQuestionJudgmentPartial ||
				question.Judgment == types.ReadingQuestionJudgmentWrong {
				errorPoints = append(errorPoints, readingTruncateRunes(question.JudgmentReason, 160))
			}
		}
		user.Sections = append(user.Sections, readingAgentSectionResult{
			SectionID: section.ID, Result: result, ErrorPoints: strings.Join(errorPoints, "；"),
		})
	}
	for _, conclusion := range conclusions {
		switch {
		case conclusion.Status == types.ReadingConclusionRejected:
			user.Rejected = append(user.Rejected, readingTruncateRunes(conclusion.Text, 160))
		case len(knowledge.Conclusions) < readingAgentMemoryConclusionCap:
			knowledge.Conclusions = append(knowledge.Conclusions, readingTruncateRunes(conclusion.Text, 300))
		}
	}
	for _, term := range terms {
		if term.Status == types.ReadingTermRejected || len(knowledge.Terms) >= readingAgentMemoryTermCap {
			continue
		}
		knowledge.Terms = append(knowledge.Terms, readingAgentTermMemory{
			Term: term.Term, Definition: readingTruncateRunes(term.Definition, 200),
		})
	}
	for _, contradiction := range contradictions {
		if len(knowledge.Contradictions) >= 10 {
			break
		}
		knowledge.Contradictions = append(knowledge.Contradictions, readingTruncateRunes(contradiction.Description, 200))
	}
	trailingSkips := readingAgentTrailingSkips(user.Sections)
	user.ConsecutiveSkips = trailingSkips
	user.Quiet = previous.User.Quiet || trailingSkips >= readingAgentMemorySkipLimit
	depth := readingRunDepth(run)
	task := readingAgentTaskMemory{
		Goal: readingGoal(run), Depth: depth, Phase: run.Phase,
		CurrentSection:  run.CurrentSectionID,
		RemainingSteps:  readingStructuredStepLimit(depth) - run.StepCount,
		RemainingTokens: readingStructuredTokenBudget(depth) - run.TokenUsage,
		Quiet:           user.Quiet,
	}
	knowledgeJSON, _ := json.Marshal(knowledge)
	userJSON, _ := json.Marshal(user)
	taskJSON, _ := json.Marshal(task)
	memory := &types.ReadingRunMemory{
		RunID: run.ID, TenantID: run.TenantID,
		Knowledge: types.JSON(knowledgeJSON), UserModel: types.JSON(userJSON),
		Task: types.JSON(taskJSON), UpdatedAt: time.Now(),
	}
	return s.repo.SaveMemory(ctx, memory)
}

// ListSelfTests returns candidate and imported self-tests for one owned run.
func (s *readingRunService) ListSelfTests(
	ctx context.Context, runID string,
) ([]*types.ReadingSelfTest, error) {
	_, run, err := s.ownedRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListSelfTests(ctx, run.ID)
}

// ImportSelfTests turns selected candidates into review cards. Each test is
// imported individually so a partial failure leaves already-imported rows
// marked and retries stay idempotent.
func (s *readingRunService) ImportSelfTests(
	ctx context.Context, runID string, req *types.ImportReadingSelfTestsRequest,
) (*types.ReadingSelfTestImportResult, error) {
	_, run, err := s.ownedRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	groupID := strings.TrimSpace(req.GroupID)
	if groupID == "" {
		return nil, werrors.NewValidationError("请选择复习卡组")
	}
	wanted := make(map[string]struct{}, len(req.SelfTestIDs))
	for _, id := range req.SelfTestIDs {
		if value := strings.TrimSpace(id); value != "" {
			wanted[value] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return nil, werrors.NewValidationError("没有可导入的自测题")
	}
	if s.cardImporter == nil {
		return nil, werrors.NewInternalServerError("复习导入服务未配置")
	}
	tests, err := s.repo.ListSelfTests(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	result := &types.ReadingSelfTestImportResult{}
	for _, test := range tests {
		if _, ok := wanted[test.ID]; !ok {
			continue
		}
		if test.Status != types.ReadingSelfTestCandidate {
			result.Skipped++
			continue
		}
		sources := make([]types.ReviewCardSourceInput, 0, 1)
		for _, evidence := range evidenceFromJSON(test.Evidence) {
			sources = append(sources, types.ReviewCardSourceInput{
				SourceType:  types.ReviewSourceTypeKnowledge,
				KnowledgeID: evidence.KnowledgeID,
				ChunkID:     evidence.ChunkID,
				Excerpt:     readingTruncateRunes(evidence.Quote, 300),
				IsPrimary:   len(sources) == 0,
			})
		}
		imported, importErr := s.cardImporter.ImportGeneratedCards(ctx, &types.ImportReviewCardsRequest{
			GroupID: groupID,
			Cards: []types.CreateReviewCardRequest{{
				GroupID: groupID, Question: test.Question, Answer: test.Answer,
				Explanation: test.Explanation, Sources: sources,
			}},
		})
		if importErr != nil {
			return result, importErr
		}
		if imported == 0 {
			result.Skipped++
			continue
		}
		updated, updateErr := s.repo.UpdateSelfTestStatus(
			ctx, run.ID, test.ID, types.ReadingSelfTestImported,
		)
		if updateErr != nil {
			return result, updateErr
		}
		if updated {
			result.Imported++
		} else {
			result.Skipped++
		}
	}
	return result, nil
}
