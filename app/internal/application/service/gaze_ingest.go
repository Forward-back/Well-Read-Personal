package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode/utf8"

	"gaze/internal/agent"
	apprepo "gaze/internal/application/repository"
	"gaze/internal/logger"
	"gaze/internal/models/chat"
	"gaze/internal/searchutil"
	"gaze/internal/tracing/langfuse"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// ErrGazeIngestConcurrent is returned by the gaze ingest handler in Lite mode
// when another run is already active for the same KB (the in-process
// liteLocks guard is held). The asynq/sync executor's RetryDelayFunc uses
// errors.Is on this sentinel to apply a short, fixed retry delay instead of
// exponential backoff.
var ErrGazeIngestConcurrent = errors.New("concurrent gaze task active")

const (
	// maxContentForGaze limits the document content sent to LLM for gaze generation
	maxContentForGaze = 32768

	// gazeLLMBackoffBase is the base delay for the exponential backoff
	// between retry attempts. The nth retry waits base << (n-1).
	gazeLLMBackoffBase = 2 * time.Second

	// gazeTaskType is the task_type stamp used in task_pending_ops and
	// task_dead_letters rows for this pipeline.
	gazeTaskType = "gaze:ingest"

	// gazeTaskScope is the scope used by both pending ops and dead letters.
	// Gaze ingest is per-KB, so every op is scoped to a knowledge_base.
	gazeTaskScope = types.TaskScopeKnowledgeBase

	// gazeIngestMaxRetry controls asynq retry budget for gaze:ingest tasks.
	gazeIngestMaxRetry = 10

	// gazeDeletedKeyPrefix is the Redis key prefix for "recently deleted
	// knowledge" tombstones. Key: gaze:deleted:{kbID}:{knowledgeID}.
	gazeDeletedKeyPrefix = "gaze:deleted:"

	// gazeDeletedTTL bounds how long we remember a deletion.
	gazeDeletedTTL = 1 * time.Hour

	// gazeLLMMaxAttempts is the total attempt count (initial + retries) for
	// every LLM call routed through generateWithTemplate.
	gazeLLMMaxAttempts = 3

	// gazeLLMMaxTokens is the completion-token budget for every LLM call
	// routed through generateWithTemplate.
	gazeLLMMaxTokens = 32768

	// gazeIngestDelay is how long to wait after a document is added before
	// the batch task fires. Debounces rapid uploads.
	gazeIngestDelay = 30 * time.Second

	// gazeMaxDocsPerRun caps how many documents a single task run processes.
	// Remaining ops stay in task_pending_ops and are picked up by the
	// follow-up task.
	gazeMaxDocsPerRun = 20

	// gazeIngestCleanupTimeout bounds detached tail cleanup after the asynq
	// task context has been cancelled or hit its timeout.
	gazeIngestCleanupTimeout = 10 * time.Second

	// gazeActiveLockTTL bounds the per-KB active lock so a crashed worker
	// cannot wedge a KB forever.
	gazeActiveLockTTL = 5 * time.Minute
)

// indexIntroSummaryCap caps how many summary pages we feed into the
// LLM when generating the gaze index intro from scratch.
const indexIntroSummaryCap = 200

// GazeDeletedTombstoneKey returns the Redis key used to mark a knowledge as
// recently deleted, so gaze_ingest tasks in flight can short-circuit.
func GazeDeletedTombstoneKey(kbID, knowledgeID string) string {
	return gazeDeletedKeyPrefix + kbID + ":" + knowledgeID
}

// GazeIngestPayload is the asynq task payload for gaze ingest batch trigger.
// The actual document IDs are stored in the task_pending_ops table; this
// payload only carries the trigger metadata so the worker can resolve
// the queue tuple (task_type, scope, scope_id) and process whatever rows
// are queued under it.
type GazeIngestPayload struct {
	types.TracingContext
	TenantID        uint64 `json:"tenant_id"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
	Language        string `json:"language,omitempty"`
}

// GazeRetractPayload is the asynq task payload for gaze content retraction
type GazeRetractPayload struct {
	types.TracingContext
	TenantID        uint64   `json:"tenant_id"`
	KnowledgeBaseID string   `json:"knowledge_base_id"`
	KnowledgeID     string   `json:"knowledge_id"`
	DocTitle        string   `json:"doc_title"`
	DocSummary      string   `json:"doc_summary,omitempty"` // one-line summary of the deleted document
	Language        string   `json:"language,omitempty"`
	PageSlugs       []string `json:"page_slugs"`
	FolderIDs       []string `json:"folder_ids,omitempty"`
}

const (
	GazeOpIngest  = "ingest"
	GazeOpRetract = "retract"
)

// GazePendingOp represents a single operation queued in task_pending_ops
// under task_type="gaze:ingest".
type GazePendingOp struct {
	Op          string `json:"op"`
	KnowledgeID string `json:"knowledge_id"`
	// Ingest fields
	Language string `json:"language,omitempty"`
	// Retract fields
	DocTitle   string   `json:"doc_title,omitempty"`
	DocSummary string   `json:"doc_summary,omitempty"`
	PageSlugs  []string `json:"page_slugs,omitempty"`
	FolderIDs  []string `json:"folder_ids,omitempty"`

	// dbID is set by peekPendingList from task_pending_ops.id.
	dbID int64 `json:"-"`
}

// gazeIngestService handles the LLM-powered gaze generation pipeline.
//
// Slim pipeline (vs. the historical Map/Reduce batch design): one task run
// processes up to gazeMaxDocsPerRun documents sequentially — per document it
// extracts entities, generates a summary page, and upserts the affected pages
// (entity pages merge via LLM, new single-source pages are written
// deterministically). No concept extraction, no cross-link injection, no
// dedup, no taxonomy planning, no chunk citations, no finalize lane.
type gazeIngestService struct {
	gazeService    interfaces.GazePageService
	kbService      interfaces.KnowledgeBaseService
	knowledgeSvc   interfaces.KnowledgeService
	knowledgeRepo  interfaces.KnowledgeRepository
	chunkRepo      interfaces.ChunkRepository
	modelService   interfaces.ModelService
	task           interfaces.TaskEnqueuer
	pendingRepo    interfaces.TaskPendingOpsRepository
	deadLetterRepo interfaces.TaskDeadLetterRepository
	redisClient    *redis.Client // nil in Lite mode (no Redis)
	spanTracker    SpanTracker
	// liteLocks provides per-KB mutual exclusion in Lite mode (no Redis).
	liteLocks sync.Map
	// llmRequests coalesces byte-identical concurrent prompts within this
	// process. Keys include tenant and model to preserve isolation.
	llmRequests singleflight.Group
	// promptWarmups serializes only the first request for a reusable Gaze page
	// prefix. Other prefixes and already-warmed cohorts stay parallel.
	promptWarmups sync.Map
}

type gazePromptWarmup struct {
	done chan struct{}
	once sync.Once
}

// NewGazeIngestService creates a new gaze ingest service
func NewGazeIngestService(
	gazeService interfaces.GazePageService,
	kbService interfaces.KnowledgeBaseService,
	knowledgeSvc interfaces.KnowledgeService,
	knowledgeRepo interfaces.KnowledgeRepository,
	chunkRepo interfaces.ChunkRepository,
	modelService interfaces.ModelService,
	task interfaces.TaskEnqueuer,
	pendingRepo interfaces.TaskPendingOpsRepository,
	deadLetterRepo interfaces.TaskDeadLetterRepository,
	redisClient *redis.Client,
	spanTracker SpanTracker,
) interfaces.TaskHandler {
	svc := &gazeIngestService{
		gazeService:    gazeService,
		kbService:      kbService,
		knowledgeSvc:   knowledgeSvc,
		knowledgeRepo:  knowledgeRepo,
		chunkRepo:      chunkRepo,
		modelService:   modelService,
		task:           task,
		pendingRepo:    pendingRepo,
		deadLetterRepo: deadLetterRepo,
		redisClient:    redisClient,
		spanTracker:    spanTracker,
	}
	return svc
}

// tracker returns a non-nil span tracker so callers don't have to
// nil-check on every Begin/End.
func (s *gazeIngestService) tracker() SpanTracker {
	if s.spanTracker == nil {
		return noopSpanTracker{}
	}
	return s.spanTracker
}

// beginGazeSubspan opens a postprocess.gaze subspan for this document
// under the knowledge's most recent attempt. Returns nil when there is
// no parse attempt to attach to.
func (s *gazeIngestService) beginGazeSubspan(ctx context.Context, knowledgeID string, input types.JSONMap) *Span {
	if knowledgeID == "" {
		return nil
	}
	attempt := s.tracker().LatestAttempt(ctx, knowledgeID)
	if attempt <= 0 {
		return nil
	}
	parent := s.tracker().LookupStage(ctx, knowledgeID, attempt, types.StagePostProcess)
	if parent == nil {
		return nil
	}
	return s.tracker().BeginSubSpan(ctx, parent, "postprocess.gaze", types.SpanKindSubSpan, input)
}

// EnqueueGazeIngest queues a document for gaze ingestion. The returned bool
// reports whether the durable pending op was persisted. A trigger enqueue
// error may therefore be returned together with true; callers can retry only
// the KB-scoped trigger without appending a duplicate operation.
func EnqueueGazeIngest(
	ctx context.Context,
	task interfaces.TaskEnqueuer,
	pendingRepo interfaces.TaskPendingOpsRepository,
	tenantID uint64,
	kbID, knowledgeID string,
) (bool, error) {
	pendingOp, err := newGazeIngestPendingOp(ctx, tenantID, kbID, knowledgeID)
	if err != nil {
		logger.Warnf(ctx, "gaze ingest: failed to marshal pending op for %s: %v", knowledgeID, err)
		return false, err
	}
	accepted, err := enqueueGazePendingOp(ctx, pendingRepo, pendingOp)
	if err != nil {
		logger.Warnf(ctx, "gaze ingest: failed to enqueue pending op for %s: %v", knowledgeID, err)
		return false, fmt.Errorf("enqueue gaze ingest pending op: %w", err)
	}
	if !accepted {
		logger.Infof(ctx, "gaze ingest: skip enqueue for deleted KB %s", kbID)
		return false, nil
	}
	if err := enqueueGazeIngestTrigger(ctx, task, tenantID, kbID); err != nil {
		return true, err
	}
	return true, nil
}

func enqueueGazePendingOp(
	ctx context.Context,
	pendingRepo interfaces.TaskPendingOpsRepository,
	op *types.TaskPendingOp,
) (bool, error) {
	if pendingRepo == nil {
		return true, nil
	}
	if guard, ok := pendingRepo.(interfaces.TaskPendingOpsKnowledgeBaseGuard); ok {
		return guard.EnqueueIfKnowledgeBaseActive(ctx, op)
	}
	if err := pendingRepo.Enqueue(ctx, op); err != nil {
		return false, err
	}
	return true, nil
}

func newGazeIngestPendingOp(
	ctx context.Context,
	tenantID uint64,
	kbID, knowledgeID string,
) (*types.TaskPendingOp, error) {
	lang := types.LanguageFromContextOrDefault(ctx)
	op := GazePendingOp{
		Op:          GazeOpIngest,
		KnowledgeID: knowledgeID,
		Language:    lang,
	}
	payloadBytes, err := json.Marshal(op)
	if err != nil {
		return nil, fmt.Errorf("marshal gaze ingest pending op: %w", err)
	}
	return &types.TaskPendingOp{
		TenantID: tenantID,
		TaskType: gazeTaskType,
		Scope:    gazeTaskScope,
		ScopeID:  kbID,
		Op:       GazeOpIngest,
		DedupKey: knowledgeID,
		Payload:  payloadBytes,
	}, nil
}

// gazeIngestTriggerTaskID coalesces concurrent trigger schedules for one KB
// into a single asynq task (asynq.TaskID dedupe), replacing the historical
// per-KB Redis lock as the primary single-flight mechanism.
func gazeIngestTriggerTaskID(kbID string) string {
	return "gaze-ingest-" + kbID
}

func enqueueGazeIngestTrigger(
	ctx context.Context,
	task interfaces.TaskEnqueuer,
	tenantID uint64,
	kbID string,
) error {
	lang := types.LanguageFromContextOrDefault(ctx)
	trigger := GazeIngestPayload{
		TenantID:        tenantID,
		KnowledgeBaseID: kbID,
		Language:        lang,
	}
	langfuse.InjectTracing(ctx, &trigger)
	triggerBytes, err := json.Marshal(trigger)
	if err != nil {
		logger.Warnf(ctx, "gaze ingest: failed to marshal trigger task: %v", err)
		return fmt.Errorf("marshal gaze ingest trigger: %w", err)
	}
	if task == nil {
		return errors.New("enqueue gaze ingest trigger: task enqueuer is nil")
	}

	t := asynq.NewTask(types.TypeGazeIngest, triggerBytes,
		asynq.Queue(types.QueueGaze),
		asynq.MaxRetry(gazeIngestMaxRetry),
		asynq.Timeout(60*time.Minute),
		asynq.ProcessIn(gazeIngestDelay),
		asynq.TaskID(gazeIngestTriggerTaskID(kbID)),
	)
	if _, err := task.Enqueue(t); err != nil {
		// A concurrent trigger for the same KB is expected: it will drain
		// the same pending rows.
		if errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask) {
			return nil
		}
		logger.Warnf(ctx, "gaze ingest: failed to enqueue trigger task: %v", err)
		return fmt.Errorf("enqueue gaze ingest trigger: %w", err)
	}
	return nil
}

// EnqueueGazeRetract queues a gaze retraction op (a delete cleanup).
func EnqueueGazeRetract(ctx context.Context, task interfaces.TaskEnqueuer,
	pendingRepo interfaces.TaskPendingOpsRepository, payload GazeRetractPayload,
) {
	_ = enqueueGazeRetract(ctx, task, pendingRepo, payload)
}

func enqueueGazeRetract(
	ctx context.Context,
	task interfaces.TaskEnqueuer,
	pendingRepo interfaces.TaskPendingOpsRepository,
	payload GazeRetractPayload,
) error {
	op := GazePendingOp{
		Op:          GazeOpRetract,
		KnowledgeID: payload.KnowledgeID,
		DocTitle:    payload.DocTitle,
		DocSummary:  payload.DocSummary,
		PageSlugs:   payload.PageSlugs,
		FolderIDs:   payload.FolderIDs,
		Language:    payload.Language,
	}
	payloadBytes, err := json.Marshal(op)
	if err != nil {
		logger.Warnf(ctx, "gaze retract: failed to marshal pending op: %v", err)
		return err
	}
	accepted, err := enqueueGazePendingOp(ctx, pendingRepo, &types.TaskPendingOp{
		TenantID: payload.TenantID,
		TaskType: gazeTaskType,
		Scope:    gazeTaskScope,
		ScopeID:  payload.KnowledgeBaseID,
		Op:       GazeOpRetract,
		DedupKey: payload.KnowledgeID,
		Payload:  payloadBytes,
	})
	if err != nil {
		logger.Warnf(ctx, "gaze retract: failed to enqueue pending op: %v", err)
		return err
	}
	if !accepted {
		logger.Infof(ctx, "gaze retract: skip enqueue for deleted KB %s", payload.KnowledgeBaseID)
		return nil
	}

	trigger := GazeIngestPayload{
		TenantID:        payload.TenantID,
		KnowledgeBaseID: payload.KnowledgeBaseID,
		Language:        payload.Language,
	}
	langfuse.InjectTracing(ctx, &trigger)
	triggerBytes, _ := json.Marshal(trigger)
	t := asynq.NewTask(types.TypeGazeIngest, triggerBytes,
		asynq.Queue(types.QueueGaze),
		asynq.MaxRetry(gazeIngestMaxRetry),
		asynq.Timeout(60*time.Minute),
		asynq.ProcessIn(5*time.Second), // Retract can trigger the batch quickly
		asynq.TaskID(gazeIngestTriggerTaskID(payload.KnowledgeBaseID)),
	)
	if _, err := task.Enqueue(t); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask) {
			return nil
		}
		logger.Warnf(ctx, "gaze retract: failed to enqueue trigger task: %v", err)
		return err
	}
	return nil
}

// Handle implements interfaces.TaskHandler for asynq task processing.
func (s *gazeIngestService) Handle(ctx context.Context, t *asynq.Task) error {
	return s.ProcessGazeIngest(ctx, t)
}

func gazeIngestCleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(ctx), gazeIngestCleanupTimeout)
}

func (s *gazeIngestService) clearDeletedKnowledgeBasePendingOps(ctx context.Context, kbID string) error {
	cleaner, ok := s.pendingRepo.(interfaces.TaskPendingOpsScopeCleaner)
	if !ok || kbID == "" {
		return nil
	}
	cleanupCtx, cancel := gazeIngestCleanupContext(ctx)
	defer cancel()
	return cleaner.DeleteByScope(cleanupCtx, types.TaskScopeKnowledgeBase, kbID)
}

func (s *gazeIngestService) peekPendingList(ctx context.Context, kbID string, limit int) (ops []GazePendingOp, peekedIDs []int64, err error) {
	if s.pendingRepo == nil {
		return nil, nil, nil
	}
	if limit <= 0 {
		limit = gazeMaxDocsPerRun
	}
	rows, err := s.pendingRepo.PeekBatch(ctx, gazeTaskType, gazeTaskScope, kbID, limit)
	if err != nil {
		return nil, nil, err
	}
	ops, peekedIDs = s.decodePendingRows(ctx, rows)
	return ops, peekedIDs, nil
}

func (s *gazeIngestService) decodePendingRows(ctx context.Context, rows []*types.TaskPendingOp) (ops []GazePendingOp, peekedIDs []int64) {
	ops = make([]GazePendingOp, 0, len(rows))
	peekedIDs = make([]int64, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		var op GazePendingOp
		if err := json.Unmarshal(row.Payload, &op); err != nil {
			logger.Warnf(ctx, "gaze ingest: skip undecodable pending row %d: %v", row.ID, err)
			continue
		}
		op.dbID = row.ID
		ops = append(ops, op)
		peekedIDs = append(peekedIDs, row.ID)
	}
	return ops, peekedIDs
}

func (s *gazeIngestService) trimPendingList(ctx context.Context, ids []int64) error {
	if s.pendingRepo == nil || len(ids) == 0 {
		return nil
	}
	return s.pendingRepo.DeleteByIDs(ctx, ids)
}

// acquireGazeKBLock takes the per-KB single-flight guard: an in-process lock
// in Lite mode, a Redis SETNX flag in standard mode. Released via the
// returned func.
func (s *gazeIngestService) acquireGazeKBLock(ctx context.Context, kbID string) (func(), error) {
	if s.redisClient == nil {
		if _, loaded := s.liteLocks.LoadOrStore(kbID, struct{}{}); loaded {
			return nil, ErrGazeIngestConcurrent
		}
		return func() { s.liteLocks.Delete(kbID) }, nil
	}
	key := "gaze:active:" + kbID
	ok, err := s.redisClient.SetNX(ctx, key, "1", gazeActiveLockTTL).Result()
	if err != nil {
		// Fail open: a Redis blip must not block gaze generation entirely.
		logger.Warnf(ctx, "gaze ingest: active-lock SetNX failed (fail open): %v", err)
		return func() {}, nil
	}
	if !ok {
		return nil, ErrGazeIngestConcurrent
	}
	return func() {
		s.redisClient.Del(context.WithoutCancel(ctx), key)
	}, nil
}

// ProcessGazeIngest drains the pending queue for one KB: per document it
// generates a summary page, extracts entities, and upserts the affected
// pages; afterwards it publishes drafts and rebuilds the index intro.
// Rows are consumed only on success, so a transient failure retries the
// whole run via asynq.
func (s *gazeIngestService) ProcessGazeIngest(ctx context.Context, t *asynq.Task) error {
	taskStartedAt := time.Now()
	var payload GazeIngestPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("gaze ingest: unmarshal payload: %w", err)
	}

	ctx = context.WithValue(ctx, types.TenantIDContextKey, payload.TenantID)
	if payload.Language != "" {
		ctx = context.WithValue(ctx, types.LanguageContextKey, payload.Language)
	}

	release, err := s.acquireGazeKBLock(ctx, payload.KnowledgeBaseID)
	if err != nil {
		logger.Infof(ctx, "gaze ingest: another run active for KB %s, deferring to asynq retry", payload.KnowledgeBaseID)
		return err
	}
	defer release()

	kb, err := s.kbService.GetKnowledgeBaseByIDOnly(ctx, payload.KnowledgeBaseID)
	if errors.Is(err, apprepo.ErrKnowledgeBaseNotFound) || (err == nil && kb == nil) {
		if cleanupErr := s.clearDeletedKnowledgeBasePendingOps(ctx, payload.KnowledgeBaseID); cleanupErr != nil {
			return fmt.Errorf("gaze ingest: clear deleted KB queue: %w", cleanupErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("gaze ingest: get KB: %w", err)
	}
	if !kb.IsGazeEnabled() {
		return fmt.Errorf("gaze ingest: KB %s is not gaze type", kb.ID)
	}

	var synthesisModelID string
	var contentInstructions string
	if kb.GazeConfig != nil {
		synthesisModelID = kb.GazeConfig.SynthesisModelID
		contentInstructions = kb.GazeConfig.ContentInstructions
	}
	if synthesisModelID == "" {
		synthesisModelID = kb.SummaryModelID
	}
	if synthesisModelID == "" {
		return fmt.Errorf("gaze ingest: no synthesis model configured for KB %s", kb.ID)
	}
	chatModel, err := s.modelService.GetChatModel(ctx, synthesisModelID)
	if err != nil {
		return fmt.Errorf("gaze ingest: get chat model: %w", err)
	}

	lang := types.LanguageNameFromContext(ctx)

	ops, peekedIDs, err := s.peekPendingList(ctx, payload.KnowledgeBaseID, gazeMaxDocsPerRun)
	if err != nil {
		return fmt.Errorf("gaze ingest: peek pending list: %w", err)
	}

	var changes []gazeIndexChange
	affectedSlugs := make([]string, 0, len(ops)*3)
	processed := 0
	anyChange := false

	for _, op := range ops {
		switch op.Op {
		case GazeOpRetract:
			changed, err := s.processRetract(ctx, chatModel, payload, op, contentInstructions)
			if err != nil {
				logger.Warnf(ctx, "gaze ingest: retract failed for %s: %v", op.KnowledgeID, err)
				continue
			}
			if changed {
				changes = append(changes, gazeIndexChange{Action: gazeIndexRemoved, DocTitle: op.DocTitle})
				affectedSlugs = append(affectedSlugs, op.PageSlugs...)
				anyChange = true
			}
		default:
			res, err := s.processDocIngest(ctx, chatModel, payload, op, contentInstructions)
			if err != nil {
				logger.Warnf(ctx, "gaze ingest: doc %s failed: %v", op.KnowledgeID, err)
				continue
			}
			if res != nil {
				changes = append(changes, gazeIndexChange{Action: gazeIndexAdded, DocTitle: res.DocTitle, DocSummary: res.Summary})
				affectedSlugs = append(affectedSlugs, res.Slugs...)
				anyChange = true
			}
		}
		processed++
	}

	// Consume only the rows we actually processed; failed ones stay for the
	// next run (bounded by asynq retry budget / dead-letter middleware).
	if processed > 0 {
		if err := s.trimPendingList(ctx, peekedIDs[:processed]); err != nil {
			return fmt.Errorf("gaze ingest: trim pending list: %w", err)
		}
	}

	// Follow-up: more rows remain, schedule the next run.
	if remaining, err := s.pendingRepo.PendingCount(ctx, gazeTaskType, gazeTaskScope, payload.KnowledgeBaseID); err == nil && remaining > 0 {
		_ = enqueueGazeIngestTrigger(ctx, s.task, payload.TenantID, payload.KnowledgeBaseID)
	}

	if anyChange {
		s.publishDraftPages(ctx, payload.KnowledgeBaseID, uniqueGazeStrings(affectedSlugs))
		changeDesc := gazeIndexChangeDescription(changes)
		if err := s.rebuildIndexPage(ctx, chatModel, payload, changeDesc, lang, contentInstructions); err != nil {
			logger.Warnf(ctx, "gaze ingest: index rebuild failed: %v", err)
		}
		// 版本过期同步(§2.15 对齐): 新文档页生成后, 同文号旧页标 outdated.
		if marked, err := s.gazeService.SyncOutdated(ctx, payload.KnowledgeBaseID); err != nil {
			logger.Warnf(ctx, "gaze ingest: outdated sync failed: %v", err)
		} else if marked > 0 {
			logger.Infof(ctx, "gaze ingest: marked %d outdated page(s) in KB %s", marked, payload.KnowledgeBaseID)
		}
	}

	logger.Infof(ctx, "gaze ingest stats: kb=%s processed=%d pages=%d elapsed=%s",
		payload.KnowledgeBaseID, processed, len(affectedSlugs), time.Since(taskStartedAt).Round(time.Millisecond))
	return nil
}

// gazeIndexChange records a doc-level add/remove entry for the index-intro
// change description.
type gazeIndexChange struct {
	Action     string `json:"action"` // gazeIndexAdded | gazeIndexRemoved
	DocTitle   string `json:"doc_title,omitempty"`
	DocSummary string `json:"doc_summary,omitempty"`
}

const (
	gazeIndexAdded   = "added"
	gazeIndexRemoved = "removed"
)

func gazeIndexChangeDescription(changes []gazeIndexChange) string {
	if len(changes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(changes))
	for _, c := range changes {
		switch c.Action {
		case gazeIndexAdded:
			parts = append(parts, fmt.Sprintf("added document %q", c.DocTitle))
		case gazeIndexRemoved:
			parts = append(parts, fmt.Sprintf("removed document %q", c.DocTitle))
		}
	}
	return strings.Join(parts, "; ")
}

func uniqueGazeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func uniqueGazeFolderIDs(values []string) []string {
	return uniqueGazeStrings(values)
}

// docIngestResult summarizes what one document's ingest produced.
type docIngestResult struct {
	KnowledgeID string
	DocTitle    string
	Summary     string // one-line summary of the document
	Slugs       []string
}

// processDocIngest runs the per-document pipeline: extract entities, generate
// the summary page, upsert all affected pages.
func (s *gazeIngestService) processDocIngest(
	ctx context.Context,
	chatModel chat.Chat,
	payload GazeIngestPayload,
	op GazePendingOp,
	contentInstructions string,
) (*docIngestResult, error) {
	knowledgeID := op.KnowledgeID
	lang := types.ResolveLanguageName(ctx, op.Language)

	if s.isKnowledgeGone(ctx, payload.KnowledgeBaseID, knowledgeID) {
		logger.Infof(ctx, "gaze ingest: knowledge %s has been deleted, skip", knowledgeID)
		return nil, nil
	}

	chunks, err := s.chunkRepo.ListChunksByKnowledgeID(ctx, payload.TenantID, knowledgeID)
	if err != nil {
		return nil, fmt.Errorf("get chunks: %w", err)
	}
	if len(chunks) == 0 {
		logger.Infof(ctx, "gaze ingest: document %s has no chunks, skip", knowledgeID)
		return nil, nil
	}

	content := reconstructEnrichedContent(ctx, s.chunkRepo, payload.TenantID, chunks)
	rawRuneCount := len([]rune(content))
	if len([]rune(content)) > maxContentForGaze {
		content = string([]rune(content)[:maxContentForGaze])
	}
	logger.Infof(ctx, "gaze ingest: doc %s chunks=%d content_len(raw=%d,truncated=%d)", knowledgeID, len(chunks), rawRuneCount, len([]rune(content)))

	if !hasSufficientTextContent(content) {
		logger.Warnf(ctx,
			"gaze ingest: doc %s has insufficient text content after stripping image markup (raw_len=%d), skipping LLM extraction",
			knowledgeID, rawRuneCount,
		)
		return nil, nil
	}

	docTitle := knowledgeID
	var kn *types.Knowledge
	if k, err := s.knowledgeSvc.GetKnowledgeByIDOnly(ctx, knowledgeID); err == nil && k != nil {
		kn = k
		if kn.Title != "" {
			docTitle = kn.Title
		}
	}
	if docTitle == knowledgeID {
		for _, ch := range chunks {
			if ch.Content != "" {
				lines := strings.SplitN(ch.Content, "\n", 2)
				if len(lines) > 0 && len(lines[0]) > 0 && len(lines[0]) < 200 {
					docTitle = strings.TrimPrefix(strings.TrimSpace(lines[0]), "# ")
					break
				}
			}
		}
	}

	sourceRef := knowledgeID

	// Entity 页由 UMS 元数据驱动(§2.15 对齐: 机构+产品双维度)。
	// 不再 LLM 抽 entity —— 机构页(org_name)与产品页(product_name)确定性生成,
	// 同机构/产品文档自动聚合到同一页面;LLM 仅负责 summary 与页面摘要。
	entities := make([]extractedItem, 0, 2)
	if kn != nil {
		if org := strings.TrimSpace(kn.OrgName); org != "" {
			entities = append(entities, extractedItem{
				Name:        org,
				Slug:        "entity/org-" + slugify(org),
				Description: "机构页面(由文档机构元数据自动生成)",
			})
		}
		if product := strings.TrimSpace(kn.ProductName); product != "" {
			entities = append(entities, extractedItem{
				Name:        product,
				Slug:        "entity/product-" + slugify(product),
				Description: "产品页面(由文档产品元数据自动生成)",
			})
		}
	}
	logger.Infof(ctx, "gaze ingest: UMS-driven entities for %s: %d (org=%s product=%s)",
		knowledgeID, len(entities), kn.OrgName, kn.ProductName)

	// Summary generation, fed with the entity slug listing for gaze-links.
	var slugListing strings.Builder
	for _, item := range entities {
		if item.Slug == "" || item.Name == "" {
			continue
		}
		aliases := ""
		if len(item.Aliases) > 0 {
			aliases = fmt.Sprintf(" (Aliases: %s)", strings.Join(item.Aliases, ", "))
		}
		fmt.Fprintf(&slugListing, "- [[%s]] = %s%s\n", item.Slug, item.Name, aliases)
	}
	summarySlug := fmt.Sprintf("summary/%s", slugify(knowledgeID))
	summaryRaw, err := s.generateWithTemplate(ctx, chatModel, agent.GazeSummaryPrompt, map[string]string{
		"Content":            content,
		"Language":           lang,
		"ExtractedSlugs":     slugListing.String(),
		"CustomInstructions": contentInstructions,
		"InstructionScope":   "gaze_content",
	})
	if err != nil {
		logger.Warnf(ctx, "gaze ingest: summary generation failed for %s: %v", knowledgeID, err)
	}
	summaryLine, summaryBody := splitSummaryLine(summaryRaw)

	// Group updates per slug and reduce sequentially.
	updatesBySlug := make(map[string][]SlugUpdate)
	updatesBySlug[summarySlug] = append(updatesBySlug[summarySlug], SlugUpdate{
		Slug:        summarySlug,
		Type:        "summary",
		DocTitle:    docTitle,
		KnowledgeID: knowledgeID,
		SourceRef:   sourceRef,
		Language:    lang,
		SummaryBody: summaryBody,
		SummaryLine: summaryLine,
	})
	for _, item := range entities {
		if item.Slug == "" || item.Name == "" {
			continue
		}
		slug := item.Slug
		if !strings.HasPrefix(slug, "entity/") {
			slug = "entity/" + strings.TrimPrefix(slug, "/")
		}
		updatesBySlug[slug] = append(updatesBySlug[slug], SlugUpdate{
			Slug:        slug,
			Type:        types.GazePageTypeEntity,
			Item:        item,
			DocTitle:    docTitle,
			KnowledgeID: knowledgeID,
			SourceRef:   sourceRef,
			Language:    lang,
			DocSummary:  summaryBody,
		})
	}

	slugs := make([]string, 0, len(updatesBySlug))
	for slug, updates := range updatesBySlug {
		if _, err := s.reduceSlugUpdate(ctx, chatModel, payload.KnowledgeBaseID, slug, updates, payload.TenantID, contentInstructions); err != nil {
			logger.Warnf(ctx, "gaze ingest: reduce failed for slug %s: %v", slug, err)
			continue
		}
		slugs = append(slugs, slug)
	}

	return &docIngestResult{
		KnowledgeID: knowledgeID,
		DocTitle:    docTitle,
		Summary:     summaryLine,
		Slugs:       slugs,
	}, nil
}

// processRetract handles pages affected by a document deletion. Pages that
// lose their last source are deleted outright (the delete path already
// handles single-source pages synchronously, this is the safety net);
// multi-source pages get an LLM pass to remove the deleted document's
// contributions.
func (s *gazeIngestService) processRetract(
	ctx context.Context,
	chatModel chat.Chat,
	payload GazeIngestPayload,
	op GazePendingOp,
	contentInstructions string,
) (bool, error) {
	lang := types.ResolveLanguageName(ctx, op.Language)
	changed := false
	for _, slug := range op.PageSlugs {
		if slug == "" || slug == "index" {
			continue
		}
		updates := []SlugUpdate{{
			Slug:              slug,
			Type:              "retract",
			DocTitle:          op.DocTitle,
			KnowledgeID:       op.KnowledgeID,
			Language:          lang,
			RetractDocContent: op.DocSummary,
		}}
		pageChanged, err := s.reduceSlugUpdate(ctx, chatModel, payload.KnowledgeBaseID, slug, updates, payload.TenantID, contentInstructions)
		if err != nil {
			logger.Warnf(ctx, "gaze ingest: retract failed for slug %s: %v", slug, err)
			continue
		}
		changed = changed || pageChanged
	}
	return changed, nil
}

// SlugUpdate represents a single update operation for a specific slug
type SlugUpdate struct {
	Slug        string
	Type        string // "entity", "summary", "retract"
	Item        extractedItem
	DocTitle    string
	KnowledgeID string
	SourceRef   string
	Language    string
	SummaryBody string // For summary
	SummaryLine string // For summary
	// RetractDocContent carries the deleted document's summary for the
	// LLM removal pass.
	RetractDocContent string
	// DocSummary is the document-level summary body produced by
	// GazeSummaryPrompt, used to frame entity additions.
	DocSummary string
}

// extractedItem represents a single extracted entity.
type extractedItem struct {
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Aliases     []string `json:"aliases"`
	Description string   `json:"description"`
	Details     string   `json:"details"`
}

// combinedExtraction represents the parsed result of the combined entity+concept extraction
type combinedExtraction struct {
	Entities []extractedItem `json:"entities"`
	Concepts []extractedItem `json:"concepts"`
}

// reduceSlugUpdate applies all updates for one slug: summary pages are
// written directly; new single-source entity pages are written
// deterministically; existing pages with additions/retractions go through
// the GazePageModifyUserPrompt LLM merge.
func (s *gazeIngestService) reduceSlugUpdate(
	ctx context.Context,
	chatModel chat.Chat,
	kbID string,
	slug string,
	updates []SlugUpdate,
	tenantID uint64,
	contentInstructions string,
) (bool, error) {
	updates = s.filterLiveUpdates(ctx, kbID, updates)
	if len(updates) == 0 {
		return false, nil
	}

	page, err := s.gazeService.GetPageBySlug(ctx, kbID, slug)
	exists := (err == nil && page != nil)

	if !exists {
		hasAdditions := false
		for _, u := range updates {
			if u.Type == types.GazePageTypeEntity || u.Type == "summary" {
				hasAdditions = true
				break
			}
		}
		if !hasAdditions {
			return false, nil
		}
		page = &types.GazePage{
			ID:              uuid.New().String(),
			TenantID:        tenantID,
			KnowledgeBaseID: kbID,
			Slug:            slug,
			Status:          types.GazePageStatusDraft,
			SourceRefs:      types.StringArray{},
			Aliases:         types.StringArray{},
		}
		err = nil
	}

	var summaryUpdate *SlugUpdate
	var retracts []SlugUpdate
	var additions []SlugUpdate

	for i, u := range updates {
		switch u.Type {
		case "summary":
			summaryUpdate = &updates[i]
		case "retract":
			retracts = append(retracts, u)
		default:
			additions = append(additions, u)
		}
	}

	changed := false

	if summaryUpdate != nil {
		page.Title = summaryUpdate.DocTitle + " - Summary"
		page.Content = summaryUpdate.SummaryBody
		page.Summary = summaryUpdate.SummaryLine
		page.PageType = types.GazePageTypeSummary
		page.SourceRefs = appendUnique(page.SourceRefs, summaryUpdate.SourceRef)
		changed = true
	}

	if len(retracts) > 0 {
		retractKIDs := make(map[string]bool)
		for _, r := range retracts {
			retractKIDs[r.KnowledgeID] = true
		}
		newRefs := types.StringArray{}
		for _, ref := range page.SourceRefs {
			if !retractKIDs[types.GazeSourceKnowledgeID(ref)] {
				newRefs = append(newRefs, ref)
			}
		}
		page.SourceRefs = newRefs
		// Last source gone: delete the page outright (the LLM removal pass
		// has nothing left to keep).
		if len(page.SourceRefs) == 0 && page.PageType != types.GazePageTypeIndex {
			if exists {
				if err := s.gazeService.DeletePage(ctx, kbID, slug); err != nil {
					return false, fmt.Errorf("delete emptied page %s: %w", slug, err)
				}
			}
			return true, nil
		}
	}

	if len(additions) > 0 {
		for _, add := range additions {
			page.Aliases = appendUnique(page.Aliases, add.Item.Aliases...)
			page.SourceRefs = appendUnique(page.SourceRefs, add.SourceRef)
			if page.Title == "" {
				page.Title = add.Item.Name
			}
			if page.PageType == "" {
				page.PageType = add.Type
			}
		}
	}

	if len(additions) > 0 || len(retracts) > 0 {
		// New page created purely from one document's additions: write it
		// deterministically, skipping the LLM merge call.
		if !exists && len(retracts) == 0 && summaryUpdate == nil {
			page.Content = buildNewEntityPageContent(additions)
			if page.Summary == "" {
				page.Summary = truncateString(additions[0].Item.Description, 200)
			}
			changed = true
		} else {
			// LLM merge for existing pages (or new pages receiving both
			// summary and entity updates in one run).
			existingContent := stripGazeInlineChunkCitations(page.Content)
			if !exists || existingContent == "" {
				existingContent = "(New page)"
			}

			var deletedContent strings.Builder
			for _, r := range retracts {
				fmt.Fprintf(&deletedContent, "<document>\n<title>%s</title>\n<content>\n%s\n</content>\n</document>\n\n", r.DocTitle, r.RetractDocContent)
			}
			var newContent strings.Builder
			var docTitles []string
			for _, add := range additions {
				fmt.Fprintf(&newContent,
					"<document>\n<title>%s</title>\n<content>\n**%s**: %s\n\n%s\n</content>\n</document>\n\n",
					add.DocTitle, add.Item.Name, add.Item.Description, add.Item.Details)
				docTitles = appendUnique(docTitles, add.DocTitle)
			}

			pageTitle := page.Title
			if pageTitle == "" {
				pageTitle = slug
			}
			pageType := string(page.PageType)
			if pageType == "" {
				pageType = "gaze page"
			}

			hasAdditionsStr := ""
			if len(additions) > 0 {
				hasAdditionsStr = "1"
			}
			hasRetractionsStr := ""
			if len(retracts) > 0 {
				hasRetractionsStr = "1"
			}

			updatedContent, genErr := s.generateWithTemplate(ctx, chatModel, agent.GazePageModifyUserPrompt, map[string]string{
				"HasAdditions":            hasAdditionsStr,
				"HasRetractions":          hasRetractionsStr,
				"PageSlug":                slug,
				"PageTitle":               pageTitle,
				"PageType":                pageType,
				"PageAliases":             strings.Join(page.Aliases, ", "),
				"ExistingContent":         existingContent,
				"SharedSourceContexts":    "",
				"NewContent":              newContent.String(),
				"DeletedContent":          deletedContent.String(),
				"RemainingSourcesContent": "(no remaining sources)",
				"AvailableSlugs":          "",
				"Language":                resolveSlugUpdateLanguage(ctx, updates),
				"CustomInstructions":      contentInstructions,
				"InstructionScope":        "gaze_content",
			})
			if genErr != nil {
				logger.Warnf(ctx, "gaze ingest: update/retract failed for slug %s: %v", slug, genErr)
				if len(additions) > 0 {
					// Leave the page unchanged; the next ingest run retries.
					return false, nil
				}
			} else if updatedContent != "" {
				updatedSummary, updatedBody := splitSummaryLine(updatedContent)
				if updatedBody != "" {
					page.Content = updatedBody
				} else {
					page.Content = updatedContent
				}
				if updatedSummary != "" {
					page.Summary = updatedSummary
				}
				changed = true
			}
		}
	}

	if !changed {
		return false, nil
	}
	if exists {
		_, err = s.gazeService.UpdatePage(ctx, page)
	} else {
		_, err = s.gazeService.CreatePage(ctx, page)
	}
	return changed, err
}

// buildNewEntityPageContent renders a new single-source entity page
// deterministically from its additions (no LLM call).
func buildNewEntityPageContent(additions []SlugUpdate) string {
	var b strings.Builder
	first := additions[0]
	fmt.Fprintf(&b, "# %s\n\n", first.Item.Name)
	if len(additions) == 1 {
		if first.Item.Description != "" {
			b.WriteString(first.Item.Description)
			b.WriteString("\n\n")
		}
		if first.Item.Details != "" {
			b.WriteString(first.Item.Details)
			b.WriteString("\n")
		}
		return b.String()
	}
	for _, add := range additions {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", add.DocTitle, add.Item.Details)
	}
	return b.String()
}

func gazePreviousSlugsText(oldPageSlugs map[string]bool) string {
	if len(oldPageSlugs) == 0 {
		return "(none — this is a new document)"
	}
	var sb strings.Builder
	for slug := range oldPageSlugs {
		if !strings.HasPrefix(slug, "entity/") {
			continue
		}
		fmt.Fprintf(&sb, "- %s\n", slug)
	}
	if sb.Len() == 0 {
		return "(none — this is a new document)"
	}
	return sb.String()
}

// resolveSlugUpdateLanguage picks the language the editor prompt should write
// the page in.
func resolveSlugUpdateLanguage(ctx context.Context, updates []SlugUpdate) string {
	for _, u := range updates {
		if u.Language != "" {
			return u.Language
		}
	}
	return types.LanguageNameFromContext(ctx)
}

func (s *gazeIngestService) getExistingPageSlugsForKnowledge(ctx context.Context, kbID, knowledgeID string) map[string]bool {
	slugs, err := s.gazeService.ListSlugsBySourceRef(ctx, kbID, knowledgeID)
	if err != nil {
		logger.Warnf(ctx, "gaze ingest: ListSlugsBySourceRef(%s) failed: %v", knowledgeID, err)
		return nil
	}
	if len(slugs) == 0 {
		return nil
	}
	out := make(map[string]bool, len(slugs))
	for _, slug := range slugs {
		if slug == "index" {
			continue
		}
		out[slug] = true
	}
	return out
}

// rebuildIndexPage refreshes the LLM-generated intro that sits on the
// index gaze_pages row. First time (empty or legacy placeholder): generate
// from all document summaries via GazeIndexIntroPrompt. Subsequent calls
// with a change description: incremental update via
// GazeIndexIntroUpdatePrompt. No change description: keep the existing
// intro untouched.
func (s *gazeIngestService) rebuildIndexPage(ctx context.Context, chatModel chat.Chat, payload GazeIngestPayload,
	changeDesc, lang, customInstructions string,
) error {
	indexPage, _ := s.gazeService.GetIndex(ctx, payload.KnowledgeBaseID)
	if indexPage == nil {
		return nil
	}

	existingIntro := strings.TrimSpace(indexPage.Content)
	if existingIntro == "" {
		existingIntro = strings.TrimSpace(indexPage.Summary)
	}
	if idx := strings.Index(existingIntro, "\n## "); idx >= 0 {
		existingIntro = strings.TrimSpace(existingIntro[:idx])
	}

	var intro string
	switch {
	case existingIntro == "" || existingIntro == "Gaze index - table of contents":
		recentSummaries, listErr := s.gazeService.ListByTypeRecent(ctx, payload.KnowledgeBaseID, types.GazePageTypeSummary, indexIntroSummaryCap)
		if listErr != nil {
			return listErr
		}
		var docSummaries strings.Builder
		for _, e := range recentSummaries {
			fmt.Fprintf(&docSummaries, "<document>\n<title>%s</title>\n<summary>%s</summary>\n</document>\n\n", e.Title, e.Summary)
		}
		totalSummaries := int64(len(recentSummaries))
		if counts, cntErr := s.gazeService.CountByType(ctx, payload.KnowledgeBaseID); cntErr == nil {
			if t, ok := counts[types.GazePageTypeSummary]; ok {
				totalSummaries = t
			}
		}
		framing := ""
		if int(totalSummaries) > len(recentSummaries) && len(recentSummaries) > 0 {
			framing = fmt.Sprintf("(showing %d most recent of %d total documents)\n\n", len(recentSummaries), totalSummaries)
		}
		if docSummaries.Len() == 0 {
			docSummaries.WriteString("(no documents yet)")
		}
		generatedIntro, genErr := s.generateWithTemplate(ctx, chatModel, agent.GazeIndexIntroPrompt, map[string]string{
			"DocumentSummaries":  framing + docSummaries.String(),
			"Language":           lang,
			"CustomInstructions": customInstructions,
			"InstructionScope":   "gaze_content",
		})
		if genErr != nil {
			intro = "# Gaze Index\n\nThis gaze contains knowledge extracted from uploaded documents.\n"
		} else {
			intro = strings.TrimSpace(generatedIntro)
		}
	case changeDesc != "":
		updatedIntro, genErr := s.generateWithTemplate(ctx, chatModel, agent.GazeIndexIntroUpdatePrompt, map[string]string{
			"ExistingIntro":      existingIntro,
			"ChangeDescription":  changeDesc,
			"DocumentSummaries":  "",
			"Language":           lang,
			"CustomInstructions": customInstructions,
			"InstructionScope":   "gaze_content",
		})
		if genErr != nil {
			intro = existingIntro // keep existing on error
		} else {
			intro = strings.TrimSpace(updatedIntro)
		}
	default:
		intro = existingIntro
	}

	if idx := strings.Index(intro, "\n## "); idx >= 0 {
		intro = strings.TrimSpace(intro[:idx])
	}

	indexPage.Content = intro
	indexPage.Summary = intro
	_, err := s.gazeService.UpdatePage(ctx, indexPage)
	return err
}

// publishDraftPages transitions draft pages to published status after ingest completes.
func (s *gazeIngestService) publishDraftPages(ctx context.Context, kbID string, slugs []string) {
	for _, slug := range slugs {
		page, err := s.gazeService.GetPageBySlug(ctx, kbID, slug)
		if err != nil || page == nil {
			continue
		}
		if page.Status == types.GazePageStatusDraft {
			page.Status = types.GazePageStatusPublished
			if err := s.gazeService.UpdatePageMeta(ctx, page); err != nil {
				logger.Warnf(ctx, "gaze ingest: failed to publish page %s: %v", slug, err)
			}
		}
	}
}

// splitSummaryLine extracts the "SUMMARY: ..." line from LLM output.
func splitSummaryLine(raw string) (summary string, content string) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "SUMMARY:") || strings.HasPrefix(raw, "SUMMARY：") {
		idx := strings.IndexByte(raw, '\n')
		if idx < 0 {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(raw, "SUMMARY:"), "SUMMARY：")), ""
		}
		summaryLine := raw[:idx]
		summaryLine = strings.TrimPrefix(summaryLine, "SUMMARY:")
		summaryLine = strings.TrimPrefix(summaryLine, "SUMMARY：")
		return strings.TrimSpace(summaryLine), strings.TrimSpace(raw[idx+1:])
	}
	return "", raw
}

func (s *gazeIngestService) generateWithTemplate(ctx context.Context, chatModel chat.Chat, promptTpl string, data map[string]string) (string, error) {
	tmpl, err := template.New("gaze").Parse(promptTpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	maskedData, urlMap := maskTemplateDataImageURLs(data)

	var buf strings.Builder
	if err := tmpl.Execute(&buf, maskedData); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	prompt := buf.String()
	purpose := gazePromptPurpose(promptTpl)
	messages := []chat.Message{{Role: "user", Content: prompt}}
	if promptTpl == agent.GazePageModifyUserPrompt {
		systemPrompt := types.AppendCustomPromptInstructions(
			agent.GazePageModifySystemPrompt,
			maskedData["CustomInstructions"],
			maskedData["InstructionScope"],
		)
		messages = []chat.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		}
	} else {
		messages[0].Content = types.AppendCustomPromptInstructions(
			prompt, maskedData["CustomInstructions"], maskedData["InstructionScope"],
		)
	}
	thinking := false
	opts := &chat.ChatOptions{Temperature: 0.3, Thinking: &thinking, MaxTokens: gazeLLMMaxTokens}
	prefixFingerprint := chat.PromptPrefixFingerprint(messages, opts)
	warmupKey := ""
	if promptTpl == agent.GazePageModifyUserPrompt {
		prefixFingerprint = chat.FingerprintPromptPrefix(
			messages[0].Content, maskedData["SharedSourceContexts"],
		)
		if tenantID, ok := types.TenantIDFromContext(ctx); ok {
			warmupKey = chat.BuildPromptCacheKey(
				tenantID, chatModel.GetModelID(), purpose, prefixFingerprint,
			)
		}
	}
	ctx = types.WithLLMCallMetadata(ctx, purpose, prefixFingerprint)

	tenantID, tenantScoped := types.TenantIDFromContext(ctx)
	requestJSON, _ := json.Marshal(struct {
		Messages []chat.Message    `json:"messages"`
		Options  *chat.ChatOptions `json:"options"`
	}{Messages: messages, Options: opts})
	requestKey := chat.BuildPromptCacheKey(
		tenantID, chatModel.GetModelID(), "gaze_exact_request",
		chat.FingerprintPromptPrefix(string(requestJSON)),
	)

	execute := func() (interface{}, error) {
		releaseWarmup := func() {}
		if tenantScoped && promptTpl == agent.GazePageModifyUserPrompt && strings.TrimSpace(maskedData["SharedSourceContexts"]) != "" {
			var warmupErr error
			releaseWarmup, warmupErr = s.awaitGazePromptWarmup(ctx, warmupKey)
			if warmupErr != nil {
				return "", warmupErr
			}
		}
		defer releaseWarmup()

		var lastErr error
		for attempt := 1; attempt <= gazeLLMMaxAttempts; attempt++ {
			response, callErr := chatModel.Chat(ctx, messages, opts)
			if callErr == nil && response != nil {
				return response.Content, nil
			}
			if callErr == nil {
				callErr = errors.New("LLM returned nil response")
			}
			lastErr = callErr

			if !isTransientLLMError(ctx, callErr) {
				return "", fmt.Errorf("LLM call failed: %w", callErr)
			}
			if attempt == gazeLLMMaxAttempts {
				break
			}

			backoff := gazeLLMBackoffBase << (attempt - 1)
			logger.Warnf(ctx, "gaze ingest: LLM call failed (attempt %d/%d), retrying in %s: %v",
				attempt, gazeLLMMaxAttempts, backoff, callErr)
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", fmt.Errorf("LLM call aborted during backoff: %w", ctx.Err())
			case <-timer.C:
			}
		}
		return "", fmt.Errorf("LLM call failed after %d attempts: %w", gazeLLMMaxAttempts, lastErr)
	}

	if !tenantScoped {
		value, executeErr := execute()
		if executeErr != nil {
			return "", executeErr
		}
		content, _ := value.(string)
		return unmaskImageURLs(content, urlMap), nil
	}
	resultCh := s.llmRequests.DoChan(requestKey, execute)

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return "", result.Err
		}
		content, _ := result.Val.(string)
		return unmaskImageURLs(content, urlMap), nil
	}
}

func gazePromptPurpose(promptTpl string) string {
	switch promptTpl {
	case agent.GazePageModifyUserPrompt:
		return "gaze_page_modify"
	case agent.GazeSummaryPrompt:
		return "gaze_summary"
	case agent.GazeIndexIntroPrompt, agent.GazeIndexIntroUpdatePrompt:
		return "gaze_index_intro"
	default:
		return "gaze_generation"
	}
}

func (s *gazeIngestService) awaitGazePromptWarmup(ctx context.Context, key string) (func(), error) {
	if key == "" {
		return func() {}, nil
	}
	candidate := &gazePromptWarmup{done: make(chan struct{})}
	actual, loaded := s.promptWarmups.LoadOrStore(key, candidate)
	entry := actual.(*gazePromptWarmup)
	if !loaded {
		return func() {
			entry.once.Do(func() { close(entry.done) })
			time.AfterFunc(4*time.Minute, func() {
				s.promptWarmups.CompareAndDelete(key, entry)
			})
		}, nil
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-entry.done:
		return func() {}, nil
	}
}

// rateLimitErrorIndicators are substrings that mark an HTTP 403 response
// body as rate limiting rather than authorization failure.
var rateLimitErrorIndicators = []string{
	"qpm",
	"qps",
	"rate limit",
	"rate_limit",
	"too many requests",
	"throttl",
	"调用频率",
	"频率超限",
	"请求过于频繁",
	"繁忙",
	"try again later",
	"retry later",
	"slow down",
}

func isTransientLLMError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx.Err() != nil {
		return false
	}

	msg := err.Error()
	for _, s := range []string{
		"status 408", "status 429",
		"status 500", "status 501", "status 502", "status 503", "status 504",
		"status 520", "status 521", "status 522", "status 523", "status 524",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}

	lower := strings.ToLower(msg)
	if strings.Contains(msg, "status 403") {
		for _, s := range rateLimitErrorIndicators {
			if strings.Contains(lower, s) {
				return true
			}
		}
	}

	for _, s := range []string{
		"timeout",
		"timed out",
		"connection reset",
		"connection refused",
		"broken pipe",
		"no such host",
		"i/o timeout",
		"unexpected eof",
		"tls handshake",
		"context deadline exceeded",
	} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// --- Helpers ---

// isKnowledgeGone returns true if the given knowledge has been deleted or is
// in the middle of being deleted.
func (s *gazeIngestService) isKnowledgeGone(ctx context.Context, kbID, knowledgeID string) bool {
	if knowledgeID == "" {
		return true
	}
	if s.redisClient != nil {
		if exists, err := s.redisClient.Exists(ctx, GazeDeletedTombstoneKey(kbID, knowledgeID)).Result(); err == nil && exists > 0 {
			return true
		}
	}
	kn, err := s.knowledgeSvc.GetKnowledgeByIDOnly(ctx, knowledgeID)
	if err != nil || kn == nil {
		return true
	}
	switch kn.ParseStatus {
	case types.ParseStatusDeleting, types.ParseStatusCancelled:
		return true
	}
	return false
}

// filterLiveUpdates drops additions/summaries whose source knowledge has been
// deleted. Retract updates are preserved so pages still get cleaned up.
func (s *gazeIngestService) filterLiveUpdates(ctx context.Context, kbID string, updates []SlugUpdate) []SlugUpdate {
	if len(updates) == 0 {
		return updates
	}
	goneCache := make(map[string]bool)
	isGone := func(kid string) bool {
		if kid == "" {
			return false
		}
		if v, ok := goneCache[kid]; ok {
			return v
		}
		v := s.isKnowledgeGone(ctx, kbID, kid)
		goneCache[kid] = v
		return v
	}
	filtered := make([]SlugUpdate, 0, len(updates))
	dropped := 0
	for _, u := range updates {
		if u.Type == "retract" {
			filtered = append(filtered, u)
			continue
		}
		if isGone(u.KnowledgeID) {
			dropped++
			continue
		}
		filtered = append(filtered, u)
	}
	if dropped > 0 {
		logger.Infof(ctx, "gaze ingest: reduce dropped %d updates for deleted knowledge(s)", dropped)
	}
	return filtered
}

// reconstructContent rebuilds document text from chunks.
func reconstructContent(chunks []*types.Chunk) string {
	var textChunks []*types.Chunk
	for _, c := range chunks {
		if c.ChunkType == types.ChunkTypeText || c.ChunkType == "" {
			textChunks = append(textChunks, c)
		}
	}
	return searchutil.MergeTextChunks(textChunks, "\n")
}

// reconstructEnrichedContent rebuilds document text and inlines image_info
// (OCR text + caption) pulled from image_ocr / image_caption child chunks.
func reconstructEnrichedContent(
	ctx context.Context,
	chunkRepo interfaces.ChunkRepository,
	tenantID uint64,
	chunks []*types.Chunk,
) string {
	content := reconstructContent(chunks)

	var textChunkIDs []string
	for _, c := range chunks {
		if c.ChunkType == types.ChunkTypeText || c.ChunkType == "" {
			if c.ID != "" {
				textChunkIDs = append(textChunkIDs, c.ID)
			}
		}
	}
	if len(textChunkIDs) == 0 || chunkRepo == nil {
		return content
	}

	imageInfoMap := searchutil.CollectImageInfoByChunkIDs(ctx, chunkRepo, tenantID, textChunkIDs)
	mergedImageInfo := searchutil.MergeImageInfoJSON(imageInfoMap)
	if mergedImageInfo == "" {
		return content
	}
	return searchutil.EnrichContentWithImageInfo(content, mergedImageInfo)
}

// slugify creates a URL-friendly slug from a string
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '/' {
			return r
		}
		if r == ' ' || r == '_' {
			return '-'
		}
		if r >= 0x4E00 && r <= 0x9FFF {
			return r
		}
		return -1
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// truncateString truncates a string to maxLen runes
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// appendUnique appends strings to a StringArray if not already present
func appendUnique(arr types.StringArray, items ...string) types.StringArray {
	for _, s := range items {
		found := false
		for _, v := range arr {
			if v == s {
				found = true
				break
			}
		}
		if !found {
			arr = append(arr, s)
		}
	}
	return arr
}

func previewText(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	r := []rune(s)
	if maxRunes <= 0 || len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "...(truncated)"
}

// minTextContentRunes is the minimum number of non-whitespace, non-image-reference
// runes required for content to be considered substantive enough for LLM
// summarization or gaze extraction.
var minTextContentRunes = 10

var (
	// Markdown image references like ![alt](path)
	mdImageRefRE = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)

	// <image_original>...</image_original> blocks
	imageOriginalBlockRE = regexp.MustCompile(`(?is)<image_original\b[^>]*>.*?</image_original>`)

	// Self-closing or attribute-only HTML <img> tags.
	htmlImgTagRE = regexp.MustCompile(`(?i)<img\b[^>]*/?>`)

	// Wrapper-style <image>, <images>, <image_caption>, <image_ocr> tags
	imageWrapperTagRE = regexp.MustCompile(`(?i)</?image[a-z_]*\b[^>]*/?>`)

	// Markdown image references with the URL captured separately so LLM-bound
	// image URLs can be frozen while captions remain editable.
	mdImageURLRE = regexp.MustCompile(`!\[[^\]]*\]\(([^)]*)\)`)

	// Enriched image blocks store the original object URL as an attribute.
	imageURLAttrRE = regexp.MustCompile(`(?i)<image\b[^>]*\surl\s*=\s*(?:"([^"]*)"|'([^']*)')`)

	imagePlaceholderTokenRE = regexp.MustCompile(`wkimg:[A-Za-z0-9_-]+`)
)

func maskTemplateDataImageURLs(data map[string]string) (map[string]string, map[string]string) {
	if len(data) == 0 {
		return data, nil
	}

	masked := make(map[string]string, len(data))
	urlToToken := make(map[string]string)
	tokenToURL := make(map[string]string)

	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		masked[key] = maskImageURLsWithState(data[key], urlToToken, tokenToURL)
	}

	return masked, tokenToURL
}

// maskImageURLs replaces image URLs with low-entropy placeholders. It only
// freezes URLs; alt/caption text remains in place for the LLM to edit.
func maskImageURLs(s string) (string, map[string]string) {
	urlToToken := make(map[string]string)
	tokenToURL := make(map[string]string)
	return maskImageURLsWithState(s, urlToToken, tokenToURL), tokenToURL
}

func maskImageURLsWithState(s string, urlToToken, tokenToURL map[string]string) string {
	urls := collectMaskableImageURLs(s)
	if len(urls) == 0 {
		return s
	}

	for _, url := range urls {
		if _, ok := urlToToken[url]; ok {
			continue
		}
		token := fmt.Sprintf("wkimg:%04d", len(tokenToURL)+1)
		urlToToken[url] = token
		tokenToURL[token] = url
	}

	replaceURLs := append([]string(nil), urls...)
	sort.SliceStable(replaceURLs, func(i, j int) bool {
		return len(replaceURLs[i]) > len(replaceURLs[j])
	})

	masked := s
	for _, url := range replaceURLs {
		masked = strings.ReplaceAll(masked, url, urlToToken[url])
	}
	return masked
}

func collectMaskableImageURLs(s string) []string {
	seen := make(map[string]struct{})
	var urls []string

	addURL := func(url string) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		urls = append(urls, url)
	}

	for _, match := range mdImageURLRE.FindAllStringSubmatch(s, -1) {
		addURL(match[1])
	}
	for _, match := range imageURLAttrRE.FindAllStringSubmatch(s, -1) {
		if match[1] != "" {
			addURL(match[1])
			continue
		}
		addURL(match[2])
	}

	return urls
}

// unmaskImageURLs restores known placeholders and drops any corrupted or
// invented image placeholders so broken image links never reach storage.
func unmaskImageURLs(out string, urlMap map[string]string) string {
	out = mdImageURLRE.ReplaceAllStringFunc(out, func(match string) string {
		parts := mdImageURLRE.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		url := strings.TrimSpace(parts[1])
		if realURL, ok := urlMap[url]; ok {
			idx := strings.LastIndex(match, "(")
			if idx < 0 {
				return match
			}
			return match[:idx+1] + realURL + ")"
		}
		if strings.HasPrefix(url, "wkimg:") {
			return ""
		}
		return match
	})

	return replaceImagePlaceholderTokensOutsideMarkdown(out, urlMap)
}

func replaceImagePlaceholderTokensOutsideMarkdown(s string, urlMap map[string]string) string {
	matches := mdImageURLRE.FindAllStringIndex(s, -1)
	if len(matches) == 0 {
		return replaceImagePlaceholderTokens(s, urlMap)
	}

	var b strings.Builder
	last := 0
	for _, match := range matches {
		if match[0] > last {
			b.WriteString(replaceImagePlaceholderTokens(s[last:match[0]], urlMap))
		}
		b.WriteString(s[match[0]:match[1]])
		last = match[1]
	}
	if last < len(s) {
		b.WriteString(replaceImagePlaceholderTokens(s[last:], urlMap))
	}
	return b.String()
}

func replaceImagePlaceholderTokens(s string, urlMap map[string]string) string {
	return imagePlaceholderTokenRE.ReplaceAllStringFunc(s, func(token string) string {
		if realURL, ok := urlMap[token]; ok {
			return realURL
		}
		return ""
	})
}

// stripImageMarkup removes image-only placeholders and unwraps the
// <image>/<image_caption>/<image_ocr> XML wrappers produced by the search
// enrichment layer, leaving any OCR or caption text as plain inline text.
func stripImageMarkup(s string) string {
	s = imageOriginalBlockRE.ReplaceAllString(s, "")
	s = mdImageRefRE.ReplaceAllString(s, "")
	s = htmlImgTagRE.ReplaceAllString(s, "")
	s = imageWrapperTagRE.ReplaceAllString(s, "")
	return s
}

// extractRealText returns the trimmed content with image markup stripped.
func extractRealText(content string) string {
	return strings.TrimSpace(stripImageMarkup(content))
}

// hasSufficientTextContent reports whether the given content carries enough
// real text (after image markup is stripped, with OCR/caption text retained)
// to warrant an LLM call.
func hasSufficientTextContent(content string) bool {
	return realTextRuneCount(content) >= minTextContentRunes
}

// realTextRuneCount returns the rune length of the content after image
// markup is stripped.
func realTextRuneCount(content string) int {
	return utf8.RuneCountInString(extractRealText(content))
}

// cleanLLMJSON strips markdown code-fence wrappers and sanitizes control characters
// from LLM-generated JSON output so it can be safely unmarshalled.
func cleanLLMJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	return sanitizeJSONString(s)
}

// sanitizeJSONString sanitizes a string that is intended to be parsed as JSON,
// by properly escaping unescaped control characters (like newlines) inside string literals.
func sanitizeJSONString(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	inString := false
	escape := false
	for _, r := range s {
		if escape {
			if r == '\n' {
				buf.WriteString(`n`)
			} else if r == '\r' {
				buf.WriteString(`r`)
			} else if r == '\t' {
				buf.WriteString(`t`)
			} else {
				buf.WriteRune(r)
			}
			escape = false
			continue
		}
		if r == '\\' {
			escape = true
			buf.WriteRune(r)
			continue
		}
		if r == '"' {
			inString = !inString
			buf.WriteRune(r)
			continue
		}
		if inString {
			if r == '\n' {
				buf.WriteString(`\n`)
				continue
			}
			if r == '\r' {
				buf.WriteString(`\r`)
				continue
			}
			if r == '\t' {
				buf.WriteString(`\t`)
				continue
			}
		}
		buf.WriteRune(r)
	}
	return buf.String()
}
