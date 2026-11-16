package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"gaze/internal/application/repository"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

func setupReviewServiceTest(t *testing.T) (interfaces.ReviewService, interfaces.ReviewRepository, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(
		&types.ReviewGroup{},
		&types.ReviewCard{},
		&types.ReviewCardSource{},
		&types.ReviewLog{},
		&types.ReviewSession{},
	))
	repo := repository.NewReviewRepository(db)
	return NewReviewService(repo), repo, types.WithExecutionTenant(context.Background(), 1)
}

func createTestCard(t *testing.T, svc interfaces.ReviewService, ctx context.Context, question string) *types.ReviewCard {
	t.Helper()
	card, err := svc.CreateCard(ctx, &types.CreateReviewCardRequest{
		Question: question,
		Answer:   "答案",
		Sources: []types.ReviewCardSourceInput{{
			SourceType:  types.ReviewSourceTypeKnowledge,
			KnowledgeID: "k-1",
			ChunkID:     "c-1",
			Excerpt:     "原文",
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, card)
	return card
}

func TestReviewServiceCreateCardAbsorbsExactDuplicate(t *testing.T) {
	svc, repo, ctx := setupReviewServiceTest(t)
	first := createTestCard(t, svc, ctx, "什么是栈？")
	second, err := svc.CreateCard(ctx, &types.CreateReviewCardRequest{
		Question: " 什么是栈 ",
		Answer:   "另一个答案",
		Sources: []types.ReviewCardSourceInput{{
			SourceType:  types.ReviewSourceTypeKnowledge,
			KnowledgeID: "k-2",
			ChunkID:     "c-2",
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "same normalized question absorbs into the existing card")

	detail, err := svc.GetCard(ctx, first.ID)
	require.NoError(t, err)
	assert.Len(t, detail.Sources, 2)
	_ = repo
}

func TestReviewServiceSubmitReviewIsIdempotent(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	card := createTestCard(t, svc, ctx, "什么是队列？")

	reviewed, err := svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          card.ID,
		Rating:          types.ReviewRatingRemembered,
		ClientRequestID: "req-1",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, reviewed.ReviewLevel)
	assert.Equal(t, 1, reviewed.ReviewCount)
	assert.True(t, reviewed.DueAt.After(time.Now().Add(48*time.Hour)))

	again, err := svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          card.ID,
		Rating:          types.ReviewRatingRemembered,
		ClientRequestID: "req-1",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, again.ReviewLevel)
	assert.Equal(t, 1, again.ReviewCount, "retry with the same request ID must not advance the card")
}

func TestReviewServiceDoesNotAutoSuspendAfterThreeForgets(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	card := createTestCard(t, svc, ctx, "什么是链表？")

	for i, requestID := range []string{"forget-1", "forget-2", "forget-3"} {
		reviewed, err := svc.SubmitReview(ctx, &types.SubmitReviewRequest{
			CardID:          card.ID,
			Rating:          types.ReviewRatingForgot,
			ClientRequestID: requestID,
		})
		require.NoError(t, err)
		assert.Equal(t, types.ReviewStatusActive, reviewed.Status)
		assert.Equal(t, i+1, reviewed.Lapses)
	}

	reviewed, err := svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          card.ID,
		Rating:          types.ReviewRatingRemembered,
		ClientRequestID: "after-three-forgets",
	})
	require.NoError(t, err, "repeatedly forgotten cards stay reviewable in V3")
	assert.Equal(t, types.ReviewStatusActive, reviewed.Status)
	assert.Equal(t, 0, reviewed.ForgetStreak)
	assert.Equal(t, 3, reviewed.Lapses)

	require.NoError(t, svc.SetCardStatus(ctx, card.ID, types.ReviewStatusSuspended))
	_, err = svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          card.ID,
		Rating:          types.ReviewRatingRemembered,
		ClientRequestID: "while-suspended",
	})
	require.Error(t, err, "only an explicit pause removes a card from the queue")
}

func TestReviewServiceTodayQueueRespectsCompletedCap(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	first := createTestCard(t, svc, ctx, "问题一")
	createTestCard(t, svc, ctx, "问题二")
	createTestCard(t, svc, ctx, "问题三")

	_, err := svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          first.ID,
		Rating:          types.ReviewRatingRemembered,
		ClientRequestID: "today-1",
	})
	require.NoError(t, err)

	queue, err := svc.TodayQueue(ctx, "", 2)
	require.NoError(t, err)
	assert.Equal(t, 1, queue.CompletedToday)
	assert.Equal(t, 1, queue.StreakDays)
	assert.LessOrEqual(t, len(queue.Due)+len(queue.New), 1)
}

func TestReviewServiceMarksOrphanWhenAllKnowledgeSourcesUnavailable(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	card := createTestCard(t, svc, ctx, "什么是树？")
	detail, err := svc.GetCard(ctx, card.ID)
	require.NoError(t, err)
	require.Len(t, detail.Sources, 1)

	require.NoError(t, svc.MarkSourceUnavailable(ctx, card.ID, detail.Sources[0].ID))
	updated, err := svc.GetCard(ctx, card.ID)
	require.NoError(t, err)
	assert.Equal(t, types.ReviewStatusOrphaned, updated.Card.Status)
}

func TestReviewServiceMergeCardsMovesSourcesAndHidesSourceCard(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	target := createTestCard(t, svc, ctx, "目标问题")
	source, err := svc.CreateCard(ctx, &types.CreateReviewCardRequest{
		Question: "来源问题",
		Answer:   "来源答案",
		Sources: []types.ReviewCardSourceInput{{
			SourceType:  types.ReviewSourceTypeKnowledge,
			KnowledgeID: "k-2",
			ChunkID:     "c-2",
		}},
	})
	require.NoError(t, err)

	require.NoError(t, svc.MergeCards(ctx, &types.MergeReviewCardsRequest{
		TargetCardID:  target.ID,
		SourceCardIDs: []string{source.ID},
	}))

	detail, err := svc.GetCard(ctx, target.ID)
	require.NoError(t, err)
	assert.Len(t, detail.Sources, 2)
	_, err = svc.GetCard(ctx, source.ID)
	require.Error(t, err, "merged source card is soft-deleted")
}

func TestReviewServiceRebindReactivateOrphan(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	card := createTestCard(t, svc, ctx, "孤儿问题")
	detail, err := svc.GetCard(ctx, card.ID)
	require.NoError(t, err)
	require.Len(t, detail.Sources, 1)

	require.NoError(t, svc.MarkSourceUnavailable(ctx, card.ID, detail.Sources[0].ID))
	orphan, err := svc.GetCard(ctx, card.ID)
	require.NoError(t, err)
	assert.Equal(t, types.ReviewStatusOrphaned, orphan.Card.Status)

	require.NoError(t, svc.RebindSource(ctx, card.ID, detail.Sources[0].ID, &types.RebindReviewCardSourceRequest{
		KnowledgeID: "k-new",
		ChunkID:     "c-new",
		Excerpt:     "新来源",
	}))
	rebound, err := svc.GetCard(ctx, card.ID)
	require.NoError(t, err)
	assert.Equal(t, types.ReviewStatusSuspended, rebound.Card.Status,
		"rebinding keeps the card paused until the owner resumes it")
	assert.Equal(t, "k-new", rebound.Sources[0].KnowledgeID)

	require.NoError(t, svc.SetCardStatus(ctx, card.ID, types.ReviewStatusActive))
	restored, err := svc.GetCard(ctx, card.ID)
	require.NoError(t, err)
	assert.Equal(t, types.ReviewStatusActive, restored.Card.Status)
}

func TestReviewServiceExportIncludesGroupsCardsSourcesLogs(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	card := createTestCard(t, svc, ctx, "导出问题")
	_, err := svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          card.ID,
		Rating:          types.ReviewRatingRemembered,
		ClientRequestID: "export-1",
	})
	require.NoError(t, err)

	data, err := svc.Export(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, data.Groups)
	assert.Len(t, data.Cards, 1)
	assert.Len(t, data.Sources, 1)
	assert.Len(t, data.Logs, 1)
	assert.Empty(t, data.Sessions, "session list is included for V2 exports and empty when no session exists")
}

func TestReviewServiceStats(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	card := createTestCard(t, svc, ctx, "统计问题")
	_, err := svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          card.ID,
		Rating:          types.ReviewRatingRemembered,
		ClientRequestID: "stats-1",
	})
	require.NoError(t, err)

	stats, err := svc.Stats(ctx, 30)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.TotalCards)
	assert.Equal(t, int64(1), stats.Reviewed30d)
	assert.Equal(t, int64(1), stats.Remembered30d)
	assert.InDelta(t, 1.0, stats.Retention30d, 0.001)
	assert.Equal(t, int64(1), stats.Reviewed)
	assert.InDelta(t, 1.0, stats.Retention, 0.001)
	assert.Equal(t, 30, stats.Days)
	assert.Len(t, stats.RatingTrend, 30)
	assert.Len(t, stats.DueForecast, 7)
	assert.Len(t, stats.GroupRanking, 1)
	assert.Equal(t, 1, stats.RatingDistribution.Remembered)
	assert.Equal(t, 1, stats.StreakDays)
}

func TestReviewServiceGroupTreeCountsSubtreeCards(t *testing.T) {
	svc, repo, ctx := setupReviewServiceTest(t)
	parent, err := svc.CreateGroup(ctx, &types.CreateReviewGroupRequest{Name: "父"})
	require.NoError(t, err)
	child, err := svc.CreateGroup(ctx, &types.CreateReviewGroupRequest{Name: "子", ParentID: parent.ID})
	require.NoError(t, err)
	card, err := svc.CreateCard(ctx, &types.CreateReviewCardRequest{
		GroupID:  child.ID,
		Question: "子树计数",
		Answer:   "答案",
		Sources: []types.ReviewCardSourceInput{{
			KnowledgeID: "k-count",
			ChunkID:     "c-count",
		}},
	})
	require.NoError(t, err)

	stored, err := repo.GetCard(ctx, 1, card.ID)
	require.NoError(t, err)
	stored.ReviewCount = 1
	stored.DueAt = time.Now().Add(-time.Hour)
	applied, err := repo.ApplyReview(ctx, stored, &types.ReviewLog{
		ID:              "log-due",
		TenantID:        1,
		CardID:          stored.ID,
		Rating:          types.ReviewRatingRemembered,
		ReviewedAt:      time.Now(),
		ClientRequestID: "group-due",
	})
	require.NoError(t, err)
	require.True(t, applied)

	tree, err := svc.ListGroupTree(ctx)
	require.NoError(t, err)
	var find func(nodes []*types.ReviewGroupNode, id string) *types.ReviewGroupNode
	find = func(nodes []*types.ReviewGroupNode, id string) *types.ReviewGroupNode {
		for _, node := range nodes {
			if node.Group.ID == id {
				return node
			}
			if nested := find(node.Children, id); nested != nil {
				return nested
			}
		}
		return nil
	}
	parentNode := find(tree, parent.ID)
	childNode := find(tree, child.ID)
	require.NotNil(t, parentNode)
	require.NotNil(t, childNode)
	assert.Equal(t, 1, parentNode.CardCount)
	assert.Equal(t, 1, childNode.CardCount)
	assert.Equal(t, 1, parentNode.DueCount)
	assert.Equal(t, 1, childNode.DueCount)
}

func TestReviewServiceDailySessionCompletesAndResumes(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	first := createTestCard(t, svc, ctx, "会话卡一")
	second := createTestCard(t, svc, ctx, "会话卡二")

	state, err := svc.StartSession(ctx, &types.StartReviewSessionRequest{Mode: types.ReviewSessionModeDaily})
	require.NoError(t, err)
	require.NotNil(t, state.Session)
	assert.Equal(t, types.ReviewSessionStatusActive, state.Session.Status)
	assert.Equal(t, 2, state.Session.PlannedCount)
	assert.Len(t, state.Cards, 2)

	active, err := svc.GetActiveSession(ctx)
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, state.Session.ID, active.Session.ID)

	_, err = svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          first.ID,
		SessionID:       state.Session.ID,
		Rating:          types.ReviewRatingRemembered,
		ClientRequestID: "session-1",
	})
	require.NoError(t, err)

	active, err = svc.GetActiveSession(ctx)
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Len(t, active.Cards, 1)

	_, err = svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          first.ID,
		SessionID:       state.Session.ID,
		Rating:          types.ReviewRatingFuzzy,
		ClientRequestID: "session-duplicate",
	})
	require.Error(t, err, "one card may be rated only once per session")

	_, err = svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          second.ID,
		SessionID:       state.Session.ID,
		Rating:          types.ReviewRatingRemembered,
		ClientRequestID: "session-2",
	})
	require.NoError(t, err)

	active, err = svc.GetActiveSession(ctx)
	require.NoError(t, err)
	assert.Nil(t, active)

	completed, err := svc.GetSession(ctx, state.Session.ID)
	require.NoError(t, err)
	assert.Equal(t, types.ReviewSessionStatusCompleted, completed.Session.Status)
	assert.Equal(t, 2, completed.Session.CompletedCount)
	assert.Equal(t, 2, completed.Session.RememberedCount)
	assert.Equal(t, 0, completed.Session.FuzzyCount)
	assert.Len(t, completed.Logs, 2)
}

func TestReviewServiceManualSessionUsesSelectedCardsOnly(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	selected := createTestCard(t, svc, ctx, "自选一")
	other := createTestCard(t, svc, ctx, "自选二")
	createTestCard(t, svc, ctx, "不选")

	state, err := svc.StartSession(ctx, &types.StartReviewSessionRequest{
		Mode:    types.ReviewSessionModeManual,
		CardIDs: []string{selected.ID, other.ID},
	})
	require.NoError(t, err)
	assert.Equal(t, types.ReviewSessionModeManual, state.Session.Mode)
	assert.Equal(t, 2, state.Session.PlannedCount)
	assert.Len(t, state.Cards, 2)

	_, err = svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          selected.ID,
		SessionID:       state.Session.ID,
		Rating:          types.ReviewRatingRemembered,
		ClientRequestID: "manual-1",
	})
	require.NoError(t, err)
	_, err = svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          other.ID,
		SessionID:       state.Session.ID,
		Rating:          types.ReviewRatingForgot,
		ClientRequestID: "manual-2",
	})
	require.NoError(t, err)

	manual, err := svc.GetSession(ctx, state.Session.ID)
	require.NoError(t, err)
	assert.Equal(t, types.ReviewSessionStatusCompleted, manual.Session.Status)
	assert.Equal(t, 1, manual.Session.ForgotCount)
}

func TestReviewServiceManualSessionRejectsFutureOrSuspendedCards(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	card := createTestCard(t, svc, ctx, "暂停卡")
	require.NoError(t, svc.SetCardStatus(ctx, card.ID, types.ReviewStatusSuspended))

	_, err := svc.StartSession(ctx, &types.StartReviewSessionRequest{
		Mode:    types.ReviewSessionModeManual,
		CardIDs: []string{card.ID},
	})
	require.Error(t, err, "suspended cards cannot enter a manual session")
}

func TestReviewServiceCardPerformanceAndWarning(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	card := createTestCard(t, svc, ctx, "表现卡")
	for i, rating := range []string{
		types.ReviewRatingForgot,
		types.ReviewRatingForgot,
		types.ReviewRatingRemembered,
	} {
		_, err := svc.SubmitReview(ctx, &types.SubmitReviewRequest{
			CardID:          card.ID,
			Rating:          rating,
			ClientRequestID: fmt.Sprintf("performance-%d", i),
		})
		require.NoError(t, err)
	}
	performance, err := svc.GetCardPerformance(ctx, card.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, performance.Counts.Total)
	assert.Equal(t, 2, performance.Counts.Forgot)
	assert.Equal(t, 1, performance.Counts.Remembered)
	assert.Equal(t, 0, performance.ForgetStreak)
	assert.Equal(t, "watch", performance.Warning)
	assert.Equal(t, types.ReviewCardLevelWarning, performance.Card.Vulnerability)

	watchCard := createTestCard(t, svc, ctx, "连续忘记卡")
	for i, rating := range []string{types.ReviewRatingForgot, types.ReviewRatingForgot} {
		_, err = svc.SubmitReview(ctx, &types.SubmitReviewRequest{
			CardID:          watchCard.ID,
			Rating:          rating,
			ClientRequestID: fmt.Sprintf("watch-%d", i),
		})
		require.NoError(t, err)
	}
	performance, err = svc.GetCardPerformance(ctx, watchCard.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, performance.ForgetStreak)
	assert.Equal(t, "high", performance.Warning)
	assert.Equal(t, types.ReviewCardLevelSevere, performance.Card.Vulnerability)

	_, err = svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          card.ID,
		Rating:          types.ReviewRatingForgot,
		ClientRequestID: "performance-4",
	})
	require.NoError(t, err)
	performance, err = svc.GetCardPerformance(ctx, card.ID)
	require.NoError(t, err)
	assert.Equal(t, "high", performance.Warning)
}
