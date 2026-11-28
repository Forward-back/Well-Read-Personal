package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	werrors "gaze/internal/errors"
	"gaze/internal/models/chat"
	"gaze/internal/types"
)

type readingFeynmanPointResponse struct {
	Text     string               `json:"text"`
	Evidence []readingEvidenceRef `json:"evidence"`
}

type readingFeynmanAngleFeedbackResponse struct {
	AngleID   string                        `json:"angle_id"`
	Status    string                        `json:"status"`
	Summary   string                        `json:"summary"`
	Omissions []readingFeynmanPointResponse `json:"omissions"`
	Errors    []readingFeynmanPointResponse `json:"errors"`
}

type readingFeynmanResponse struct {
	Score         int                                   `json:"score"`
	Coverage      int                                   `json:"coverage"`
	Accuracy      int                                   `json:"accuracy"`
	EvidenceScore int                                   `json:"evidence_score"`
	Summary       string                                `json:"summary"`
	Omissions     []readingFeynmanPointResponse         `json:"omissions"`
	Errors        []readingFeynmanPointResponse         `json:"errors"`
	AngleFeedback []readingFeynmanAngleFeedbackResponse `json:"angle_feedback"`
}

const readingFeynmanPromptVersion = 1

type readingFeynmanAngle struct {
	ID     string
	Label  string
	Prompt string
}

var readingFeynmanAngles = []readingFeynmanAngle{
	{ID: "macro", Label: "宏观主旨", Prompt: "这一节回答什么问题？核心结论是什么？与上下文的关系？"},
	{ID: "mechanism", Label: "机制/因果", Prompt: "关键步骤、条件与因果链是什么？为什么这样设计或运行？"},
	{ID: "detail", Label: "微观细节", Prompt: "关键术语、参数、数字、边界与例外有哪些？"},
	{ID: "application", Label: "应用/边界", Prompt: "什么场景适用？何时失效？与相似概念的区别？"},
}

func readingFeynmanAngleByID(id string) (readingFeynmanAngle, bool) {
	for _, angle := range readingFeynmanAngles {
		if angle.ID == id {
			return angle, true
		}
	}
	return readingFeynmanAngle{}, false
}

func clampReadingScore(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func (s *readingRunService) SubmitFeynman(
	ctx context.Context, runID string, req *types.SubmitReadingFeynmanRequest,
) (*types.ReadingFeynmanAttempt, error) {
	caller, run, err := s.ownedRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, werrors.NewValidationError("缺少费曼自述内容")
	}
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return nil, werrors.NewInternalServerError("阅读计划损坏")
	}
	section := findReadingSection(&plan, req.SectionID)
	if section == nil {
		return nil, werrors.NewValidationError("章节不存在")
	}
	answers, retelling, promptVersion, err := normalizeReadingFeynmanAnswers(req)
	if err != nil {
		return nil, err
	}
	_, chunkInfo, chunkSection, err := s.loadReadingDocuments(ctx, caller, run)
	if err != nil {
		return nil, err
	}
	model, _, err := s.readingAgentModel(ctx, run, true)
	if err != nil {
		return nil, err
	}
	depth := readingRunDepth(run)
	msgs := buildFeynmanMessages(s, ctx, run, &plan, section, answers, retelling)
	var parsed *readingFeynmanResponse
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		resp, callErr := model.Chat(ctx, msgs, readingAgentSynthesisOptions(readingRoundMaxTokens(depth)))
		if callErr != nil {
			lastErr = callErr
			break
		}
		if strings.TrimSpace(resp.ReasoningContent) != "" {
			msgs = append(msgs, chat.Message{
				Role: "assistant", Content: resp.Content, ReasoningContent: resp.ReasoningContent,
			})
		}
		candidate, parseErr := parseReadingFeynmanResponse(resp.Content)
		if parseErr == nil {
			parsed = candidate
			break
		}
		lastErr = parseErr
		msgs = append(msgs, chat.Message{
			Role: "user", Content: "上次输出不是有效 JSON。请只返回严格 JSON，不要包含解释或 Markdown。",
		})
	}
	if parsed == nil {
		return nil, werrors.NewInternalServerError(fmt.Sprintf("费曼评估失败: %v", lastErr))
	}
	omissions, omissionEvidence := strictReadingFeynmanPoints(parsed.Omissions, chunkInfo, chunkSection)
	errorsFound, errorEvidence := strictReadingFeynmanPoints(parsed.Errors, chunkInfo, chunkSection)
	angleFeedback, angleEvidence := strictReadingFeynmanAngleFeedback(
		parsed.AngleFeedback, answers, chunkInfo, chunkSection,
	)
	evidence := append(omissionEvidence, errorEvidence...)
	evidence = append(evidence, angleEvidence...)
	omissionJSON, _ := json.Marshal(omissions)
	errorJSON, _ := json.Marshal(errorsFound)
	evidenceJSON, _ := json.Marshal(evidence)
	answersJSON, _ := json.Marshal(answers)
	feedbackJSON, _ := json.Marshal(angleFeedback)
	attempt := &types.ReadingFeynmanAttempt{
		ID: uuid.NewString(), RunID: run.ID, TenantID: run.TenantID,
		SectionID: section.ID, Text: retelling,
		Score: clampReadingScore(parsed.Score), Coverage: clampReadingScore(parsed.Coverage),
		Accuracy: clampReadingScore(parsed.Accuracy), EvidenceScore: clampReadingScore(parsed.EvidenceScore),
		Summary:   strings.TrimSpace(parsed.Summary),
		Omissions: types.JSON(omissionJSON), Errors: types.JSON(errorJSON), Evidence: types.JSON(evidenceJSON),
		Answers: types.JSON(answersJSON), AngleFeedback: types.JSON(feedbackJSON),
		PromptVersion: promptVersion, CreatedAt: time.Now(),
	}
	if err := s.repo.CreateFeynmanAttempt(ctx, attempt); err != nil {
		return nil, err
	}
	return attempt, nil
}

func normalizeReadingFeynmanAnswers(
	req *types.SubmitReadingFeynmanRequest,
) ([]types.ReadingFeynmanAnswer, string, int, error) {
	const maxAnswerRunes = 2000
	const maxTotalRunes = 6000
	provided := map[string]string{}
	for _, answer := range req.Answers {
		id := strings.TrimSpace(answer.AngleID)
		text := strings.TrimSpace(answer.Text)
		if id == "" || text == "" {
			continue
		}
		provided[id] = text
	}
	freeform := strings.TrimSpace(req.Freeform)
	legacy := strings.TrimSpace(req.Text)
	out := make([]types.ReadingFeynmanAnswer, 0, len(readingFeynmanAngles)+2)
	for _, angle := range readingFeynmanAngles {
		text, ok := provided[angle.ID]
		if !ok {
			continue
		}
		if len([]rune(text)) > maxAnswerRunes {
			return nil, "", 0, werrors.NewValidationError(angle.Label + "内容最多 2000 字")
		}
		out = append(out, types.ReadingFeynmanAnswer{
			AngleID: angle.ID, Label: angle.Label, Prompt: angle.Prompt, Text: text,
		})
	}
	if freeform != "" {
		if len([]rune(freeform)) > maxAnswerRunes {
			return nil, "", 0, werrors.NewValidationError("其他理解最多 2000 字")
		}
		out = append(out, types.ReadingFeynmanAnswer{
			AngleID: "freeform", Label: "其他理解", Prompt: "其他理解或疑问", Text: freeform,
		})
	}
	promptVersion := readingFeynmanPromptVersion
	if len(out) == 0 && legacy != "" {
		if len([]rune(legacy)) > 4000 {
			return nil, "", 0, werrors.NewValidationError("复述内容最多 4000 字")
		}
		out = append(out, types.ReadingFeynmanAnswer{
			AngleID: "legacy", Label: "复述", Prompt: "复述本章内容", Text: legacy,
		})
		promptVersion = 0
	}
	if len(out) == 0 {
		return nil, "", 0, werrors.NewValidationError("请至少填写一个理解角度")
	}
	longest := 0
	total := 0
	for _, answer := range out {
		n := len([]rune(answer.Text))
		if n > longest {
			longest = n
		}
		total += n
	}
	if longest < 20 {
		return nil, "", 0, werrors.NewValidationError("复述内容过短，请至少写 20 个字")
	}
	if total > maxTotalRunes {
		return nil, "", 0, werrors.NewValidationError("复述内容合计最多 6000 字")
	}
	var builder strings.Builder
	for _, answer := range out {
		builder.WriteString("【" + answer.Label + "】\n" + answer.Text + "\n\n")
	}
	return out, strings.TrimSpace(builder.String()), promptVersion, nil
}

func (s *readingRunService) ListFeynman(
	ctx context.Context, runID, sectionID string,
) ([]*types.ReadingFeynmanAttempt, error) {
	_, run, err := s.ownedRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListFeynmanAttempts(ctx, run.ID, strings.TrimSpace(sectionID))
}

func (s *readingRunService) CreateAnnotation(
	ctx context.Context, runID string, req *types.CreateReadingAnnotationRequest,
) (*types.ReadingAnnotation, error) {
	caller, run, err := s.ownedRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, werrors.NewValidationError("缺少批注内容")
	}
	quote := strings.TrimSpace(req.Quote)
	if len([]rune(quote)) < 2 {
		return nil, werrors.NewValidationError("选区过短")
	}
	if len([]rune(quote)) > 1200 {
		return nil, werrors.NewValidationError("选区过长，请缩小范围")
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 2000 {
		return nil, werrors.NewValidationError("批注最多 2000 字")
	}
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return nil, werrors.NewInternalServerError("阅读计划损坏")
	}
	section := findReadingSection(&plan, req.SectionID)
	if section == nil {
		return nil, werrors.NewValidationError("章节不存在")
	}
	_, chunkInfo, _, err := s.loadReadingDocuments(ctx, caller, run)
	if err != nil {
		return nil, err
	}
	chunkID := ""
	knowledgeID := ""
	for _, candidate := range readingSectionChunkIDs(section) {
		info := chunkInfo[candidate]
		if info == nil || info.Chunk == nil {
			continue
		}
		if readingQuoteContained(quote, info.Chunk.Content) {
			chunkID = candidate
			knowledgeID = info.KnowledgeID
			break
		}
	}
	if chunkID == "" {
		// The drawer can show a different document than the current section;
		// fall back to any section that contains the quoted passage.
		for index := range plan.Sections {
			candidateSection := &plan.Sections[index]
			for _, candidate := range readingSectionChunkIDs(candidateSection) {
				info := chunkInfo[candidate]
				if info == nil || info.Chunk == nil {
					continue
				}
				if readingQuoteContained(quote, info.Chunk.Content) {
					section = candidateSection
					chunkID = candidate
					knowledgeID = info.KnowledgeID
					break
				}
			}
			if chunkID != "" {
				break
			}
		}
	}
	if chunkID == "" {
		return nil, werrors.NewValidationError("选区不在当前章节原文中")
	}
	now := time.Now()
	annotation := &types.ReadingAnnotation{
		ID: uuid.NewString(), RunID: run.ID, TenantID: run.TenantID,
		SectionID: section.ID, KnowledgeID: knowledgeID, ChunkID: chunkID,
		Quote: quote, Comment: comment, Status: "active",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CreateAnnotation(ctx, annotation); err != nil {
		return nil, err
	}
	return annotation, nil
}

func (s *readingRunService) DeleteAnnotation(
	ctx context.Context, runID, annotationID string,
) error {
	_, run, err := s.ownedRun(ctx, runID)
	if err != nil {
		return err
	}
	annotation, err := s.repo.GetAnnotation(ctx, run.ID, annotationID)
	if err != nil || annotation == nil {
		return werrors.NewNotFoundError("批注不存在")
	}
	return s.repo.DeleteAnnotation(ctx, run.ID, annotationID)
}

func (s *readingRunService) CreateCardFromAnnotation(
	ctx context.Context, runID, annotationID string,
	req *types.CreateReadingAnnotationCardRequest,
) (*types.ReadingAnnotationCardResult, error) {
	_, run, err := s.ownedRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.GroupID) == "" {
		return nil, werrors.NewValidationError("请选择复习卡组")
	}
	if s.cardImporter == nil {
		return nil, werrors.NewInternalServerError("复习导入服务未配置")
	}
	annotation, err := s.repo.GetAnnotation(ctx, run.ID, annotationID)
	if err != nil || annotation == nil {
		return nil, werrors.NewNotFoundError("批注不存在")
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		question = strings.TrimSpace(annotation.Comment)
	}
	if question == "" {
		question = "解释这段原文：" + readingTruncateRunes(annotation.Quote, 120)
	}
	answer := strings.TrimSpace(req.Answer)
	if answer == "" {
		answer = annotation.Quote
	}
	imported, err := s.cardImporter.ImportGeneratedCards(ctx, &types.ImportReviewCardsRequest{
		GroupID: req.GroupID,
		Cards: []types.CreateReviewCardRequest{{
			GroupID: req.GroupID, Question: question, Answer: answer,
			Sources: []types.ReviewCardSourceInput{{
				SourceType:  types.ReviewSourceTypeKnowledge,
				KnowledgeID: annotation.KnowledgeID,
				ChunkID:     annotation.ChunkID,
				Excerpt:     readingTruncateRunes(annotation.Quote, 300),
				IsPrimary:   true,
			}},
		}},
	})
	if err != nil {
		return nil, err
	}
	return &types.ReadingAnnotationCardResult{Imported: imported}, nil
}

func buildFeynmanMessages(
	s *readingRunService, ctx context.Context, run *types.ReadingRun,
	plan *types.ReadingPlan, section *types.ReadingSection,
	answers []types.ReadingFeynmanAnswer, retelling string,
) []chat.Message {
	var material strings.Builder
	if len(section.Segments) > 0 {
		for _, segment := range section.Segments {
			material.WriteString(fmt.Sprintf("[chunk %s | %s]\n%s\n\n", segment.ID, segment.Heading, segment.Content))
		}
	} else {
		material.WriteString(section.Content)
		if len(section.ChunkIDs) > 0 {
			material.WriteString("\n\n可用 chunk_id：" + strings.Join(section.ChunkIDs, ", "))
		}
	}
	var answerText strings.Builder
	if len(answers) > 0 {
		for _, answer := range answers {
			answerText.WriteString("【" + answer.Label + "】\n" + answer.Text + "\n\n")
		}
	} else {
		answerText.WriteString(retelling)
	}
	user := fmt.Sprintf(`阅读目标：%s
阅读深度：%s
当前章节：%s · %s

本章原文：
%s

用户按角度作答（所有角度权重相同，自由补充也算一个角度）：
%s

请只依据上面的原文评估这些回答。返回严格 JSON（不要 Markdown、不要解释）：
{
  "score": 0,
  "coverage": 0,
  "accuracy": 0,
  "evidence_score": 0,
  "summary": "2-3 句总体评价",
  "omissions": [{"text": "遗漏的关键点", "evidence": [{"chunk_id": "chunk id", "quote": "原文连续片段"}]}],
  "errors": [{"text": "错误或与原文不一致之处", "evidence": [{"chunk_id": "chunk id", "quote": "原文连续片段"}]}],
  "angle_feedback": [
    {
      "angle_id": "macro|mechanism|detail|application|freeform|legacy",
      "status": "strong|partial|missing",
      "summary": "该角度的一句话评价",
      "omissions": [{"text": "该角度遗漏点", "evidence": [{"chunk_id": "chunk id", "quote": "原文连续片段"}]}],
      "errors": [{"text": "该角度错误点", "evidence": [{"chunk_id": "chunk id", "quote": "原文连续片段"}]}]
    }
  ]
}

规则：
- 四个分数字段为 0-100；score 是总体分数，coverage/accuracy/evidence_score 分别衡量覆盖、准确与证据意识。
- 逐个评估用户提供的角度，angle_id 必须使用输入中出现过的 id；未回答的角度不要编造反馈。
- status：strong=该角度覆盖准确；partial=部分覆盖或有小错；missing=该角度没有有效内容。
- 自由补充与预设角度同权参与总体评分；它可以补足预设未覆盖的内容，也可以产生遗漏或错误。
- 只判断回答中出现的说法与本章原文；不得引入本章之外的资料。
- 每个 omission / error 必须给出本章原文中可验证的连续 quote 与 chunk_id；无法引用原文的点不要输出。
- 回答正确且完整时，omissions 与 errors 可以是空数组。
- 不要复述用户原文，不要输出 JSON 之外的内容。
`, readingGoal(run), readingDepthLabel(plan.Depth), section.Title, section.Heading,
		material.String(), answerText.String())
	return []chat.Message{
		{Role: "system", Content: s.readingInstructions(ctx, run) + "\n\n你负责费曼自述评估，必须严格依据原文证据并返回严格 JSON。"},
		{Role: "user", Content: user},
	}
}

func readingFeynmanPointTexts(raw types.JSON) []string {
	var points []types.ReadingFeynmanPoint
	if len(raw) == 0 || json.Unmarshal(raw, &points) != nil {
		return nil
	}
	out := make([]string, 0, len(points))
	for _, point := range points {
		if text := strings.TrimSpace(point.Text); text != "" {
			out = append(out, readingTruncateRunes(text, 160))
		}
	}
	return out
}

func findReadingSection(plan *types.ReadingPlan, sectionID string) *types.ReadingSection {
	sectionID = strings.TrimSpace(sectionID)
	for index := range plan.Sections {
		if plan.Sections[index].ID == sectionID {
			return &plan.Sections[index]
		}
	}
	return nil
}

func readingSectionChunkIDs(section *types.ReadingSection) []string {
	ids := make([]string, 0, len(section.Segments)+len(section.ChunkIDs))
	seen := map[string]struct{}{}
	for _, segment := range section.Segments {
		if segment.ID != "" {
			if _, ok := seen[segment.ID]; !ok {
				seen[segment.ID] = struct{}{}
				ids = append(ids, segment.ID)
			}
		}
	}
	for _, id := range section.ChunkIDs {
		if id != "" {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func parseReadingFeynmanAngleFeedback(raw types.JSON) []types.ReadingFeynmanAngleFeedback {
	var feedback []types.ReadingFeynmanAngleFeedback
	if len(raw) == 0 || json.Unmarshal(raw, &feedback) != nil {
		return nil
	}
	return feedback
}

func readingFeynmanAngleLabel(id string) string {
	if angle, ok := readingFeynmanAngleByID(id); ok {
		return angle.Label
	}
	if id == "freeform" {
		return "其他理解"
	}
	if id == "legacy" {
		return "复述"
	}
	return id
}

func readingFeynmanWeakAngles(feynman *types.ReadingFeynmanAttempt) []string {
	feedback := parseReadingFeynmanAngleFeedback(feynman.AngleFeedback)
	out := make([]string, 0, len(feedback))
	for _, item := range feedback {
		if item.Status == "strong" || item.Status == "skipped" {
			continue
		}
		label := readingFeynmanAngleLabel(item.AngleID)
		if strings.TrimSpace(item.Summary) != "" {
			label += "：" + readingTruncateRunes(item.Summary, 120)
		}
		out = append(out, label)
	}
	return out
}

func strictReadingFeynmanPoints(
	points []readingFeynmanPointResponse,
	chunkInfo map[string]*readingChunkInfo,
	chunkSection map[string]string,
) ([]types.ReadingFeynmanPoint, []types.ReadingEvidence) {
	out := make([]types.ReadingFeynmanPoint, 0, len(points))
	evidenceAll := make([]types.ReadingEvidence, 0)
	for _, point := range points {
		text := strings.TrimSpace(point.Text)
		if text == "" {
			continue
		}
		evidence := readingStrictEvidence(point.Evidence, chunkInfo, chunkSection)
		if len(evidence) == 0 {
			continue
		}
		out = append(out, types.ReadingFeynmanPoint{Text: readingTruncateRunes(text, 300), Evidence: evidence})
		evidenceAll = append(evidenceAll, evidence...)
	}
	return out, evidenceAll
}

func normalizeReadingFeynmanStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strong", "covered", "complete", "full":
		return "strong"
	case "missing", "none", "absent", "empty":
		return "missing"
	case "skipped", "skip":
		return "skipped"
	default:
		return "partial"
	}
}

func strictReadingFeynmanAngleFeedback(
	raw []readingFeynmanAngleFeedbackResponse,
	answers []types.ReadingFeynmanAnswer,
	chunkInfo map[string]*readingChunkInfo,
	chunkSection map[string]string,
) ([]types.ReadingFeynmanAngleFeedback, []types.ReadingEvidence) {
	allowed := map[string]struct{}{}
	for _, answer := range answers {
		allowed[answer.AngleID] = struct{}{}
	}
	out := make([]types.ReadingFeynmanAngleFeedback, 0, len(raw))
	evidenceAll := make([]types.ReadingEvidence, 0)
	seen := map[string]struct{}{}
	for _, item := range raw {
		id := strings.TrimSpace(item.AngleID)
		if _, ok := allowed[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		omissions, omissionEvidence := strictReadingFeynmanPoints(item.Omissions, chunkInfo, chunkSection)
		errorsFound, errorEvidence := strictReadingFeynmanPoints(item.Errors, chunkInfo, chunkSection)
		evidenceAll = append(evidenceAll, omissionEvidence...)
		evidenceAll = append(evidenceAll, errorEvidence...)
		out = append(out, types.ReadingFeynmanAngleFeedback{
			AngleID:   id,
			Status:    normalizeReadingFeynmanStatus(item.Status),
			Summary:   readingTruncateRunes(strings.TrimSpace(item.Summary), 300),
			Omissions: omissions,
			Errors:    errorsFound,
		})
	}
	return out, evidenceAll
}

func parseReadingFeynmanResponse(content string) (*readingFeynmanResponse, error) {
	raw := stripJSONFence(content)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("费曼评估未返回 JSON")
	}
	var parsed readingFeynmanResponse
	if err := json.Unmarshal([]byte(raw[start:end+1]), &parsed); err != nil {
		return nil, fmt.Errorf("费曼评估 JSON 无效: %w", err)
	}
	if strings.TrimSpace(parsed.Summary) == "" && len(parsed.Omissions) == 0 && len(parsed.Errors) == 0 {
		return nil, fmt.Errorf("费曼评估返回空结果")
	}
	return &parsed, nil
}

type readingFinalArgumentChain struct {
	ArgumentChain []readingAgentArgument `json:"argument_chain"`
}

func readingMermaidLabel(value string, limit int) string {
	text := readingTruncateRunes(strings.TrimSpace(value), limit)
	// Mermaid treats $...$ as math and backticks/pipes/angle brackets as
	// markdown or diagram syntax; raw source quotes often contain them.
	replacer := strings.NewReplacer(
		"\"", "'", "\n", " ", "\r", " ",
		"[", "［", "]", "］", "{", "（", "}", "）",
		"$", "＄", "\\", " ", "`", "'", "*", "",
		"|", "｜", "<", "＜", ">", "＞", "&", "＆", "#", "＃",
	)
	return replacer.Replace(text)
}

// buildReadingArgumentChainMermaid derives a read-only graph from the
// synthesis argument chain plus contradiction evidence. No new storage: the
// graph follows the current structured data on every read.
func buildReadingArgumentChainMermaid(run *types.ReadingRun) (string, []*types.ReadingArgumentChainNode) {
	if run == nil {
		return "", nil
	}
	var final readingFinalArgumentChain
	_ = json.Unmarshal(run.Final, &final)
	if len(final.ArgumentChain) == 0 && len(run.Contradictions) == 0 {
		return "", nil
	}
	type chunkMeta struct {
		KnowledgeID   string
		DocumentTitle string
		SectionID     string
		Heading       string
	}
	metaByChunk := map[string]chunkMeta{}
	var plan types.ReadingPlan
	if json.Unmarshal(run.Plan, &plan) == nil {
		for _, section := range plan.Sections {
			for _, id := range readingSectionChunkIDs(&section) {
				metaByChunk[id] = chunkMeta{
					KnowledgeID: section.KnowledgeID, DocumentTitle: section.Title,
					SectionID: section.ID, Heading: section.Heading,
				}
			}
		}
	}
	var builder strings.Builder
	builder.WriteString("flowchart LR\n")
	builder.WriteString("  classDef claim fill:#DFE7EA,stroke:#47677A,color:#24323A;\n")
	builder.WriteString("  classDef evidence fill:#FBF9F5,stroke:#C9D4DB,color:#3F3A34;\n")
	builder.WriteString("  classDef rebuttal fill:#EBDCD7,stroke:#8A4E47,color:#5A3531;\n")
	builder.WriteString("  classDef resolved fill:#EDE7DD,stroke:#857C70,color:#6E675E;\n")
	evidenceNodes := map[string]string{}
	evidenceList := make([]*types.ReadingArgumentChainNode, 0)
	nextEvidence := 1
	evidenceNode := func(evidence types.ReadingEvidence) string {
		key := strings.TrimSpace(evidence.ChunkID) + "|" + readingTruncateRunes(strings.TrimSpace(evidence.Quote), 160)
		if id, ok := evidenceNodes[key]; ok {
			return id
		}
		id := fmt.Sprintf("E%d", nextEvidence)
		nextEvidence++
		evidenceNodes[key] = id
		builder.WriteString(fmt.Sprintf("  %s[\"%s\"]:::evidence\n", id, readingMermaidLabel(evidence.Quote, 90)))
		evidenceList = append(evidenceList, &types.ReadingArgumentChainNode{ID: id, Evidence: evidence})
		return id
	}
	claims := 0
	for _, argument := range final.ArgumentChain {
		claim := strings.TrimSpace(argument.Claim)
		if claim == "" || len(argument.Evidence) == 0 {
			continue
		}
		claims++
		if claims > 12 {
			break
		}
		claimID := fmt.Sprintf("C%d", claims)
		builder.WriteString(fmt.Sprintf("  %s[\"%s\"]:::claim\n", claimID, readingMermaidLabel(claim, 100)))
		for _, reference := range argument.Evidence {
			meta := metaByChunk[reference.ChunkID]
			node := evidenceNode(types.ReadingEvidence{
				KnowledgeID: meta.KnowledgeID, DocumentTitle: meta.DocumentTitle,
				ChunkID: reference.ChunkID, SectionID: meta.SectionID,
				Heading: meta.Heading, Quote: reference.Quote,
			})
			builder.WriteString(fmt.Sprintf("  %s --> %s\n", claimID, node))
		}
	}
	rebuttals := 0
	for _, contradiction := range run.Contradictions {
		rebuttals++
		if rebuttals > 8 {
			break
		}
		rebuttalID := fmt.Sprintf("R%d", rebuttals)
		label := readingMermaidLabel(contradiction.Description, 100)
		if strings.TrimSpace(contradiction.Resolution) != "" {
			label += "<br/>处理：" + readingMermaidLabel(contradiction.Resolution, 60)
		}
		class := "rebuttal"
		if contradiction.Status == "resolved" {
			class = "resolved"
		}
		builder.WriteString(fmt.Sprintf("  %s{\"%s\"}:::%s\n", rebuttalID, label, class))
		for _, evidence := range evidenceFromJSON(contradiction.EvidenceA) {
			node := evidenceNode(evidence)
			builder.WriteString(fmt.Sprintf("  %s --> %s\n", node, rebuttalID))
		}
		for _, evidence := range evidenceFromJSON(contradiction.EvidenceB) {
			node := evidenceNode(evidence)
			builder.WriteString(fmt.Sprintf("  %s --> %s\n", node, rebuttalID))
		}
	}
	if claims == 0 && rebuttals == 0 {
		return "", nil
	}
	return builder.String(), evidenceList
}

func buildReadingReaderLayerMarkdown(run *types.ReadingRun, plan types.ReadingPlan) string {
	if len(run.Annotations) == 0 && len(run.FeynmanAttempts) == 0 {
		return ""
	}
	sectionTitles := make(map[string]string, len(plan.Sections))
	for _, section := range plan.Sections {
		title := strings.TrimSpace(section.Title)
		if title == "" {
			title = section.ID
		}
		sectionTitles[section.ID] = title
	}
	var builder strings.Builder
	if len(run.Annotations) > 0 {
		builder.WriteString("\n\n## 原文批注\n\n")
		for _, annotation := range run.Annotations {
			title := sectionTitles[annotation.SectionID]
			if title == "" {
				title = annotation.SectionID
			}
			builder.WriteString(fmt.Sprintf("- **%s** 「%s」", title, readingTruncateRunes(annotation.Quote, 160)))
			if strings.TrimSpace(annotation.Comment) != "" {
				builder.WriteString(" — " + readingTruncateRunes(annotation.Comment, 300))
			}
			builder.WriteString("\n")
		}
	}
	if len(run.FeynmanAttempts) > 0 {
		builder.WriteString("\n\n## 费曼自述\n\n")
		for _, attempt := range run.FeynmanAttempts {
			title := sectionTitles[attempt.SectionID]
			if title == "" {
				title = attempt.SectionID
			}
			builder.WriteString(fmt.Sprintf("- **%s** %d/100（覆盖 %d、准确 %d、证据 %d）",
				title, attempt.Score, attempt.Coverage, attempt.Accuracy, attempt.EvidenceScore))
			if strings.TrimSpace(attempt.Summary) != "" {
				builder.WriteString(" — " + readingTruncateRunes(attempt.Summary, 300))
			}
			builder.WriteString("\n")
			for _, text := range readingFeynmanPointTexts(attempt.Omissions) {
				builder.WriteString("  - 遗漏：" + text + "\n")
			}
			for _, text := range readingFeynmanPointTexts(attempt.Errors) {
				builder.WriteString("  - 错误：" + text + "\n")
			}
			for _, item := range parseReadingFeynmanAngleFeedback(attempt.AngleFeedback) {
				builder.WriteString(fmt.Sprintf("  - %s：%s", readingFeynmanAngleLabel(item.AngleID), item.Status))
				if strings.TrimSpace(item.Summary) != "" {
					builder.WriteString(" — " + readingTruncateRunes(item.Summary, 200))
				}
				builder.WriteString("\n")
			}
		}
	}
	return builder.String()
}
