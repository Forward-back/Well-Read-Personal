package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	werrors "gaze/internal/errors"
	"gaze/internal/types"
)

const reviewBatchLimit = 100

func (s *reviewService) ResetCardSchedule(
	ctx context.Context, id string,
) (*types.ReviewCard, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	card, err := s.repo.GetCard(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, werrors.NewNotFoundError("复习卡不存在")
	}
	if card.Status == types.ReviewStatusOrphaned {
		return nil, werrors.NewConflictError("孤儿卡请先重绑来源")
	}
	if err := s.repo.ResetCardSchedules(
		ctx, tenantID, []string{card.ID}, time.Now(), ReviewInitialStability,
	); err != nil {
		return nil, err
	}
	updated, err := s.repo.GetCard(ctx, tenantID, card.ID)
	if err != nil || updated == nil {
		return updated, err
	}
	logs, err := s.repo.ListLogsByCardIDs(ctx, tenantID, []string{card.ID})
	if err != nil {
		return nil, err
	}
	attachReviewCardLevels([]*types.ReviewCard{updated}, groupReviewLogsByCard(logs), time.Now())
	return updated, nil
}

// BatchCards validates the whole selection before touching any row. A single
// missing, deleted, or ineligible card fails the batch; partial success is
// never reported.
func (s *reviewService) BatchCards(
	ctx context.Context, req *types.BatchReviewCardsRequest,
) (*types.BatchReviewCardsResult, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	if req == nil {
		return nil, werrors.NewValidationError("缺少批量操作参数")
	}
	ids := uniqueReviewCardIDs(req.CardIDs)
	if len(ids) == 0 {
		return nil, werrors.NewValidationError("请至少选择一张卡片")
	}
	if len(ids) > reviewBatchLimit {
		return nil, werrors.NewValidationError("单次批量操作最多 100 张卡片")
	}
	cards, err := s.repo.ListCardsByIDs(ctx, tenantID, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*types.ReviewCard, len(cards))
	for _, card := range cards {
		byID[card.ID] = card
	}
	for _, id := range ids {
		if byID[id] == nil {
			return nil, werrors.NewConflictError("部分卡片不存在或已删除，请刷新后重试")
		}
	}

	switch req.Action {
	case types.ReviewBatchActionDelete:
		if err := s.repo.BatchDeleteCards(ctx, tenantID, ids); err != nil {
			return nil, err
		}
	case types.ReviewBatchActionMove:
		groupID := strings.TrimSpace(req.GroupID)
		if groupID == "" {
			return nil, werrors.NewValidationError("缺少目标卡组")
		}
		group, err := s.repo.GetGroup(ctx, tenantID, groupID)
		if err != nil {
			return nil, err
		}
		if group == nil {
			return nil, werrors.NewValidationError("卡组不存在")
		}
		if err := s.repo.BatchMoveCards(ctx, tenantID, ids, groupID); err != nil {
			return nil, err
		}
	case types.ReviewBatchActionAddTags:
		extra, err := normalizeReviewTags(req.Tags)
		if err != nil {
			return nil, err
		}
		if len(extra) == 0 {
			return nil, werrors.NewValidationError("请至少填写一个标签")
		}
		tagsByCard := make(map[string]types.JSON, len(ids))
		overflow := 0
		for _, id := range ids {
			merged, err := mergeReviewTags(reviewTagsFromJSON(byID[id].Tags), extra)
			if err != nil {
				overflow++
				continue
			}
			tagsByCard[id] = reviewTagsJSON(merged)
		}
		if overflow > 0 {
			return nil, werrors.NewConflictError(fmt.Sprintf(
				"有 %d 张卡片追加后将超过 10 个标签，整批未执行", overflow,
			))
		}
		if err := s.repo.BatchUpdateCardTags(ctx, tenantID, tagsByCard); err != nil {
			return nil, err
		}
	case types.ReviewBatchActionReset:
		for _, id := range ids {
			if byID[id].Status == types.ReviewStatusOrphaned {
				return nil, werrors.NewConflictError("选中包含孤儿卡，请先重绑来源")
			}
		}
		if err := s.repo.ResetCardSchedules(
			ctx, tenantID, ids, time.Now(), ReviewInitialStability,
		); err != nil {
			return nil, err
		}
	default:
		return nil, werrors.NewValidationError("不支持的批量操作")
	}
	return &types.BatchReviewCardsResult{Affected: len(ids)}, nil
}
