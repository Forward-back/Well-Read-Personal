package interfaces

import (
	"context"
	"time"

	"gaze/internal/types"
)

// ReviewRepository persists review groups, cards, sources, and logs. Every
// method is tenant-scoped; callers must not pass cross-tenant IDs.
type ReviewRepository interface {
	EnsureDefaultGroup(ctx context.Context, tenantID uint64) (*types.ReviewGroup, error)
	ListGroups(ctx context.Context, tenantID uint64) ([]*types.ReviewGroup, error)
	GetGroup(ctx context.Context, tenantID uint64, id string) (*types.ReviewGroup, error)
	CreateGroup(ctx context.Context, group *types.ReviewGroup) error
	UpdateGroup(ctx context.Context, group *types.ReviewGroup) error
	DeleteGroup(ctx context.Context, tenantID uint64, id string, deleteCards bool) error

	CreateCard(ctx context.Context, card *types.ReviewCard, sources []*types.ReviewCardSource) error
	GetCard(ctx context.Context, tenantID uint64, id string) (*types.ReviewCard, error)
	GetCardWithSources(ctx context.Context, tenantID uint64, id string) (*types.ReviewCardDetail, error)
	ListCards(ctx context.Context, tenantID uint64, filter *types.ReviewCardFilter) ([]*types.ReviewCard, error)
	UpdateCard(ctx context.Context, card *types.ReviewCard) error
	MoveCard(ctx context.Context, tenantID uint64, cardID, groupID string) error
	SetCardStatus(ctx context.Context, tenantID uint64, cardID, status string) error
	SoftDeleteCard(ctx context.Context, tenantID uint64, cardID string) error
	RestoreCard(ctx context.Context, tenantID uint64, cardID string) error
	FindCardByQuestionHash(ctx context.Context, tenantID uint64, hash string) (*types.ReviewCard, error)

	AddSources(ctx context.Context, tenantID uint64, cardID string, sources []*types.ReviewCardSource) error
	ListSources(ctx context.Context, tenantID uint64, cardID string) ([]*types.ReviewCardSource, error)
	UpdateSource(ctx context.Context, tenantID uint64, source *types.ReviewCardSource) error
	MarkSourceUnavailable(ctx context.Context, tenantID uint64, cardID, sourceID string) error
	CountActiveKnowledgeSources(ctx context.Context, tenantID uint64, cardID string) (int64, error)
	MergeCards(ctx context.Context, tenantID uint64, targetCardID string, sourceCardIDs []string) error
	ListAllCards(ctx context.Context, tenantID uint64) ([]*types.ReviewCard, error)
	ListAllSources(ctx context.Context, tenantID uint64) ([]*types.ReviewCardSource, error)
	ListAllLogs(ctx context.Context, tenantID uint64) ([]*types.ReviewLog, error)

	ListDueCards(ctx context.Context, tenantID uint64, groupIDs []string, now time.Time, limit int) ([]*types.ReviewCard, error)
	ListNewCards(ctx context.Context, tenantID uint64, groupIDs []string, limit int) ([]*types.ReviewCard, error)
	ListCardsByIDs(ctx context.Context, tenantID uint64, ids []string) ([]*types.ReviewCard, error)
	CountCompletedToday(ctx context.Context, tenantID uint64, from, to time.Time) (int64, error)
	BatchDeleteCards(ctx context.Context, tenantID uint64, cardIDs []string) error
	BatchMoveCards(ctx context.Context, tenantID uint64, cardIDs []string, groupID string) error
	BatchUpdateCardTags(ctx context.Context, tenantID uint64, tagsByCard map[string]types.JSON) error
	ResetCardSchedules(ctx context.Context, tenantID uint64, cardIDs []string, dueAt time.Time, stability float64) error

	// Sessions
	CreateSession(ctx context.Context, session *types.ReviewSession) error
	GetSession(ctx context.Context, tenantID uint64, id string) (*types.ReviewSession, error)
	GetActiveSession(ctx context.Context, tenantID uint64) (*types.ReviewSession, error)
	UpdateSession(ctx context.Context, session *types.ReviewSession) error
	ExpireSessions(ctx context.Context, tenantID uint64, before time.Time) error
	ListSessions(ctx context.Context, tenantID uint64, limit int) ([]*types.ReviewSession, error)
	ListLogsBySession(ctx context.Context, tenantID uint64, sessionID string) ([]*types.ReviewLog, error)
	ListLogsByCard(ctx context.Context, tenantID uint64, cardID string, limit int) ([]*types.ReviewLog, error)
	ListLogsByCardIDs(ctx context.Context, tenantID uint64, cardIDs []string) ([]*types.ReviewLog, error)
	FindLogBySessionCard(ctx context.Context, tenantID uint64, sessionID, cardID string) (*types.ReviewLog, error)

	// ApplyReview inserts one idempotent review log and updates the card in one
	// transaction. It returns false when the client request ID was already used
	// (the card must not be updated a second time).
	ApplyReview(ctx context.Context, card *types.ReviewCard, log *types.ReviewLog) (bool, error)
	// ApplyReviewSession additionally updates the session counters and closes
	// the session when its planned cards are complete.
	ApplyReviewSession(ctx context.Context, card *types.ReviewCard, log *types.ReviewLog, session *types.ReviewSession) (bool, error)
}

// ReviewService is the card-group and spaced-repetition domain service.
type ReviewService interface {
	EnsureDefaultGroup(ctx context.Context) (*types.ReviewGroup, error)
	ListGroupTree(ctx context.Context) ([]*types.ReviewGroupNode, error)
	CreateGroup(ctx context.Context, req *types.CreateReviewGroupRequest) (*types.ReviewGroup, error)
	UpdateGroup(ctx context.Context, id string, req *types.UpdateReviewGroupRequest) (*types.ReviewGroup, error)
	MoveGroup(ctx context.Context, id, newParentID string) error
	DeleteGroup(ctx context.Context, id string, deleteCards bool) error

	CreateCard(ctx context.Context, req *types.CreateReviewCardRequest) (*types.ReviewCard, error)
	GetCard(ctx context.Context, id string) (*types.ReviewCardDetail, error)
	ListCards(ctx context.Context, filter *types.ReviewCardFilter) ([]*types.ReviewCard, error)
	UpdateCard(ctx context.Context, id string, req *types.UpdateReviewCardRequest) (*types.ReviewCard, error)
	MoveCard(ctx context.Context, id, groupID string) error
	SetCardStatus(ctx context.Context, id, status string) error
	DeleteCard(ctx context.Context, id string) error
	RestoreCard(ctx context.Context, id string) error
	ResetCardSchedule(ctx context.Context, id string) (*types.ReviewCard, error)
	BatchCards(ctx context.Context, req *types.BatchReviewCardsRequest) (*types.BatchReviewCardsResult, error)
	Heatmap(ctx context.Context, groupID string) (*types.ReviewHeatmap, error)
	ClusterDetail(ctx context.Context, rowID, columnKey string) (*types.ReviewClusterDetail, error)

	TodayQueue(ctx context.Context, groupID string, limit int) (*types.ReviewQueue, error)
	StartSession(ctx context.Context, req *types.StartReviewSessionRequest) (*types.ReviewSessionState, error)
	GetActiveSession(ctx context.Context) (*types.ReviewSessionState, error)
	GetSession(ctx context.Context, id string) (*types.ReviewSessionState, error)
	AbandonSession(ctx context.Context, id string) error
	ListSessions(ctx context.Context, limit int) ([]*types.ReviewSession, error)
	GetCardPerformance(ctx context.Context, cardID string) (*types.ReviewCardPerformance, error)
	SubmitReview(ctx context.Context, req *types.SubmitReviewRequest) (*types.ReviewCard, error)
	MarkSourceUnavailable(ctx context.Context, cardID, sourceID string) error
	RebindSource(ctx context.Context, cardID, sourceID string, req *types.RebindReviewCardSourceRequest) error
	MergeCards(ctx context.Context, req *types.MergeReviewCardsRequest) error
	Stats(ctx context.Context, days int) (*types.ReviewStats, error)
	Export(ctx context.Context) (*types.ReviewExport, error)
	PreviewGeneration(ctx context.Context, req *types.GenerateReviewCardsRequest) (*types.ReviewGenerationPreview, error)
	ImportGeneratedCards(ctx context.Context, req *types.ImportReviewCardsRequest) (int, error)
}
