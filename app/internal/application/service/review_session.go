package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	werrors "gaze/internal/errors"
	"gaze/internal/types"
)

func (s *reviewService) StartSession(
	ctx context.Context, req *types.StartReviewSessionRequest,
) (*types.ReviewSessionState, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	if err := s.resumeExpiredSuspensions(ctx, tenantID); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, werrors.NewValidationError("缺少复习参数")
	}
	now := time.Now()
	dayStart := ReviewDayStart(now)
	if err := s.repo.ExpireSessions(ctx, tenantID, dayStart); err != nil {
		return nil, err
	}
	active, err := s.repo.GetActiveSession(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if active != nil {
		if !req.ForceNew {
			return nil, werrors.NewConflictError("已有进行中的复习会话")
		}
		completedAt := now
		active.Status = types.ReviewSessionStatusAbandoned
		active.CompletedAt = &completedAt
		if err := s.repo.UpdateSession(ctx, active); err != nil {
			return nil, err
		}
	}

	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = types.ReviewSessionModeDaily
	}
	if mode != types.ReviewSessionModeDaily && mode != types.ReviewSessionModeManual {
		return nil, werrors.NewValidationError("不支持的复习模式")
	}
	var cards []*types.ReviewCard
	if mode == types.ReviewSessionModeDaily {
		queue, err := s.TodayQueue(ctx, req.GroupID, req.Limit)
		if err != nil {
			return nil, err
		}
		cards = append(cards, queue.Due...)
		cards = append(cards, queue.New...)
	} else {
		ids := uniqueReviewCardIDs(req.CardIDs)
		if len(ids) == 0 {
			return nil, werrors.NewValidationError("自选复习至少选择一张卡片")
		}
		if len(ids) > 100 {
			ids = ids[:100]
		}
		found, err := s.repo.ListCardsByIDs(ctx, tenantID, ids)
		if err != nil {
			return nil, err
		}
		byID := make(map[string]*types.ReviewCard, len(found))
		for _, card := range found {
			byID[card.ID] = card
		}
		for _, id := range ids {
			card := byID[id]
			if card != nil && card.Status == types.ReviewStatusActive {
				cards = append(cards, card)
			}
		}
	}
	if len(cards) == 0 {
		return nil, werrors.NewValidationError("没有可复习的卡片")
	}
	ids := make([]string, 0, len(cards))
	for _, card := range cards {
		ids = append(ids, card.ID)
	}
	encoded, _ := json.Marshal(ids)
	session := &types.ReviewSession{
		ID:             uuid.NewString(),
		TenantID:       tenantID,
		Mode:           mode,
		Status:         types.ReviewSessionStatusActive,
		GroupID:        strings.TrimSpace(req.GroupID),
		PlannedCardIDs: types.JSON(encoded),
		PlannedCount:   len(cards),
		ReviewDay:      dayStart,
		StartedAt:      now,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return &types.ReviewSessionState{Session: session, Cards: cards, Logs: []*types.ReviewLog{}}, nil
}

func (s *reviewService) GetActiveSession(ctx context.Context) (*types.ReviewSessionState, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	if err := s.repo.ExpireSessions(ctx, tenantID, ReviewDayStart(time.Now())); err != nil {
		return nil, err
	}
	session, err := s.repo.GetActiveSession(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}
	return s.sessionState(ctx, tenantID, session)
}

func (s *reviewService) GetSession(ctx context.Context, id string) (*types.ReviewSessionState, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	if err := s.repo.ExpireSessions(ctx, tenantID, ReviewDayStart(time.Now())); err != nil {
		return nil, err
	}
	session, err := s.repo.GetSession(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, werrors.NewNotFoundError("复习会话不存在")
	}
	return s.sessionState(ctx, tenantID, session)
}

func (s *reviewService) AbandonSession(ctx context.Context, id string) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	session, err := s.repo.GetSession(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if session == nil {
		return werrors.NewNotFoundError("复习会话不存在")
	}
	if session.Status != types.ReviewSessionStatusActive {
		return nil
	}
	now := time.Now()
	session.Status = types.ReviewSessionStatusAbandoned
	session.CompletedAt = &now
	return s.repo.UpdateSession(ctx, session)
}

func (s *reviewService) ListSessions(ctx context.Context, limit int) ([]*types.ReviewSession, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	if err := s.repo.ExpireSessions(ctx, tenantID, ReviewDayStart(time.Now())); err != nil {
		return nil, err
	}
	return s.repo.ListSessions(ctx, tenantID, limit)
}

func (s *reviewService) sessionState(
	ctx context.Context, tenantID uint64, session *types.ReviewSession,
) (*types.ReviewSessionState, error) {
	ids := reviewSessionCardIDs(session)
	cards, err := s.repo.ListCardsByIDs(ctx, tenantID, ids)
	if err != nil {
		return nil, err
	}
	logs, err := s.repo.ListLogsBySession(ctx, tenantID, session.ID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*types.ReviewCard, len(cards))
	for _, card := range cards {
		byID[card.ID] = card
	}
	completed := make(map[string]bool, len(logs))
	for _, log := range logs {
		completed[log.CardID] = true
	}
	ordered := make([]*types.ReviewCard, 0, len(ids))
	for _, id := range ids {
		card := byID[id]
		if card == nil {
			continue
		}
		if session.Status == types.ReviewSessionStatusActive && completed[id] {
			continue
		}
		ordered = append(ordered, card)
	}
	return &types.ReviewSessionState{Session: session, Cards: ordered, Logs: logs}, nil
}

func reviewSessionCardIDs(session *types.ReviewSession) []string {
	if session == nil || len(session.PlannedCardIDs) == 0 {
		return nil
	}
	var ids []string
	_ = json.Unmarshal(session.PlannedCardIDs, &ids)
	return ids
}

func uniqueReviewCardIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
