package interfaces

import (
	"context"

	"gaze/internal/types"
)

// ReadingRunRepository persists interactive reading runs and steps.
type ReadingRunRepository interface {
	CreateRun(ctx context.Context, run *types.ReadingRun) error
	GetRun(ctx context.Context, tenantID uint64, userID, runID string) (*types.ReadingRun, error)
	GetActiveRun(ctx context.Context, tenantID uint64, userID, sessionID string) (*types.ReadingRun, error)
	ListHistoryRuns(ctx context.Context, tenantID uint64, userID string, statuses []types.ReadingRunStatus, limit int) ([]*types.ReadingRun, error)
	UpdateRun(ctx context.Context, run *types.ReadingRun) error
	StopActiveRuns(ctx context.Context, tenantID uint64, userID, sessionID string) error
	CreateStep(ctx context.Context, step *types.ReadingRunStep) error
	ListSteps(ctx context.Context, runID string) ([]*types.ReadingRunStep, error)
	UpdateStep(ctx context.Context, step *types.ReadingRunStep) error
	ListQuestions(ctx context.Context, runID string) ([]*types.ReadingQuestionNode, error)
	UpdateQuestion(ctx context.Context, question *types.ReadingQuestionNode) error
	ListConclusions(ctx context.Context, runID string) ([]*types.ReadingConclusion, error)
	UpdateConclusion(ctx context.Context, conclusion *types.ReadingConclusion) error
	ListTerms(ctx context.Context, runID string) ([]*types.ReadingTerm, error)
	UpdateTerm(ctx context.Context, term *types.ReadingTerm) error
	ListContradictions(ctx context.Context, runID string) ([]*types.ReadingContradiction, error)
	ListUnderstandings(ctx context.Context, runID string) ([]*types.ReadingUnderstanding, error)
	SaveUnderstanding(ctx context.Context, understanding *types.ReadingUnderstanding) error
	UpdateUnderstanding(ctx context.Context, understanding *types.ReadingUnderstanding) error
	GetMemory(ctx context.Context, runID string) (*types.ReadingRunMemory, error)
	SaveMemory(ctx context.Context, memory *types.ReadingRunMemory) error
	ListSelfTests(ctx context.Context, runID string) ([]*types.ReadingSelfTest, error)
	// CreateSelfTests inserts candidate self-tests; duplicate IDs are ignored so
	// regenerating after a partial failure cannot double-insert.
	CreateSelfTests(ctx context.Context, tests []*types.ReadingSelfTest) (int, error)
	// UpdateSelfTestStatus moves a candidate to imported/rejected. Returns false
	// when the test is missing or no longer a candidate.
	UpdateSelfTestStatus(
		ctx context.Context, runID, selfTestID, status string,
	) (bool, error)
	CreateFeynmanAttempt(ctx context.Context, attempt *types.ReadingFeynmanAttempt) error
	ListFeynmanAttempts(ctx context.Context, runID, sectionID string) ([]*types.ReadingFeynmanAttempt, error)
	CreateAnnotation(ctx context.Context, annotation *types.ReadingAnnotation) error
	ListAnnotations(ctx context.Context, runID string) ([]*types.ReadingAnnotation, error)
	GetAnnotation(ctx context.Context, runID, annotationID string) (*types.ReadingAnnotation, error)
	DeleteAnnotation(ctx context.Context, runID, annotationID string) error
	ListDrafts(ctx context.Context, tenantID uint64, userID string) ([]*types.ReadingDraft, error)
	GetDraft(ctx context.Context, tenantID uint64, userID, draftID string) (*types.ReadingDraft, error)
	CreateDraft(ctx context.Context, draft *types.ReadingDraft) error
	UpdateDraft(ctx context.Context, draft *types.ReadingDraft) error
	DeleteDraft(ctx context.Context, tenantID uint64, userID, draftID string) error
	// SaveRound commits one reading round: run state, structured rows, and the
	// round log must land together or not at all.
	SaveRound(
		ctx context.Context, run *types.ReadingRun,
		questions []*types.ReadingQuestionNode,
		conclusions []*types.ReadingConclusion,
		terms []*types.ReadingTerm,
		contradictions []*types.ReadingContradiction,
		step *types.ReadingRunStep,
	) error
	// SaveFeedback commits a user answer and its derived conclusion.
	SaveFeedback(
		ctx context.Context, run *types.ReadingRun,
		question *types.ReadingQuestionNode,
		conclusion *types.ReadingConclusion,
	) error
	// ResolveContradiction commits a user adjudication, the derived decision
	// conclusion, and any conclusions promoted to the chosen side.
	ResolveContradiction(
		ctx context.Context,
		contradiction *types.ReadingContradiction,
		conclusion *types.ReadingConclusion,
		updatedConclusions []*types.ReadingConclusion,
	) error
	GetReportDraft(
		ctx context.Context, tenantID uint64, userID, runID string,
	) (*types.ReadingReportDraft, error)
	// SaveReportDraft applies optimistic concurrency: baseVersion 0 creates the
	// first draft, a positive baseVersion updates only when the stored version
	// still matches. Returns false when another editor won the race.
	SaveReportDraft(
		ctx context.Context, draft *types.ReadingReportDraft, baseVersion int,
	) (bool, error)
	// DeleteReportDraft deletes the draft when version < 0 (force) or when the
	// stored version matches. Returns false when a versioned delete conflicts.
	DeleteReportDraft(
		ctx context.Context, tenantID uint64, userID, runID string, version int,
	) (bool, error)
	// ListLegacyFinishedRuns returns finished runs that still use the old notes
	// JSON state. The batch migration filters state_version in Go.
	ListLegacyFinishedRuns(ctx context.Context, limit int) ([]*types.ReadingRun, error)
	// MigrateLegacyRun writes migrated rows and the updated plan/final in one
	// transaction; inserts are idempotent for deterministic IDs.
	MigrateLegacyRun(
		ctx context.Context, run *types.ReadingRun,
		questions []*types.ReadingQuestionNode,
		conclusions []*types.ReadingConclusion,
		terms []*types.ReadingTerm,
		contradictions []*types.ReadingContradiction,
	) error
}

// ReadingRunService is the interactive reading orchestrator.
type ReadingRunService interface {
	Create(ctx context.Context, req *types.CreateReadingRunRequest) (*types.ReadingRun, error)
	Get(ctx context.Context, runID string) (*types.ReadingRun, error)
	GetActive(ctx context.Context, sessionID string) (*types.ReadingRun, error)
	ListHistory(ctx context.Context, limit int) ([]*types.ReadingHistoryItem, error)
	Step(ctx context.Context, runID string) (*types.ReadingRun, error)
	Feedback(ctx context.Context, runID string, req *types.ReadingFeedbackRequest) (*types.ReadingRun, error)
	JudgeQuestion(ctx context.Context, runID, questionID string) (*types.ReadingRun, error)
	ResolveContradiction(
		ctx context.Context, runID, contradictionID string,
		req *types.ReadingContradictionResolutionRequest,
	) (*types.ReadingRun, error)
	GetReport(ctx context.Context, runID string) (*types.ReadingReport, error)
	SaveReport(ctx context.Context, runID string, req *types.SaveReadingReportRequest) (*types.ReadingReport, error)
	DeleteReport(ctx context.Context, runID string, baseVersion int) (*types.ReadingReport, error)
	GetDocuments(ctx context.Context, runID string) (*types.ReadingRunDocuments, error)
	MigrateLegacyFinishedRuns(ctx context.Context, limit int) (int, error)
	AddDocuments(ctx context.Context, runID string, req *types.AddReadingDocumentsRequest) (*types.ReadingRun, error)
	Stop(ctx context.Context, runID string) (*types.ReadingRun, error)
	Abandon(ctx context.Context, runID string) (*types.ReadingRun, error)
	Resume(ctx context.Context, runID string) (*types.ReadingRun, error)
	Jump(ctx context.Context, runID string, req *types.JumpReadingSectionRequest) (*types.ReadingRun, error)
	UpdateUnderstanding(
		ctx context.Context, runID string,
		req *types.UpdateReadingUnderstandingRequest,
	) (*types.ReadingRun, error)
	UpdateTerm(
		ctx context.Context, runID, termID string,
		req *types.UpdateReadingNoteRequest,
	) (*types.ReadingTerm, error)
	UpdateConclusion(
		ctx context.Context, runID, conclusionID string,
		req *types.UpdateReadingNoteRequest,
	) (*types.ReadingConclusion, error)
	ListDrafts(ctx context.Context) ([]*types.ReadingDraft, error)
	CreateDraft(ctx context.Context, req *types.ReadingDraftRequest) (*types.ReadingDraft, error)
	UpdateDraft(ctx context.Context, draftID string, req *types.ReadingDraftRequest) (*types.ReadingDraft, error)
	DeleteDraft(ctx context.Context, draftID string) error
	Export(ctx context.Context, runID string, format string) (*types.ReadingExport, error)
	ListSelfTests(ctx context.Context, runID string) ([]*types.ReadingSelfTest, error)
	ImportSelfTests(
		ctx context.Context, runID string, req *types.ImportReadingSelfTestsRequest,
	) (*types.ReadingSelfTestImportResult, error)
	SubmitFeynman(
		ctx context.Context, runID string, req *types.SubmitReadingFeynmanRequest,
	) (*types.ReadingFeynmanAttempt, error)
	ListFeynman(
		ctx context.Context, runID, sectionID string,
	) ([]*types.ReadingFeynmanAttempt, error)
	CreateAnnotation(
		ctx context.Context, runID string, req *types.CreateReadingAnnotationRequest,
	) (*types.ReadingAnnotation, error)
	DeleteAnnotation(ctx context.Context, runID, annotationID string) error
	CreateCardFromAnnotation(
		ctx context.Context, runID, annotationID string,
		req *types.CreateReadingAnnotationCardRequest,
	) (*types.ReadingAnnotationCardResult, error)
}
