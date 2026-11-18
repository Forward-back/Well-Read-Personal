package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gaze/internal/types"
)

func TestReviewServiceApplyCardSplitSuspendsOriginal(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	original := createTestCard(t, svc, ctx, "原卡")
	result, err := svc.ApplyCardSplit(ctx, original.ID, &types.ApplyCardSplitRequest{
		Cards: []types.CreateReviewCardRequest{
			{Question: "拆分后的一", Answer: "答案一"},
			{Question: "拆分后的二", Answer: "答案二"},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Created, 2)
	assert.True(t, result.ArchivedOriginal)

	detail, err := svc.GetCard(ctx, original.ID)
	require.NoError(t, err)
	assert.Equal(t, types.ReviewStatusSuspended, detail.Card.Status, "the original card is paused after a split")

	for _, created := range result.Created {
		child, err := svc.GetCard(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, original.GroupID, child.Card.GroupID)
		assert.Equal(t, types.ReviewStatusActive, child.Card.Status)
		require.NotEmpty(t, child.Sources)
		assert.Equal(t, "原文", child.Sources[0].Excerpt, "source excerpts are copied, not invented")
	}
}

func TestReviewServiceRootCauseFallsBackToRules(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	card := createTaggedCard(t, svc, ctx, "根因卡", "k-1", "Go")
	submitRating(t, svc, ctx, card.ID, types.ReviewRatingForgot, "root-1")
	heatmap, err := svc.Heatmap(ctx, &types.ReviewHeatmapOptions{})
	require.NoError(t, err)
	require.Len(t, heatmap.Rows, 1)
	require.Len(t, heatmap.Priorities, 1)
	result, err := svc.RootCause(ctx, &types.ReviewRootCauseRequest{
		RowGroupID: heatmap.Rows[0].ID,
		ColumnKey:  heatmap.Priorities[0].ColumnKey,
	})
	require.NoError(t, err)
	assert.Equal(t, "rule", result.Source)
	assert.NotEmpty(t, result.Summary)
	assert.NotEmpty(t, result.Reasons)
	assert.Contains(t, result.EvidenceCardIDs, card.ID)
}

func TestParseRootCauseResponse(t *testing.T) {
	result, err := parseRootCauseResponse("```json\n{\"summary\":\"概念混淆\",\"reasons\":[\"把优点和缺点混在一起\"],\"focus_suggestions\":[\"回原文\"],\"evidence_card_ids\":[\"card-1\"]}\n```")
	require.NoError(t, err)
	assert.Equal(t, "概念混淆", result.Summary)
	assert.Equal(t, []string{"把优点和缺点混在一起"}, result.Reasons)
	assert.Equal(t, []string{"card-1"}, result.EvidenceCardIDs)
}
