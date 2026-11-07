package repository

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"gaze/internal/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// gazePagesTestDDL is a minimal SQLite-compatible subset of the
// production gaze_pages DDL (migrations/versioned/000037_gaze_and_indexing
// plus 000100_gaze_outdated). JSONB is stored as TEXT in SQLite; the
// StringArray Scan/Value pair handles the JSON round-trip unchanged.
const gazePagesTestDDL = `
CREATE TABLE IF NOT EXISTS gaze_pages (
    id                VARCHAR(36) PRIMARY KEY,
    tenant_id         INTEGER NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    slug              VARCHAR(255) NOT NULL,
    title             VARCHAR(512) NOT NULL DEFAULT '',
    page_type         VARCHAR(32) NOT NULL DEFAULT 'summary',
    status            VARCHAR(32) NOT NULL DEFAULT 'published',
    content           TEXT NOT NULL DEFAULT '',
    summary           TEXT NOT NULL DEFAULT '',
    parent_slug       VARCHAR(255) NOT NULL DEFAULT '',
    folder_id         VARCHAR(36) NOT NULL DEFAULT '',
    category_path     TEXT DEFAULT '[]',
    gaze_path         VARCHAR(1024) NOT NULL DEFAULT '',
    depth             INTEGER NOT NULL DEFAULT 0,
    sort_order        INTEGER NOT NULL DEFAULT 0,
    source_refs       TEXT DEFAULT '[]',
    chunk_refs        TEXT DEFAULT '[]',
    in_links          TEXT DEFAULT '[]',
    out_links         TEXT DEFAULT '[]',
    page_metadata     TEXT DEFAULT '{}',
    aliases           TEXT DEFAULT '[]',
    version           INTEGER NOT NULL DEFAULT 1,
    last_edit_source  VARCHAR(16) NOT NULL DEFAULT '',
    last_editor_id    VARCHAR(64) NOT NULL DEFAULT '',
    outdated          BOOLEAN NOT NULL DEFAULT 0,
    superseded_by_slug VARCHAR(255) NOT NULL DEFAULT '',
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at        DATETIME
);
`

// gazePageRevisionsTestDDL mirrors the production gaze_page_revisions DDL
// (migrations/versioned/000075_gaze_page_revisions.up.sql) for SQLite.
const gazePageRevisionsTestDDL = `
CREATE TABLE IF NOT EXISTS gaze_page_revisions (
    id                VARCHAR(36) PRIMARY KEY,
    tenant_id         INTEGER NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    page_id           VARCHAR(36) NOT NULL,
    slug              VARCHAR(255) NOT NULL,
    version           INTEGER NOT NULL,
    title             VARCHAR(512) NOT NULL DEFAULT '',
    page_type         VARCHAR(32) NOT NULL DEFAULT 'summary',
    status            VARCHAR(32) NOT NULL DEFAULT 'published',
    content           TEXT NOT NULL DEFAULT '',
    summary           TEXT NOT NULL DEFAULT '',
    aliases           TEXT DEFAULT '[]',
    edit_source       VARCHAR(16) NOT NULL DEFAULT '',
    editor_id         VARCHAR(64) NOT NULL DEFAULT '',
    edited_at         DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_gaze_page_revisions_page_version
    ON gaze_page_revisions (page_id, version);
`

// gazeFoldersTestDDL mirrors the production gaze_folders DDL for SQLite.
const gazeFoldersTestDDL = `
CREATE TABLE IF NOT EXISTS gaze_folders (
    id                VARCHAR(36) PRIMARY KEY,
    tenant_id         INTEGER NOT NULL DEFAULT 0,
    knowledge_base_id VARCHAR(36) NOT NULL,
    parent_id         VARCHAR(36) NOT NULL DEFAULT '',
    name              VARCHAR(255) NOT NULL,
    path              VARCHAR(1024) NOT NULL DEFAULT '',
    depth             INTEGER NOT NULL DEFAULT 0,
    sort_order        INTEGER NOT NULL DEFAULT 0,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at        DATETIME
);
`

func setupGazePagesTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(gazePagesTestDDL).Error)
	require.NoError(t, db.Exec(gazeFoldersTestDDL).Error)
	for _, stmt := range strings.Split(strings.TrimSpace(gazePageRevisionsTestDDL), ";") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		require.NoError(t, db.Exec(stmt).Error)
	}
	return db
}

// makeGazePage builds a minimal GazePage suitable for insert. Title is
// derived from the slug so ORDER BY title ASC yields a predictable
// test ordering without callers having to spell out both fields.
func makeGazePage(kbID, slug, pageType, status string) *types.GazePage {
	title := slug
	if idx := strings.LastIndex(slug, "/"); idx >= 0 {
		title = slug[idx+1:]
	}
	return &types.GazePage{
		ID:              uuid.New().String(),
		TenantID:        1,
		KnowledgeBaseID: kbID,
		Slug:            slug,
		Title:           title,
		PageType:        pageType,
		Status:          status,
		Content:         "body of " + slug,
		Summary:         "summary of " + slug,
		GazePath:        pageType + "/" + title,
		Version:         1,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func makeCategorizedGazePage(kbID, slug, pageType, status string, categoryPath ...string) *types.GazePage {
	page := makeGazePage(kbID, slug, pageType, status)
	page.CategoryPath = types.StringArray(categoryPath)
	if len(categoryPath) > 0 {
		page.GazePath = pageType + "/" + strings.Join(categoryPath, "/") + "/" + page.Title
		page.Depth = len(categoryPath)
	}
	return page
}

// TestList_GazePathSortReturnsCategorizedPagesFirst protects the sidebar's
// IDE-like tree contract. Pagination happens in the repository, so the DB
// must return pages with category_path before loose root pages; otherwise the
// frontend cannot know about directories hiding on later pages.
func TestList_GazePathSortReturnsCategorizedPagesFirst(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	pages := []*types.GazePage{
		makeGazePage("kb-a", "entity/000-root", types.GazePageTypeEntity, types.GazePageStatusPublished),
		makeCategorizedGazePage("kb-a", "entity/999-child", types.GazePageTypeEntity, types.GazePageStatusPublished, "zzz-folder"),
		makeGazePage("kb-a", "entity/001-root", types.GazePageTypeEntity, types.GazePageStatusPublished),
	}
	for _, p := range pages {
		require.NoError(t, repo.Create(ctx, p))
	}

	got, total, err := repo.List(ctx, &types.GazePageListRequest{
		KnowledgeBaseID: "kb-a",
		PageType:        types.GazePageTypeEntity,
		Page:            1,
		PageSize:        10,
		SortBy:          "gaze_path",
		SortOrder:       "asc",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, got, 3)
	assert.Equal(t, "entity/999-child", got[0].Slug)
	assert.Equal(t, "entity/000-root", got[1].Slug)
	assert.Equal(t, "entity/001-root", got[2].Slug)
}

// TestList_CategoryPathFilterMatchesOnSQLite protects the directory view on the
// SQLite build: the sidebar loads a folder's pages with category_path +
// category_depth, and StringArray is stored as a BLOB there, so a plain
// equality against a TEXT bind never matched and every folder listed empty.
func TestList_CategoryPathFilterMatchesOnSQLite(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	entity, published := types.GazePageTypeEntity, types.GazePageStatusPublished
	pages := []*types.GazePage{
		makeCategorizedGazePage("kb-c", "entity/in-ai", entity, published, "AI"),
		makeCategorizedGazePage("kb-c", "entity/in-ai-llm", entity, published, "AI", "LLM"),
		makeCategorizedGazePage("kb-c", "entity/in-people", entity, published, "人物"),
		makeGazePage("kb-c", "entity/root", entity, published),
	}
	for _, p := range pages {
		require.NoError(t, repo.Create(ctx, p))
	}

	depth := 1
	got, total, err := repo.List(ctx, &types.GazePageListRequest{
		KnowledgeBaseID: "kb-c",
		PageType:        types.GazePageTypeEntity,
		CategoryPath:    types.StringArray{"AI"},
		CategoryDepth:   &depth,
		Page:            1,
		PageSize:        10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, got, 1)
	assert.Equal(t, "entity/in-ai", got[0].Slug)

	got, total, err = repo.List(ctx, &types.GazePageListRequest{
		KnowledgeBaseID: "kb-c",
		PageType:        types.GazePageTypeEntity,
		CategoryPath:    types.StringArray{"人物"},
		Page:            1,
		PageSize:        10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, got, 1)
	assert.Equal(t, "entity/in-people", got[0].Slug)
}

// TestFolderTree_CRUDAndChildListing exercises the gaze_folders repository:
// child listing ordered by sort_order/name, find-by-name, page counting under
// a folder, and that ListDistinctCategoryPaths reflects the folder paths.
func TestFolderTree_CRUDAndChildListing(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	mk := func(id, parentID, name, path string, depth int) *types.GazeFolder {
		return &types.GazeFolder{
			ID: id, TenantID: 1, KnowledgeBaseID: "kb-f",
			ParentID: parentID, Name: name, Path: path, Depth: depth,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
	}
	ai := mk("f-ai", types.GazeFolderRootID, "AI", "AI", 1)
	people := mk("f-people", types.GazeFolderRootID, "人物", "人物", 1)
	llm := mk("f-llm", "f-ai", "LLM", "AI/LLM", 2)
	for _, f := range []*types.GazeFolder{ai, people, llm} {
		require.NoError(t, repo.CreateFolder(ctx, f))
	}

	// Root children: AI, 人物 (ordered by name within equal sort_order).
	roots, err := repo.ListChildFolders(ctx, "kb-f", types.GazeFolderRootID)
	require.NoError(t, err)
	require.Len(t, roots, 2)

	// Direct child of AI is LLM.
	child, err := repo.GetChildFolderByName(ctx, "kb-f", "f-ai", "LLM")
	require.NoError(t, err)
	assert.Equal(t, "f-llm", child.ID)

	// Missing child surfaces the typed not-found error.
	_, err = repo.GetChildFolderByName(ctx, "kb-f", "f-ai", "Nope")
	assert.ErrorIs(t, err, ErrGazeFolderNotFound)

	// Pages filed into folders are counted (archived excluded).
	pAI := makeCategorizedGazePage("kb-f", "entity/a1", types.GazePageTypeEntity, types.GazePageStatusPublished, "AI")
	pAI.FolderID = "f-ai"
	pLLM := makeCategorizedGazePage("kb-f", "entity/a2", types.GazePageTypeEntity, types.GazePageStatusPublished, "AI", "LLM")
	pLLM.FolderID = "f-llm"
	pArch := makeCategorizedGazePage("kb-f", "entity/a3", types.GazePageTypeEntity, types.GazePageStatusArchived, "AI")
	pArch.FolderID = "f-ai"
	for _, p := range []*types.GazePage{pAI, pLLM, pArch} {
		require.NoError(t, repo.Create(ctx, p))
	}

	aiCount, err := repo.CountPagesInFolder(ctx, "kb-f", "f-ai")
	require.NoError(t, err)
	assert.Equal(t, int64(1), aiCount, "archived page excluded; LLM page is under f-llm, not f-ai")

	// ListDistinctCategoryPaths returns the folder paths split into segments.
	paths, err := repo.ListDistinctCategoryPaths(ctx, "kb-f", 100)
	require.NoError(t, err)
	assert.Contains(t, paths, []string{"AI"})
	assert.Contains(t, paths, []string{"AI", "LLM"})
	assert.Contains(t, paths, []string{"人物"})

	// Pages can be fetched by their folder ids for subtree recompute.
	pages, err := repo.ListPagesByFolderIDs(ctx, "kb-f", []string{"f-ai", "f-llm"})
	require.NoError(t, err)
	assert.Len(t, pages, 3)

	// Repository deletion re-checks emptiness atomically, so a concurrent page
	// move / child create cannot slip between the service check and soft delete.
	err = repo.DeleteFolder(ctx, "kb-f", "f-ai")
	assert.ErrorIs(t, err, ErrGazeFolderNotEmpty)
	require.NoError(t, repo.DeleteFolder(ctx, "kb-f", "f-people"))
	_, err = repo.GetFolderByID(ctx, "kb-f", "f-people")
	assert.ErrorIs(t, err, ErrGazeFolderNotFound)
}

// TestListByTypeLight_ProjectsNarrowColumnsAndExcludesArchived verifies
// that the index-view projection only emits the slug/title/summary
// triples and respects the archived-status filter. This is the whole
// point of splitting the method off ListByType — the index reader must
// not pay for TEXT content transport.
func TestListByTypeLight_ProjectsNarrowColumnsAndExcludesArchived(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	pages := []*types.GazePage{
		makeGazePage("kb-a", "entity/alpha", types.GazePageTypeEntity, types.GazePageStatusPublished),
		makeGazePage("kb-a", "entity/beta", types.GazePageTypeEntity, types.GazePageStatusDraft),
		makeGazePage("kb-a", "entity/gamma", types.GazePageTypeEntity, types.GazePageStatusArchived),
		makeGazePage("kb-a", "concept/delta", types.GazePageTypeConcept, types.GazePageStatusPublished),
		makeGazePage("kb-other", "entity/leaked", types.GazePageTypeEntity, types.GazePageStatusPublished),
	}
	for _, p := range pages {
		require.NoError(t, repo.Create(ctx, p))
	}

	entries, total, err := repo.ListByTypeLight(ctx, "kb-a", types.GazePageTypeEntity, 50, 0)
	require.NoError(t, err)

	// Archived is excluded; sibling KB is excluded; everything else
	// surfaces regardless of draft/published status (the index shows
	// both so admins notice newly-drafted pages).
	assert.Equal(t, int64(2), total)
	require.Len(t, entries, 2)

	// ORDER BY title ASC => alpha, beta.
	assert.Equal(t, "entity/alpha", entries[0].Slug)
	assert.Equal(t, "alpha", entries[0].Title)
	assert.Equal(t, "summary of entity/alpha", entries[0].Summary)
	assert.Equal(t, "entity/beta", entries[1].Slug)
}

// TestListByTypeLight_Pagination walks the type list using offsets and
// asserts the count stays stable regardless of where in the list we
// are — the index handler uses total to render "showing N of M".
func TestListByTypeLight_Pagination(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	for _, s := range []string{"entity/a", "entity/b", "entity/c", "entity/d", "entity/e"} {
		require.NoError(t, repo.Create(ctx, makeGazePage("kb-a", s, types.GazePageTypeEntity, types.GazePageStatusPublished)))
	}

	page1, total1, err := repo.ListByTypeLight(ctx, "kb-a", types.GazePageTypeEntity, 2, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total1)
	require.Len(t, page1, 2)
	assert.Equal(t, "entity/a", page1[0].Slug)
	assert.Equal(t, "entity/b", page1[1].Slug)

	page2, total2, err := repo.ListByTypeLight(ctx, "kb-a", types.GazePageTypeEntity, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total2, "total should be stable across pages")
	require.Len(t, page2, 2)
	assert.Equal(t, "entity/c", page2[0].Slug)
	assert.Equal(t, "entity/d", page2[1].Slug)

	page3, total3, err := repo.ListByTypeLight(ctx, "kb-a", types.GazePageTypeEntity, 2, 4)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total3)
	require.Len(t, page3, 1)
	assert.Equal(t, "entity/e", page3[0].Slug)

	// Offset past the end yields an empty list, not an error — the
	// handler relies on this to short-circuit pagination tails.
	page4, total4, err := repo.ListByTypeLight(ctx, "kb-a", types.GazePageTypeEntity, 2, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total4)
	assert.Empty(t, page4)
}

// TestListByTypeLight_EmptyType_ReturnsZero exercises the "no rows"
// short-circuit. We skip the SELECT entirely when count is zero, so
// a KB with no pages of a type shouldn't burn a pointless query.
func TestListByTypeLight_EmptyType_ReturnsZero(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	entries, total, err := repo.ListByTypeLight(ctx, "kb-empty", types.GazePageTypeSynthesis, 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, entries)
}

func TestListPagesCursorExcludesArchivedPages(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	mk := func(id, slug, status string) *types.GazePage {
		page := makeGazePage("kb-lint", slug, types.GazePageTypeConcept, status)
		page.ID = id
		return page
	}
	pages := []*types.GazePage{
		mk("001", "concept/live-a", types.GazePageStatusPublished),
		mk("002", "concept/archived", types.GazePageStatusArchived),
		mk("003", "concept/live-b", types.GazePageStatusDraft),
		mk("004", "concept/other-kb", types.GazePageStatusPublished),
	}
	pages[3].KnowledgeBaseID = "kb-other"
	for _, p := range pages {
		require.NoError(t, repo.Create(ctx, p))
	}

	first, next, err := repo.ListPagesCursor(ctx, "kb-lint", "", 1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	assert.Equal(t, "concept/live-a", first[0].Slug)
	assert.Equal(t, "001", next)

	second, next, err := repo.ListPagesCursor(ctx, "kb-lint", next, 1)
	require.NoError(t, err)
	require.Len(t, second, 1)
	assert.Equal(t, "concept/live-b", second[0].Slug)
	assert.Equal(t, "003", next)

	third, next, err := repo.ListPagesCursor(ctx, "kb-lint", next, 1)
	require.NoError(t, err)
	assert.Empty(t, third)
	assert.Empty(t, next)
}

// TestListByTypeLight_ClampsLimit verifies the [1, 200] clamp. We don't
// want a client passing limit=100000 and forcing the DB to return a
// multi-MB response.
func TestListByTypeLight_ClampsLimit(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	for i := 0; i < 250; i++ {
		slug := "entity/bulk-"
		// Use a stable, zero-padded suffix so title ordering is
		// deterministic for the cap assertion below.
		slug += string(rune('a'+(i/26)%26)) + string(rune('a'+i%26))
		require.NoError(t, repo.Create(ctx, makeGazePage("kb-cap", slug, types.GazePageTypeEntity, types.GazePageStatusPublished)))
	}

	// limit=0 falls back to the default of 50.
	defaultEntries, _, err := repo.ListByTypeLight(ctx, "kb-cap", types.GazePageTypeEntity, 0, 0)
	require.NoError(t, err)
	assert.Len(t, defaultEntries, 50)

	// limit=5000 clamps to 200.
	clampedEntries, _, err := repo.ListByTypeLight(ctx, "kb-cap", types.GazePageTypeEntity, 5000, 0)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(clampedEntries), 200)
}

func TestCountOrphans_SQLiteCountsEmptyInLinks(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	orphan := makeGazePage("kb-orphans", "entity/orphan", types.GazePageTypeEntity, types.GazePageStatusPublished)
	orphan.InLinks = types.StringArray{}
	linked := makeGazePage("kb-orphans", "entity/linked", types.GazePageTypeEntity, types.GazePageStatusPublished)
	linked.InLinks = types.StringArray{"entity/source"}
	indexPage := makeGazePage("kb-orphans", "index", types.GazePageTypeIndex, types.GazePageStatusPublished)
	indexPage.InLinks = types.StringArray{}
	otherKB := makeGazePage("kb-other", "entity/other", types.GazePageTypeEntity, types.GazePageStatusPublished)
	otherKB.InLinks = types.StringArray{}

	for _, p := range []*types.GazePage{orphan, linked, indexPage, otherKB} {
		require.NoError(t, repo.Create(ctx, p))
	}

	got, err := repo.CountOrphans(ctx, "kb-orphans")
	require.NoError(t, err)
	assert.Equal(t, int64(1), got)
}

func TestGazeStatsQueriesExcludeArchivedPages(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	live := makeGazePage("kb-stats", "entity/live", types.GazePageTypeEntity, types.GazePageStatusPublished)
	live.InLinks = types.StringArray{"index"}
	live.OutLinks = types.StringArray{"concept/live-target"}
	archived := makeGazePage("kb-stats", "entity/archived", types.GazePageTypeEntity, types.GazePageStatusArchived)
	archived.InLinks = types.StringArray{}
	archived.OutLinks = types.StringArray{"concept/archived-target"}
	indexPage := makeGazePage("kb-stats", "index", types.GazePageTypeIndex, types.GazePageStatusPublished)
	indexPage.InLinks = types.StringArray{}
	otherKB := makeGazePage("kb-other", "entity/other", types.GazePageTypeEntity, types.GazePageStatusPublished)

	for _, p := range []*types.GazePage{live, archived, indexPage, otherKB} {
		require.NoError(t, repo.Create(ctx, p))
	}

	counts, err := repo.CountByType(ctx, "kb-stats")
	require.NoError(t, err)
	assert.Equal(t, int64(1), counts[types.GazePageTypeEntity])
	assert.Equal(t, int64(1), counts[types.GazePageTypeIndex])

	orphans, err := repo.CountOrphans(ctx, "kb-stats")
	require.NoError(t, err)
	assert.Equal(t, int64(0), orphans)

	pages, err := repo.ListAll(ctx, "kb-stats")
	require.NoError(t, err)
	assert.Len(t, pages, 2)
	for _, page := range pages {
		assert.NotEqual(t, types.GazePageStatusArchived, page.Status)
		assert.Equal(t, "kb-stats", page.KnowledgeBaseID)
	}
}

// makeGazeRevision builds a snapshot row for the given page state.
func makeGazeRevision(page *types.GazePage, version int, editSource string) *types.GazePageRevision {
	return &types.GazePageRevision{
		ID:              uuid.New().String(),
		TenantID:        page.TenantID,
		KnowledgeBaseID: page.KnowledgeBaseID,
		PageID:          page.ID,
		Slug:            page.Slug,
		Version:         version,
		Title:           page.Title,
		PageType:        page.PageType,
		Status:          page.Status,
		Content:         "content of v" + strconv.Itoa(version),
		EditSource:      editSource,
		EditedAt:        time.Now(),
		CreatedAt:       time.Now(),
	}
}

func countGazeRevisions(t *testing.T, db *gorm.DB, pageID string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&types.GazePageRevision{}).Where("page_id = ?", pageID).Count(&n).Error)
	return n
}

func TestUpdateWithRevisionRollsBackSnapshotOnVersionConflict(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	page := makeGazePage("kb-tx", "concept/tx", types.GazePageTypeConcept, types.GazePageStatusPublished)
	require.NoError(t, repo.Create(ctx, page))

	// Simulate a writer working from a stale read: it expects v1 while the
	// row has already moved on.
	require.NoError(t, db.Model(&types.GazePage{}).Where("id = ?", page.ID).
		Update("version", 7).Error)

	stale := *page
	stale.Version = 1
	stale.Content = "loser body"
	err := repo.UpdateWithRevision(ctx, &stale, makeGazeRevision(page, 1, types.GazeEditSourceUser))

	require.ErrorIs(t, err, ErrGazePageConflict)
	assert.Equal(t, 1, stale.Version, "a rejected write must not leave a bumped version behind")
	assert.Zero(t, countGazeRevisions(t, db, page.ID),
		"the snapshot must roll back with the failed update, or history would list a still-current version")
}

func TestUpdateWithRevisionIgnoresDuplicateSnapshot(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	page := makeGazePage("kb-dup", "concept/dup", types.GazePageTypeConcept, types.GazePageStatusPublished)
	require.NoError(t, repo.Create(ctx, page))
	require.NoError(t, repo.UpdateWithRevision(ctx, page, makeGazeRevision(page, 1, types.GazeEditSourceUser)))
	require.Equal(t, 2, page.Version)

	// A concurrent writer that already snapshotted v1 must not fail here.
	again := *page
	again.Version = 2
	require.NoError(t, repo.UpdateWithRevision(ctx, &again, makeGazeRevision(page, 1, types.GazeEditSourceUser)))
	assert.Equal(t, int64(1), countGazeRevisions(t, db, page.ID))
}

func TestPruneRevisionsKeepsHumanEditsUntilHardCap(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	page := makeGazePage("kb-prune", "concept/prune", types.GazePageTypeConcept, types.GazePageStatusPublished)
	require.NoError(t, repo.Create(ctx, page))

	// v1 is a human edit buried under a long tail of pipeline rewrites.
	require.NoError(t, db.Create(makeGazeRevision(page, 1, types.GazeEditSourceUser)).Error)
	require.NoError(t, db.Create(makeGazeRevision(page, 2, types.GazeEditSourceAgent)).Error)
	for v := 3; v <= 120; v++ {
		require.NoError(t, db.Create(makeGazeRevision(page, v, types.GazeEditSourcePipeline)).Error)
	}

	// Soft cap only: pipeline snapshots below v71 go, the two authored ones stay.
	require.NoError(t, repo.PruneRevisions(ctx, types.GazeRevisionPruneRequest{
		PageID:          page.ID,
		KeepFromVersion: 121 - types.GazeMaxRevisionsPerPage,
		PrunableSources: types.GazePrunableEditSources,
	}))
	for _, v := range []int{1, 2} {
		_, err := repo.GetRevision(ctx, page.KnowledgeBaseID, page.ID, v)
		require.NoError(t, err, "authored v%d must survive pipeline churn", v)
	}
	_, err := repo.GetRevision(ctx, page.KnowledgeBaseID, page.ID, 3)
	require.ErrorIs(t, err, ErrGazePageNotFound)
	assert.Equal(t, int64(types.GazeMaxRevisionsPerPage+2), countGazeRevisions(t, db, page.ID))

	// Hard cap ignores authorship.
	require.NoError(t, repo.PruneRevisions(ctx, types.GazeRevisionPruneRequest{
		PageID:              page.ID,
		KeepFromVersion:     121 - types.GazeMaxRevisionsPerPage,
		PrunableSources:     types.GazePrunableEditSources,
		HardKeepFromVersion: 100,
	}))
	_, err = repo.GetRevision(ctx, page.KnowledgeBaseID, page.ID, 1)
	require.ErrorIs(t, err, ErrGazePageNotFound)
	assert.Equal(t, int64(21), countGazeRevisions(t, db, page.ID))
}

func TestDeleteRevisionsByPageOnlyTouchesThatPage(t *testing.T) {
	db := setupGazePagesTestDB(t)
	repo := NewGazePageRepository(db)
	ctx := context.Background()

	victim := makeGazePage("kb-del", "concept/victim", types.GazePageTypeConcept, types.GazePageStatusPublished)
	bystander := makeGazePage("kb-del", "concept/bystander", types.GazePageTypeConcept, types.GazePageStatusPublished)
	require.NoError(t, repo.Create(ctx, victim))
	require.NoError(t, repo.Create(ctx, bystander))
	require.NoError(t, db.Create(makeGazeRevision(victim, 1, types.GazeEditSourceUser)).Error)
	require.NoError(t, db.Create(makeGazeRevision(bystander, 1, types.GazeEditSourceUser)).Error)

	require.NoError(t, repo.DeleteRevisionsByPage(ctx, victim.ID))
	assert.Zero(t, countGazeRevisions(t, db, victim.ID))
	assert.Equal(t, int64(1), countGazeRevisions(t, db, bystander.ID))

	// An empty id must not turn into a table-wide delete.
	require.NoError(t, repo.DeleteRevisionsByPage(ctx, ""))
	assert.Equal(t, int64(1), countGazeRevisions(t, db, bystander.ID))
}
