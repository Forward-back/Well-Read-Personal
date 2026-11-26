package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"gaze/internal/application/repository"
	"gaze/internal/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newReadingHistoryService(t *testing.T) (*readingRunService, *gorm.DB) {
	t.Helper()
	dsn := "file:" + uuid.New().String() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(
		&types.ReadingRun{}, &types.Session{}, &types.Knowledge{},
	))
	return &readingRunService{
		repo:          repository.NewReadingRunRepository(db),
		sessionRepo:   repository.NewSessionRepository(db),
		knowledgeRepo: repository.NewKnowledgeRepository(db),
	}, db
}

func historyCallerContext() context.Context {
	return types.WithCaller(context.Background(), types.Caller{TenantID: 1, UserID: "owner"})
}

func mustJSON(t *testing.T, value any) types.JSON {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return types.JSON(encoded)
}

func TestListHistoryAggregatesSessionDocumentsAndProgress(t *testing.T) {
	svc, db := newReadingHistoryService(t)
	base := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)

	session := &types.Session{TenantID: 1, UserID: "owner", Title: "退款条款精读"}
	require.NoError(t, db.Create(session).Error)
	require.NotEmpty(t, session.ID)
	require.NoError(t, db.Create(&types.Knowledge{
		ID: "doc-1", TenantID: 1, KnowledgeBaseID: "kb-1", Type: "file",
		Title: "退款制度", FileName: "refund.md", ParseStatus: types.ParseStatusPending,
	}).Error)

	plan := types.ReadingPlan{
		Goal: "读懂退款条款", Depth: types.ReadingDepthStandard,
		Sections: []types.ReadingSection{
			{ID: "s1", Read: true}, {ID: "s2", Read: true}, {ID: "s3", Read: false},
		},
	}
	require.NoError(t, db.Create(&types.ReadingRun{
		ID: "run-finished", TenantID: 1, UserID: "owner", SessionID: session.ID,
		DocumentIDs: mustJSON(t, []string{"doc-1", "doc-missing"}),
		Plan:        mustJSON(t, plan), Notes: types.JSON("{}"), Final: types.JSON("{}"),
		Status: types.ReadingRunFinished, CreatedAt: base, UpdatedAt: base.Add(2 * time.Hour),
	}).Error)
	require.NoError(t, db.Create(&types.ReadingRun{
		ID: "run-stopped", TenantID: 1, UserID: "owner", SessionID: "session-deleted",
		DocumentIDs: types.JSON("[]"), Plan: types.JSON("{}"), Notes: types.JSON("{}"),
		Final: types.JSON("{}"), Status: types.ReadingRunStopped,
		CreatedAt: base, UpdatedAt: base.Add(time.Hour),
	}).Error)
	for _, excluded := range []types.ReadingRunStatus{types.ReadingRunReading, types.ReadingRunFailed} {
		require.NoError(t, db.Create(&types.ReadingRun{
			ID: "run-" + string(excluded), TenantID: 1, UserID: "owner", SessionID: "session-finished",
			DocumentIDs: types.JSON("[]"), Plan: types.JSON("{}"), Notes: types.JSON("{}"),
			Final: types.JSON("{}"), Status: excluded, CreatedAt: base, UpdatedAt: base.Add(3 * time.Hour),
		}).Error)
	}

	items, err := svc.ListHistory(historyCallerContext(), 0)
	require.NoError(t, err)
	require.Len(t, items, 2)

	finished := items[0]
	require.Equal(t, "run-finished", finished.ID)
	require.Equal(t, "退款条款精读", finished.Title)
	require.Equal(t, "读懂退款条款", finished.Goal)
	require.Equal(t, "standard", finished.Depth)
	require.Equal(t, 2, finished.ReadSections)
	require.Equal(t, 3, finished.TotalSections)
	require.Len(t, finished.Documents, 2)
	require.Equal(t, "退款制度", finished.Documents[0].Title)
	require.Equal(t, "refund.md", finished.Documents[0].FileName)
	require.Equal(t, "doc-missing", finished.Documents[1].ID)
	require.Empty(t, finished.Documents[1].Title)

	stopped := items[1]
	require.Equal(t, "run-stopped", stopped.ID)
	require.Empty(t, stopped.Title)

	limited, err := svc.ListHistory(historyCallerContext(), 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
	require.Equal(t, "run-finished", limited[0].ID)
}

func TestListHistoryDefaultsToFiftyRuns(t *testing.T) {
	svc, db := newReadingHistoryService(t)
	for i := 0; i < 55; i++ {
		require.NoError(t, db.Create(&types.ReadingRun{
			ID: uuid.New().String(), TenantID: 1, UserID: "owner", SessionID: "",
			DocumentIDs: types.JSON("[]"), Plan: types.JSON("{}"), Notes: types.JSON("{}"),
			Final: types.JSON("{}"), Status: types.ReadingRunFinished,
		}).Error)
	}
	items, err := svc.ListHistory(historyCallerContext(), 0)
	require.NoError(t, err)
	require.Len(t, items, 50)
}

func TestListHistoryRequiresCaller(t *testing.T) {
	svc, _ := newReadingHistoryService(t)
	_, err := svc.ListHistory(context.Background(), 0)
	require.Error(t, err)
}
