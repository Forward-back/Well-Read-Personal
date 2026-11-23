package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"gaze/internal/application/repository"
	"gaze/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newLegacyReadingTestService(t *testing.T) (*readingRunService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(
		&types.ReadingRun{},
		&types.ReadingQuestionNode{},
		&types.ReadingConclusion{},
		&types.ReadingTerm{},
		&types.ReadingContradiction{},
	))
	return &readingRunService{repo: repository.NewReadingRunRepository(db)}, db
}

func TestMaybeMigrateLegacyRunUpgradesLegacyNotesRun(t *testing.T) {
	svc, db := newLegacyReadingTestService(t)
	ctx := context.Background()

	planJSON, _ := json.Marshal(types.ReadingPlan{
		Goal: "读第二章", Depth: types.ReadingDepthStandard, StateVersion: 1,
	})
	notesJSON, _ := json.Marshal(types.ReadingNotes{
		Confirmed:      []string{"结论一"},
		Terms:          []string{"术语甲：定义甲"},
		OpenQuestions:  []string{"待确认问题"},
		Contradictions: []string{"说法冲突"},
		Decisions:      []string{"用户裁定"},
	})
	run := &types.ReadingRun{
		ID: "legacy-run", TenantID: 1, UserID: "owner", SessionID: "session-1",
		DocumentIDs: types.JSON("[]"),
		Status:      types.ReadingRunFinished, Plan: types.JSON(planJSON), Notes: types.JSON(notesJSON),
		Final: types.JSON("{}"), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(run).Error)

	require.NoError(t, svc.maybeMigrateLegacyRun(ctx, run))

	var migrated types.ReadingRun
	require.NoError(t, db.First(&migrated, "id = ?", run.ID).Error)
	var plan types.ReadingPlan
	require.NoError(t, json.Unmarshal(migrated.Plan, &plan))
	require.Equal(t, readingStructuredStateVersion, plan.StateVersion)

	conclusions, err := svc.repo.ListConclusions(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, conclusions, 2)
	for _, conclusion := range conclusions {
		require.Equal(t, types.ReadingConclusionConfirmed, conclusion.Status)
	}

	terms, err := svc.repo.ListTerms(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, terms, 1)
	require.Equal(t, "术语甲", terms[0].Term)
	require.Equal(t, 0, terms[0].Round)
	require.Equal(t, types.ReadingTermCandidate, terms[0].Status)

	questions, err := svc.repo.ListQuestions(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, questions, 1)
	require.Equal(t, types.ReadingQuestionPending, questions[0].Status)

	contradictions, err := svc.repo.ListContradictions(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, contradictions, 1)

	// Re-running the migration must stay idempotent.
	require.NoError(t, svc.maybeMigrateLegacyRun(ctx, &migrated))
	terms, err = svc.repo.ListTerms(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, terms, 1)
}

func TestMaybeMigrateLegacyRunLeavesActiveOrStructuredRunsUntouched(t *testing.T) {
	svc, db := newLegacyReadingTestService(t)
	ctx := context.Background()

	activePlan, _ := json.Marshal(types.ReadingPlan{StateVersion: 1})
	active := &types.ReadingRun{
		ID: "active-run", TenantID: 1, UserID: "owner", SessionID: "session-1",
		DocumentIDs: types.JSON("[]"),
		Status:      types.ReadingRunReading, Plan: types.JSON(activePlan), Notes: types.JSON("{}"),
		Final: types.JSON("{}"), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(active).Error)
	require.NoError(t, svc.maybeMigrateLegacyRun(ctx, active))

	structuredPlan, _ := json.Marshal(types.ReadingPlan{StateVersion: readingStructuredStateVersion})
	structured := &types.ReadingRun{
		ID: "structured-run", TenantID: 1, UserID: "owner", SessionID: "session-1",
		DocumentIDs: types.JSON("[]"),
		Status:      types.ReadingRunFinished, Plan: types.JSON(structuredPlan), Notes: types.JSON("{}"),
		Final: types.JSON("{}"), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(structured).Error)
	require.NoError(t, svc.maybeMigrateLegacyRun(ctx, structured))

	for id, wantVersion := range map[string]int{
		"active-run":     1,
		"structured-run": readingStructuredStateVersion,
	} {
		var stored types.ReadingRun
		require.NoError(t, db.First(&stored, "id = ?", id).Error)
		var plan types.ReadingPlan
		require.NoError(t, json.Unmarshal(stored.Plan, &plan))
		require.Equal(t, wantVersion, plan.StateVersion, id)
		terms, err := svc.repo.ListTerms(ctx, id)
		require.NoError(t, err)
		require.Empty(t, terms, id)
	}
}
