package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

type stubKnowledgeResolver struct {
	titles map[string]string
	bases  map[string]string
}

func (s *stubKnowledgeResolver) GetKnowledgeBatch(
	_ context.Context, _ uint64, ids []string,
) ([]*types.Knowledge, error) {
	out := make([]*types.Knowledge, 0, len(ids))
	for _, id := range ids {
		out = append(out, &types.Knowledge{
			ID:              id,
			Title:           s.titles[id],
			KnowledgeBaseID: s.bases[id],
		})
	}
	return out, nil
}

func createTaggedCard(
	t *testing.T, svc interfaces.ReviewService, ctx context.Context,
	question, knowledgeID string, tags ...string,
) *types.ReviewCard {
	t.Helper()
	card, err := svc.CreateCard(ctx, &types.CreateReviewCardRequest{
		Question: question,
		Answer:   "答案",
		Tags:     tags,
		Sources: []types.ReviewCardSourceInput{{
			SourceType:  types.ReviewSourceTypeKnowledge,
			KnowledgeID: knowledgeID,
			ChunkID:     knowledgeID + "-chunk",
			Excerpt:     "原文",
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, card)
	return card
}

func submitRating(
	t *testing.T, svc interfaces.ReviewService, ctx context.Context,
	cardID, rating, requestID string,
) *types.ReviewCard {
	t.Helper()
	card, err := svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          cardID,
		Rating:          rating,
		ClientRequestID: requestID,
	})
	require.NoError(t, err)
	return card
}

func TestReviewServiceTodayQueueCapsNewCards(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	for i := 0; i < 25; i++ {
		createTaggedCard(t, svc, ctx, fmt.Sprintf("新卡 %02d", i), fmt.Sprintf("k-%d", i))
	}

	queue, err := svc.TodayQueue(ctx, "", 0)
	require.NoError(t, err)
	assert.Equal(t, ReviewDailyTotalLimit, queue.Limit)
	assert.Equal(t, ReviewDailyNewCardLimit, queue.NewLimit)
	assert.Len(t, queue.New, ReviewDailyNewCardLimit)
	assert.Empty(t, queue.Due)
}

func TestReviewServiceStartSessionRequiresExplicitReplace(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	createTaggedCard(t, svc, ctx, "会话冲突一", "k-1")
	createTaggedCard(t, svc, ctx, "会话冲突二", "k-2")

	first, err := svc.StartSession(ctx, &types.StartReviewSessionRequest{})
	require.NoError(t, err)

	_, err = svc.StartSession(ctx, &types.StartReviewSessionRequest{})
	require.Error(t, err, "an active session must be reported, not silently resumed")

	second, err := svc.StartSession(ctx, &types.StartReviewSessionRequest{ForceNew: true})
	require.NoError(t, err)
	assert.NotEqual(t, first.Session.ID, second.Session.ID)

	old, err := svc.GetSession(ctx, first.Session.ID)
	require.NoError(t, err)
	assert.Equal(t, types.ReviewSessionStatusAbandoned, old.Session.Status)
}

func TestReviewServiceEasyRatingCountsAndStats(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	first := createTaggedCard(t, svc, ctx, "秒答一", "k-1")
	second := createTaggedCard(t, svc, ctx, "秒答二", "k-2")

	state, err := svc.StartSession(ctx, &types.StartReviewSessionRequest{
		Mode:    types.ReviewSessionModeManual,
		CardIDs: []string{first.ID, second.ID},
	})
	require.NoError(t, err)
	reviewed, err := svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          first.ID,
		SessionID:       state.Session.ID,
		Rating:          types.ReviewRatingEasy,
		ClientRequestID: "easy-1",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, reviewed.ReviewLevel, "easy adds two levels")
	assert.Equal(t, 50.0, reviewed.Stability)
	_, err = svc.SubmitReview(ctx, &types.SubmitReviewRequest{
		CardID:          second.ID,
		SessionID:       state.Session.ID,
		Rating:          types.ReviewRatingRemembered,
		ClientRequestID: "easy-2",
	})
	require.NoError(t, err)

	session, err := svc.GetSession(ctx, state.Session.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, session.Session.EasyCount)
	assert.Equal(t, 1, session.Session.RememberedCount)

	stats, err := svc.Stats(ctx, 30)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.RatingDistribution.Easy)
	assert.Equal(t, 2, stats.RatingDistribution.Total)
	assert.InDelta(t, 1.0, stats.Retention, 0.001, "easy counts as a remembered success")
	require.Len(t, stats.RatingTrend, 30)
	assert.Equal(t, int64(1), stats.RatingTrend[29].Easy)

	performance, err := svc.GetCardPerformance(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, performance.Counts.Easy)
	assert.Equal(t, 1, performance.RecentCounts.Easy)
}

func TestReviewServiceHeatmapAndClusterDetail(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	impl := svc.(*reviewService)
	impl.knowledgeResolver = &stubKnowledgeResolver{
		titles: map[string]string{"k-severe": "数据结构笔记"},
		bases:  map[string]string{"k-severe": "kb-1"},
	}

	severe := createTaggedCard(t, svc, ctx, "严重卡", "k-severe", "Go")
	warning := createTaggedCard(t, svc, ctx, "警告卡", "k-warning", "go")
	healthy := createTaggedCard(t, svc, ctx, "健康卡", "k-healthy", "Go")
	createTaggedCard(t, svc, ctx, "新卡", "k-new", "Go")

	submitRating(t, svc, ctx, severe.ID, types.ReviewRatingForgot, "severe-1")
	submitRating(t, svc, ctx, severe.ID, types.ReviewRatingForgot, "severe-2")
	submitRating(t, svc, ctx, severe.ID, types.ReviewRatingForgot, "severe-3")
	submitRating(t, svc, ctx, warning.ID, types.ReviewRatingForgot, "warning-1")
	submitRating(t, svc, ctx, healthy.ID, types.ReviewRatingRemembered, "healthy-1")

	heatmap, err := svc.Heatmap(ctx, "")
	require.NoError(t, err)
	require.Len(t, heatmap.Rows, 1)
	require.Len(t, heatmap.Columns, 1)
	assert.Equal(t, "tag:go", heatmap.Columns[0].Key, "tag columns are case-insensitive")
	require.Len(t, heatmap.Cells, 1)
	cell := heatmap.Cells[0]
	assert.Equal(t, 4, cell.Total)
	assert.Equal(t, 1, cell.Severe)
	assert.Equal(t, 1, cell.Warning)
	assert.Equal(t, 1, cell.Healthy)
	assert.Equal(t, 1, cell.NoData)
	assert.InDelta(t, 0.375, cell.Intensity, 0.001)
	assert.Equal(t, types.ReviewCardLevelSevere, cell.Level,
		"a single severe card lifts the cluster above its diluted intensity")
	assert.True(t, cell.Clickable)
	require.Len(t, heatmap.Priorities, 1)
	assert.Equal(t, cell.ColumnKey, heatmap.Priorities[0].ColumnKey)

	detail, err := svc.ClusterDetail(ctx, heatmap.Rows[0].ID, cell.ColumnKey)
	require.NoError(t, err)
	require.Len(t, detail.Cards, 4)
	assert.Equal(t, types.ReviewCardLevelSevere, detail.Cards[0].Level)
	assert.Equal(t, "relearn", detail.Cards[0].SuggestedAction)
	require.NotNil(t, detail.Cards[0].Source)
	assert.Equal(t, "kb-1", detail.Cards[0].KnowledgeBaseID)
}

func TestReviewServiceHeatmapFallsBackToSourceTitle(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	impl := svc.(*reviewService)
	impl.knowledgeResolver = &stubKnowledgeResolver{
		titles: map[string]string{"k-doc": "系统设计"},
		bases:  map[string]string{"k-doc": "kb-9"},
	}
	card := createTaggedCard(t, svc, ctx, "无标签卡", "k-doc")
	submitRating(t, svc, ctx, card.ID, types.ReviewRatingRemembered, "doc-1")

	heatmap, err := svc.Heatmap(ctx, "")
	require.NoError(t, err)
	require.Len(t, heatmap.Columns, 1)
	assert.Equal(t, "source:k-doc", heatmap.Columns[0].Key)
	assert.Equal(t, "系统设计", heatmap.Columns[0].Title)
	assert.Equal(t, "kb-9", heatmap.Columns[0].KnowledgeBaseID)
}

func TestReviewServiceBatchCardsAndReset(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	first := createTaggedCard(t, svc, ctx, "批量一", "k-1", "旧")
	second := createTaggedCard(t, svc, ctx, "批量二", "k-2")
	third := createTaggedCard(t, svc, ctx, "批量三", "k-3")
	ids := []string{first.ID, second.ID, third.ID}

	result, err := svc.BatchCards(ctx, &types.BatchReviewCardsRequest{
		CardIDs: ids,
		Action:  types.ReviewBatchActionAddTags,
		Tags:    []string{"  Go ", "go", "DB"},
	})
	require.NoError(t, err)
	assert.Equal(t, 3, result.Affected)
	detail, err := svc.GetCard(ctx, second.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"Go", "DB"}, reviewTagsFromJSON(detail.Card.Tags))

	group, err := svc.CreateGroup(ctx, &types.CreateReviewGroupRequest{Name: "批量目标"})
	require.NoError(t, err)
	_, err = svc.BatchCards(ctx, &types.BatchReviewCardsRequest{
		CardIDs: ids,
		Action:  types.ReviewBatchActionMove,
		GroupID: group.ID,
	})
	require.NoError(t, err)
	moved, err := svc.GetCard(ctx, third.ID)
	require.NoError(t, err)
	assert.Equal(t, group.ID, moved.Card.GroupID)

	submitRating(t, svc, ctx, first.ID, types.ReviewRatingForgot, "batch-1")
	submitRating(t, svc, ctx, first.ID, types.ReviewRatingForgot, "batch-2")
	submitRating(t, svc, ctx, first.ID, types.ReviewRatingForgot, "batch-3")
	reset, err := svc.ResetCardSchedule(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, reset.ReviewLevel)
	assert.Equal(t, ReviewInitialStability, reset.Stability)
	assert.Equal(t, 0, reset.Lapses)
	assert.Equal(t, 0, reset.ForgetStreak)
	assert.Equal(t, 3, reset.ReviewCount, "logs and review count survive a reset")
	assert.Equal(t, types.ReviewCardLevelWarning, reset.Vulnerability,
		"recent forgotten logs still mark the card weak after a reset")

	_, err = svc.BatchCards(ctx, &types.BatchReviewCardsRequest{
		CardIDs: []string{first.ID, "missing"},
		Action:  types.ReviewBatchActionDelete,
	})
	require.Error(t, err, "a batch with a missing card fails as a whole")

	_, err = svc.BatchCards(ctx, &types.BatchReviewCardsRequest{
		CardIDs: ids,
		Action:  types.ReviewBatchActionDelete,
	})
	require.NoError(t, err)
	cards, err := svc.ListCards(ctx, &types.ReviewCardFilter{})
	require.NoError(t, err)
	assert.Empty(t, cards)
}
