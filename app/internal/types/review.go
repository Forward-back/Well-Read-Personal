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
	ID             string         `json:"id"              gorm:"type:varchar(64);primaryKey"`
	TenantID       uint64         `json:"tenant_id"       gorm:"not null;index"`
	GroupID        string         `json:"group_id"        gorm:"type:varchar(64);not null;index"`
	CardType       string         `json:"card_type"       gorm:"type:varchar(20);not null;default:'qa'"`
	Question       string         `json:"question"        gorm:"type:text;not null"`
	Answer         string         `json:"answer"          gorm:"type:text;not null"`
	Explanation    string         `json:"explanation"     gorm:"type:text;not null;default:''"`
	Tags           JSON           `json:"tags"            gorm:"type:jsonb;not null"`
	Status         string         `json:"status"          gorm:"type:varchar(20);not null;default:'active'"`
	ReviewLevel    int            `json:"review_level"    gorm:"not null;default:0"`
	DueAt          time.Time      `json:"due_at"          gorm:"not null"`
	Lapses         int            `json:"lapses"          gorm:"not null;default:0"`
	ReviewCount    int            `json:"review_count"    gorm:"not null;default:0"`
	QuestionHash   string         `json:"question_hash"   gorm:"type:varchar(64);not null"`
	LastReviewedAt *time.Time     `json:"last_reviewed_at"`
	SourceUpdated  bool           `json:"source_updated"  gorm:"not null;default:false"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-"               gorm:"index"`
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
	ReviewedAt      time.Time `json:"reviewed_at"       gorm:"not null"`
	ClientRequestID string    `json:"client_request_id" gorm:"type:varchar(64);not null;uniqueIndex:uk_review_logs_request,priority:2"`
	CreatedAt       time.Time `json:"created_at"`
}

func (ReviewLog) TableName() string { return "review_logs" }

// ReviewGroupNode is the tree projection returned by the group list.
type ReviewGroupNode struct {
	Group    *ReviewGroup       `json:"group"`
	Children []*ReviewGroupNode `json:"children"`
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
}

type SubmitReviewRequest struct {
	CardID          string `json:"card_id"`
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
	TotalCards    int64   `json:"total_cards"`
	Reviewed30d   int64   `json:"reviewed_30d"`
	Remembered30d int64   `json:"remembered_30d"`
	Retention30d  float64 `json:"retention_30d"`
}

type ReviewExport struct {
	ExportedAt time.Time           `json:"exported_at"`
	Groups     []*ReviewGroup      `json:"groups"`
	Cards      []*ReviewCard       `json:"cards"`
	Sources    []*ReviewCardSource `json:"sources"`
	Logs       []*ReviewLog        `json:"logs"`
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
