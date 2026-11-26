package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	werrors "gaze/internal/errors"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

type reviewService struct {
	repo              interfaces.ReviewRepository
	chunkRepo         interfaces.ChunkRepository
	knowledgeRepo     interfaces.KnowledgeRepository
	knowledgeResolver reviewKnowledgeResolver
	modelService      interfaces.ModelService

	rootCauseMu    sync.Mutex
	rootCauseCache map[string]reviewRootCauseCacheEntry
}

type reviewKnowledgeResolverAdapter struct {
	knowledge     interfaces.KnowledgeRepository
	knowledgeBase interfaces.KnowledgeBaseRepository
}

func (a *reviewKnowledgeResolverAdapter) GetKnowledgeBatch(
	ctx context.Context, tenantID uint64, ids []string,
) ([]*types.Knowledge, error) {
	if a.knowledge == nil {
		return nil, nil
	}
	return a.knowledge.GetKnowledgeBatch(ctx, tenantID, ids)
}

func (a *reviewKnowledgeResolverAdapter) GetKnowledgeBaseByIDs(
	ctx context.Context, ids []string,
) ([]*types.KnowledgeBase, error) {
	if a.knowledgeBase == nil {
		return nil, nil
	}
	return a.knowledgeBase.GetKnowledgeBaseByIDs(ctx, ids)
}

// reviewKnowledgeResolver is the narrow slice of the knowledge repository the
// vulnerability view needs: document titles and knowledge-base names.
type reviewKnowledgeResolver interface {
	GetKnowledgeBatch(ctx context.Context, tenantID uint64, ids []string) ([]*types.Knowledge, error)
	GetKnowledgeBaseByIDs(ctx context.Context, ids []string) ([]*types.KnowledgeBase, error)
}

func NewReviewService(repo interfaces.ReviewRepository) interfaces.ReviewService {
	return &reviewService{repo: repo}
}

func NewReviewServiceWithGeneration(
	repo interfaces.ReviewRepository,
	chunkRepo interfaces.ChunkRepository,
	knowledgeRepo interfaces.KnowledgeRepository,
	knowledgeBaseRepo interfaces.KnowledgeBaseRepository,
	modelService interfaces.ModelService,
) interfaces.ReviewService {
	service := &reviewService{
		repo:          repo,
		chunkRepo:     chunkRepo,
		knowledgeRepo: knowledgeRepo,
		modelService:  modelService,
	}
	if knowledgeRepo != nil || knowledgeBaseRepo != nil {
		service.knowledgeResolver = &reviewKnowledgeResolverAdapter{
			knowledge:     knowledgeRepo,
			knowledgeBase: knowledgeBaseRepo,
		}
	}
	return service
}

func (s *reviewService) EnsureDefaultGroup(ctx context.Context) (*types.ReviewGroup, error) {
	return s.repo.EnsureDefaultGroup(ctx, types.MustTenantIDFromContext(ctx))
}

func (s *reviewService) ListGroupTree(ctx context.Context) ([]*types.ReviewGroupNode, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	if _, err := s.repo.EnsureDefaultGroup(ctx, tenantID); err != nil {
		return nil, err
	}
	groups, err := s.repo.ListGroups(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	nodes := make(map[string]*types.ReviewGroupNode, len(groups))
	var roots []*types.ReviewGroupNode
	for _, group := range groups {
		nodes[group.ID] = &types.ReviewGroupNode{Group: group, Children: []*types.ReviewGroupNode{}}
	}
	for _, group := range groups {
		node := nodes[group.ID]
		if group.ParentID != "" {
			if parent, ok := nodes[group.ParentID]; ok {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		roots = append(roots, node)
	}
	cards, err := s.repo.ListAllCards(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	direct := make(map[string]int, len(groups))
	directDue := make(map[string]int, len(groups))
	for _, card := range cards {
		if card != nil && card.GroupID != "" {
			direct[card.GroupID]++
			if card.Status == types.ReviewStatusActive && card.ReviewCount > 0 && !card.DueAt.After(now) {
				directDue[card.GroupID]++
			}
		}
	}
	var fill func(node *types.ReviewGroupNode) (int, int)
	fill = func(node *types.ReviewGroupNode) (int, int) {
		count := direct[node.Group.ID]
		due := directDue[node.Group.ID]
		for _, child := range node.Children {
			childCount, childDue := fill(child)
			count += childCount
			due += childDue
		}
		node.CardCount = count
		node.DueCount = due
		return count, due
	}
	for _, root := range roots {
		fill(root)
	}
	return roots, nil
}

func (s *reviewService) CreateGroup(
	ctx context.Context, req *types.CreateReviewGroupRequest,
) (*types.ReviewGroup, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 200 {
		return nil, werrors.NewValidationError("卡组名称不能为空且不能超过 200 字")
	}
	if req.ParentID != "" {
		parent, err := s.repo.GetGroup(ctx, tenantID, req.ParentID)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			return nil, werrors.NewValidationError("父卡组不存在")
		}
	}
	groups, err := s.repo.ListGroups(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, existing := range groups {
		if existing.ParentID == req.ParentID && strings.EqualFold(strings.TrimSpace(existing.Name), name) {
			return nil, werrors.NewConflictError("同一父卡组下已存在同名卡组")
		}
	}
	group := &types.ReviewGroup{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		ParentID:  req.ParentID,
		Name:      name,
		SortOrder: req.SortOrder,
	}
	if err := s.repo.CreateGroup(ctx, group); err != nil {
		return nil, err
	}
	return group, nil
}

func (s *reviewService) UpdateGroup(
	ctx context.Context, id string, req *types.UpdateReviewGroupRequest,
) (*types.ReviewGroup, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	group, err := s.repo.GetGroup(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, werrors.NewNotFoundError("卡组不存在")
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len(name) > 200 {
			return nil, werrors.NewValidationError("卡组名称不能为空且不能超过 200 字")
		}
		groups, err := s.repo.ListGroups(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		for _, existing := range groups {
			if existing.ID != group.ID && existing.ParentID == group.ParentID &&
				strings.EqualFold(strings.TrimSpace(existing.Name), name) {
				return nil, werrors.NewConflictError("同一父卡组下已存在同名卡组")
			}
		}
		group.Name = name
	}
	if req.SortOrder != nil {
		group.SortOrder = *req.SortOrder
	}
	if err := s.repo.UpdateGroup(ctx, group); err != nil {
		return nil, err
	}
	return group, nil
}

func (s *reviewService) MoveGroup(ctx context.Context, id, newParentID string) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	group, err := s.repo.GetGroup(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if group == nil {
		return werrors.NewNotFoundError("卡组不存在")
	}
	if group.IsDefault {
		return werrors.NewConflictError("默认卡组不能移动")
	}
	if id == newParentID {
		return werrors.NewValidationError("卡组不能移动到自身")
	}
	groups, err := s.repo.ListGroups(ctx, tenantID)
	if err != nil {
		return err
	}
	byID := make(map[string]*types.ReviewGroup, len(groups))
	for _, item := range groups {
		byID[item.ID] = item
	}
	if newParentID != "" {
		if _, ok := byID[newParentID]; !ok {
			return werrors.NewValidationError("目标父卡组不存在")
		}
		for cursor := newParentID; cursor != ""; {
			if cursor == id {
				return werrors.NewValidationError("卡组不能移动到自己的子卡组")
			}
			parent, ok := byID[cursor]
			if !ok {
				break
			}
			cursor = parent.ParentID
		}
	}
	group.ParentID = newParentID
	return s.repo.UpdateGroup(ctx, group)
}

func (s *reviewService) DeleteGroup(ctx context.Context, id string, deleteCards bool) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	group, err := s.repo.GetGroup(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if group == nil {
		return werrors.NewNotFoundError("卡组不存在")
	}
	if group.IsDefault {
		return werrors.NewConflictError("默认卡组不能删除")
	}
	return s.repo.DeleteGroup(ctx, tenantID, id, deleteCards)
}

func (s *reviewService) CreateCard(
	ctx context.Context, req *types.CreateReviewCardRequest,
) (*types.ReviewCard, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	question := strings.TrimSpace(req.Question)
	answer := strings.TrimSpace(req.Answer)
	if question == "" || answer == "" {
		return nil, werrors.NewValidationError("题干和答案不能为空")
	}
	groupID := req.GroupID
	if groupID == "" {
		group, err := s.repo.EnsureDefaultGroup(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		groupID = group.ID
	}
	group, err := s.repo.GetGroup(ctx, tenantID, groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, werrors.NewValidationError("卡组不存在")
	}
	sources, err := buildReviewSources(tenantID, req.Sources)
	if err != nil {
		return nil, err
	}
	tags, err := normalizeReviewTags(req.Tags)
	if err != nil {
		return nil, err
	}
	hash := types.HashReviewQuestion(question)
	existing, err := s.repo.FindCardByQuestionHash(ctx, tenantID, hash)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err := s.absorbReviewSources(ctx, tenantID, existing, sources); err != nil {
			return nil, err
		}
		if existing.Status != types.ReviewStatusActive {
			if err := s.repo.SetCardStatus(ctx, tenantID, existing.ID, types.ReviewStatusActive); err != nil {
				return nil, err
			}
		}
		return s.repo.GetCard(ctx, tenantID, existing.ID)
	}
	card := &types.ReviewCard{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		GroupID:      groupID,
		CardType:     types.ReviewCardTypeQA,
		Question:     question,
		Answer:       answer,
		Explanation:  strings.TrimSpace(req.Explanation),
		Tags:         reviewTagsJSON(tags),
		Status:       types.ReviewStatusActive,
		Stability:    ReviewInitialStability,
		DueAt:        time.Now(),
		QuestionHash: hash,
	}
	if err := s.repo.CreateCard(ctx, card, sources); err != nil {
		return nil, err
	}
	return card, nil
}

func (s *reviewService) absorbReviewSources(
	ctx context.Context, tenantID uint64, card *types.ReviewCard, sources []*types.ReviewCardSource,
) error {
	existing, err := s.repo.ListSources(ctx, tenantID, card.ID)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(existing))
	for _, source := range existing {
		seen[reviewSourceKey(source)] = true
	}
	var fresh []*types.ReviewCardSource
	for _, source := range sources {
		key := reviewSourceKey(source)
		if seen[key] {
			continue
		}
		seen[key] = true
		fresh = append(fresh, source)
	}
	return s.repo.AddSources(ctx, tenantID, card.ID, fresh)
}

func (s *reviewService) GetCard(ctx context.Context, id string) (*types.ReviewCardDetail, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	detail, err := s.repo.GetCardWithSources(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, werrors.NewNotFoundError("复习卡不存在")
	}
	logs, err := s.repo.ListLogsByCardIDs(ctx, tenantID, []string{detail.Card.ID})
	if err != nil {
		return nil, err
	}
	attachReviewCardLevels(
		[]*types.ReviewCard{detail.Card}, groupReviewLogsByCard(logs), time.Now(),
	)
	return detail, nil
}

func (s *reviewService) ListCards(
	ctx context.Context, filter *types.ReviewCardFilter,
) ([]*types.ReviewCard, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	if err := s.resumeExpiredSuspensions(ctx, tenantID); err != nil {
		return nil, err
	}
	if filter == nil {
		filter = &types.ReviewCardFilter{}
	}
	if len(filter.Statuses) == 0 {
		filter.Statuses = []string{
			types.ReviewStatusActive, types.ReviewStatusSuspended, types.ReviewStatusOrphaned,
		}
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.GroupID != "" {
		groupIDs, err := s.groupSubtree(ctx, tenantID, filter.GroupID)
		if err != nil {
			return nil, err
		}
		if filter.IncludeSubtree {
			filter.GroupIDs = groupIDs
		} else {
			filter.GroupIDs = []string{filter.GroupID}
		}
	}
	cards, err := s.repo.ListCards(ctx, tenantID, filter)
	if err != nil {
		return nil, err
	}
	if len(cards) == 0 {
		return cards, nil
	}
	ids := make([]string, 0, len(cards))
	for _, card := range cards {
		ids = append(ids, card.ID)
	}
	logs, err := s.repo.ListLogsByCardIDs(ctx, tenantID, ids)
	if err != nil {
		return nil, err
	}
	attachReviewCardLevels(cards, groupReviewLogsByCard(logs), time.Now())
	return cards, nil
}

func (s *reviewService) UpdateCard(
	ctx context.Context, id string, req *types.UpdateReviewCardRequest,
) (*types.ReviewCard, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	card, err := s.repo.GetCard(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, werrors.NewNotFoundError("复习卡不存在")
	}
	if req.Question != nil {
		question := strings.TrimSpace(*req.Question)
		if question == "" {
			return nil, werrors.NewValidationError("题干不能为空")
		}
		card.Question = question
		hash := types.HashReviewQuestion(question)
		if hash != card.QuestionHash {
			other, err := s.repo.FindCardByQuestionHash(ctx, tenantID, hash)
			if err != nil {
				return nil, err
			}
			if other != nil && other.ID != card.ID {
				return nil, werrors.NewConflictError("已存在相同知识点的复习卡")
			}
			card.QuestionHash = hash
		}
	}
	if req.Answer != nil {
		answer := strings.TrimSpace(*req.Answer)
		if answer == "" {
			return nil, werrors.NewValidationError("答案不能为空")
		}
		card.Answer = answer
	}
	if req.Explanation != nil {
		card.Explanation = strings.TrimSpace(*req.Explanation)
	}
	if req.Tags != nil {
		tags, err := normalizeReviewTags(req.Tags)
		if err != nil {
			return nil, err
		}
		card.Tags = reviewTagsJSON(tags)
	}
	if err := s.repo.UpdateCard(ctx, card); err != nil {
		return nil, err
	}
	return card, nil
}

func (s *reviewService) MoveCard(ctx context.Context, id, groupID string) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	card, err := s.repo.GetCard(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if card == nil {
		return werrors.NewNotFoundError("复习卡不存在")
	}
	group, err := s.repo.GetGroup(ctx, tenantID, groupID)
	if err != nil {
		return err
	}
	if group == nil {
		return werrors.NewValidationError("卡组不存在")
	}
	return s.repo.MoveCard(ctx, tenantID, id, groupID)
}

func (s *reviewService) SetCardStatus(ctx context.Context, id, status string) error {
	if status != types.ReviewStatusActive && status != types.ReviewStatusSuspended {
		return werrors.NewValidationError("不支持的卡片状态")
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	card, err := s.repo.GetCard(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if card == nil {
		return werrors.NewNotFoundError("复习卡不存在")
	}
	if card.Status == types.ReviewStatusOrphaned {
		return werrors.NewConflictError("孤儿卡请先重绑来源")
	}
	return s.repo.SetCardStatus(ctx, tenantID, id, status)
}

func (s *reviewService) SetCardStatusUntil(
	ctx context.Context, id, status string, until *time.Time,
) error {
	if status != types.ReviewStatusSuspended || until == nil {
		return werrors.NewValidationError("暂停日期不能为空")
	}
	if !until.After(time.Now()) {
		return werrors.NewValidationError("暂停日期需晚于当前时间")
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	card, err := s.repo.GetCard(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if card == nil {
		return werrors.NewNotFoundError("复习卡不存在")
	}
	if card.Status == types.ReviewStatusOrphaned {
		return werrors.NewConflictError("孤儿卡请先重绑来源")
	}
	return s.repo.SuspendCardUntil(ctx, tenantID, id, until)
}

func (s *reviewService) resumeExpiredSuspensions(ctx context.Context, tenantID uint64) error {
	return s.repo.ResumeExpiredSuspensions(ctx, tenantID)
}

func (s *reviewService) DeleteCard(ctx context.Context, id string) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	return s.repo.SoftDeleteCard(ctx, tenantID, id)
}

func (s *reviewService) RestoreCard(ctx context.Context, id string) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	if err := s.repo.RestoreCard(ctx, tenantID, id); err != nil {
		return err
	}
	active, err := s.repo.CountActiveKnowledgeSources(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if active == 0 {
		return s.repo.SetCardStatus(ctx, tenantID, id, types.ReviewStatusOrphaned)
	}
	return nil
}

func (s *reviewService) TodayQueue(
	ctx context.Context, groupID string, limit int,
) (*types.ReviewQueue, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	if err := s.resumeExpiredSuspensions(ctx, tenantID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > ReviewDailyTotalLimit {
		limit = ReviewDailyTotalLimit
	}
	now := time.Now()
	groupIDs, err := s.groupSubtree(ctx, tenantID, groupID)
	if err != nil {
		return nil, err
	}
	completed, err := s.repo.CountCompletedToday(ctx, tenantID, ReviewDayStart(now), ReviewDayEnd(now))
	if err != nil {
		return nil, err
	}
	remaining := limit - int(completed)
	logs, err := s.repo.ListAllLogs(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	queue := &types.ReviewQueue{
		Due:            []*types.ReviewCard{},
		New:            []*types.ReviewCard{},
		CompletedToday: int(completed),
		Limit:          limit,
		NewLimit:       ReviewDailyNewCardLimit,
		StreakDays:     reviewStreakDays(logs, ReviewDayStart(now)),
	}
	if remaining <= 0 {
		return queue, nil
	}
	due, err := s.repo.ListDueCards(ctx, tenantID, groupIDs, now, remaining)
	if err != nil {
		return nil, err
	}
	queue.Due = due
	newLimit := remaining - len(due)
	if newLimit > ReviewDailyNewCardLimit {
		newLimit = ReviewDailyNewCardLimit
	}
	if newLimit > 0 {
		fresh, err := s.repo.ListNewCards(ctx, tenantID, groupIDs, newLimit)
		if err != nil {
			return nil, err
		}
		queue.New = fresh
	}
	return queue, nil
}

func (s *reviewService) SubmitReview(
	ctx context.Context, req *types.SubmitReviewRequest,
) (*types.ReviewCard, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	if req.CardID == "" {
		return nil, werrors.NewValidationError("缺少复习卡 ID")
	}
	card, err := s.repo.GetCard(ctx, tenantID, req.CardID)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, werrors.NewNotFoundError("复习卡不存在")
	}
	if card.Status != types.ReviewStatusActive {
		return nil, werrors.NewConflictError("该卡当前不在复习队列中")
	}
	now := time.Now()
	levelBefore := card.ReviewLevel
	stabilityBefore := card.Stability
	if stabilityBefore <= 0 {
		stabilityBefore = ReviewInitialStability
	}
	newStability, newLevel, dueAt, intervalDays, err := ApplyReviewSchedule(
		stabilityBefore, levelBefore, req.Rating, now,
	)
	if err != nil {
		return nil, werrors.NewValidationError(err.Error())
	}
	if req.Rating == types.ReviewRatingForgot {
		card.Lapses++
		card.ForgetStreak++
	} else {
		card.ForgetStreak = 0
	}
	card.ReviewLevel = newLevel
	card.Stability = newStability
	card.DueAt = dueAt
	card.ReviewCount++
	card.LastReviewedAt = &now
	// V3 keeps repeatedly forgotten cards active so the vulnerability view
	// stays honest. Suspension is only ever an explicit user action.
	clientRequestID := strings.TrimSpace(req.ClientRequestID)
	if clientRequestID == "" {
		clientRequestID = uuid.NewString()
	}
	sessionID := strings.TrimSpace(req.SessionID)
	var session *types.ReviewSession
	if sessionID != "" {
		session, err = s.repo.GetSession(ctx, tenantID, sessionID)
		if err != nil {
			return nil, err
		}
		if session == nil {
			return nil, werrors.NewNotFoundError("复习会话不存在")
		}
		if session.Status != types.ReviewSessionStatusActive {
			return nil, werrors.NewConflictError("复习会话已结束")
		}
		if !reviewSessionContains(session, card.ID) {
			return nil, werrors.NewValidationError("卡片不属于本次复习会话")
		}
		if existing, err := s.repo.FindLogBySessionCard(ctx, tenantID, sessionID, card.ID); err != nil {
			return nil, err
		} else if existing != nil {
			return nil, werrors.NewConflictError("该卡片本轮已评分")
		}
	}
	log := &types.ReviewLog{
		ID:              uuid.NewString(),
		TenantID:        tenantID,
		CardID:          card.ID,
		SessionID:       sessionID,
		Rating:          req.Rating,
		LevelBefore:     levelBefore,
		LevelAfter:      newLevel,
		IntervalDays:    intervalDays,
		StabilityBefore: stabilityBefore,
		StabilityAfter:  newStability,
		ReviewedAt:      now,
		ClientRequestID: clientRequestID,
	}
	var applied bool
	if session != nil {
		applied, err = s.repo.ApplyReviewSession(ctx, card, log, session)
	} else {
		applied, err = s.repo.ApplyReview(ctx, card, log)
	}
	if err != nil {
		return nil, err
	}
	if !applied {
		return s.repo.GetCard(ctx, tenantID, card.ID)
	}
	return card, nil
}

func (s *reviewService) MarkSourceUnavailable(ctx context.Context, cardID, sourceID string) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	card, err := s.repo.GetCard(ctx, tenantID, cardID)
	if err != nil {
		return err
	}
	if card == nil {
		return werrors.NewNotFoundError("复习卡不存在")
	}
	if err := s.repo.MarkSourceUnavailable(ctx, tenantID, cardID, sourceID); err != nil {
		return err
	}
	active, err := s.repo.CountActiveKnowledgeSources(ctx, tenantID, cardID)
	if err != nil {
		return err
	}
	if active == 0 {
		return s.repo.SetCardStatus(ctx, tenantID, cardID, types.ReviewStatusOrphaned)
	}
	return nil
}

func (s *reviewService) RebindSource(
	ctx context.Context, cardID, sourceID string, req *types.RebindReviewCardSourceRequest,
) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	card, err := s.repo.GetCard(ctx, tenantID, cardID)
	if err != nil {
		return err
	}
	if card == nil {
		return werrors.NewNotFoundError("复习卡不存在")
	}
	sources, err := s.repo.ListSources(ctx, tenantID, cardID)
	if err != nil {
		return err
	}
	var source *types.ReviewCardSource
	for _, item := range sources {
		if item.ID == sourceID {
			source = item
			break
		}
	}
	if source == nil {
		return werrors.NewNotFoundError("卡片来源不存在")
	}
	if source.SourceType != types.ReviewSourceTypeKnowledge {
		return werrors.NewValidationError("只有知识来源可以重绑")
	}
	if strings.TrimSpace(req.KnowledgeID) == "" {
		return werrors.NewValidationError("重绑来源缺少 knowledge_id")
	}
	source.KnowledgeID = strings.TrimSpace(req.KnowledgeID)
	source.ChunkID = strings.TrimSpace(req.ChunkID)
	source.Path = strings.TrimSpace(req.Path)
	source.Symbol = strings.TrimSpace(req.Symbol)
	source.StartLine = req.StartLine
	source.EndLine = req.EndLine
	source.ContentHash = strings.TrimSpace(req.ContentHash)
	source.Excerpt = types.CleanReviewExcerpt(req.Excerpt)
	if err := s.repo.UpdateSource(ctx, tenantID, source); err != nil {
		return err
	}
	if card.Status == types.ReviewStatusOrphaned {
		// Rebound orphans stay paused until the owner explicitly resumes
		// them, so a source fix never silently re-enters the queue.
		return s.repo.SetCardStatus(ctx, tenantID, cardID, types.ReviewStatusSuspended)
	}
	return nil
}

func (s *reviewService) MergeCards(ctx context.Context, req *types.MergeReviewCardsRequest) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	if req == nil || strings.TrimSpace(req.TargetCardID) == "" || len(req.SourceCardIDs) == 0 {
		return werrors.NewValidationError("缺少合并目标或来源卡片")
	}
	targetID := strings.TrimSpace(req.TargetCardID)
	target, err := s.repo.GetCard(ctx, tenantID, targetID)
	if err != nil {
		return err
	}
	if target == nil {
		return werrors.NewNotFoundError("目标复习卡不存在")
	}
	sourceIDs := make([]string, 0, len(req.SourceCardIDs))
	seen := make(map[string]bool, len(req.SourceCardIDs))
	for _, sourceID := range req.SourceCardIDs {
		sourceID = strings.TrimSpace(sourceID)
		if sourceID == "" || sourceID == targetID || seen[sourceID] {
			continue
		}
		seen[sourceID] = true
		sourceIDs = append(sourceIDs, sourceID)
	}
	if len(sourceIDs) == 0 {
		return werrors.NewValidationError("没有可合并的其他卡片")
	}
	if err := s.repo.MergeCards(ctx, tenantID, targetID, sourceIDs); err != nil {
		return err
	}
	if target.Status == types.ReviewStatusOrphaned {
		if active, err := s.repo.CountActiveKnowledgeSources(ctx, tenantID, targetID); err == nil && active > 0 {
			return s.repo.SetCardStatus(ctx, tenantID, targetID, types.ReviewStatusSuspended)
		}
	}
	return nil
}

func (s *reviewService) Export(ctx context.Context) (*types.ReviewExport, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	if _, err := s.repo.EnsureDefaultGroup(ctx, tenantID); err != nil {
		return nil, err
	}
	groups, err := s.repo.ListGroups(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	cards, err := s.repo.ListAllCards(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	sources, err := s.repo.ListAllSources(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	logs, err := s.repo.ListAllLogs(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	sessions, err := s.repo.ListSessions(ctx, tenantID, 10000)
	if err != nil {
		return nil, err
	}
	for _, source := range sources {
		source.Excerpt = types.CleanReviewExcerpt(source.Excerpt)
	}
	return &types.ReviewExport{
		ExportedAt: time.Now(),
		Groups:     groups,
		Cards:      cards,
		Sources:    sources,
		Logs:       logs,
		Sessions:   sessions,
	}, nil
}

func reviewSessionContains(session *types.ReviewSession, cardID string) bool {
	for _, id := range reviewSessionCardIDs(session) {
		if id == cardID {
			return true
		}
	}
	return false
}

func (s *reviewService) groupSubtree(
	ctx context.Context, tenantID uint64, rootID string,
) ([]string, error) {
	if rootID == "" {
		return nil, nil
	}
	groups, err := s.repo.ListGroups(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	children := make(map[string][]string, len(groups))
	found := false
	for _, group := range groups {
		if group.ID == rootID {
			found = true
		}
		if group.ParentID != "" {
			children[group.ParentID] = append(children[group.ParentID], group.ID)
		}
	}
	if !found {
		return nil, werrors.NewValidationError("卡组不存在")
	}
	ids := []string{rootID}
	for i := 0; i < len(ids); i++ {
		ids = append(ids, children[ids[i]]...)
	}
	return ids, nil
}

func buildReviewSources(
	tenantID uint64, inputs []types.ReviewCardSourceInput,
) ([]*types.ReviewCardSource, error) {
	if len(inputs) == 0 {
		return nil, werrors.NewValidationError("每张复习卡至少需要一条知识来源")
	}
	seen := make(map[string]bool, len(inputs))
	sources := make([]*types.ReviewCardSource, 0, len(inputs))
	hasKnowledge := false
	for _, input := range inputs {
		source := &types.ReviewCardSource{
			ID:          uuid.NewString(),
			TenantID:    tenantID,
			SourceType:  strings.TrimSpace(input.SourceType),
			KnowledgeID: strings.TrimSpace(input.KnowledgeID),
			ChunkID:     strings.TrimSpace(input.ChunkID),
			MessageID:   strings.TrimSpace(input.MessageID),
			Path:        strings.TrimSpace(input.Path),
			Symbol:      strings.TrimSpace(input.Symbol),
			StartLine:   input.StartLine,
			EndLine:     input.EndLine,
			ContentHash: strings.TrimSpace(input.ContentHash),
			Excerpt:     types.CleanReviewExcerpt(input.Excerpt),
			IsPrimary:   input.IsPrimary,
			Status:      types.ReviewSourceStatusActive,
		}
		if source.SourceType == "" {
			if source.KnowledgeID != "" {
				source.SourceType = types.ReviewSourceTypeKnowledge
			} else {
				source.SourceType = types.ReviewSourceTypeMessage
			}
		}
		switch source.SourceType {
		case types.ReviewSourceTypeKnowledge:
			if source.KnowledgeID == "" {
				return nil, werrors.NewValidationError("知识来源缺少 knowledge_id")
			}
			hasKnowledge = true
		case types.ReviewSourceTypeMessage:
			if source.MessageID == "" {
				return nil, werrors.NewValidationError("会话来源缺少 message_id")
			}
		default:
			return nil, werrors.NewValidationError("不支持的来源类型")
		}
		key := reviewSourceKey(source)
		if seen[key] {
			continue
		}
		seen[key] = true
		sources = append(sources, source)
	}
	if !hasKnowledge {
		return nil, werrors.NewValidationError("复习卡必须至少有一条知识来源")
	}
	primarySet := false
	for _, source := range sources {
		if source.IsPrimary && !primarySet {
			primarySet = true
			continue
		}
		if source.IsPrimary {
			source.IsPrimary = false
		}
	}
	if !primarySet && len(sources) > 0 {
		sources[0].IsPrimary = true
	}
	return sources, nil
}

func reviewSourceKey(source *types.ReviewCardSource) string {
	return strings.Join([]string{
		source.SourceType, source.KnowledgeID, source.ChunkID, source.MessageID,
		source.Path, source.Symbol,
	}, "\x00")
}

func (s *reviewService) GetCardPerformance(
	ctx context.Context, cardID string,
) (*types.ReviewCardPerformance, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	card, err := s.repo.GetCard(ctx, tenantID, cardID)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, werrors.NewNotFoundError("复习卡不存在")
	}
	logs, err := s.repo.ListLogsByCard(ctx, tenantID, cardID, 100)
	if err != nil {
		return nil, err
	}
	chronological := make([]*types.ReviewLog, len(logs))
	for i, log := range logs {
		chronological[len(logs)-1-i] = log
	}
	level, reason := reviewCardLevelDetail(card, chronological, time.Now())
	card.Vulnerability = level
	card.VulnerabilityReason = reason
	performance := &types.ReviewCardPerformance{
		Card:         card,
		ForgetStreak: card.ForgetStreak,
		Logs:         logs,
		Warning:      "none",
	}
	recentLogs := logs
	if len(recentLogs) > 5 {
		recentLogs = recentLogs[:5]
	}
	for _, log := range logs {
		performance.Counts.Total++
		switch log.Rating {
		case types.ReviewRatingEasy:
			performance.Counts.Easy++
		case types.ReviewRatingRemembered:
			performance.Counts.Remembered++
		case types.ReviewRatingFuzzy:
			performance.Counts.Fuzzy++
		case types.ReviewRatingForgot:
			performance.Counts.Forgot++
		}
	}
	for _, log := range recentLogs {
		performance.RecentCounts.Total++
		switch log.Rating {
		case types.ReviewRatingEasy:
			performance.RecentCounts.Easy++
		case types.ReviewRatingRemembered:
			performance.RecentCounts.Remembered++
		case types.ReviewRatingFuzzy:
			performance.RecentCounts.Fuzzy++
		case types.ReviewRatingForgot:
			performance.RecentCounts.Forgot++
		}
	}
	if performance.RecentCounts.Total > 0 {
		performance.RecentForgetRate = float64(performance.RecentCounts.Forgot) / float64(performance.RecentCounts.Total)
	}
	switch card.Vulnerability {
	case types.ReviewCardLevelSevere:
		performance.Warning = "high"
		performance.WarningReason = reason
		if performance.WarningReason == "" {
			performance.WarningReason = "这张卡近期反复忘记，建议重学并回原文巩固"
		}
	case types.ReviewCardLevelWarning:
		performance.Warning = "watch"
		performance.WarningReason = reason
		if performance.WarningReason == "" {
			performance.WarningReason = "这张卡近期表现不稳，建议回原文复习"
		}
	case types.ReviewCardLevelObserve:
		performance.Warning = "watch"
		performance.WarningReason = reason
		if performance.WarningReason == "" {
			performance.WarningReason = "这张卡最近想得慢或已逾期，暂时继续观察"
		}
	}
	return performance, nil
}
