package service

import (
	"context"
	"path/filepath"
	"testing"

	"gaze/internal/application/repository"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type documentKBLookup struct {
	interfaces.KnowledgeBaseService
	values map[string]*types.KnowledgeBase
	err    error
}

func (r *documentKBLookup) GetKnowledgeBaseByID(_ context.Context, id string) (*types.KnowledgeBase, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.values[id] == nil {
		return nil, repository.ErrKnowledgeBaseNotFound
	}
	return r.values[id], nil
}

type documentKBRepo struct {
	interfaces.KnowledgeBaseRepository
	lookup *documentKBLookup
}

func (r documentKBRepo) GetKnowledgeBaseByID(ctx context.Context, id string) (*types.KnowledgeBase, error) {
	return r.lookup.GetKnowledgeBaseByID(ctx, id)
}

type documentKnowledgeSpy struct {
	interfaces.KnowledgeRepository
	writes int
}

func (r *documentKnowledgeSpy) UpdateKnowledge(ctx context.Context, k *types.Knowledge) error {
	r.writes++
	return r.KnowledgeRepository.UpdateKnowledge(ctx, k)
}

func (r *documentKnowledgeSpy) SetKnowledgeTags(ctx context.Context, id string, tags []string) error {
	r.writes++
	return r.KnowledgeRepository.SetKnowledgeTags(ctx, id, tags)
}

func (r *documentKnowledgeSpy) DeleteKnowledgeList(ctx context.Context, tenant uint64, ids []string) error {
	r.writes++
	return r.KnowledgeRepository.DeleteKnowledgeList(ctx, tenant, ids)
}

type documentChunkSpy struct {
	interfaces.ChunkRepository
	writes   int
	imageErr error
}

func (r *documentChunkSpy) CreateChunks(ctx context.Context, chunks []*types.Chunk) error {
	r.writes++
	return r.ChunkRepository.CreateChunks(ctx, chunks)
}

func (r *documentChunkSpy) UpdateChunks(ctx context.Context, chunks []*types.Chunk) error {
	r.writes++
	return r.ChunkRepository.UpdateChunks(ctx, chunks)
}

func (r *documentChunkSpy) UpdateChunk(ctx context.Context, chunk *types.Chunk) error {
	r.writes++
	return r.ChunkRepository.UpdateChunk(ctx, chunk)
}

func (r *documentChunkSpy) DeleteChunks(ctx context.Context, tenant uint64, ids []string) error {
	r.writes++
	return r.ChunkRepository.DeleteChunks(ctx, tenant, ids)
}

func (r *documentChunkSpy) DeleteChunk(ctx context.Context, tenant uint64, id string) error {
	r.writes++
	return r.ChunkRepository.DeleteChunk(ctx, tenant, id)
}

func (r *documentChunkSpy) DeleteByKnowledgeList(ctx context.Context, tenant uint64, ids []string) error {
	r.writes++
	return r.ChunkRepository.DeleteByKnowledgeList(ctx, tenant, ids)
}

func (r *documentChunkSpy) DeleteChunksByKnowledgeID(ctx context.Context, tenant uint64, id string) error {
	r.writes++
	return r.ChunkRepository.DeleteChunksByKnowledgeID(ctx, tenant, id)
}

func (r *documentChunkSpy) SaveChunkRevision(
	ctx context.Context,
	chunk *types.Chunk,
	rev *types.ChunkRevision,
	expected int,
) error {
	r.writes++
	return r.ChunkRepository.SaveChunkRevision(ctx, chunk, rev, expected)
}

func (r *documentChunkSpy) ListImageInfoByKnowledgeIDs(
	ctx context.Context,
	tenant uint64,
	ids []string,
) ([]interfaces.ChunkImageInfo, error) {
	if r.imageErr != nil {
		return nil, r.imageErr
	}
	return r.ChunkRepository.ListImageInfoByKnowledgeIDs(ctx, tenant, ids)
}

type documentGraphSpy struct {
	interfaces.RetrieveGraphRepository
	err   error
	calls int
}

func (r *documentGraphSpy) DelGraph(context.Context, []types.NameSpace) error {
	r.calls++
	return r.err
}

type documentTenantSpy struct {
	interfaces.TenantRepository
	adjustments []int64
}

func (r *documentTenantSpy) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	return &types.Tenant{ID: id}, nil
}

func (r *documentTenantSpy) AdjustStorageUsed(_ context.Context, _ uint64, delta int64) error {
	r.adjustments = append(r.adjustments, delta)
	return nil
}

type documentFileSpy struct {
	interfaces.FileService
	deleted      []string
	beforeDelete func()
}

func (r *documentFileSpy) DeleteFile(_ context.Context, path string) error {
	if r.beforeDelete != nil {
		r.beforeDelete()
	}
	r.deleted = append(r.deleted, path)
	return nil
}

type documentWriteFixture struct {
	svc       *knowledgeService
	chunks    *chunkService
	repo      *documentKnowledgeSpy
	chunkRepo *documentChunkSpy
	kbs       *documentKBLookup
	graph     *documentGraphSpy
	files     *documentFileSpy
	tenants   *documentTenantSpy
	db        *gorm.DB
	ctx       context.Context
}

func newDocumentWriteFixture(t *testing.T) *documentWriteFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "document.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(
		t,
		db.AutoMigrate(
			&types.Knowledge{},
			&types.Chunk{},
			&types.KnowledgeTag{},
			&types.KnowledgeTagRelation{},
			&types.ChunkRevision{},
		),
	)
	repo := &documentKnowledgeSpy{KnowledgeRepository: repository.NewKnowledgeRepository(db)}
	chunkRepo := &documentChunkSpy{ChunkRepository: repository.NewChunkRepository(db)}
	kbs := &documentKBLookup{values: map[string]*types.KnowledgeBase{
		"kb": {ID: "kb", TenantID: 7}, "other": {ID: "other", TenantID: 7},
	}}
	for _, row := range []*types.Knowledge{
		{
			ID:              "doc",
			TenantID:        7,
			KnowledgeBaseID: "kb",
			Title:           "original",
			Type:            types.KnowledgeTypeManual,
			ParseStatus:     types.ManualKnowledgeStatusDraft,
			StorageSize:     5,
		},

		{
			ID:              "other-doc",
			TenantID:        7,
			KnowledgeBaseID: "other",
			Title:           "other",
			ParseStatus:     types.ParseStatusCompleted,
		},
	} {
		require.NoError(t, db.Create(row).Error)
	}
	for _, row := range []*types.Chunk{
		{
			ID:              "chunk",
			TenantID:        7,
			KnowledgeID:     "doc",
			KnowledgeBaseID: "kb",
			ChunkType:       types.ChunkTypeText,
			Content:         "original",
			IndexStatus:     "ready",
		},

		{
			ID:              "other-chunk",
			TenantID:        7,
			KnowledgeID:     "other-doc",
			KnowledgeBaseID: "other",
			ChunkType:       types.ChunkTypeText,
		},
	} {
		require.NoError(t, db.Create(row).Error)
	}
	require.NoError(t, db.Create(&types.KnowledgeTag{ID: "tag", TenantID: 7, KnowledgeBaseID: "kb", Name: "tag"}).Error)
	graph, files, tenants := &documentGraphSpy{}, &documentFileSpy{}, &documentTenantSpy{}
	chunks := &chunkService{chunkRepository: chunkRepo, knowledgeRepo: repo, kbRepository: documentKBRepo{lookup: kbs}}
	svc := &knowledgeService{
		repo:         repo,
		chunkRepo:    chunkRepo,
		chunkService: chunks,
		kbService:    kbs,
		graphEngine:  graph,
		fileSvc:      files,
		tenantRepo:   tenants,
		tagRepo:      repository.NewKnowledgeTagRepository(db),
	}
	ctx := types.WithCaller(
		context.Background(),
		types.Caller{TenantID: 7, UserID: "owner", Role: types.TenantRoleOwner},
	)
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, &types.Tenant{ID: 7})
	return &documentWriteFixture{
		svc:       svc,
		chunks:    chunks,
		repo:      repo,
		chunkRepo: chunkRepo,
		kbs:       kbs,
		graph:     graph,
		files:     files,
		tenants:   tenants,
		db:        db,
		ctx:       ctx,
	}
}

func (f *documentWriteFixture) requireNoWrites(t *testing.T) {
	t.Helper()
	require.Zero(t, f.repo.writes)
	require.Zero(t, f.chunkRepo.writes)
	require.Zero(t, f.graph.calls)
	require.Empty(t, f.files.deleted)
}
