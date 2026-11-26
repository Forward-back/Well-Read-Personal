package repository

import (
	"context"
	"testing"
	"time"

	"gaze/internal/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newReadingRunHistoryRepo(t *testing.T) *readingRunRepository {
	t.Helper()
	dsn := "file:" + uuid.New().String() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&types.ReadingRun{}))
	return NewReadingRunRepository(db).(*readingRunRepository)
}

func createHistoryRun(t *testing.T, repo *readingRunRepository, run *types.ReadingRun) {
	t.Helper()
	if run.ID == "" {
		run.ID = uuid.New().String()
	}
	require.NoError(t, repo.CreateRun(context.Background(), run))
}

func TestListHistoryRunsFiltersStatusesAndScope(t *testing.T) {
	repo := newReadingRunHistoryRepo(t)
	base := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	create := func(id string, tenantID uint64, userID string, status types.ReadingRunStatus, updated time.Time) {
		createHistoryRun(t, repo, &types.ReadingRun{
			ID: id, TenantID: tenantID, UserID: userID, SessionID: "session-" + id,
			DocumentIDs: types.JSON("[]"), Plan: types.JSON("{}"), Notes: types.JSON("{}"),
			Final: types.JSON("{}"), Status: status, CreatedAt: base, UpdatedAt: updated,
		})
	}
	create("finished-old", 1, "owner", types.ReadingRunFinished, base)
	create("stopped-mid", 1, "owner", types.ReadingRunStopped, base.Add(time.Hour))
	create("abandoned-new", 1, "owner", types.ReadingRunAbandoned, base.Add(2*time.Hour))
	create("active", 1, "owner", types.ReadingRunReading, base.Add(3*time.Hour))
	create("failed", 1, "owner", types.ReadingRunFailed, base.Add(4*time.Hour))
	create("other-user", 1, "someone", types.ReadingRunFinished, base.Add(5*time.Hour))
	create("other-tenant", 2, "owner", types.ReadingRunFinished, base.Add(6*time.Hour))

	statuses := []types.ReadingRunStatus{
		types.ReadingRunFinished, types.ReadingRunStopped, types.ReadingRunAbandoned,
	}
	runs, err := repo.ListHistoryRuns(context.Background(), 1, "owner", statuses, 50)
	require.NoError(t, err)
	require.Len(t, runs, 3)
	require.Equal(t, "abandoned-new", runs[0].ID)
	require.Equal(t, "stopped-mid", runs[1].ID)
	require.Equal(t, "finished-old", runs[2].ID)

	limited, err := repo.ListHistoryRuns(context.Background(), 1, "owner", statuses, 2)
	require.NoError(t, err)
	require.Len(t, limited, 2)
	require.Equal(t, "abandoned-new", limited[0].ID)
}

func TestListHistoryRunsZeroLimitReturnsAll(t *testing.T) {
	repo := newReadingRunHistoryRepo(t)
	for i := 0; i < 3; i++ {
		createHistoryRun(t, repo, &types.ReadingRun{
			ID: uuid.New().String(), TenantID: 1, UserID: "owner",
			DocumentIDs: types.JSON("[]"), Plan: types.JSON("{}"), Notes: types.JSON("{}"),
			Final: types.JSON("{}"), Status: types.ReadingRunFinished,
		})
	}
	runs, err := repo.ListHistoryRuns(context.Background(), 1, "owner", []types.ReadingRunStatus{types.ReadingRunFinished}, 0)
	require.NoError(t, err)
	require.Len(t, runs, 3)
}
