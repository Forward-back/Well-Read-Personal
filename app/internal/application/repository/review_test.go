package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"gaze/internal/types"
)

func setupReviewTestDB(t *testing.T) *gorm.DB {
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
	return db
}

func reviewContext(tenantID uint64) context.Context {
	return types.WithExecutionTenant(context.Background(), tenantID)
}

func newRepositoryTestCard(groupID string) (*types.ReviewCard, []*types.ReviewCardSource) {
	card := &types.ReviewCard{
		ID:           uuid.NewString(),
		TenantID:     1,
		GroupID:      groupID,
		CardType:     types.ReviewCardTypeQA,
		Question:     "什么是栈？",
		Answer:       "后进先出。",
		Tags:         types.JSON(`[]`),
		Status:       types.ReviewStatusActive,
		DueAt:        time.Now().Add(-time.Hour),
		ReviewCount:  1,
		QuestionHash: types.HashReviewQuestion("什么是栈？"),
	}
	sources := []*types.ReviewCardSource{{
		ID:          uuid.NewString(),
		TenantID:    1,
		CardID:      card.ID,
		SourceType:  types.ReviewSourceTypeKnowledge,
		KnowledgeID: "k-1",
		ChunkID:     "c-1",
		Status:      types.ReviewSourceStatusActive,
		IsPrimary:   true,
	}}
	return card, sources
}

func TestReviewRepositoryCreateQueueAndIdempotentReview(t *testing.T) {
	db := setupReviewTestDB(t)
	repo := NewReviewRepository(db)
	ctx := reviewContext(1)

	group, err := repo.EnsureDefaultGroup(ctx, 1)
	require.NoError(t, err)
	require.NotEmpty(t, group.ID)

	card, sources := newRepositoryTestCard(group.ID)
	require.NoError(t, repo.CreateCard(ctx, card, sources))

	detail, err := repo.GetCardWithSources(ctx, 1, card.ID)
	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, card.ID, detail.Card.ID)
	assert.Len(t, detail.Sources, 1)

	due, err := repo.ListDueCards(ctx, 1, nil, time.Now(), 10)
	require.NoError(t, err)
	require.Len(t, due, 1)

	completed, err := repo.CountCompletedToday(ctx, 1, time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(0), completed)

	reviewedCard := *card
	reviewedCard.ReviewLevel = 1
	reviewedCard.ReviewCount = 2
	reviewedCard.DueAt = time.Now().Add(72 * time.Hour)
	now := time.Now()
	reviewedCard.LastReviewedAt = &now
	log := &types.ReviewLog{
		ID:              uuid.NewString(),
		TenantID:        1,
		CardID:          card.ID,
		Rating:          types.ReviewRatingRemembered,
		LevelBefore:     0,
		LevelAfter:      1,
		IntervalDays:    3,
		ReviewedAt:      now,
		ClientRequestID: "req-1",
	}
	applied, err := repo.ApplyReview(ctx, &reviewedCard, log)
	require.NoError(t, err)
	assert.True(t, applied)

	duplicate := *log
	duplicate.ID = uuid.NewString()
	applied, err = repo.ApplyReview(ctx, &reviewedCard, &duplicate)
	require.NoError(t, err)
	assert.False(t, applied, "same client request ID must not apply twice")

	completed, err = repo.CountCompletedToday(ctx, 1, time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(1), completed)
}

func TestReviewRepositoryDeleteAndRestoreCard(t *testing.T) {
	db := setupReviewTestDB(t)
	repo := NewReviewRepository(db)
	ctx := reviewContext(1)
	group, err := repo.EnsureDefaultGroup(ctx, 1)
	require.NoError(t, err)
	card, sources := newRepositoryTestCard(group.ID)
	card.Lapses = 3
	card.ForgetStreak = 2
	require.NoError(t, repo.CreateCard(ctx, card, sources))

	require.NoError(t, repo.SoftDeleteCard(ctx, 1, card.ID))
	deleted, err := repo.GetCard(ctx, 1, card.ID)
	require.NoError(t, err)
	assert.Nil(t, deleted)

	deletedList, err := repo.ListCards(ctx, 1, &types.ReviewCardFilter{
		Statuses: []string{types.ReviewStatusDeleted},
	})
	require.NoError(t, err)
	require.Len(t, deletedList, 1, "deleted status filter must surface soft-deleted cards for restore")
	assert.Equal(t, card.ID, deletedList[0].ID)

	visibleList, err := repo.ListCards(ctx, 1, &types.ReviewCardFilter{
		Statuses: []string{types.ReviewStatusActive},
	})
	require.NoError(t, err)
	assert.Empty(t, visibleList, "normal status filters keep deleted cards hidden")

	require.NoError(t, repo.RestoreCard(ctx, 1, card.ID))
	restored, err := repo.GetCard(ctx, 1, card.ID)
	require.NoError(t, err)
	require.NotNil(t, restored)
	assert.Equal(t, types.ReviewStatusSuspended, restored.Status,
		"restored cards stay paused until explicitly resumed")
	assert.Equal(t, 3, restored.Lapses, "restoring a deleted card keeps its schedule history")
	assert.Equal(t, 2, restored.ForgetStreak)
}

func TestReviewRepositoryDeleteGroupMovesChildrenAndCards(t *testing.T) {
	db := setupReviewTestDB(t)
	repo := NewReviewRepository(db)
	ctx := reviewContext(1)
	root, err := repo.EnsureDefaultGroup(ctx, 1)
	require.NoError(t, err)

	parent := &types.ReviewGroup{ID: uuid.NewString(), TenantID: 1, Name: "父"}
	child := &types.ReviewGroup{ID: uuid.NewString(), TenantID: 1, Name: "子", ParentID: parent.ID}
	require.NoError(t, repo.CreateGroup(ctx, parent))
	require.NoError(t, repo.CreateGroup(ctx, child))
	card, sources := newRepositoryTestCard(parent.ID)
	require.NoError(t, repo.CreateCard(ctx, card, sources))

	require.NoError(t, repo.DeleteGroup(ctx, 1, parent.ID, false))
	movedChild, err := repo.GetGroup(ctx, 1, child.ID)
	require.NoError(t, err)
	require.NotNil(t, movedChild)
	assert.Equal(t, "", movedChild.ParentID, "child of a root group reattaches to top level")

	movedCard, err := repo.GetCard(ctx, 1, card.ID)
	require.NoError(t, err)
	require.NotNil(t, movedCard)
	assert.Equal(t, root.ID, movedCard.GroupID, "cards of a root group move to the default group")
}

func TestReviewRepositoryDeleteGroupCanDeleteCards(t *testing.T) {
	db := setupReviewTestDB(t)
	repo := NewReviewRepository(db)
	ctx := reviewContext(1)
	group, err := repo.EnsureDefaultGroup(ctx, 1)
	require.NoError(t, err)
	custom := &types.ReviewGroup{ID: uuid.NewString(), TenantID: 1, Name: "临时"}
	require.NoError(t, repo.CreateGroup(ctx, custom))
	card, sources := newRepositoryTestCard(custom.ID)
	require.NoError(t, repo.CreateCard(ctx, card, sources))

	require.NoError(t, repo.DeleteGroup(ctx, 1, custom.ID, true))
	got, err := repo.GetCard(ctx, 1, card.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
	_ = group
}

func TestReviewRepositoryExpireSessions(t *testing.T) {
	db := setupReviewTestDB(t)
	repo := NewReviewRepository(db)
	ctx := reviewContext(1)
	now := time.Now()
	session := &types.ReviewSession{
		ID:             uuid.NewString(),
		TenantID:       1,
		Mode:           types.ReviewSessionModeDaily,
		Status:         types.ReviewSessionStatusActive,
		PlannedCount:   1,
		ReviewDay:      now.Add(-48 * time.Hour),
		StartedAt:      now.Add(-48 * time.Hour),
		PlannedCardIDs: types.JSON(`["k-1"]`),
	}
	require.NoError(t, repo.CreateSession(ctx, session))
	require.NoError(t, repo.ExpireSessions(ctx, 1, now))
	got, err := repo.GetSession(ctx, 1, session.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, types.ReviewSessionStatusExpired, got.Status)
}
