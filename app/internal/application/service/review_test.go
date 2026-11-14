package service

import (
	"context"
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

func TestReviewServiceSuspendsAfterThreeForgets(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	card := createTestCard(t, svc, ctx, "什么是链表？")

	for i, requestID := range []string{"forget-1", "forget-2", "forget-3"} {
		reviewed, err := svc.SubmitReview(ctx, &types.SubmitReviewRequest{
			CardID:          card.ID,
			Rating:          types.ReviewRatingForgot,
			ClientRequestID: requestID,
		})
		require.NoError(t, err)
		if i < 2 {
			assert.Equal(t, types.ReviewStatusActive, reviewed.Status)
		} else {
			assert.Equal(t, types.ReviewStatusSuspended, reviewed.Status)
			assert.Equal(t, 3, reviewed.Lapses)
		}
	}

	_, err := svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          card.ID,
		Rating:          types.ReviewRatingRemembered,
		ClientRequestID: "after-suspend",
	})
	require.Error(t, err, "suspended cards must leave the queue")
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
	restored, err := svc.GetCard(ctx, card.ID)
	require.NoError(t, err)
	assert.Equal(t, types.ReviewStatusActive, restored.Card.Status)
	assert.Equal(t, "k-new", restored.Sources[0].KnowledgeID)
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

	stats, err := svc.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.TotalCards)
	assert.Equal(t, int64(1), stats.Reviewed30d)
	assert.Equal(t, int64(1), stats.Remembered30d)
	assert.InDelta(t, 1.0, stats.Retention30d, 0.001)
}
