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

type readingFeynmanResponse struct {
	Score         int                           `json:"score"`
	Coverage      int                           `json:"coverage"`
	Accuracy      int                           `json:"accuracy"`
	EvidenceScore int                           `json:"evidence_score"`
	Summary       string                        `json:"summary"`
	Omissions     []readingFeynmanPointResponse `json:"omissions"`
	Errors        []readingFeynmanPointResponse `json:"errors"`
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
	text := strings.TrimSpace(req.Text)
	if len([]rune(text)) < 20 {
		return nil, werrors.NewValidationError("复述内容过短，请至少写 20 个字")
	}
	if len([]rune(text)) > 4000 {
		return nil, werrors.NewValidationError("复述内容最多 4000 字")
	}
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return nil, werrors.NewInternalServerError("阅读计划损坏")
	}
	section := findReadingSection(&plan, req.SectionID)
	if section == nil {
		return nil, werrors.NewValidationError("章节不存在")
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
	msgs := buildFeynmanMessages(s, ctx, run, &plan, section, text)
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
	evidence := append(omissionEvidence, errorEvidence...)
	omissionJSON, _ := json.Marshal(omissions)
	errorJSON, _ := json.Marshal(errorsFound)
	evidenceJSON, _ := json.Marshal(evidence)
	attempt := &types.ReadingFeynmanAttempt{
		ID: uuid.NewString(), RunID: run.ID, TenantID: run.TenantID,
		SectionID: section.ID, Text: text,
		Score: clampReadingScore(parsed.Score), Coverage: clampReadingScore(parsed.Coverage),
		Accuracy: clampReadingScore(parsed.Accuracy), EvidenceScore: clampReadingScore(parsed.EvidenceScore),
		Summary:   strings.TrimSpace(parsed.Summary),
		Omissions: types.JSON(omissionJSON), Errors: types.JSON(errorJSON),
		Evidence: types.JSON(evidenceJSON), CreatedAt: time.Now(),
	}
	if err := s.repo.CreateFeynmanAttempt(ctx, attempt); err != nil {
		return nil, err
	}
	return attempt, nil
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
	plan *types.ReadingPlan, section *types.ReadingSection, retelling string,
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
	user := fmt.Sprintf(`阅读目标：%s
阅读深度：%s
当前章节：%s · %s

本章原文：
%s

用户复述：
%s

请只依据上面的原文评估这段复述。返回严格 JSON（不要 Markdown、不要解释）：
{
  "score": 0,
  "coverage": 0,
  "accuracy": 0,
  "evidence_score": 0,
  "summary": "2-3 句总体评价",
  "omissions": [{"text": "遗漏的关键点", "evidence": [{"chunk_id": "chunk id", "quote": "原文连续片段"}]}],
  "errors": [{"text": "错误或与原文不一致之处", "evidence": [{"chunk_id": "chunk id", "quote": "原文连续片段"}]}]
}

规则：
- 四个分数字段为 0-100；score 是总体分数，coverage/accuracy/evidence_score 分别衡量覆盖、准确与证据意识。
- 只判断复述中出现的说法与本章原文；不得引入本章之外的资料。
- 每个 omission / error 必须给出本章原文中可验证的连续 quote 与 chunk_id；无法引用原文的点不要输出。
- 复述正确且完整时，omissions 与 errors 可以是空数组。
- 不要复述用户原文，不要输出 JSON 之外的内容。
`, readingGoal(run), readingDepthLabel(plan.Depth), section.Title, section.Heading, material.String(), retelling)
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
		}
	}
	return builder.String()
}
