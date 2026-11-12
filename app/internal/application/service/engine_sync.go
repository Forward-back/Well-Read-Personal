package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"gaze/internal/engine"
	werrors "gaze/internal/errors"
	"gaze/internal/logger"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"github.com/robfig/cron/v3"
)

// engineSyncTimeout bounds one YLSW re-ingest (embedding included).
const engineSyncTimeout = 10 * time.Minute

// EngineSyncService re-pushes app-side chunk edits to YLSW. CR acceptance only
// flags engine_sync_status=pending; the actual push runs here so the HTTP path
// never blocks on document conversion. Two triggers:
//   - an admin button (TriggerKBSync) for one document or every pending doc in a KB;
//   - a tenant-level daily sweep (config default off), started by Start.
type EngineSyncService struct {
	knowledgeRepo interfaces.KnowledgeRepository
	chunkRepo     interfaces.ChunkRepository
	kbService     interfaces.KnowledgeBaseService
	tenantRepo    interfaces.TenantRepository
	ragEngine     engine.RAGEngine

	cron    *cron.Cron
	mu      sync.Mutex
	started bool
	running map[string]struct{}
	pending map[string]struct{}
	locks   map[string]*sync.Mutex
}

// NewEngineSyncService creates the resync service.
func NewEngineSyncService(
	knowledgeRepo interfaces.KnowledgeRepository,
	chunkRepo interfaces.ChunkRepository,
	kbService interfaces.KnowledgeBaseService,
	tenantRepo interfaces.TenantRepository,
	ragEngine engine.RAGEngine,
) *EngineSyncService {
	return &EngineSyncService{
		knowledgeRepo: knowledgeRepo,
		chunkRepo:     chunkRepo,
		kbService:     kbService,
		tenantRepo:    tenantRepo,
		ragEngine:     ragEngine,
		running:       make(map[string]struct{}),
		pending:       make(map[string]struct{}),
		locks:         make(map[string]*sync.Mutex),
		cron: cron.New(cron.WithSeconds(), cron.WithChain(
			cron.Recover(cron.DefaultLogger),
		)),
	}
}

// TriggerKBSync flags the given documents (all pending/failed docs in the KB
// when knowledgeIDs is empty) and pushes them in the background. Admin only:
// re-ingesting rewrites what the retrieval engine sees for the whole KB.
func (s *EngineSyncService) TriggerKBSync(
	ctx context.Context, kbID string, knowledgeIDs []string,
) (int, error) {
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil || kb == nil {
		return 0, werrors.NewNotFoundError("知识库不存在")
	}

	targets := make([]*types.Knowledge, 0, len(knowledgeIDs))
	if len(knowledgeIDs) == 0 {
		targets, err = s.knowledgeRepo.ListKnowledgeByEngineSyncStatus(
			ctx, kb.ID,
			[]string{types.EngineSyncStatusPending, types.EngineSyncStatusFailed},
		)
		if err != nil {
			return 0, werrors.NewInternalServerError("读取待重推文档失败")
		}
	} else {
		seen := make(map[string]struct{}, len(knowledgeIDs))
		for _, id := range knowledgeIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			knowledge, err := s.knowledgeRepo.GetKnowledgeByIDOnly(ctx, id)
			if err != nil || knowledge == nil ||
				knowledge.KnowledgeBaseID != kb.ID || knowledge.TenantID != kb.TenantID {
				return 0, werrors.NewValidationError("文档不属于该知识库")
			}
			targets = append(targets, knowledge)
		}
	}

	baseCtx := context.WithoutCancel(ctx)
	for _, knowledge := range targets {
		if err := s.knowledgeRepo.MarkEngineSyncPending(ctx, knowledge.TenantID, knowledge.ID); err != nil {
			logger.Warnf(ctx, "[EngineSync] mark pending failed for knowledge %s: %v", knowledge.ID, err)
			continue
		}
		go func(k *types.Knowledge) {
			if err := s.syncOne(baseCtx, k); err != nil {
				logger.Warnf(baseCtx, "[EngineSync] resync knowledge %s failed: %v", k.ID, err)
			}
		}(knowledge)
	}
	return len(targets), nil
}

// ScheduleKnowledgeSync re-ingests one document in the background after a
// content change (CR merge, version rollback, chunk edit). Publishing should
// not wait for an admin to press "重推"; the pending marker stays on failure so
// the daily sweep or the manual button can retry. Safe on a nil receiver so
// tests that build services without a sync service keep working.
func (s *EngineSyncService) ScheduleKnowledgeSync(ctx context.Context, tenantID uint64, knowledgeID string) {
	if s == nil || knowledgeID == "" {
		return
	}
	knowledge, err := s.knowledgeRepo.GetKnowledgeByIDOnly(ctx, knowledgeID)
	if err != nil || knowledge == nil || knowledge.TenantID != tenantID {
		logger.Warnf(ctx, "[EngineSync] schedule skipped for knowledge %s: %v", knowledgeID, err)
		return
	}
	if err := s.knowledgeRepo.MarkEngineSyncPending(ctx, tenantID, knowledgeID); err != nil {
		logger.Warnf(ctx, "[EngineSync] mark pending failed for knowledge %s: %v", knowledgeID, err)
		return
	}
	baseCtx := context.WithoutCancel(ctx)
	key := fmt.Sprintf("%d/%s", tenantID, knowledgeID)
	s.mu.Lock()
	s.pending[key] = struct{}{}
	if _, busy := s.running[key]; busy {
		s.mu.Unlock()
		return
	}
	s.running[key] = struct{}{}
	s.mu.Unlock()
	go s.drainScheduledSync(baseCtx, key, knowledgeID)
}

// drainScheduledSync runs one resync per queued request. A schedule that
// arrives while a push is in flight sets pending again, so the loop re-reads
// chunks and pushes a second time instead of dropping the update.
func (s *EngineSyncService) drainScheduledSync(ctx context.Context, key, knowledgeID string) {
	for {
		s.mu.Lock()
		if _, queued := s.pending[key]; !queued {
			delete(s.running, key)
			s.mu.Unlock()
			return
		}
		delete(s.pending, key)
		s.mu.Unlock()

		knowledge, err := s.knowledgeRepo.GetKnowledgeByIDOnly(ctx, knowledgeID)
		if err != nil || knowledge == nil {
			logger.Warnf(ctx, "[EngineSync] scheduled resync skipped for knowledge %s: %v", knowledgeID, err)
			continue
		}
		lock := s.keyMutex(key)
		lock.Lock()
		err = s.syncOneCore(ctx, knowledge)
		lock.Unlock()
		if err != nil {
			logger.Warnf(ctx, "[EngineSync] auto resync knowledge %s failed: %v", knowledgeID, err)
		}
	}
}

// engineSyncPublishWait bounds how long a publish (merge/rollback) waits for
// the engine to catch up. Finishing inside the window gives read-your-writes;
// larger documents keep syncing in the background.
const engineSyncPublishWait = 5 * time.Second

// PublishKnowledgeChange re-syncs a document after a content change and waits
// briefly so the just-published version is visible to the next query. The
// remaining sync work (if any) continues in the background; status still lands
// on engine_sync_status so failures can be retried.
func (s *EngineSyncService) PublishKnowledgeChange(ctx context.Context, tenantID uint64, knowledgeID string) {
	if s == nil || knowledgeID == "" {
		return
	}
	knowledge, err := s.knowledgeRepo.GetKnowledgeByIDOnly(ctx, knowledgeID)
	if err != nil || knowledge == nil || knowledge.TenantID != tenantID {
		logger.Warnf(ctx, "[EngineSync] publish skipped for knowledge %s: %v", knowledgeID, err)
		return
	}
	if err := s.knowledgeRepo.MarkEngineSyncPending(ctx, tenantID, knowledgeID); err != nil {
		logger.Warnf(ctx, "[EngineSync] mark pending failed for knowledge %s: %v", knowledgeID, err)
		return
	}
	baseCtx := context.WithoutCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- s.syncOne(baseCtx, knowledge) }()
	select {
	case err := <-done:
		if err != nil {
			logger.Warnf(baseCtx, "[EngineSync] publish resync knowledge %s failed: %v", knowledgeID, err)
		}
	case <-time.After(engineSyncPublishWait):
		logger.Infof(baseCtx, "[EngineSync] knowledge %s still syncing after %s; continuing in background",
			knowledgeID, engineSyncPublishWait)
	}
}

// Start registers the daily index resync sweep. Kill switch:
// GAZE_ENGINE_SYNC_CRON_ENABLED=0.
func (s *EngineSyncService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	if !engineSyncCronEnabled() {
		logger.Infof(ctx, "[EngineSync] daily sweep disabled via GAZE_ENGINE_SYNC_CRON_ENABLED=false")
		return nil
	}
	if _, err := s.cron.AddFunc("0 30 3 * * *", func() {
		s.runDaily(context.Background())
	}); err != nil {
		return err
	}
	s.cron.Start()
	s.started = true
	logger.Infof(ctx, "[EngineSync] daily sweep registered (index 03:30)")
	return nil
}

// Stop halts the cron scheduler. Idempotent.
func (s *EngineSyncService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return
	}
	c := s.cron.Stop()
	<-c.Done()
	s.started = false
}

// runDaily pushes every pending/failed document whose tenant opted in. Per-
// tenant check keeps the default-off contract: a tenant that never enabled
// the switch accumulates pending markers until an admin presses the button.
func (s *EngineSyncService) runDaily(ctx context.Context) {
	rows, err := s.knowledgeRepo.ListKnowledgeByEngineSyncStatus(
		ctx, "",
		[]string{types.EngineSyncStatusPending, types.EngineSyncStatusFailed},
	)
	if err != nil {
		logger.Warnf(ctx, "[EngineSync] list pending documents failed: %v", err)
		return
	}
	tenantEnabled := make(map[uint64]bool)
	var synced int
	for _, knowledge := range rows {
		if knowledge == nil {
			continue
		}
		enabled, ok := tenantEnabled[knowledge.TenantID]
		if !ok {
			tenant, err := s.tenantRepo.GetTenantByID(ctx, knowledge.TenantID)
			enabled = err == nil && tenant != nil &&
				tenant.EngineSyncConfig != nil && tenant.EngineSyncConfig.Enabled
			tenantEnabled[knowledge.TenantID] = enabled
		}
		if !enabled {
			continue
		}
		if err := s.syncOne(ctx, knowledge); err != nil {
			logger.Warnf(ctx, "[EngineSync] daily resync knowledge %s failed: %v", knowledge.ID, err)
			continue
		}
		synced++
	}
	if synced > 0 {
		logger.Infof(ctx, "[EngineSync] daily sweep re-pushed %d documents", synced)
	}
}

// syncOne rebuilds markdown from the current chunks and re-ingests with the
// same document ID (YLSW /ingest is an idempotent rebuild). Pushes for the
// same document are serialized so a caller that marked the document pending
// always pushes after the previous push finishes.
func (s *EngineSyncService) syncOne(ctx context.Context, knowledge *types.Knowledge) error {
	if knowledge == nil || knowledge.ID == "" || knowledge.TenantID == 0 {
		return nil
	}
	key := fmt.Sprintf("%d/%s", knowledge.TenantID, knowledge.ID)
	lock := s.keyMutex(key)
	lock.Lock()
	defer lock.Unlock()
	return s.syncOneCore(ctx, knowledge)
}

func (s *EngineSyncService) keyMutex(key string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, ok := s.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		s.locks[key] = lock
	}
	return lock
}

func (s *EngineSyncService) syncOneCore(ctx context.Context, knowledge *types.Knowledge) error {
	now := time.Now()
	fail := func(err error) error {
		if updErr := s.knowledgeRepo.UpdateEngineSyncStatus(
			ctx, knowledge.TenantID, knowledge.ID, types.EngineSyncStatusFailed,
			err.Error(), &now,
		); updErr != nil {
			logger.Warnf(ctx, "[EngineSync] record failure for knowledge %s failed: %v", knowledge.ID, updErr)
		}
		return err
	}

	if s.ragEngine == nil {
		return fail(fmt.Errorf("RAG engine 未配置"))
	}
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, knowledge.KnowledgeBaseID)
	if err != nil || kb == nil || kb.TenantID != knowledge.TenantID {
		return fail(fmt.Errorf("知识库不存在或已变更"))
	}
	chunks, err := s.chunkRepo.ListChunksByKnowledgeIDAndTypes(
		ctx, knowledge.TenantID, knowledge.ID,
		[]types.ChunkType{types.ChunkTypeText, types.ChunkTypeImageOCR, types.ChunkTypeImageCaption},
	)
	if err != nil {
		return fail(fmt.Errorf("读取文档分块失败: %w", err))
	}
	markdown := buildEngineMarkdown(chunks)
	if markdown == "" {
		return fail(fmt.Errorf("文档没有可同步的分块"))
	}

	pushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), engineSyncTimeout)
	defer cancel()
	if err := s.ragEngine.IngestDocument(
		pushCtx, kb.ID, knowledge.ID, knowledge.FileName, knowledge.FileType,
		markdown, toEngineIngestChunks(chunks), buildEngineDocumentMetadata(knowledge),
	); err != nil {
		return fail(err)
	}
	completedAt := time.Now()
	if err := s.knowledgeRepo.UpdateEngineSyncStatus(
		ctx, knowledge.TenantID, knowledge.ID, types.EngineSyncStatusSynced, "", &completedAt,
	); err != nil {
		return err
	}
	logger.Infof(ctx, "[EngineSync] knowledge %s re-pushed (%d chunks)", knowledge.ID, len(chunks))
	return nil
}

// buildEngineMarkdown concatenates enabled text chunks in chunk_index order.
// Disabled chunks are user-removed content and stay out of the engine.
func buildEngineMarkdown(chunks []*types.Chunk) string {
	parts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk == nil || !chunk.IsEnabled || chunk.ChunkType != types.ChunkTypeText {
			continue
		}
		content := strings.TrimSpace(chunk.Content)
		if content == "" {
			continue
		}
		parts = append(parts, content)
	}
	return strings.Join(parts, "\n\n")
}

// toEngineIngestChunks mirrors enabled app chunks for YLSW's app-owner map.
// Image OCR/caption children are merged per (URL, parent text chunk) into one
// image record carrying caption/OCR only; the image bytes stay in app storage.
func toEngineIngestChunks(chunks []*types.Chunk) []engine.IngestChunk {
	out := make([]engine.IngestChunk, 0, len(chunks))
	imageIndex := make(map[string]int)
	for _, chunk := range chunks {
		if chunk == nil || !chunk.IsEnabled {
			continue
		}
		if chunk.ChunkType == types.ChunkTypeImageOCR || chunk.ChunkType == types.ChunkTypeImageCaption {
			var infos []types.ImageInfo
			if chunk.ImageInfo == "" || json.Unmarshal([]byte(chunk.ImageInfo), &infos) != nil {
				continue
			}
			for _, info := range infos {
				url := strings.TrimSpace(info.URL)
				if url == "" {
					url = strings.TrimSpace(info.OriginalURL)
				}
				if url == "" {
					continue
				}
				key := url + "|" + chunk.ParentChunkID
				caption := strings.TrimSpace(info.Caption)
				ocr := strings.TrimSpace(info.OCRText)
				if idx, ok := imageIndex[key]; ok {
					if out[idx].ImageCaption == "" && caption != "" {
						out[idx].ImageCaption = caption
					}
					if out[idx].ImageOCR == "" && ocr != "" {
						out[idx].ImageOCR = ocr
					}
					out[idx].Content = imageRecordContent(caption, ocr)
					continue
				}
				content := imageRecordContent(caption, ocr)
				if content == "" {
					continue
				}
				out = append(out, engine.IngestChunk{
					ID:            chunk.ID,
					ChunkIndex:    chunk.ChunkIndex,
					ChunkType:     "image",
					ParentChunkID: chunk.ParentChunkID,
					ImageURL:      url,
					ImageCaption:  caption,
					ImageOCR:      ocr,
					Content:       content,
				})
				imageIndex[key] = len(out) - 1
			}
			continue
		}
		if strings.TrimSpace(chunk.Content) == "" {
			continue
		}
		out = append(out, engine.IngestChunk{
			ID:         chunk.ID,
			ChunkIndex: chunk.ChunkIndex,
			StartAt:    chunk.StartAt,
			EndAt:      chunk.EndAt,
			Content:    chunk.Content,
			ChunkType:  "text",
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ChunkIndex < out[j].ChunkIndex })
	return out
}

func imageRecordContent(caption, ocr string) string {
	if caption == "" && ocr == "" {
		return "[图片]"
	}
	if caption == "" {
		return ocr
	}
	if ocr == "" {
		return caption
	}
	return caption + "\n" + ocr
}

// engineSyncCronEnabled mirrors ylswReconcileEnabled(): default on, opt-out
// via GAZE_ENGINE_SYNC_CRON_ENABLED=0/false/off/no.
func engineSyncCronEnabled() bool {
	v := strings.TrimSpace(os.Getenv("GAZE_ENGINE_SYNC_CRON_ENABLED"))
	if v == "" {
		return true
	}
	switch strings.ToLower(v) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}
