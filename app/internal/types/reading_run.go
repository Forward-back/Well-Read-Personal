package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ReadingRunStatus is the fixed lifecycle of one interactive reading run.
type ReadingRunStatus string

const (
	ReadingRunPlanning    ReadingRunStatus = "planning"
	ReadingRunReading     ReadingRunStatus = "reading"
	ReadingRunWaitingUser ReadingRunStatus = "waiting_user"
	ReadingRunFinished    ReadingRunStatus = "finished"
	ReadingRunStopped     ReadingRunStatus = "stopped"
	ReadingRunAbandoned   ReadingRunStatus = "abandoned"
	ReadingRunFailed      ReadingRunStatus = "failed"
)

const (
	ReadingDepthQuick    = "quick"
	ReadingDepthStandard = "standard"
	ReadingDepthDeep     = "deep"

	ReadingPolicyGrillWithDocs = "grill-with-docs"
)

// Reading run phases for state_version=3 runs. They coexist with Status:
// phase advances through the agent pipeline, status keeps the user-facing
// lifecycle including waiting_user.
const (
	ReadingPhaseStructure = "structure"
	ReadingPhaseReading   = "reading"
	ReadingPhaseSynthesis = "synthesis"
	ReadingPhaseSelfTest  = "self_test"
	ReadingPhaseDone      = "done"
)

const (
	ReadingKnowledgeTypeConcept    = "concept"
	ReadingKnowledgeTypeArgument   = "argument"
	ReadingKnowledgeTypeProcess    = "process"
	ReadingKnowledgeTypeComparison = "comparison"
	ReadingKnowledgeTypeExample    = "example"
	ReadingKnowledgeTypeReference  = "reference"
	ReadingKnowledgeTypeUnknown    = "unknown"
)

// ReadingSectionStructure is one section's one-sentence thesis and knowledge
// type produced by the structure pre-read. Missing sections fall back to the
// heading and unknown at render time.
type ReadingSectionStructure struct {
	SectionID     string `json:"section_id"`
	Thesis        string `json:"thesis"`
	KnowledgeType string `json:"knowledge_type"`
}

// ReadingChunkSegment is one chunk fragment rendered into a reading window.
type ReadingChunkSegment struct {
	ID      string `json:"id"`
	Heading string `json:"heading,omitempty"`
	Content string `json:"content"`
}

// ReadingSection is one readable window of the selected documents.
type ReadingSection struct {
	ID              string                `json:"id"`
	KnowledgeID     string                `json:"knowledge_id"`
	KnowledgeBaseID string                `json:"knowledge_base_id,omitempty"`
	Title           string                `json:"title"`
	Heading         string                `json:"heading,omitempty"`
	Index           int                   `json:"index"`
	Content         string                `json:"content"`
	ChunkIDs        []string              `json:"chunk_ids,omitempty"`
	Segments        []ReadingChunkSegment `json:"segments,omitempty"`
	Read            bool                  `json:"read"`
}

// ReadingPlan is the ordered section list. section_id values are stable for
// one run so read progress survives step polling/reloads.
type ReadingPlan struct {
	Goal              string                    `json:"goal,omitempty"`
	Depth             string                    `json:"depth,omitempty"`
	Policy            string                    `json:"policy,omitempty"`
	KnowledgeBaseID   string                    `json:"knowledge_base_id,omitempty"`
	KnowledgeBaseIDs  []string                  `json:"knowledge_base_ids,omitempty"`
	StateVersion      int                       `json:"state_version,omitempty"`
	Structure         []ReadingSectionStructure `json:"structure,omitempty"`
	StructureDegraded bool                      `json:"structure_degraded,omitempty"`
	StructureError    string                    `json:"structure_error,omitempty"`
	Sections          []ReadingSection          `json:"sections"`
}

// ReadingQuestion is the single checkpoint question for a step.
type ReadingQuestion struct {
	Text              string `json:"text"`
	RecommendedAnswer string `json:"recommended_answer"`
	Why               string `json:"why,omitempty"`
}

// ReadingNotes is the incrementally maintained reading notebook.
type ReadingNotes struct {
	Confirmed      []string `json:"confirmed"`
	Terms          []string `json:"terms"`
	OpenQuestions  []string `json:"open_questions"`
	Contradictions []string `json:"contradictions"`
	Decisions      []string `json:"decisions"`
	Citations      []string `json:"citations"`
}

// ReadingNotesPatch is the LLM's per-step patch, merged into ReadingNotes.
type ReadingNotesPatch struct {
	Confirmed      []string `json:"confirmed"`
	Terms          []string `json:"terms"`
	OpenQuestions  []string `json:"open_questions"`
	Contradictions []string `json:"contradictions"`
	Decisions      []string `json:"decisions"`
	Citations      []string `json:"citations"`
}

// UnmarshalJSON accepts both strings and objects in note arrays. Models often
// return terms as {term, definition} objects even when the prompt asks for
// strings; rejecting that would fail the whole reading step.
func (p *ReadingNotesPatch) UnmarshalJSON(data []byte) error {
	var raw struct {
		Confirmed      []any `json:"confirmed"`
		Terms          []any `json:"terms"`
		OpenQuestions  []any `json:"open_questions"`
		Contradictions []any `json:"contradictions"`
		Decisions      []any `json:"decisions"`
		Citations      []any `json:"citations"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Confirmed = readingNoteStrings(raw.Confirmed)
	p.Terms = readingNoteStrings(raw.Terms)
	p.OpenQuestions = readingNoteStrings(raw.OpenQuestions)
	p.Contradictions = readingNoteStrings(raw.Contradictions)
	p.Decisions = readingNoteStrings(raw.Decisions)
	p.Citations = readingNoteStrings(raw.Citations)
	return nil
}

func readingNoteStrings(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if text := readingNoteString(value); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func readingNoteString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		parts := make([]string, 0, 4)
		for _, key := range []string{"term", "name", "title", "text", "definition", "description", "summary", "value"} {
			if text, ok := typed[key].(string); ok && strings.TrimSpace(text) != "" {
				parts = append(parts, strings.TrimSpace(text))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "：")
		}
		encoded, _ := json.Marshal(typed)
		return string(encoded)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

// ReadingRun is one interactive reading session.
type ReadingRun struct {
	ID                string                  `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID          uint64                  `json:"tenant_id" gorm:"not null;index"`
	SessionID         string                  `json:"session_id" gorm:"type:varchar(36);not null;default:''"`
	UserID            string                  `json:"user_id" gorm:"type:varchar(36);not null;default:''"`
	MessageID         string                  `json:"message_id" gorm:"type:varchar(36);not null;default:''"`
	DocumentIDs       JSON                    `json:"document_ids" gorm:"type:jsonb;not null"`
	SkillID           string                  `json:"skill_id" gorm:"type:varchar(36);not null;default:''"`
	ReadingModelID    string                  `json:"reading_model_id" gorm:"type:varchar(36);not null;default:''"`
	ExtractionModelID string                  `json:"extraction_model_id" gorm:"type:varchar(36);not null;default:''"`
	SynthesisModelID  string                  `json:"synthesis_model_id" gorm:"type:varchar(36);not null;default:''"`
	Phase             string                  `json:"phase" gorm:"type:varchar(16);not null;default:''"`
	Status            ReadingRunStatus        `json:"status" gorm:"type:varchar(16);not null;default:'planning'"`
	Plan              JSON                    `json:"plan" gorm:"type:jsonb;not null"`
	Notes             JSON                    `json:"notes" gorm:"type:jsonb;not null"`
	Final             JSON                    `json:"final" gorm:"type:jsonb;not null"`
	CurrentSectionID  string                  `json:"current_section_id" gorm:"type:varchar(64);not null;default:''"`
	StepCount         int                     `json:"step_count" gorm:"not null;default:0"`
	TokenUsage        int                     `json:"token_usage" gorm:"not null;default:0"`
	Steps             []*ReadingRunStep       `json:"steps,omitempty" gorm:"-"`
	Questions         []*ReadingQuestionNode  `json:"questions,omitempty" gorm:"-"`
	Conclusions       []*ReadingConclusion    `json:"conclusions,omitempty" gorm:"-"`
	Terms             []*ReadingTerm          `json:"terms,omitempty" gorm:"-"`
	Contradictions    []*ReadingContradiction `json:"contradictions,omitempty" gorm:"-"`
	Understandings    []*ReadingUnderstanding `json:"understandings,omitempty" gorm:"-"`
	// Quiet mirrors the distilled task memory for state_version=3 runs so the
	// workbench can show the quiet badge without a second request.
	Quiet     bool      `json:"quiet,omitempty" gorm:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ReadingRun) TableName() string { return "reading_runs" }

// ReadingRunStep is one executed action.
type ReadingRunStep struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	RunID        string    `json:"run_id" gorm:"type:varchar(36);not null;index"`
	Seq          int       `json:"seq" gorm:"not null"`
	Action       string    `json:"action" gorm:"type:varchar(24);not null"`
	SectionID    string    `json:"section_id" gorm:"type:varchar(64);not null;default:''"`
	Question     JSON      `json:"question" gorm:"type:jsonb;not null"`
	UserFeedback string    `json:"user_feedback" gorm:"type:text;not null;default:''"`
	NotesPatch   JSON      `json:"notes_patch" gorm:"type:jsonb;not null"`
	Citations    JSON      `json:"citations" gorm:"type:jsonb;not null"`
	Status       string    `json:"status" gorm:"type:varchar(16);not null;default:'done'"`
	CreatedAt    time.Time `json:"created_at"`
}

func (ReadingRunStep) TableName() string { return "reading_run_steps" }

const (
	ReadingQuestionPending      = "pending"
	ReadingQuestionAutoAnswered = "auto_answered"
	ReadingQuestionAnswered     = "answered"
	ReadingQuestionContradicted = "contradicted"
	ReadingQuestionResolved     = "resolved"
	ReadingQuestionSkipped      = "skipped"
)

// ReadingEvidence is a verifiable citation to a chunk in a selected document.
// Quote is validated against the chunk content before it is persisted.
type ReadingEvidence struct {
	KnowledgeID   string `json:"knowledge_id"`
	DocumentTitle string `json:"document_title,omitempty"`
	ChunkID       string `json:"chunk_id"`
	SectionID     string `json:"section_id,omitempty"`
	Heading       string `json:"heading,omitempty"`
	Quote         string `json:"quote"`
}

// ReadingQuestionOption is one candidate answer shown for a checkpoint
// question. Exactly one option is normally recommended.
type ReadingQuestionOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Text        string `json:"text"`
	Recommended bool   `json:"recommended,omitempty"`
}

const (
	ReadingQuestionJudgmentUnjudged = ""
	ReadingQuestionJudgmentCorrect  = "correct"
	ReadingQuestionJudgmentPartial  = "partial"
	ReadingQuestionJudgmentWrong    = "wrong"
)

// ReadingQuestionNode is one question in the reading question tree.
type ReadingQuestionNode struct {
	ID                string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	RunID             string     `json:"run_id" gorm:"type:varchar(36);not null;index"`
	TenantID          uint64     `json:"tenant_id" gorm:"not null"`
	ParentID          string     `json:"parent_id" gorm:"type:varchar(36);not null;default:''"`
	SectionID         string     `json:"section_id" gorm:"type:varchar(64);not null;default:''"`
	Round             int        `json:"round" gorm:"not null;default:0"`
	Text              string     `json:"text" gorm:"type:text;not null;default:''"`
	RecommendedAnswer string     `json:"recommended_answer" gorm:"type:text;not null;default:''"`
	Why               string     `json:"why" gorm:"type:text;not null;default:''"`
	SearchQuery       string     `json:"search_query" gorm:"type:text;not null;default:''"`
	Kind              string     `json:"kind" gorm:"type:varchar(16);not null;default:''"`
	GapType           string     `json:"gap_type" gorm:"type:varchar(24);not null;default:''"`
	Options           JSON       `json:"options" gorm:"type:jsonb;not null"`
	Evidence          JSON       `json:"evidence" gorm:"type:jsonb;not null"`
	Answer            string     `json:"answer" gorm:"type:text;not null;default:''"`
	AnswerSource      string     `json:"answer_source" gorm:"type:varchar(16);not null;default:''"`
	Judgment          string     `json:"judgment" gorm:"type:varchar(16);not null;default:''"`
	JudgmentReason    string     `json:"judgment_reason" gorm:"type:text;not null;default:''"`
	JudgedAt          *time.Time `json:"judged_at,omitempty"`
	Status            string     `json:"status" gorm:"type:varchar(20);not null;default:'pending'"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (ReadingQuestionNode) TableName() string { return "reading_questions" }

const (
	ReadingConclusionCandidate    = "candidate"
	ReadingConclusionConfirmed    = "confirmed"
	ReadingConclusionContradicted = "contradicted"
	ReadingConclusionRejected     = "rejected"

	// ReadingConclusionSourceLegacy marks conclusions migrated from the old
	// notes JSON. They have no structured evidence and are never counted as
	// verified by the current three-gate rule.
	ReadingConclusionSourceLegacy = "legacy"
)

// ReadingConclusionGates records the three convergence gates and whether the
// conclusion came from an explicit user adjudication.
type ReadingConclusionGates struct {
	ReferencesOK       bool `json:"references_ok"`
	ConflictFree       bool `json:"conflict_free"`
	CrossDocConsistent bool `json:"cross_doc_consistent"`
	UserAdjudicated    bool `json:"user_adjudicated,omitempty"`
}

// ReadingConclusion is one evidence-backed conclusion.
type ReadingConclusion struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	RunID     string    `json:"run_id" gorm:"type:varchar(36);not null;index"`
	TenantID  uint64    `json:"tenant_id" gorm:"not null"`
	SectionID string    `json:"section_id" gorm:"type:varchar(64);not null;default:''"`
	Round     int       `json:"round" gorm:"not null;default:0"`
	Text      string    `json:"text" gorm:"type:text;not null;default:''"`
	Evidence  JSON      `json:"evidence" gorm:"type:jsonb;not null"`
	Status    string    `json:"status" gorm:"type:varchar(20);not null;default:'candidate'"`
	Source    string    `json:"source" gorm:"type:varchar(16);not null;default:'model'"`
	Gates     JSON      `json:"gates" gorm:"type:jsonb;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ReadingConclusion) TableName() string { return "reading_conclusions" }

const (
	ReadingTermCandidate = "candidate"
	ReadingTermConfirmed = "confirmed"
	ReadingTermRejected  = "rejected"
)

// ReadingTerm is one glossary entry with a source.
type ReadingTerm struct {
	ID         string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	RunID      string    `json:"run_id" gorm:"type:varchar(36);not null;index"`
	TenantID   uint64    `json:"tenant_id" gorm:"not null"`
	Round      int       `json:"round" gorm:"not null;default:0"`
	Term       string    `json:"term" gorm:"type:varchar(200);not null;default:''"`
	Definition string    `json:"definition" gorm:"type:text;not null;default:''"`
	Source     JSON      `json:"source" gorm:"type:jsonb;not null"`
	Canonical  bool      `json:"canonical" gorm:"not null;default:false"`
	Status     string    `json:"status" gorm:"type:varchar(20);not null;default:'candidate'"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (ReadingTerm) TableName() string { return "reading_terms" }

const (
	ReadingContradictionOpen     = "open"
	ReadingContradictionResolved = "resolved"
	ReadingContradictionClosed   = "closed"
)

// ReadingContradiction records a conflict between two pieces of evidence.
type ReadingContradiction struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	RunID       string    `json:"run_id" gorm:"type:varchar(36);not null;index"`
	TenantID    uint64    `json:"tenant_id" gorm:"not null"`
	Description string    `json:"description" gorm:"type:text;not null;default:''"`
	EvidenceA   JSON      `json:"evidence_a" gorm:"type:jsonb;not null"`
	EvidenceB   JSON      `json:"evidence_b" gorm:"type:jsonb;not null"`
	Status      string    `json:"status" gorm:"type:varchar(20);not null;default:'open'"`
	Resolution  string    `json:"resolution" gorm:"type:text;not null;default:''"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (ReadingContradiction) TableName() string { return "reading_contradictions" }

// CreateReadingRunRequest starts a run over 1..5 explicitly selected docs.
type CreateReadingRunRequest struct {
	SessionID   string   `json:"session_id" binding:"required"`
	DocumentIDs []string `json:"document_ids" binding:"required"`
	SkillID     string   `json:"skill_id"`
	Goal        string   `json:"goal"`
	Depth       string   `json:"depth"`
	MessageID   string   `json:"message_id"`
}

// ReadingFeedbackRequest records the user's answer to a checkpoint question.
type ReadingFeedbackRequest struct {
	Feedback   string `json:"feedback"`
	QuestionID string `json:"question_id"`
	OptionID   string `json:"option_id"`
	Skip       bool   `json:"skip"`
}

// ReadingContradictionResolutionRequest records a user adjudication of a
// contradiction. Choice is "a", "b", or "custom".
type ReadingContradictionResolutionRequest struct {
	Choice     string `json:"choice"`
	Resolution string `json:"resolution"`
}

// ReadingReportDraft is a user-edited Markdown report for one structured run.
type ReadingReportDraft struct {
	RunID     string    `json:"run_id" gorm:"type:varchar(36);primaryKey"`
	TenantID  uint64    `json:"tenant_id" gorm:"not null;index"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);not null"`
	ContentMD string    `json:"content_md" gorm:"type:text;not null;default:''"`
	Version   int       `json:"version" gorm:"not null;default:0"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ReadingReportDraft) TableName() string { return "reading_report_drafts" }

// ReadingReport is the report editor payload. Edited reports override the
// Markdown export; JSON export always projects the structured tables.
type ReadingReport struct {
	Content   string     `json:"content"`
	Edited    bool       `json:"edited"`
	Version   int        `json:"version"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// SaveReadingReportRequest replaces the edited Markdown report for a run.
// BaseVersion is the version the editor loaded; 0 means create the first draft.
type SaveReadingReportRequest struct {
	Content     string `json:"content" binding:"required"`
	BaseVersion int    `json:"base_version"`
}

// AddReadingDocumentsRequest explicitly adds documents (max 5 total).
type AddReadingDocumentsRequest struct {
	DocumentIDs []string `json:"document_ids" binding:"required"`
}

// ReadingDraft is one reusable reading configuration.
type ReadingDraft struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID    uint64    `json:"tenant_id" gorm:"not null;index"`
	UserID      string    `json:"user_id" gorm:"type:varchar(36);not null;index"`
	Name        string    `json:"name" gorm:"type:varchar(120);not null"`
	DocumentIDs JSON      `json:"document_ids" gorm:"type:jsonb;not null"`
	Depth       string    `json:"depth" gorm:"type:varchar(16);not null;default:'standard'"`
	Goal        string    `json:"goal" gorm:"type:text;not null;default:''"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (ReadingDraft) TableName() string { return "reading_drafts" }

// ReadingDraftRequest creates or updates a draft. DocumentIDs is required on
// create; an update may omit it to keep the stored selection.
type ReadingDraftRequest struct {
	Name        string   `json:"name"`
	DocumentIDs []string `json:"document_ids"`
	Depth       string   `json:"depth"`
	Goal        string   `json:"goal"`
}

const (
	ReadingUnderstandingPending   = "pending"
	ReadingUnderstandingConfirmed = "confirmed"
	ReadingUnderstandingCorrected = "corrected"
	ReadingUnderstandingSkipped   = "skipped"
)

// ReadingUnderstanding persists the user's decision on one round's
// understanding statement so it can influence the next round and survive a
// reload.
type ReadingUnderstanding struct {
	ID         string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	RunID      string    `json:"run_id" gorm:"type:varchar(36);not null;index"`
	TenantID   uint64    `json:"tenant_id" gorm:"not null"`
	SectionID  string    `json:"section_id" gorm:"type:varchar(64);not null;default:''"`
	Round      int       `json:"round" gorm:"not null;default:0"`
	Text       string    `json:"text" gorm:"type:text;not null;default:''"`
	Decision   string    `json:"decision" gorm:"type:varchar(16);not null;default:'pending'"`
	Correction string    `json:"correction" gorm:"type:text;not null;default:''"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (ReadingUnderstanding) TableName() string { return "reading_understandings" }

// UpdateReadingUnderstandingRequest confirms, corrects, or skips the current
// round's understanding statement.
type UpdateReadingUnderstandingRequest struct {
	Decision   string `json:"decision" binding:"required"`
	Correction string `json:"correction"`
}

// UpdateReadingNoteRequest updates one note's text or review status.
type UpdateReadingNoteRequest struct {
	Status string `json:"status"`
	Text   string `json:"text"`
}

// JumpReadingSectionRequest moves the reading cursor back to a section. The
// target and all later sections become unread so "continue from this chapter"
// actually resumes there.
type JumpReadingSectionRequest struct {
	SectionID string `json:"section_id" binding:"required"`
}

// ReadingRunMemory is the per-run distilled state used by state_version=3.
// Exactly one row per run; each field is replaced as a whole, never appended.
type ReadingRunMemory struct {
	RunID     string    `json:"run_id" gorm:"type:varchar(36);primaryKey"`
	TenantID  uint64    `json:"tenant_id" gorm:"not null;index"`
	Knowledge JSON      `json:"knowledge" gorm:"type:jsonb;not null"`
	UserModel JSON      `json:"user_model" gorm:"type:jsonb;not null"`
	Task      JSON      `json:"task" gorm:"type:jsonb;not null"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ReadingRunMemory) TableName() string { return "reading_run_memories" }

const (
	ReadingSelfTestCandidate = "candidate"
	ReadingSelfTestImported  = "imported"
	ReadingSelfTestRejected  = "rejected"
)

// ReadingSelfTest is one generated self-test question with its evidence.
type ReadingSelfTest struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	RunID       string    `json:"run_id" gorm:"type:varchar(36);not null;index"`
	TenantID    uint64    `json:"tenant_id" gorm:"not null"`
	SectionID   string    `json:"section_id" gorm:"type:varchar(64);not null;default:''"`
	Question    string    `json:"question" gorm:"type:text;not null;default:''"`
	Answer      string    `json:"answer" gorm:"type:text;not null;default:''"`
	Explanation string    `json:"explanation" gorm:"type:text;not null;default:''"`
	Evidence    JSON      `json:"evidence" gorm:"type:jsonb;not null"`
	Status      string    `json:"status" gorm:"type:varchar(16);not null;default:'candidate'"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (ReadingSelfTest) TableName() string { return "reading_self_tests" }

// ImportReadingSelfTestsRequest imports selected self-tests into a review
// group. The server re-checks run ownership and candidate status.
type ImportReadingSelfTestsRequest struct {
	GroupID     string   `json:"group_id" binding:"required"`
	SelfTestIDs []string `json:"self_test_ids" binding:"required"`
}

// ReadingSelfTestImportResult reports how many candidates became review cards
// and how many were skipped because they were already imported, rejected, or
// no longer candidates.
type ReadingSelfTestImportResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

// ReadingCoverage summarizes how much of the selected reading plan was covered.
type ReadingCoverage struct {
	Documents     int `json:"documents"`
	TotalSections int `json:"total_sections"`
	ReadSections  int `json:"read_sections"`
	StepCount     int `json:"step_count"`
	TokenUsage    int `json:"token_usage"`
}

// ReadingExport is the server-built, permission-checked reading report payload.
type ReadingExport struct {
	Format      string          `json:"format"`
	Filename    string          `json:"filename"`
	Content     string          `json:"content"`
	Coverage    ReadingCoverage `json:"coverage"`
	GeneratedAt time.Time       `json:"generated_at"`
}

// ReadingDocumentSection is the plan metadata shown alongside one full
// document. It intentionally has no chunk boundaries: the left pane renders
// the complete document, not the reading windows.
type ReadingDocumentSection struct {
	ID      string `json:"id"`
	Title   string `json:"title,omitempty"`
	Heading string `json:"heading,omitempty"`
	Read    bool   `json:"read"`
	// Start/End are UTF-16 offsets into ReadingDocument.Content, matching
	// JavaScript string indices used by the left-pane highlighter.
	Start int `json:"start"`
	End   int `json:"end"`
}

// ReadingDocument is one selected document rendered as full text.
type ReadingDocument struct {
	ID       string                   `json:"id"`
	Title    string                   `json:"title"`
	FileName string                   `json:"file_name,omitempty"`
	Content  string                   `json:"content"`
	Sections []ReadingDocumentSection `json:"sections,omitempty"`
}

type ReadingRunDocuments struct {
	Documents []ReadingDocument `json:"documents"`
}
