package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"gaze/internal/types"
)

func openReadingAgentRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&types.ReadingRun{},
		&types.ReadingRunMemory{},
		&types.ReadingSelfTest{},
	))
	return db
}

func TestReadingRunMemoryReplace(t *testing.T) {
	db := openReadingAgentRepoTestDB(t)
	repo := NewReadingRunRepository(db)
	ctx := context.Background()

	first := &types.ReadingRunMemory{
		RunID: "run-1", TenantID: 7,
		Knowledge: types.JSON(`{"conclusions":["a"]}`),
		UserModel: types.JSON(`{"quiet":false}`),
		Task:      types.JSON(`{"phase":"reading"}`),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.SaveMemory(ctx, first))

	stored, err := repo.GetMemory(ctx, "run-1")
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.JSONEq(t, `{"conclusions":["a"]}`, string(stored.Knowledge))

	second := &types.ReadingRunMemory{
		RunID: "run-1", TenantID: 7,
		Knowledge: types.JSON(`{"conclusions":["b"]}`),
		UserModel: types.JSON(`{"quiet":true,"consecutive_skips":2}`),
		Task:      types.JSON(`{"phase":"synthesis"}`),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.SaveMemory(ctx, second))

	stored, err = repo.GetMemory(ctx, "run-1")
	require.NoError(t, err)
	require.JSONEq(t, `{"conclusions":["b"]}`, string(stored.Knowledge))
	require.JSONEq(t, `{"quiet":true,"consecutive_skips":2}`, string(stored.UserModel))
	require.JSONEq(t, `{"phase":"synthesis"}`, string(stored.Task))

	missing, err := repo.GetMemory(ctx, "run-missing")
	require.NoError(t, err)
	require.Nil(t, missing)
}

func TestReadingSelfTestLifecycle(t *testing.T) {
	db := openReadingAgentRepoTestDB(t)
	repo := NewReadingRunRepository(db)
	ctx := context.Background()

	now := time.Now()
	tests := []*types.ReadingSelfTest{
		{
			ID: "st-1", RunID: "run-1", TenantID: 7, SectionID: "s1",
			Question: "问题一", Answer: "答案一", Explanation: "",
			Evidence: types.JSON(`[]`), Status: types.ReadingSelfTestCandidate,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "st-2", RunID: "run-1", TenantID: 7, SectionID: "s1",
			Question: "问题二", Answer: "答案二", Explanation: "",
			Evidence: types.JSON(`[]`), Status: types.ReadingSelfTestCandidate,
			CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
		},
	}
	inserted, err := repo.CreateSelfTests(ctx, tests)
	require.NoError(t, err)
	require.Equal(t, 2, inserted)

	// Re-inserting the same IDs is a no-op, so a retried generation cannot
	// duplicate rows.
	inserted, err = repo.CreateSelfTests(ctx, tests)
	require.NoError(t, err)
	require.Equal(t, 0, inserted)

	rows, err := repo.ListSelfTests(ctx, "run-1")
	require.NoError(t, err)
	require.Len(t, rows, 2)

	updated, err := repo.UpdateSelfTestStatus(ctx, "run-1", "st-1", types.ReadingSelfTestImported)
	require.NoError(t, err)
	require.True(t, updated)
	updated, err = repo.UpdateSelfTestStatus(ctx, "run-1", "st-1", types.ReadingSelfTestImported)
	require.NoError(t, err)
	require.False(t, updated, "only candidates can transition")

	updated, err = repo.UpdateSelfTestStatus(ctx, "run-other", "st-2", types.ReadingSelfTestImported)
	require.NoError(t, err)
	require.False(t, updated, "run ownership must be checked")
}

func TestReadingRunUpdatePersistsAgentFields(t *testing.T) {
	db := openReadingAgentRepoTestDB(t)
	repo := NewReadingRunRepository(db)
	ctx := context.Background()

	now := time.Now()
	run := &types.ReadingRun{
		ID: "run-1", TenantID: 7, UserID: "user-1", SessionID: "sess-1",
		DocumentIDs: types.JSON(`[]`), Plan: types.JSON(`{}`), Notes: types.JSON(`{}`),
		Final: types.JSON(`{}`), Status: types.ReadingRunPlanning,
		ExtractionModelID: "extract-1", SynthesisModelID: "synth-1",
		Phase: types.ReadingPhaseStructure, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.CreateRun(ctx, run))

	run.ExtractionModelID = "extract-2"
	run.SynthesisModelID = "synth-2"
	run.Phase = types.ReadingPhaseSynthesis
	run.UpdatedAt = now.Add(time.Minute)
	require.NoError(t, repo.UpdateRun(ctx, run))

	stored, err := repo.GetRun(ctx, 7, "user-1", "run-1")
	require.NoError(t, err)
	require.Equal(t, "extract-2", stored.ExtractionModelID)
	require.Equal(t, "synth-2", stored.SynthesisModelID)
	require.Equal(t, types.ReadingPhaseSynthesis, stored.Phase)
}
