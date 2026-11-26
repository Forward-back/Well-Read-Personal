package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gaze/internal/types"
)

func TestReviewSuspendUntilKeepsCardPausedThenAutoResumes(t *testing.T) {
	svc, repo, ctx := setupReviewServiceTest(t)
	card := createTestCard(t, svc, ctx, "暂停到期的卡片")

	until := time.Now().Add(48 * time.Hour)
	require.NoError(t, svc.SetCardStatusUntil(ctx, card.ID, types.ReviewStatusSuspended, &until))

	detail, err := svc.GetCard(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, types.ReviewStatusSuspended, detail.Card.Status)
	require.NotNil(t, detail.Card.SuspendedUntil)

	past := time.Now().Add(-time.Hour)
	require.NoError(t, repo.SuspendCardUntil(ctx, 1, card.ID, &past))

	_, err = svc.ListCards(ctx, &types.ReviewCardFilter{GroupID: card.GroupID})
	require.NoError(t, err)

	detail, err = svc.GetCard(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, types.ReviewStatusActive, detail.Card.Status)
	require.Nil(t, detail.Card.SuspendedUntil)
}

func TestReviewSuspendUntilRejectsPastDate(t *testing.T) {
	svc, _, ctx := setupReviewServiceTest(t)
	card := createTestCard(t, svc, ctx, "暂停日期校验")
	past := time.Now().Add(-time.Hour)
	require.Error(t, svc.SetCardStatusUntil(ctx, card.ID, types.ReviewStatusSuspended, &past))
}
