package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"gaze/internal/application/repository"
	"gaze/internal/engine"
	"gaze/internal/logger"
	"gaze/internal/types"
)

const (
	retrievalAuditBatchSize     = 50
	retrievalAuditFlushInterval = 2 * time.Second
	retrievalAuditMaxBuffer     = 2048
)

type retrievalAuditService struct {
	repo          repository.RetrievalAuditRepository
	batchSize     int
	flushInterval time.Duration
	maxBuffer     int

	mu         sync.Mutex
	buffer     []*types.RetrievalAuditLog
	flushTimer *time.Timer
}

func NewRetrievalAuditService(repo repository.RetrievalAuditRepository) engine.RetrievalAuditSink {
	return newRetrievalAuditService(repo, retrievalAuditBatchSize, retrievalAuditFlushInterval, retrievalAuditMaxBuffer)
}

func newRetrievalAuditService(
	repo repository.RetrievalAuditRepository,
	batchSize int,
	flushInterval time.Duration,
	maxBuffer int,
) *retrievalAuditService {
	if batchSize <= 0 {
		batchSize = retrievalAuditBatchSize
	}
	if flushInterval <= 0 {
		flushInterval = retrievalAuditFlushInterval
	}
	if maxBuffer <= 0 {
		maxBuffer = retrievalAuditMaxBuffer
	}
	return &retrievalAuditService{
		repo:          repo,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		maxBuffer:     maxBuffer,
	}
}

func (s *retrievalAuditService) LogRetrieval(ctx context.Context, event engine.RetrievalAuditEvent) {
	if s == nil || s.repo == nil || event.TenantID == 0 {
		return
	}
	entry := &types.RetrievalAuditLog{
		TenantID:      event.TenantID,
		UserID:        event.UserID,
		PrincipalKind: normalizePrincipalKind(event),
		SessionID:     event.SessionID,
		QueryDigest:   queryDigest(event.Query),
		KBIDs:         jsonString(event.KnowledgeBaseIDs),
		HitDocIDs:     jsonString(uniqueNonEmpty(event.HitDocumentIDs)),
		Decision:      event.Decision,
		Error:         truncateAuditRunes(event.Error, 500),
		CreatedAt:     time.Now(),
	}
	if entry.Decision == "" {
		entry.Decision = "ok"
	}
	s.enqueue(entry)
}

func (s *retrievalAuditService) enqueue(entry *types.RetrievalAuditLog) {
	s.mu.Lock()
	if len(s.buffer) >= s.maxBuffer {
		s.mu.Unlock()
		logger.Warnf(context.Background(),
			"retrieval audit buffer full (%d); dropping event", s.maxBuffer)
		return
	}
	s.buffer = append(s.buffer, entry)
	if len(s.buffer) >= s.batchSize {
		entries := s.buffer
		s.buffer = nil
		if s.flushTimer != nil {
			s.flushTimer.Stop()
			s.flushTimer = nil
		}
		s.mu.Unlock()
		go s.flush(entries)
		return
	}
	if s.flushTimer == nil {
		s.flushTimer = time.AfterFunc(s.flushInterval, s.flushBuffered)
	}
	s.mu.Unlock()
}

func (s *retrievalAuditService) flushBuffered() {
	s.mu.Lock()
	entries := s.buffer
	s.buffer = nil
	s.flushTimer = nil
	s.mu.Unlock()
	s.flush(entries)
}

func (s *retrievalAuditService) flush(entries []*types.RetrievalAuditLog) {
	if len(entries) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.repo.CreateBatch(ctx, entries); err != nil {
		logger.Warnf(ctx, "retrieval audit batch write failed (%d entries): %v", len(entries), err)
	}
}

func normalizePrincipalKind(event engine.RetrievalAuditEvent) string {
	if strings.TrimSpace(event.UserID) == "" {
		return "portal"
	}
	if event.PrincipalKind != "" {
		return event.PrincipalKind
	}
	return "member"
}

func queryDigest(query string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(query)))
	return hex.EncodeToString(sum[:16])
}

func jsonString(values []string) string {
	raw, err := json.Marshal(uniqueNonEmpty(values))
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func truncateAuditRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
