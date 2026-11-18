package types

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"
)

// Review card lifecycle and source constants. The status is the only state the
// scheduler consults; source_updated is a flag, not a status.
const (
	ReviewGroupDefaultName = "默认卡组"

	ReviewCardTypeQA = "qa"

	ReviewStatusActive    = "active"
	ReviewStatusSuspended = "suspended"
	ReviewStatusOrphaned  = "orphaned"
	ReviewStatusDeleted   = "deleted"

	ReviewSourceTypeKnowledge = "knowledge_chunk"
	ReviewSourceTypeMessage   = "session_message"

	ReviewSourceStatusActive      = "active"
	ReviewSourceStatusUnavailable = "unavailable"

	ReviewRatingForgot     = "forgot"
	ReviewRatingFuzzy      = "fuzzy"
	ReviewRatingRemembered = "remembered"
	ReviewRatingEasy       = "easy"

	// Card vulnerability levels. None means the card has never been reviewed
	// and is not judged healthy.
	ReviewCardLevelSevere  = "severe"
	ReviewCardLevelWarning = "warning"
	ReviewCardLevelObserve = "observe"
	ReviewCardLevelHealthy = "healthy"
	ReviewCardLevelNone    = "none"

	ReviewSessionModeDaily  = "daily"
	ReviewSessionModeManual = "manual"

	ReviewSessionStatusActive    = "active"
	ReviewSessionStatusCompleted = "completed"
	ReviewSessionStatusAbandoned = "abandoned"
	ReviewSessionStatusExpired   = "expired"
)

// ReviewScheduleIntervals maps level L0-L6 to the next interval in days.
var ReviewScheduleIntervals = [...]int{1, 3, 7, 15, 30, 60, 120}

// ReviewGroup is one node of the multi-level card group tree.
type ReviewGroup struct {
	ID        string         `json:"id"         gorm:"type:varchar(64);primaryKey"`
	TenantID  uint64         `json:"tenant_id"  gorm:"not null;index"`
	ParentID  string         `json:"parent_id"  gorm:"type:varchar(64);not null;default:''"`
	Name      string         `json:"name"       gorm:"type:varchar(200);not null"`
	SortOrder int            `json:"sort_order" gorm:"not null;default:0"`
	IsDefault bool           `json:"is_default" gorm:"not null;default:false"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"          gorm:"index"`
}

func (ReviewGroup) TableName() string { return "review_groups" }

// ReviewCard is one knowledge point. One card owns one schedule and one or
// more sources; the source list is loaded separately.
type ReviewCard struct {
	ID             string     `json:"id"              gorm:"type:varchar(64);primaryKey"`
	TenantID       uint64     `json:"tenant_id"       gorm:"not null;index"`
	GroupID        string     `json:"group_id"        gorm:"type:varchar(64);not null;index"`
	CardType       string     `json:"card_type"       gorm:"type:varchar(20);not null;default:'qa'"`
	Question       string     `json:"question"        gorm:"type:text;not null"`
	Answer         string     `json:"answer"          gorm:"type:text;not null"`
	Explanation    string     `json:"explanation"     gorm:"type:text;not null;default:''"`
	Tags           JSON       `json:"tags"            gorm:"type:jsonb;not null"`
	Status         string     `json:"status"          gorm:"type:varchar(20);not null;default:'active'"`
	ReviewLevel    int        `json:"review_level"    gorm:"not null;default:0"`
	Stability      float64    `json:"stability"      gorm:"not null;default:10"`
	ForgetStreak   int        `json:"forget_streak"  gorm:"not null;default:0"`
	DueAt          time.Time  `json:"due_at"          gorm:"not null"`
	Lapses         int        `json:"lapses"          gorm:"not null;default:0"`
	ReviewCount    int        `json:"review_count"    gorm:"not null;default:0"`
	QuestionHash   string     `json:"question_hash"   gorm:"type:varchar(64);not null"`
	LastReviewedAt *time.Time `json:"last_reviewed_at"`
	SourceUpdated  bool       `json:"source_updated"  gorm:"not null;default:false"`
	// Vulnerability is a computed single-card level for list and detail
	// responses. It is never persisted.
	Vulnerability string `json:"vulnerability,omitempty" gorm:"-"`
	// VulnerabilityReason is a readable explanation of the computed level.
	VulnerabilityReason string         `json:"vulnerability_reason,omitempty" gorm:"-"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `json:"-"               gorm:"index"`
}

func (ReviewCard) TableName() string { return "review_cards" }

// ReviewCardSource anchors a card to the material it came from. Knowledge
// sources are rebindable; message sources are background snapshots only.
type ReviewCardSource struct {
	ID          string    `json:"id"            gorm:"type:varchar(64);primaryKey"`
	TenantID    uint64    `json:"tenant_id"     gorm:"not null;index"`
	CardID      string    `json:"card_id"       gorm:"type:varchar(64);not null;index"`
	SourceType  string    `json:"source_type"   gorm:"type:varchar(24);not null"`
	KnowledgeID string    `json:"knowledge_id"  gorm:"type:varchar(64);not null;default:''"`
	ChunkID     string    `json:"chunk_id"      gorm:"type:varchar(64);not null;default:''"`
	MessageID   string    `json:"message_id"    gorm:"type:varchar(64);not null;default:''"`
	Path        string    `json:"path"          gorm:"type:varchar(1024);not null;default:''"`
	Symbol      string    `json:"symbol"        gorm:"type:varchar(256);not null;default:''"`
	StartLine   int       `json:"start_line"    gorm:"not null;default:0"`
	EndLine     int       `json:"end_line"      gorm:"not null;default:0"`
	ContentHash string    `json:"content_hash"  gorm:"type:varchar(64);not null;default:''"`
	Excerpt     string    `json:"excerpt"       gorm:"type:text;not null;default:''"`
	IsPrimary   bool      `json:"is_primary"    gorm:"not null;default:false"`
	Status      string    `json:"status"        gorm:"type:varchar(20);not null;default:'active'"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (ReviewCardSource) TableName() string { return "review_card_sources" }

// ReviewLog is one rating submission. ClientRequestID makes retries
// idempotent; ReviewedAt is the server timestamp used for day accounting.
type ReviewLog struct {
	ID              string    `json:"id"                gorm:"type:varchar(64);primaryKey"`
	TenantID        uint64    `json:"tenant_id"         gorm:"not null;uniqueIndex:uk_review_logs_request,priority:1"`
	CardID          string    `json:"card_id"           gorm:"type:varchar(64);not null;index"`
	Rating          string    `json:"rating"            gorm:"type:varchar(20);not null"`
	LevelBefore     int       `json:"level_before"      gorm:"not null"`
	LevelAfter      int       `json:"level_after"       gorm:"not null"`
	IntervalDays    int       `json:"interval_days"     gorm:"not null"`
	SessionID       string    `json:"session_id"        gorm:"type:varchar(64);not null;default:'';index"`
	StabilityBefore float64   `json:"stability_before"  gorm:"not null;default:0"`
	StabilityAfter  float64   `json:"stability_after"   gorm:"not null;default:0"`
	ReviewedAt      time.Time `json:"reviewed_at"       gorm:"not null"`
	ClientRequestID string    `json:"client_request_id" gorm:"type:varchar(64);not null;uniqueIndex:uk_review_logs_request,priority:2"`
	CreatedAt       time.Time `json:"created_at"`
}

func (ReviewLog) TableName() string { return "review_logs" }

// ReviewSession is one bounded review run. PlannedCardIDs preserves the order
// and composition so an interrupted session can resume or be reported exactly.
type ReviewSession struct {
	ID              string     `json:"id"                  gorm:"type:varchar(64);primaryKey"`
	TenantID        uint64     `json:"tenant_id"           gorm:"not null;index"`
	Mode            string     `json:"mode"                gorm:"type:varchar(20);not null"`
	Status          string     `json:"status"              gorm:"type:varchar(20);not null;default:'active'"`
	GroupID         string     `json:"group_id"            gorm:"type:varchar(64);not null;default:''"`
	PlannedCardIDs  JSON       `json:"planned_card_ids"    gorm:"type:jsonb;not null"`
	PlannedCount    int        `json:"planned_count"       gorm:"not null;default:0"`
	CompletedCount  int        `json:"completed_count"     gorm:"not null;default:0"`
	RememberedCount int        `json:"remembered_count"    gorm:"not null;default:0"`
	EasyCount       int        `json:"easy_count"          gorm:"not null;default:0"`
	FuzzyCount      int        `json:"fuzzy_count"         gorm:"not null;default:0"`
	ForgotCount     int        `json:"forgot_count"        gorm:"not null;default:0"`
	ReviewDay       time.Time  `json:"review_day"          gorm:"not null"`
	StartedAt       time.Time  `json:"started_at"          gorm:"not null"`
	CompletedAt     *time.Time `json:"completed_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (ReviewSession) TableName() string { return "review_sessions" }

type StartReviewSessionRequest struct {
	Mode    string   `json:"mode"`
	GroupID string   `json:"group_id"`
	CardIDs []string `json:"card_ids"`
	Limit   int      `json:"limit"`
	// ForceNew abandons an active session before starting. Without it an
	// existing active session is reported as a conflict instead of silently
	// resumed, so the UI can ask continue-or-replace.
	ForceNew bool `json:"force_new"`
}

type ReviewSessionState struct {
	Session *ReviewSession `json:"session"`
	Cards   []*ReviewCard  `json:"cards"`
	Logs    []*ReviewLog   `json:"logs"`
}

type ReviewRatingCounts struct {
	Easy       int `json:"easy"`
	Remembered int `json:"remembered"`
	Fuzzy      int `json:"fuzzy"`
	Forgot     int `json:"forgot"`
	Total      int `json:"total"`
}

type ReviewCardPerformance struct {
	Card             *ReviewCard        `json:"card"`
	Counts           ReviewRatingCounts `json:"counts"`
	RecentCounts     ReviewRatingCounts `json:"recent_counts"`
	RecentForgetRate float64            `json:"recent_forget_rate"`
	ForgetStreak     int                `json:"forget_streak"`
	Warning          string             `json:"warning"`
	WarningReason    string             `json:"warning_reason"`
	Logs             []*ReviewLog       `json:"logs"`
}

// ReviewGroupNode is the tree projection returned by the group list.
type ReviewGroupNode struct {
	Group     *ReviewGroup       `json:"group"`
	Children  []*ReviewGroupNode `json:"children"`
	CardCount int                `json:"card_count"`
	DueCount  int                `json:"due_count"`
}

// ReviewCardSourceInput is the save-time source shape.
type ReviewCardSourceInput struct {
	SourceType  string `json:"source_type"`
	KnowledgeID string `json:"knowledge_id"`
	ChunkID     string `json:"chunk_id"`
	MessageID   string `json:"message_id"`
	Path        string `json:"path"`
	Symbol      string `json:"symbol"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	ContentHash string `json:"content_hash"`
	Excerpt     string `json:"excerpt"`
	IsPrimary   bool   `json:"is_primary"`
}

// CreateReviewGroupRequest creates one group under ParentID (root when empty).
type CreateReviewGroupRequest struct {
	ParentID  string `json:"parent_id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type UpdateReviewGroupRequest struct {
	Name      *string `json:"name"`
	SortOrder *int    `json:"sort_order"`
}

type CreateReviewCardRequest struct {
	GroupID     string                  `json:"group_id"`
	Question    string                  `json:"question"`
	Answer      string                  `json:"answer"`
	Explanation string                  `json:"explanation"`
	Tags        []string                `json:"tags"`
	Sources     []ReviewCardSourceInput `json:"sources"`
}

type UpdateReviewCardRequest struct {
	Question    *string  `json:"question"`
	Answer      *string  `json:"answer"`
	Explanation *string  `json:"explanation"`
	Tags        []string `json:"tags"`
}

type ReviewCardFilter struct {
	GroupID        string
	IncludeSubtree bool
	GroupIDs       []string
	Statuses       []string
	Keyword        string
	Tag            string
	DueOnly        bool
	Limit          int
	Offset         int
}

type ReviewCardDetail struct {
	Card    *ReviewCard         `json:"card"`
	Sources []*ReviewCardSource `json:"sources"`
}

// ReviewQueue is today's bounded workload. Due and New are already capped by
// the caller's limit; CompletedToday counts distinct cards.
type ReviewQueue struct {
	Due            []*ReviewCard `json:"due"`
	New            []*ReviewCard `json:"new"`
	CompletedToday int           `json:"completed_today"`
	Limit          int           `json:"limit"`
	NewLimit       int           `json:"new_limit"`
	StreakDays     int           `json:"streak_days"`
}

type SubmitReviewRequest struct {
	CardID          string `json:"card_id"`
	SessionID       string `json:"session_id"`
	Rating          string `json:"rating"`
	ClientRequestID string `json:"client_request_id"`
}

type MergeReviewCardsRequest struct {
	TargetCardID  string   `json:"target_card_id"`
	SourceCardIDs []string `json:"source_card_ids"`
}

type RebindReviewCardSourceRequest struct {
	KnowledgeID string `json:"knowledge_id"`
	ChunkID     string `json:"chunk_id"`
	Path        string `json:"path"`
	Symbol      string `json:"symbol"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	ContentHash string `json:"content_hash"`
	Excerpt     string `json:"excerpt"`
}

type ReviewStats struct {
	Days               int                      `json:"days"`
	TotalCards         int64                    `json:"total_cards"`
	Reviewed           int64                    `json:"reviewed"`
	Remembered         int64                    `json:"remembered"`
	Retention          float64                  `json:"retention"`
	RatingDistribution ReviewRatingCounts       `json:"rating_distribution"`
	GroupRanking       []*ReviewGroupStat       `json:"group_ranking"`
	DifficultCards     []*ReviewCardStat        `json:"difficult_cards"`
	DueForecast        []*ReviewDueForecast     `json:"due_forecast"`
	StreakDays         int                      `json:"streak_days"`
	RatingTrend        []*ReviewRatingTrend     `json:"rating_trend"`
	ForgettingCurve    []*ReviewForgettingPoint `json:"forgetting_curve"`
	Reviewed30d        int64                    `json:"reviewed_30d"`
	Remembered30d      int64                    `json:"remembered_30d"`
	Easy30d            int64                    `json:"easy_30d"`
	Retention30d       float64                  `json:"retention_30d"`
	WeakTopics         []*ReviewWeakTopic       `json:"weak_topics"`
}

type ReviewGroupStat struct {
	GroupID   string  `json:"group_id"`
	GroupName string  `json:"group_name"`
	Reviewed  int64   `json:"reviewed"`
	Retention float64 `json:"retention"`
}

type ReviewCardStat struct {
	CardID       string  `json:"card_id"`
	Question     string  `json:"question"`
	GroupID      string  `json:"group_id"`
	GroupName    string  `json:"group_name"`
	Easy         int64   `json:"easy"`
	Remembered   int64   `json:"remembered"`
	Fuzzy        int64   `json:"fuzzy"`
	Forgot       int64   `json:"forgot"`
	Total        int64   `json:"total"`
	ForgetRate   float64 `json:"forget_rate"`
	Lapses       int     `json:"lapses"`
	ForgetStreak int     `json:"forget_streak"`
}

type ReviewDueForecast struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type ReviewRatingTrend struct {
	Date       string  `json:"date"`
	Easy       int64   `json:"easy"`
	Remembered int64   `json:"remembered"`
	Fuzzy      int64   `json:"fuzzy"`
	Forgot     int64   `json:"forgot"`
	Total      int64   `json:"total"`
	Retention  float64 `json:"retention"`
}

type ReviewForgettingPoint struct {
	Day         int     `json:"day"`
	Actual      float64 `json:"actual"`
	Theoretical float64 `json:"theoretical"`
	Sample      int64   `json:"sample"`
}

type ReviewWeakTopic struct {
	GroupID   string  `json:"group_id"`
	GroupName string  `json:"group_name"`
	Reviewed  int64   `json:"reviewed"`
	Retention float64 `json:"retention"`
}

// ReviewHeatmap is the server-side vulnerability aggregate. Rows are card
// groups at the next layer under the selected scope, columns are tags with a
// knowledge-base fallback, and cells are their intersection clusters.
// Multi-tag cards appear in several columns, so the intensity percentages do
// not add up.
type ReviewHeatmap struct {
	Rows            []*ReviewHeatmapRow    `json:"rows"`
	Columns         []*ReviewHeatmapColumn `json:"columns"`
	Cells           []*ReviewHeatmapCell   `json:"cells"`
	Priorities      []*ReviewHeatmapCell   `json:"priorities"`
	ObserveCount    int                    `json:"observe_count"`
	Insight         *ReviewHeatmapInsight  `json:"insight"`
	ScopeGroupID    string                 `json:"scope_group_id"`
	ScopePath       string                 `json:"scope_path"`
	RowTotal        int                    `json:"row_total"`
	ColumnTotal     int                    `json:"column_total"`
	RowTruncated    bool                   `json:"row_truncated"`
	ColumnTruncated bool                   `json:"column_truncated"`
	Formula         *ReviewHeatmapFormula  `json:"formula"`
}

type ReviewHeatmapOptions struct {
	GroupID     string
	RowLimit    int
	ColumnLimit int
	Expand      bool
}

const (
	ReviewHeatmapRowKindGroup  = "group"
	ReviewHeatmapRowKindDirect = "direct"
	ReviewHeatmapRowKindOther  = "other"
)

type ReviewHeatmapRow struct {
	ID         string `json:"id"`
	GroupID    string `json:"group_id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Drillable  bool   `json:"drillable"`
	Virtual    bool   `json:"virtual"`
	TotalCards int    `json:"total_cards"`
}

const (
	ReviewHeatmapColumnKindTag           = "tag"
	ReviewHeatmapColumnKindKnowledgeBase = "knowledge_base"
	ReviewHeatmapColumnKindSource        = "source"
	ReviewHeatmapColumnKindUncategorized = "uncategorized"
	ReviewHeatmapOtherRowID              = "other_row"
	ReviewHeatmapOtherColumnKey          = "other_column"
)

type ReviewHeatmapColumn struct {
	Key             string `json:"key"`
	Title           string `json:"title"`
	Kind            string `json:"kind"`
	SourceID        string `json:"source_id"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
}

type ReviewHeatmapCell struct {
	RowID     string  `json:"row_id"`
	ColumnKey string  `json:"column_key"`
	Level     string  `json:"level"`
	Intensity float64 `json:"intensity"`
	Total     int     `json:"total"`
	Severe    int     `json:"severe"`
	Warning   int     `json:"warning"`
	Observe   int     `json:"observe"`
	Healthy   int     `json:"healthy"`
	NoData    int     `json:"no_data"`
	Clickable bool    `json:"clickable"`
}

type ReviewHeatmapFormula struct {
	SevereWeight  float64 `json:"severe_weight"`
	WarningWeight float64 `json:"warning_weight"`
	ObserveWeight float64 `json:"observe_weight"`
	Denominator   int     `json:"denominator"`
	Text          string  `json:"text"`
}

type ReviewHeatmapInsight struct {
	Kind      string `json:"kind"`
	RowID     string `json:"row_id"`
	ColumnKey string `json:"column_key"`
	Message   string `json:"message"`
}

type ReviewClusterDetailRequest struct {
	ScopeGroupID string
	RowGroupID   string
	ColumnKey    string
	Direct       bool
}

type ReviewClusterDetail struct {
	Cell         *ReviewHeatmapCell    `json:"cell"`
	Row          *ReviewHeatmapRow     `json:"row"`
	Column       *ReviewHeatmapColumn  `json:"column"`
	Cards        []*ReviewClusterCard  `json:"cards"`
	Reasons      []string              `json:"reasons"`
	Formula      *ReviewHeatmapFormula `json:"formula"`
	ScopeGroupID string                `json:"scope_group_id"`
}

type ReviewClusterCard struct {
	Card            *ReviewCard       `json:"card"`
	Level           string            `json:"level"`
	LastRating      string            `json:"last_rating"`
	SuggestedAction string            `json:"suggested_action"`
	Source          *ReviewCardSource `json:"source"`
	KnowledgeBaseID string            `json:"knowledge_base_id"`
}

type GenerateCardVariantsRequest struct {
	Count int `json:"count"`
}

type ApplyCardSplitRequest struct {
	Cards []CreateReviewCardRequest `json:"cards"`
}

type ApplyCardSplitResult struct {
	Created          []*ReviewCard `json:"created"`
	ArchivedOriginal bool          `json:"archived_original"`
}

type ReviewRootCauseRequest struct {
	ScopeGroupID string `json:"scope_group_id"`
	RowGroupID   string `json:"row_group_id"`
	ColumnKey    string `json:"column_key"`
	Direct       bool   `json:"direct"`
}

type ReviewRootCause struct {
	Summary          string   `json:"summary"`
	Reasons          []string `json:"reasons"`
	FocusSuggestions []string `json:"focus_suggestions"`
	EvidenceCardIDs  []string `json:"evidence_card_ids"`
	ModelID          string   `json:"model_id"`
	Source           string   `json:"source"`
}

const (
	ReviewBatchActionDelete  = "delete"
	ReviewBatchActionMove    = "move"
	ReviewBatchActionAddTags = "add_tags"
	ReviewBatchActionReset   = "reset"
)

type BatchReviewCardsRequest struct {
	CardIDs []string `json:"card_ids"`
	Action  string   `json:"action"`
	GroupID string   `json:"group_id"`
	Tags    []string `json:"tags"`
}

type BatchReviewCardsResult struct {
	Affected int `json:"affected"`
}

type GenerateReviewCardsRequest struct {
	KnowledgeBaseID string   `json:"knowledge_base_id"`
	KnowledgeIDs    []string `json:"knowledge_ids"`
	GroupID         string   `json:"group_id"`
	Count           int      `json:"count"`
}

type GeneratedReviewCard struct {
	Question          string                  `json:"question"`
	Answer            string                  `json:"answer"`
	Explanation       string                  `json:"explanation"`
	SourceKnowledgeID string                  `json:"source_knowledge_id"`
	SourceChunkID     string                  `json:"source_chunk_id"`
	SourceTitle       string                  `json:"source_title"`
	SourceExcerpt     string                  `json:"source_excerpt"`
	Duplicate         bool                    `json:"duplicate"`
	ExistingCardID    string                  `json:"existing_card_id"`
	Sources           []ReviewCardSourceInput `json:"sources"`
}

type ReviewGenerationPreview struct {
	Cards   []*GeneratedReviewCard `json:"cards"`
	ModelID string                 `json:"model_id"`
}

type ImportReviewCardsRequest struct {
	GroupID string                    `json:"group_id"`
	Cards   []CreateReviewCardRequest `json:"cards"`
}

type ReviewExport struct {
	ExportedAt time.Time           `json:"exported_at"`
	Groups     []*ReviewGroup      `json:"groups"`
	Cards      []*ReviewCard       `json:"cards"`
	Sources    []*ReviewCardSource `json:"sources"`
	Logs       []*ReviewLog        `json:"logs"`
	Sessions   []*ReviewSession    `json:"sessions"`
}

// NormalizeReviewQuestion makes the hash insensitive to whitespace, case and
// punctuation. It is intentionally conservative: wording changes still create
// a separate hash and are handled by the later trigram/vector layers.
func NormalizeReviewQuestion(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(text)) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func HashReviewQuestion(text string) string {
	sum := sha256.Sum256([]byte(NormalizeReviewQuestion(text)))
	return hex.EncodeToString(sum[:])
}
