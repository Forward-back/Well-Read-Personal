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

type reviewRootCauseCacheEntry struct {
	result  *types.ReviewRootCause
	expires time.Time
}

func (s *reviewService) PreviewCardSplit(
	ctx context.Context, cardID string, count int,
) (*types.ReviewGenerationPreview, error) {
	if count <= 0 {
		count = 2
	}
	if count > 4 {
		count = 4
	}
	return s.previewCardVariants(ctx, cardID, count, "split")
}

func (s *reviewService) PreviewCardAngles(
	ctx context.Context, cardID string, count int,
) (*types.ReviewGenerationPreview, error) {
	if count <= 0 {
		count = 1
	}
	if count > 3 {
		count = 3
	}
	return s.previewCardVariants(ctx, cardID, count, "angles")
}

func (s *reviewService) previewCardVariants(
	ctx context.Context, cardID string, count int, mode string,
) (*types.ReviewGenerationPreview, error) {
	if s.modelService == nil {
		return nil, werrors.NewInternalServerError("复习卡生成服务未配置")
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	card, sources, sampled, err := s.generationCardSources(ctx, tenantID, cardID)
	if err != nil {
		return nil, err
	}
	if len(sampled) == 0 {
		return nil, werrors.NewValidationError("这张卡没有可用的知识来源文本")
	}
	modelID, err := s.generationModelID(ctx)
	if err != nil {
		return nil, err
	}
	chatModel, err := s.modelService.GetChatModel(ctx, modelID)
	if err != nil {
		return nil, err
	}
	systemPrompt := "你是复习卡编辑器。只输出 JSON 数组，不要输出 Markdown 代码块或解释。每项格式为 {\"question\":\"...\",\"answer\":\"...\",\"explanation\":\"...\",\"source_index\":1}。题目必须能独立回答；答案一到两句；解析可选。"
	userPrompt := buildCardVariantPrompt(card, sampled, count, mode)
	response, err := chatModel.Chat(ctx, []chat.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, &chat.ChatOptions{Temperature: 0.25, MaxTokens: 1800})
	if err != nil {
		return nil, err
	}
	rawCards, err := parseGeneratedReviewCards(response.Content)
	if err != nil {
		return nil, werrors.NewInternalServerError("模型返回的复习卡不是有效 JSON")
	}
	preview := &types.ReviewGenerationPreview{
		ModelID: modelID,
		Cards:   []*types.GeneratedReviewCard{},
	}
	seen := make(map[string]bool, len(rawCards))
	for _, raw := range rawCards {
		question := strings.TrimSpace(raw.Question)
		answer := strings.TrimSpace(raw.Answer)
		if question == "" || answer == "" {
			continue
		}
		hash := types.HashReviewQuestion(question)
		if seen[hash] {
			continue
		}
		seen[hash] = true
		index := raw.SourceIndex
		if index < 1 || index > len(sampled) {
			index = 1
		}
		source := sampled[index-1]
		excerpt := types.CleanReviewExcerpt(source.content)
		if len([]rune(excerpt)) > 300 {
			excerpt = string([]rune(excerpt)[:300])
		}
		generated := &types.GeneratedReviewCard{
			Question:          question,
			Answer:            answer,
			Explanation:       strings.TrimSpace(raw.Explanation),
			SourceKnowledgeID: source.knowledgeID,
			SourceChunkID:     source.chunkID,
			SourceTitle:       source.title,
			SourceExcerpt:     excerpt,
			Sources: []types.ReviewCardSourceInput{{
				SourceType:  types.ReviewSourceTypeKnowledge,
				KnowledgeID: source.knowledgeID,
				ChunkID:     source.chunkID,
				Excerpt:     excerpt,
			}},
		}
		if existing, err := s.repo.FindCardByQuestionHash(ctx, tenantID, hash); err == nil && existing != nil {
			generated.Duplicate = true
			generated.ExistingCardID = existing.ID
		}
		preview.Cards = append(preview.Cards, generated)
		if len(preview.Cards) >= count {
			break
		}
	}
	if len(preview.Cards) == 0 {
		return nil, werrors.NewValidationError("模型没有生成有效复习卡，请重试")
	}
	_ = sources
	return preview, nil
}

func (s *reviewService) ApplyCardSplit(
	ctx context.Context, cardID string, req *types.ApplyCardSplitRequest,
) (*types.ApplyCardSplitResult, error) {
	if req == nil || len(req.Cards) == 0 {
		return nil, werrors.NewValidationError("没有可拆分的候选卡")
	}
	if len(req.Cards) > 4 {
		return nil, werrors.NewValidationError("一次最多拆出 4 张卡片")
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	card, err := s.repo.GetCard(ctx, tenantID, cardID)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, werrors.NewNotFoundError("复习卡不存在")
	}
	if card.Status != types.ReviewStatusActive {
		return nil, werrors.NewValidationError("只有正常卡片可以拆分")
	}
	sources, err := s.repo.ListSources(ctx, tenantID, cardID)
	if err != nil {
		return nil, err
	}
	sourceInputs := make([]types.ReviewCardSourceInput, 0, len(sources))
	for _, source := range sources {
		if source.SourceType != types.ReviewSourceTypeKnowledge ||
			source.Status != types.ReviewSourceStatusActive {
			continue
		}
		sourceInputs = append(sourceInputs, types.ReviewCardSourceInput{
			SourceType:  source.SourceType,
			KnowledgeID: source.KnowledgeID,
			ChunkID:     source.ChunkID,
			MessageID:   source.MessageID,
			Path:        source.Path,
			Symbol:      source.Symbol,
			StartLine:   source.StartLine,
			EndLine:     source.EndLine,
			ContentHash: source.ContentHash,
			Excerpt:     source.Excerpt,
			IsPrimary:   source.IsPrimary,
		})
	}
	if len(sourceInputs) == 0 {
		return nil, werrors.NewValidationError("这张卡没有可用的知识来源，请先重绑来源")
	}
	tags := reviewTagsFromJSON(card.Tags)
	newCards := make([]*types.ReviewCard, 0, len(req.Cards))
	sourcesByCard := make([][]*types.ReviewCardSource, 0, len(req.Cards))
	seen := make(map[string]bool, len(req.Cards))
	for _, item := range req.Cards {
		question := strings.TrimSpace(item.Question)
		answer := strings.TrimSpace(item.Answer)
		if question == "" || answer == "" {
			return nil, werrors.NewValidationError("拆分卡片的题干和答案不能为空")
		}
		hash := types.HashReviewQuestion(question)
		if seen[hash] {
			return nil, werrors.NewConflictError("拆分结果里存在重复题干")
		}
		seen[hash] = true
		existing, err := s.repo.FindCardByQuestionHash(ctx, tenantID, hash)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, werrors.NewConflictError("拆分结果与已有卡片题干重复")
		}
		builtSources, err := buildReviewSources(tenantID, sourceInputs)
		if err != nil {
			return nil, err
		}
		now := time.Now()
		newCards = append(newCards, &types.ReviewCard{
			ID:           uuid.NewString(),
			TenantID:     tenantID,
			GroupID:      card.GroupID,
			CardType:     types.ReviewCardTypeQA,
			Question:     question,
			Answer:       answer,
			Explanation:  strings.TrimSpace(item.Explanation),
			Tags:         reviewTagsJSON(tags),
			Status:       types.ReviewStatusActive,
			Stability:    ReviewInitialStability,
			DueAt:        now,
			QuestionHash: hash,
		})
		sourcesByCard = append(sourcesByCard, builtSources)
	}
	if err := s.repo.CreateDerivedCardsAndSuspend(
		ctx, tenantID, cardID, newCards, sourcesByCard,
	); err != nil {
		return nil, err
	}
	return &types.ApplyCardSplitResult{Created: newCards, ArchivedOriginal: true}, nil
}

func (s *reviewService) generationCardSources(
	ctx context.Context, tenantID uint64, cardID string,
) (*types.ReviewCard, []*types.ReviewCardSource, []sampledReviewChunk, error) {
	card, err := s.repo.GetCard(ctx, tenantID, cardID)
	if err != nil {
		return nil, nil, nil, err
	}
	if card == nil {
		return nil, nil, nil, werrors.NewNotFoundError("复习卡不存在")
	}
	sources, err := s.repo.ListSources(ctx, tenantID, cardID)
	if err != nil {
		return nil, nil, nil, err
	}
	active := make([]*types.ReviewCardSource, 0, len(sources))
	knowledgeIDs := make(map[string]bool)
	for _, source := range sources {
		if source.SourceType != types.ReviewSourceTypeKnowledge ||
			source.Status != types.ReviewSourceStatusActive {
			continue
		}
		active = append(active, source)
		if source.KnowledgeID != "" {
			knowledgeIDs[source.KnowledgeID] = true
		}
	}
	titles, _ := s.reviewKnowledgeMeta(ctx, tenantID, knowledgeIDs)
	sampled := make([]sampledReviewChunk, 0, len(active))
	for _, source := range active {
		content := strings.TrimSpace(source.Excerpt)
		if s.chunkRepo != nil && source.ChunkID != "" {
			if chunk, err := s.chunkRepo.GetChunkByID(ctx, tenantID, source.ChunkID); err == nil && chunk != nil {
				if value := strings.TrimSpace(chunk.Content); value != "" {
					content = value
				}
			}
		}
		if content == "" {
			continue
		}
		runes := []rune(content)
		if len(runes) > 800 {
			content = string(runes[:800])
		}
		title := titles[source.KnowledgeID]
		if title == "" {
			title = source.Path
		}
		if title == "" {
			title = "原始资料"
		}
		sampled = append(sampled, sampledReviewChunk{
			knowledgeID: source.KnowledgeID,
			chunkID:     source.ChunkID,
			title:       title,
			content:     content,
		})
	}
	return card, active, sampled, nil
}

func buildCardVariantPrompt(
	card *types.ReviewCard, sampled []sampledReviewChunk, count int, mode string,
) string {
	var b strings.Builder
	if mode == "split" {
		fmt.Fprintf(&b, "把下面这张复习卡拆成 %d 张更小、可独立回答的卡片。\n", count)
	} else {
		fmt.Fprintf(&b, "基于下面这张复习卡生成 %d 张换问法的卡片，考察同一知识点，但不要只是改写措辞。\n", count)
	}
	b.WriteString("\n【原卡】\n")
	fmt.Fprintf(&b, "题干：%s\n答案：%s\n", card.Question, card.Answer)
	if strings.TrimSpace(card.Explanation) != "" {
		fmt.Fprintf(&b, "解析：%s\n", card.Explanation)
	}
	b.WriteString("\n【参考资料】\n")
	for i, item := range sampled {
		fmt.Fprintf(&b, "资料%d：%s\n%s\n\n", i+1, item.title, item.content)
	}
	b.WriteString("输出 JSON 数组，每项包含 question、answer、explanation、source_index；source_index 对应上面的资料编号。不要输出其他文字。")
	return b.String()
}

func (s *reviewService) RootCause(
	ctx context.Context, req *types.ReviewRootCauseRequest,
) (*types.ReviewRootCause, error) {
	if req == nil {
		return nil, werrors.NewValidationError("缺少漏洞簇参数")
	}
	detail, err := s.ClusterDetail(ctx, &types.ReviewClusterDetailRequest{
		ScopeGroupID: req.ScopeGroupID,
		RowGroupID:   req.RowGroupID,
		ColumnKey:    req.ColumnKey,
		Direct:       req.Direct,
	})
	if err != nil {
		return nil, err
	}
	fallback := reviewRuleRootCause(detail)
	if s.modelService == nil {
		return fallback, nil
	}
	key := reviewRootCauseCacheKey(types.MustTenantIDFromContext(ctx), detail)
	if cached := s.cachedRootCause(key); cached != nil {
		return cached, nil
	}
	modelID, err := s.generationModelID(ctx)
	if err != nil {
		return fallback, nil
	}
	chatModel, err := s.modelService.GetChatModel(ctx, modelID)
	if err != nil {
		return fallback, nil
	}
	response, err := chatModel.Chat(ctx, []chat.Message{
		{Role: "system", Content: "你是复习漏洞分析助手。只输出 JSON 对象，不要 Markdown 代码块或解释。格式为 {\"summary\":\"...\",\"reasons\":[\"...\"],\"focus_suggestions\":[\"...\"],\"evidence_card_ids\":[\"...\"]}。只依据给定卡片和资料，不要虚构页码或原文。"},
		{Role: "user", Content: buildRootCausePrompt(detail)},
	}, &chat.ChatOptions{Temperature: 0.2, MaxTokens: 900})
	if err != nil {
		return fallback, nil
	}
	parsed, err := parseRootCauseResponse(response.Content)
	if err != nil {
		return fallback, nil
	}
	parsed.ModelID = modelID
	parsed.Source = "llm"
	if parsed.Summary == "" {
		parsed.Summary = fallback.Summary
	}
	if len(parsed.Reasons) == 0 {
		parsed.Reasons = fallback.Reasons
	}
	if len(parsed.FocusSuggestions) == 0 {
		parsed.FocusSuggestions = fallback.FocusSuggestions
	}
	allowedEvidence := make(map[string]bool, len(detail.Cards))
	for _, item := range detail.Cards {
		allowedEvidence[item.Card.ID] = true
	}
	evidence := make([]string, 0, len(parsed.EvidenceCardIDs))
	for _, id := range parsed.EvidenceCardIDs {
		if allowedEvidence[id] {
			evidence = append(evidence, id)
		}
	}
	parsed.EvidenceCardIDs = evidence
	s.storeRootCause(key, parsed)
	return parsed, nil
}

func reviewRuleRootCause(detail *types.ReviewClusterDetail) *types.ReviewRootCause {
	reasons := append([]string{}, detail.Reasons...)
	if len(reasons) == 0 {
		reasons = []string{"这个簇暂无明显记忆漏洞"}
	}
	relearnCount := 0
	evidence := make([]string, 0, len(detail.Cards))
	for _, item := range detail.Cards {
		switch item.Level {
		case types.ReviewCardLevelSevere, types.ReviewCardLevelWarning:
			relearnCount++
			evidence = append(evidence, item.Card.ID)
		case types.ReviewCardLevelObserve:
			evidence = append(evidence, item.Card.ID)
		}
	}
	suggestions := []string{
		fmt.Sprintf("先重学 %d 张严重或警告卡，再回原文核对上下文", relearnCount),
		"对反复忘记的卡片，优先确认题干是否过长或混入了多个知识点",
	}
	summary := "规则判断：" + strings.Join(reasons, "；")
	return &types.ReviewRootCause{
		Summary:          summary,
		Reasons:          reasons,
		FocusSuggestions: suggestions,
		EvidenceCardIDs:  evidence,
		Source:           "rule",
	}
}

func buildRootCausePrompt(detail *types.ReviewClusterDetail) string {
	var b strings.Builder
	fmt.Fprintf(&b, "簇：%s · %s，级别 %s，加权强度 %.0f%%，共 %d 张卡。\n", detail.Row.Path, detail.Column.Title, detail.Cell.Level, detail.Cell.Intensity*100, detail.Cell.Total)
	if len(detail.Reasons) > 0 {
		fmt.Fprintf(&b, "规则原因：%s\n", strings.Join(detail.Reasons, "；"))
	}
	b.WriteString("\n异常卡片：\n")
	for i, item := range detail.Cards {
		if i >= 8 {
			break
		}
		if item.Level == types.ReviewCardLevelHealthy || item.Level == types.ReviewCardLevelNone {
			continue
		}
		fmt.Fprintf(&b, "- id=%s | 级别=%s | 原因=%s | 题干=%s | 答案=%s\n", item.Card.ID, item.Level, item.Card.VulnerabilityReason, item.Card.Question, item.Card.Answer)
		if item.Source != nil {
			if item.Source.Path != "" {
				fmt.Fprintf(&b, "  来源：%s\n", item.Source.Path)
			}
			if item.Source.Excerpt != "" {
				fmt.Fprintf(&b, "  摘录：%s\n", item.Source.Excerpt)
			}
		}
	}
	b.WriteString("\n请给出原因归纳、2-4 条补强建议，并列出支撑判断的 evidence_card_ids。evidence_card_ids 只能使用上面出现的 id。不要虚构页码。")
	return b.String()
}

func parseRootCauseResponse(content string) (*types.ReviewRootCause, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON object in model response")
	}
	var parsed types.ReviewRootCause
	if err := json.Unmarshal([]byte(content[start:end+1]), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func reviewRootCauseCacheKey(tenantID uint64, detail *types.ReviewClusterDetail) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d|%s|%s|%s|%s", tenantID, detail.Row.ID, detail.Column.Key, detail.Cell.Level, detail.Cell.RowID)
	for _, item := range detail.Cards {
		fmt.Fprintf(&b, "|%s:%s", item.Card.ID, item.Level)
	}
	return b.String()
}

func (s *reviewService) cachedRootCause(key string) *types.ReviewRootCause {
	s.rootCauseMu.Lock()
	defer s.rootCauseMu.Unlock()
	entry, ok := s.rootCauseCache[key]
	if !ok || time.Now().After(entry.expires) {
		if ok {
			delete(s.rootCauseCache, key)
		}
		return nil
	}
	return entry.result
}

func (s *reviewService) storeRootCause(key string, result *types.ReviewRootCause) {
	s.rootCauseMu.Lock()
	defer s.rootCauseMu.Unlock()
	if s.rootCauseCache == nil {
		s.rootCauseCache = make(map[string]reviewRootCauseCacheEntry)
	}
	s.rootCauseCache[key] = reviewRootCauseCacheEntry{
		result:  result,
		expires: time.Now().Add(15 * time.Minute),
	}
}
