package service

import (
	"context"
	"fmt"
	"testing"

	"gaze/internal/types"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeMultimodalEngineSync struct {
	calls       int
	tenantID    uint64
	knowledgeID string
}

func (f *fakeMultimodalEngineSync) ScheduleKnowledgeSync(_ context.Context, tenantID uint64, knowledgeID string) {
	f.calls++
	f.tenantID = tenantID
	f.knowledgeID = knowledgeID
}

func TestImageMultimodalFinalizeSchedulesEngineResync(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	const tenantID = uint64(7)
	const knowledgeID = "k-engine-sync"
	redisKey := fmt.Sprintf("multimodal:pending:%s", knowledgeID)
	if err := rdb.Set(context.Background(), redisKey, 1, 0).Err(); err != nil {
		t.Fatal(err)
	}

	sync := &fakeMultimodalEngineSync{}
	enqueuer := &orphanTaskEnqueuer{}
	svc := &ImageMultimodalService{
		redisClient:  rdb,
		taskEnqueuer: enqueuer,
		engineSync:   sync,
	}
	svc.checkAndFinalizeAllImages(context.Background(), types.ImageMultimodalPayload{
		TenantID:    tenantID,
		KnowledgeID: knowledgeID,
	}, true)

	if sync.calls != 1 {
		t.Fatalf("expected one engine resync, got %d", sync.calls)
	}
	if sync.tenantID != tenantID || sync.knowledgeID != knowledgeID {
		t.Fatalf("engine resync target = %d/%s", sync.tenantID, sync.knowledgeID)
	}
	if mr.Exists(redisKey) {
		t.Fatal("pending counter should be cleared")
	}
	if len(enqueuer.enqueued) != 1 || enqueuer.enqueued[0].Type() != types.TypeKnowledgePostProcess {
		t.Fatalf("expected post-process enqueue, got %d tasks", len(enqueuer.enqueued))
	}
}

func TestImageMultimodalDropDoesNotScheduleEngineResync(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	const knowledgeID = "k-dropped"
	redisKey := fmt.Sprintf("multimodal:pending:%s", knowledgeID)
	if err := rdb.Set(context.Background(), redisKey, 1, 0).Err(); err != nil {
		t.Fatal(err)
	}

	sync := &fakeMultimodalEngineSync{}
	svc := &ImageMultimodalService{
		redisClient:  rdb,
		taskEnqueuer: &orphanTaskEnqueuer{},
		engineSync:   sync,
	}
	svc.checkAndFinalizeAllImages(context.Background(), types.ImageMultimodalPayload{
		TenantID:    7,
		KnowledgeID: knowledgeID,
	}, false)

	if sync.calls != 0 {
		t.Fatalf("dropped image must not resync engine, got %d calls", sync.calls)
	}
}
