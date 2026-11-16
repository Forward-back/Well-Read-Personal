package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

type reviewRepository struct{ db *gorm.DB }

func NewReviewRepository(db *gorm.DB) interfaces.ReviewRepository {
	return &reviewRepository{db: db}
}

func (r *reviewRepository) EnsureDefaultGroup(
	ctx context.Context, tenantID uint64,
) (*types.ReviewGroup, error) {
	var group types.ReviewGroup
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND is_default = ?", tenantID, true).
		First(&group).Error
	if err == nil {
		return &group, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	group = types.ReviewGroup{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		Name:      types.ReviewGroupDefaultName,
		IsDefault: true,
	}
	if err := r.db.WithContext(ctx).Create(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *reviewRepository) ListGroups(
	ctx context.Context, tenantID uint64,
) ([]*types.ReviewGroup, error) {
	var groups []*types.ReviewGroup
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("sort_order ASC, created_at ASC").
		Find(&groups).Error
	return groups, err
}

func (r *reviewRepository) GetGroup(
	ctx context.Context, tenantID uint64, id string,
) (*types.ReviewGroup, error) {
	var group types.ReviewGroup
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&group).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *reviewRepository) CreateGroup(ctx context.Context, group *types.ReviewGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *reviewRepository) UpdateGroup(ctx context.Context, group *types.ReviewGroup) error {
	return r.db.WithContext(ctx).
		Model(&types.ReviewGroup{}).
		Where("tenant_id = ? AND id = ?", group.TenantID, group.ID).
		Updates(map[string]any{
			"parent_id":  group.ParentID,
			"name":       group.Name,
			"sort_order": group.SortOrder,
			"updated_at": time.Now(),
		}).Error
}

func (r *reviewRepository) DeleteGroup(
	ctx context.Context, tenantID uint64, id string, deleteCards bool,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var group types.ReviewGroup
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).First(&group).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		childParent := group.ParentID
		cardTarget := group.ParentID
		if cardTarget == "" {
			var defaultGroup types.ReviewGroup
			if err := tx.Where("tenant_id = ? AND is_default = ?", tenantID, true).
				First(&defaultGroup).Error; err != nil {
				return err
			}
			cardTarget = defaultGroup.ID
		}
		if err := tx.Model(&types.ReviewGroup{}).
			Where("tenant_id = ? AND parent_id = ? AND id <> ?", tenantID, group.ID, group.ID).
			Updates(map[string]any{"parent_id": childParent, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		if deleteCards {
			now := time.Now()
			if err := tx.Model(&types.ReviewCard{}).
				Where("tenant_id = ? AND group_id = ?", tenantID, group.ID).
				Updates(map[string]any{
					"status":     types.ReviewStatusDeleted,
					"deleted_at": now,
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&types.ReviewCard{}).
			Where("tenant_id = ? AND group_id = ?", tenantID, group.ID).
			Update("group_id", cardTarget).Error; err != nil {
			return err
		}
		return tx.Delete(&types.ReviewGroup{}, "tenant_id = ? AND id = ?", tenantID, id).Error
	})
}

func (r *reviewRepository) CreateCard(
	ctx context.Context, card *types.ReviewCard, sources []*types.ReviewCardSource,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(card).Error; err != nil {
			return err
		}
		if len(sources) == 0 {
			return nil
		}
		for _, source := range sources {
			source.CardID = card.ID
			source.TenantID = card.TenantID
		}
		return tx.Create(&sources).Error
	})
}

func (r *reviewRepository) GetCard(
	ctx context.Context, tenantID uint64, id string,
) (*types.ReviewCard, error) {
	var card types.ReviewCard
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&card).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &card, nil
}

func (r *reviewRepository) GetCardWithSources(
	ctx context.Context, tenantID uint64, id string,
) (*types.ReviewCardDetail, error) {
	card, err := r.GetCard(ctx, tenantID, id)
	if err != nil || card == nil {
		return nil, err
	}
	sources, err := r.ListSources(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	return &types.ReviewCardDetail{Card: card, Sources: sources}, nil
}

func (r *reviewRepository) ListCards(
	ctx context.Context, tenantID uint64, filter *types.ReviewCardFilter,
) ([]*types.ReviewCard, error) {
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if filter != nil {
		if len(filter.GroupIDs) > 0 {
			q = q.Where("group_id IN ?", filter.GroupIDs)
		}
		if len(filter.Statuses) > 0 {
			q = q.Where("status IN ?", filter.Statuses)
		}
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			q = q.Where("question LIKE ? OR answer LIKE ?", like, like)
		}
		if tag := strings.ToLower(strings.TrimSpace(filter.Tag)); tag != "" {
			q = q.Where("LOWER(CAST(tags AS TEXT)) LIKE ?", "%"+tag+"%")
		}
		if filter.DueOnly {
			q = q.Where("due_at <= ? AND review_count > 0", time.Now())
		}
	}
	if filter != nil && filter.Limit > 0 {
		q = q.Limit(filter.Limit)
	}
	if filter != nil && filter.Offset > 0 {
		q = q.Offset(filter.Offset)
	}
	var cards []*types.ReviewCard
	err := q.Order("updated_at DESC").Find(&cards).Error
	return cards, err
}

func (r *reviewRepository) UpdateCard(ctx context.Context, card *types.ReviewCard) error {
	return r.db.WithContext(ctx).
		Model(&types.ReviewCard{}).
		Where("tenant_id = ? AND id = ?", card.TenantID, card.ID).
		Updates(map[string]any{
			"question":       card.Question,
			"answer":         card.Answer,
			"explanation":    card.Explanation,
			"tags":           card.Tags,
			"question_hash":  card.QuestionHash,
			"source_updated": card.SourceUpdated,
			"updated_at":     time.Now(),
		}).Error
}

func (r *reviewRepository) MoveCard(
	ctx context.Context, tenantID uint64, cardID, groupID string,
) error {
	return r.db.WithContext(ctx).
		Model(&types.ReviewCard{}).
		Where("tenant_id = ? AND id = ?", tenantID, cardID).
		Updates(map[string]any{"group_id": groupID, "updated_at": time.Now()}).Error
}

func (r *reviewRepository) SetCardStatus(
	ctx context.Context, tenantID uint64, cardID, status string,
) error {
	return r.db.WithContext(ctx).
		Model(&types.ReviewCard{}).
		Where("tenant_id = ? AND id = ?", tenantID, cardID).
		Updates(map[string]any{
			"status": status,
			// Resuming a paused card clears the streak but keeps the lifetime
			// lapse count; a card with a history of forgetting stays vulnerable.
			"forget_streak": gorm.Expr("CASE WHEN ? THEN 0 ELSE forget_streak END", status == types.ReviewStatusActive),
			"updated_at":    time.Now(),
		}).Error
}

func (r *reviewRepository) SoftDeleteCard(
	ctx context.Context, tenantID uint64, cardID string,
) error {
	now := time.Now()
	return r.db.WithContext(ctx).Unscoped().
		Model(&types.ReviewCard{}).
		Where("tenant_id = ? AND id = ?", tenantID, cardID).
		Updates(map[string]any{
			"status":     types.ReviewStatusDeleted,
			"deleted_at": now,
			"updated_at": now,
		}).Error
}

func (r *reviewRepository) RestoreCard(
	ctx context.Context, tenantID uint64, cardID string,
) error {
	return r.db.WithContext(ctx).Unscoped().
		Model(&types.ReviewCard{}).
		Where("tenant_id = ? AND id = ?", tenantID, cardID).
		Updates(map[string]any{
			"deleted_at": nil,
			// Restored cards stay paused; the owner explicitly resumes them so
			// a restore never silently drops a card back into today's queue.
			"status":     types.ReviewStatusSuspended,
			"updated_at": time.Now(),
		}).Error
}

func (r *reviewRepository) FindCardByQuestionHash(
	ctx context.Context, tenantID uint64, hash string,
) (*types.ReviewCard, error) {
	var card types.ReviewCard
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND question_hash = ?", tenantID, hash).
		Order("created_at ASC").
		First(&card).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &card, nil
}

func (r *reviewRepository) AddSources(
	ctx context.Context, tenantID uint64, cardID string, sources []*types.ReviewCardSource,
) error {
	if len(sources) == 0 {
		return nil
	}
	for _, source := range sources {
		source.TenantID = tenantID
		source.CardID = cardID
	}
	return r.db.WithContext(ctx).Create(&sources).Error
}

func (r *reviewRepository) ListSources(
	ctx context.Context, tenantID uint64, cardID string,
) ([]*types.ReviewCardSource, error) {
	var sources []*types.ReviewCardSource
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND card_id = ?", tenantID, cardID).
		Order("is_primary DESC, created_at ASC").
		Find(&sources).Error
	return sources, err
}

func (r *reviewRepository) MarkSourceUnavailable(
	ctx context.Context, tenantID uint64, cardID, sourceID string,
) error {
	return r.db.WithContext(ctx).
		Model(&types.ReviewCardSource{}).
		Where("tenant_id = ? AND card_id = ? AND id = ?", tenantID, cardID, sourceID).
		Updates(map[string]any{
			"status":     types.ReviewSourceStatusUnavailable,
			"updated_at": time.Now(),
		}).Error
}

func (r *reviewRepository) CountActiveKnowledgeSources(
	ctx context.Context, tenantID uint64, cardID string,
) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.ReviewCardSource{}).
		Where("tenant_id = ? AND card_id = ? AND source_type = ? AND status = ?",
			tenantID, cardID, types.ReviewSourceTypeKnowledge, types.ReviewSourceStatusActive).
		Count(&count).Error
	return count, err
}

func (r *reviewRepository) ListDueCards(
	ctx context.Context, tenantID uint64, groupIDs []string, now time.Time, limit int,
) ([]*types.ReviewCard, error) {
	if limit <= 0 {
		limit = 30
	}
	q := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ? AND review_count > 0 AND due_at <= ?",
			tenantID, types.ReviewStatusActive, now)
	if len(groupIDs) > 0 {
		q = q.Where("group_id IN ?", groupIDs)
	}
	var cards []*types.ReviewCard
	err := q.Order("due_at ASC").Limit(limit).Find(&cards).Error
	return cards, err
}

func (r *reviewRepository) ListNewCards(
	ctx context.Context, tenantID uint64, groupIDs []string, limit int,
) ([]*types.ReviewCard, error) {
	if limit <= 0 {
		return nil, nil
	}
	q := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ? AND review_count = 0",
			tenantID, types.ReviewStatusActive)
	if len(groupIDs) > 0 {
		q = q.Where("group_id IN ?", groupIDs)
	}
	var cards []*types.ReviewCard
	err := q.Order("created_at ASC").Limit(limit).Find(&cards).Error
	return cards, err
}

func (r *reviewRepository) CountCompletedToday(
	ctx context.Context, tenantID uint64, from, to time.Time,
) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("review_logs").
		Joins("JOIN review_cards ON review_cards.id = review_logs.card_id AND review_cards.deleted_at IS NULL").
		Joins("LEFT JOIN review_sessions ON review_sessions.id = review_logs.session_id").
		Where("review_logs.tenant_id = ? AND review_logs.reviewed_at >= ? AND review_logs.reviewed_at < ?",
			tenantID, from, to).
		Where("review_logs.session_id = '' OR review_sessions.mode = ?", types.ReviewSessionModeDaily).
		Distinct("review_logs.card_id").
		Count(&count).Error
	return count, err
}

func (r *reviewRepository) ApplyReview(
	ctx context.Context, card *types.ReviewCard, log *types.ReviewLog,
) (bool, error) {
	return r.applyReview(ctx, card, log, nil)
}

func (r *reviewRepository) ApplyReviewSession(
	ctx context.Context, card *types.ReviewCard, log *types.ReviewLog, session *types.ReviewSession,
) (bool, error) {
	return r.applyReview(ctx, card, log, session)
}

func (r *reviewRepository) applyReview(
	ctx context.Context, card *types.ReviewCard, log *types.ReviewLog, session *types.ReviewSession,
) (bool, error) {
	applied := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "client_request_id"}},
			DoNothing: true,
		}).Create(log)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		applied = true
		if err := tx.Model(&types.ReviewCard{}).
			Where("tenant_id = ? AND id = ? AND status = ? AND deleted_at IS NULL",
				card.TenantID, card.ID, types.ReviewStatusActive).
			Updates(map[string]any{
				"review_level":     card.ReviewLevel,
				"stability":        card.Stability,
				"forget_streak":    card.ForgetStreak,
				"due_at":           card.DueAt,
				"lapses":           card.Lapses,
				"review_count":     card.ReviewCount,
				"last_reviewed_at": card.LastReviewedAt,
				"status":           card.Status,
				"source_updated":   card.SourceUpdated,
				"updated_at":       time.Now(),
			}).Error; err != nil {
			return err
		}
		if session == nil {
			return nil
		}
		completed := session.CompletedCount+1 >= session.PlannedCount
		var completedAt any
		if completed {
			now := time.Now()
			completedAt = &now
		}
		if err := tx.Model(&types.ReviewSession{}).
			Where("tenant_id = ? AND id = ? AND status = ?",
				session.TenantID, session.ID, types.ReviewSessionStatusActive).
			Updates(map[string]any{
				"completed_count":  gorm.Expr("completed_count + 1"),
				"easy_count":       gorm.Expr("easy_count + CASE WHEN ? THEN 1 ELSE 0 END", log.Rating == types.ReviewRatingEasy),
				"remembered_count": gorm.Expr("remembered_count + CASE WHEN ? THEN 1 ELSE 0 END", log.Rating == types.ReviewRatingRemembered),
				"fuzzy_count":      gorm.Expr("fuzzy_count + CASE WHEN ? THEN 1 ELSE 0 END", log.Rating == types.ReviewRatingFuzzy),
				"forgot_count":     gorm.Expr("forgot_count + CASE WHEN ? THEN 1 ELSE 0 END", log.Rating == types.ReviewRatingForgot),
				"status":           gorm.Expr("CASE WHEN completed_count + 1 >= planned_count THEN ? ELSE status END", types.ReviewSessionStatusCompleted),
				"completed_at":     gorm.Expr("CASE WHEN completed_count + 1 >= planned_count THEN ? ELSE completed_at END", completedAt),
				"updated_at":       time.Now(),
			}).Error; err != nil {
			return err
		}
		session.CompletedCount++
		switch log.Rating {
		case types.ReviewRatingEasy:
			session.EasyCount++
		case types.ReviewRatingRemembered:
			session.RememberedCount++
		case types.ReviewRatingFuzzy:
			session.FuzzyCount++
		case types.ReviewRatingForgot:
			session.ForgotCount++
		}
		if completed {
			session.Status = types.ReviewSessionStatusCompleted
			session.CompletedAt = completedAt.(*time.Time)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return applied, nil
}

func (r *reviewRepository) UpdateSource(
	ctx context.Context, tenantID uint64, source *types.ReviewCardSource,
) error {
	return r.db.WithContext(ctx).
		Model(&types.ReviewCardSource{}).
		Where("tenant_id = ? AND card_id = ? AND id = ?", tenantID, source.CardID, source.ID).
		Updates(map[string]any{
			"knowledge_id": source.KnowledgeID,
			"chunk_id":     source.ChunkID,
			"path":         source.Path,
			"symbol":       source.Symbol,
			"start_line":   source.StartLine,
			"end_line":     source.EndLine,
			"content_hash": source.ContentHash,
			"excerpt":      source.Excerpt,
			"status":       types.ReviewSourceStatusActive,
			"updated_at":   time.Now(),
		}).Error
}

func (r *reviewRepository) MergeCards(
	ctx context.Context, tenantID uint64, targetCardID string, sourceCardIDs []string,
) error {
	if targetCardID == "" || len(sourceCardIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var target types.ReviewCard
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, targetCardID).
			First(&target).Error; err != nil {
			return err
		}
		now := time.Now()
		seen := make(map[string]bool, len(sourceCardIDs))
		for _, sourceID := range sourceCardIDs {
			if sourceID == "" || sourceID == targetCardID || seen[sourceID] {
				continue
			}
			seen[sourceID] = true
			if err := tx.Model(&types.ReviewCardSource{}).
				Where("tenant_id = ? AND card_id = ?", tenantID, sourceID).
				Update("card_id", targetCardID).Error; err != nil {
				return err
			}
			if err := tx.Model(&types.ReviewCard{}).
				Where("tenant_id = ? AND id = ?", tenantID, sourceID).
				Updates(map[string]any{
					"status":     types.ReviewStatusDeleted,
					"deleted_at": now,
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *reviewRepository) ListAllCards(
	ctx context.Context, tenantID uint64,
) ([]*types.ReviewCard, error) {
	var cards []*types.ReviewCard
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at ASC").
		Find(&cards).Error
	return cards, err
}

func (r *reviewRepository) ListAllSources(
	ctx context.Context, tenantID uint64,
) ([]*types.ReviewCardSource, error) {
	var sources []*types.ReviewCardSource
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at ASC").
		Find(&sources).Error
	return sources, err
}

func (r *reviewRepository) ListAllLogs(
	ctx context.Context, tenantID uint64,
) ([]*types.ReviewLog, error) {
	var logs []*types.ReviewLog
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("reviewed_at ASC").
		Find(&logs).Error
	return logs, err
}

func (r *reviewRepository) CreateSession(ctx context.Context, session *types.ReviewSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *reviewRepository) GetSession(
	ctx context.Context, tenantID uint64, id string,
) (*types.ReviewSession, error) {
	var session types.ReviewSession
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *reviewRepository) GetActiveSession(
	ctx context.Context, tenantID uint64,
) (*types.ReviewSession, error) {
	var session types.ReviewSession
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", tenantID, types.ReviewSessionStatusActive).
		Order("started_at DESC").
		First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *reviewRepository) UpdateSession(ctx context.Context, session *types.ReviewSession) error {
	return r.db.WithContext(ctx).
		Model(&types.ReviewSession{}).
		Where("tenant_id = ? AND id = ?", session.TenantID, session.ID).
		Updates(map[string]any{
			"status":           session.Status,
			"completed_count":  session.CompletedCount,
			"remembered_count": session.RememberedCount,
			"easy_count":       session.EasyCount,
			"fuzzy_count":      session.FuzzyCount,
			"forgot_count":     session.ForgotCount,
			"completed_at":     session.CompletedAt,
			"updated_at":       time.Now(),
		}).Error
}

func (r *reviewRepository) ExpireSessions(
	ctx context.Context, tenantID uint64, before time.Time,
) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&types.ReviewSession{}).
		Where("tenant_id = ? AND status = ? AND review_day < ?",
			tenantID, types.ReviewSessionStatusActive, before).
		Updates(map[string]any{
			"status":       types.ReviewSessionStatusExpired,
			"completed_at": now,
			"updated_at":   now,
		}).Error
}

func (r *reviewRepository) ListSessions(
	ctx context.Context, tenantID uint64, limit int,
) ([]*types.ReviewSession, error) {
	if limit <= 0 {
		limit = 50
	}
	var sessions []*types.ReviewSession
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("started_at DESC").
		Limit(limit).
		Find(&sessions).Error
	return sessions, err
}

func (r *reviewRepository) ListLogsBySession(
	ctx context.Context, tenantID uint64, sessionID string,
) ([]*types.ReviewLog, error) {
	var logs []*types.ReviewLog
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID).
		Order("reviewed_at ASC").
		Find(&logs).Error
	return logs, err
}

func (r *reviewRepository) FindLogBySessionCard(
	ctx context.Context, tenantID uint64, sessionID, cardID string,
) (*types.ReviewLog, error) {
	var log types.ReviewLog
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND card_id = ?", tenantID, sessionID, cardID).
		First(&log).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *reviewRepository) ListCardsByIDs(
	ctx context.Context, tenantID uint64, ids []string,
) ([]*types.ReviewCard, error) {
	if len(ids) == 0 {
		return []*types.ReviewCard{}, nil
	}
	var cards []*types.ReviewCard
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Find(&cards).Error
	return cards, err
}

func (r *reviewRepository) ListLogsByCard(
	ctx context.Context, tenantID uint64, cardID string, limit int,
) ([]*types.ReviewLog, error) {
	if limit <= 0 {
		limit = 100
	}
	var logs []*types.ReviewLog
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND card_id = ?", tenantID, cardID).
		Order("reviewed_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func (r *reviewRepository) ListLogsByCardIDs(
	ctx context.Context, tenantID uint64, cardIDs []string,
) ([]*types.ReviewLog, error) {
	if len(cardIDs) == 0 {
		return []*types.ReviewLog{}, nil
	}
	var logs []*types.ReviewLog
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND card_id IN ?", tenantID, cardIDs).
		Order("reviewed_at ASC").
		Find(&logs).Error
	return logs, err
}

func (r *reviewRepository) BatchDeleteCards(
	ctx context.Context, tenantID uint64, cardIDs []string,
) error {
	if len(cardIDs) == 0 {
		return nil
	}
	now := time.Now()
	return r.db.WithContext(ctx).Unscoped().
		Model(&types.ReviewCard{}).
		Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", tenantID, cardIDs).
		Updates(map[string]any{
			"status":     types.ReviewStatusDeleted,
			"deleted_at": now,
			"updated_at": now,
		}).Error
}

func (r *reviewRepository) BatchMoveCards(
	ctx context.Context, tenantID uint64, cardIDs []string, groupID string,
) error {
	if len(cardIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&types.ReviewCard{}).
		Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", tenantID, cardIDs).
		Updates(map[string]any{"group_id": groupID, "updated_at": time.Now()}).Error
}

func (r *reviewRepository) BatchUpdateCardTags(
	ctx context.Context, tenantID uint64, tagsByCard map[string]types.JSON,
) error {
	if len(tagsByCard) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for cardID, tags := range tagsByCard {
			if err := tx.Model(&types.ReviewCard{}).
				Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, cardID).
				Updates(map[string]any{"tags": tags, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *reviewRepository) ResetCardSchedules(
	ctx context.Context, tenantID uint64, cardIDs []string, dueAt time.Time, stability float64,
) error {
	if len(cardIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&types.ReviewCard{}).
			Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", tenantID, cardIDs).
			Updates(map[string]any{
				"review_level":  0,
				"stability":     stability,
				"forget_streak": 0,
				"lapses":        0,
				"due_at":        dueAt,
				"updated_at":    time.Now(),
			}).Error
	})
}
