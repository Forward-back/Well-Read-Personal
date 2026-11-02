package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrGazePageNotFound is returned when a gaze page is not found
var ErrGazePageNotFound = errors.New("gaze page not found")

// ErrGazePageConflict is returned when an optimistic lock conflict is detected
var ErrGazePageConflict = errors.New("gaze page version conflict")

// gazePageRepository implements the GazePageRepository interface
type gazePageRepository struct {
	db *gorm.DB
}

// NewGazePageRepository creates a new gaze page repository
func NewGazePageRepository(db *gorm.DB) interfaces.GazePageRepository {
	return &gazePageRepository{db: db}
}

func (r *gazePageRepository) gazeDialect() string {
	if r.db == nil || r.db.Dialector == nil {
		return ""
	}
	return r.db.Name()
}

func (r *gazePageRepository) gazeCategoryRankOrder() string {
	if r.db != nil && r.db.Dialector != nil && r.db.Dialector.Name() == "sqlite" {
		return "CASE WHEN COALESCE(json_array_length(category_path), 0) > 0 THEN 0 ELSE 1 END ASC"
	}
	return "CASE WHEN COALESCE(jsonb_array_length(category_path), 0) > 0 THEN 0 ELSE 1 END ASC"
}

// gazePageListSortColumn maps a caller-supplied sort_by to a quoted identifier.
// Each Name is a string literal so user input never enters the ORDER BY text
// (GORM quotes the column; direction is clause.OrderByColumn.Desc, not sprintf).
func gazePageListSortColumn(sortBy string) clause.Column {
	switch sortBy {
	case "title":
		return clause.Column{Name: "title"}
	case "created_at":
		return clause.Column{Name: "created_at"}
	case "updated_at":
		return clause.Column{Name: "updated_at"}
	case "page_type":
		return clause.Column{Name: "page_type"}
	case "gaze_path":
		return clause.Column{Name: "gaze_path"}
	case "sort_order":
		return clause.Column{Name: "sort_order"}
	case "depth":
		return clause.Column{Name: "depth"}
	default:
		return clause.Column{Name: "updated_at"}
	}
}

func (r *gazePageRepository) applyGazePageListOrder(query *gorm.DB, sortBy, sortOrder string) *gorm.DB {
	col := gazePageListSortColumn(sortBy)
	desc := sortOrder != "asc"
	if col.Name == "gaze_path" {
		return query.Order(r.gazeCategoryRankOrder()).
			Order(clause.OrderByColumn{Column: col, Desc: desc}).
			Order(clause.OrderByColumn{Column: clause.Column{Name: "sort_order"}}).
			Order(clause.OrderByColumn{Column: clause.Column{Name: "title"}})
	}
	return query.Order(clause.OrderByColumn{Column: col, Desc: desc})
}

func (r *gazePageRepository) gazeEmptyInLinksPredicate() string {
	if r.db != nil && r.db.Dialector != nil && r.db.Dialector.Name() == "sqlite" {
		return "(in_links IS NULL OR json_array_length(in_links) = 0)"
	}
	return "(in_links IS NULL OR in_links = '[]'::JSONB)"
}

// Create inserts a new gaze page record
func (r *gazePageRepository) Create(ctx context.Context, page *types.GazePage) error {
	return r.db.WithContext(ctx).Create(page).Error
}

// Update updates an existing gaze page record with optimistic locking.
// Increments version — use only for content changes visible to the user.
// The caller must set page.Version to the expected current version.
//
// The write goes through an explicit column map (not GORM's struct Updates)
// so cleared fields persist: struct Updates skips zero values, which used to
// make "empty the summary" silently not stick while the follow-up UpdateMeta
// map call *did* write empty status — one inconsistent half-update. The map
// covers every column UpdatePage mutates, so no UpdateMeta chaser is needed.
func (r *gazePageRepository) Update(ctx context.Context, page *types.GazePage) error {
	return updateGazePageRow(r.db.WithContext(ctx), page)
}

// UpdateWithRevision snapshots the version being superseded and applies the
// update atomically. Doing both in one transaction is what makes the history
// trustworthy: a failed update can no longer leave behind a snapshot of a
// version that is still current, which would show up twice in the history
// and be un-revertable.
func (r *gazePageRepository) UpdateWithRevision(
	ctx context.Context, page *types.GazePage, rev *types.GazePageRevision,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if rev != nil {
			// An already-present (page_id, version) pair means a concurrent
			// writer snapshotted the same version first; its copy is
			// identical, so leaving it alone is correct.
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "page_id"}, {Name: "version"}},
				DoNothing: true,
			}).Create(rev).Error; err != nil {
				return err
			}
		}
		return updateGazePageRow(tx, page)
	})
}

// updateGazePageRow performs the versioned page write on the given handle
// (plain connection or transaction). On failure page.Version is restored so
// the caller never observes a bumped version for a write that did not land.
func updateGazePageRow(db *gorm.DB, page *types.GazePage) error {
	expectedVersion := page.Version
	page.Version = expectedVersion + 1

	result := db.
		Model(page).
		Where("id = ? AND version = ?", page.ID, expectedVersion).
		Updates(map[string]interface{}{
			"title":            page.Title,
			"content":          page.Content,
			"summary":          page.Summary,
			"page_type":        page.PageType,
			"status":           page.Status,
			"aliases":          page.Aliases,
			"out_links":        page.OutLinks,
			"source_refs":      page.SourceRefs,
			"chunk_refs":       page.ChunkRefs,
			"page_metadata":    page.PageMetadata,
			"parent_slug":      page.ParentSlug,
			"folder_id":        page.FolderID,
			"category_path":    page.CategoryPath,
			"gaze_path":        page.GazePath,
			"depth":            page.Depth,
			"sort_order":       page.SortOrder,
			"last_edit_source": page.LastEditSource,
			"last_editor_id":   page.LastEditorID,
			"version":          page.Version,
			"updated_at":       page.UpdatedAt,
		})
	if result.Error != nil {
		page.Version = expectedVersion
		return result.Error
	}
	if result.RowsAffected == 0 {
		page.Version = expectedVersion
		// Could be not found or version conflict — check which
		var count int64
		db.Model(&types.GazePage{}).Where("id = ?", page.ID).Count(&count)
		if count == 0 {
			return ErrGazePageNotFound
		}
		return ErrGazePageConflict
	}
	return nil
}

// gazeRevisionListColumns is the projection for revision listings — every
// column except the potentially multi-hundred-KB content body.
const gazeRevisionListColumns = "id, tenant_id, knowledge_base_id, page_id, slug, version, " +
	"title, page_type, status, summary, aliases, edit_source, editor_id, edited_at, created_at"

// ListRevisions returns snapshots for a page newest-first, content omitted.
func (r *gazePageRepository) ListRevisions(
	ctx context.Context, kbID string, pageID string, limit int, offset int,
) ([]*types.GazePageRevision, int64, error) {
	base := r.db.WithContext(ctx).Model(&types.GazePageRevision{}).
		Where("knowledge_base_id = ? AND page_id = ?", kbID, pageID)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var revs []*types.GazePageRevision
	if err := base.
		Select(gazeRevisionListColumns).
		Order("version DESC").
		Limit(limit).
		Offset(offset).
		Find(&revs).Error; err != nil {
		return nil, 0, err
	}
	return revs, total, nil
}

// GetRevision returns one snapshot with content.
func (r *gazePageRepository) GetRevision(
	ctx context.Context, kbID string, pageID string, version int,
) (*types.GazePageRevision, error) {
	var rev types.GazePageRevision
	if err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND page_id = ? AND version = ?", kbID, pageID, version).
		First(&rev).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGazePageNotFound
		}
		return nil, err
	}
	return &rev, nil
}

// PruneRevisions applies the two-tier retention described by req: the soft
// cap only touches snapshots whose author is listed as prunable, the hard cap
// applies to everything.
func (r *gazePageRepository) PruneRevisions(ctx context.Context, req types.GazeRevisionPruneRequest) error {
	if req.PageID == "" {
		return nil
	}
	db := r.db.WithContext(ctx)
	if req.KeepFromVersion > 0 && len(req.PrunableSources) > 0 {
		if err := db.
			Where("page_id = ? AND version < ? AND edit_source IN ?",
				req.PageID, req.KeepFromVersion, req.PrunableSources).
			Delete(&types.GazePageRevision{}).Error; err != nil {
			return err
		}
	}
	if req.HardKeepFromVersion > 0 {
		if err := db.
			Where("page_id = ? AND version < ?", req.PageID, req.HardKeepFromVersion).
			Delete(&types.GazePageRevision{}).Error; err != nil {
			return err
		}
	}
	return nil
}

// DeleteRevisionsByPage hard-deletes a page's whole snapshot history. Pages
// themselves are soft-deleted, but a soft-deleted page is unreachable through
// every read path, so its snapshots would be dead weight — and they are the
// bulkiest rows the gaze stores.
func (r *gazePageRepository) DeleteRevisionsByPage(ctx context.Context, pageID string) error {
	if pageID == "" {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("page_id = ?", pageID).
		Delete(&types.GazePageRevision{}).Error
}

// UpdateAutoLinkedContent persists content changes produced by the automatic
// link decorators (cross-link injection, dead-link cleanup) without bumping
// `version`. These passes rewrite the same revision with gaze-link markup
// added or removed; treating them as real edits would make newly-ingested
// pages appear as v2 on first view and confuse users who expect `version` to
// correspond to the number of intentional revisions.
func (r *gazePageRepository) UpdateAutoLinkedContent(ctx context.Context, page *types.GazePage) error {
	result := r.db.WithContext(ctx).
		Model(page).
		Where("id = ?", page.ID).
		Updates(map[string]interface{}{
			"content":    page.Content,
			"out_links":  page.OutLinks,
			"updated_at": page.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGazePageNotFound
	}
	return nil
}

// UpdateMeta updates bookkeeping / provenance fields WITHOUT incrementing the
// version number. "Content" for versioning purposes is the user-visible page
// body (title/content/summary/page_type/status); everything else — links,
// source refs, chunk refs, page_metadata — is considered bookkeeping and is
// refreshed here so the version counter only advances on real edits.
//
// Used by link maintenance, re-ingest (same-content case), and status changes.
func (r *gazePageRepository) UpdateMeta(ctx context.Context, page *types.GazePage) error {
	result := r.db.WithContext(ctx).
		Model(page).
		Where("id = ?", page.ID).
		Updates(map[string]interface{}{
			"in_links":           page.InLinks,
			"out_links":          page.OutLinks,
			"aliases":            page.Aliases,
			"status":             page.Status,
			"source_refs":        page.SourceRefs,
			"chunk_refs":         page.ChunkRefs,
			"page_metadata":      page.PageMetadata,
			"parent_slug":        page.ParentSlug,
			"folder_id":          page.FolderID,
			"category_path":      page.CategoryPath,
			"gaze_path":          page.GazePath,
			"depth":              page.Depth,
			"sort_order":         page.SortOrder,
			"outdated":           page.Outdated,
			"superseded_by_slug": page.SupersededBySlug,
			"updated_at":         page.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGazePageNotFound
	}
	return nil
}

// GetByID retrieves a gaze page by its unique ID
func (r *gazePageRepository) GetByID(ctx context.Context, id string) (*types.GazePage, error) {
	var page types.GazePage
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&page).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGazePageNotFound
		}
		return nil, err
	}
	return &page, nil
}

// GetBySlug retrieves a gaze page by slug within a knowledge base
func (r *gazePageRepository) GetBySlug(ctx context.Context, kbID string, slug string) (*types.GazePage, error) {
	var page types.GazePage
	if err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND slug = ?", kbID, slug).
		First(&page).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGazePageNotFound
		}
		return nil, err
	}
	return &page, nil
}

// List retrieves gaze pages with filtering and pagination
func (r *gazePageRepository) List(ctx context.Context, req *types.GazePageListRequest) ([]*types.GazePage, int64, error) {
	query := r.db.WithContext(ctx).Model(&types.GazePage{}).
		Where("knowledge_base_id = ?", req.KnowledgeBaseID)

	if pageTypes := types.SplitGazePageTypes(req.PageType); len(pageTypes) == 1 {
		query = query.Where("page_type = ?", pageTypes[0])
	} else if len(pageTypes) > 1 {
		query = query.Where("page_type IN ?", pageTypes)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.Query != "" {
		// Use PostgreSQL full-text search + ILIKE for aliases
		query = query.Where(
			"(to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(content, '')) @@ plainto_tsquery('simple', ?) OR aliases::text ILIKE ?)",
			req.Query,
			"%"+req.Query+"%",
		)
	}
	// Directory filters are pushed to SQL so the DB does the counting and
	// pagination instead of loading every page of the type into memory. `depth`
	// is a cached column (= len(category_path)); `category_path` is a JSON column
	// whose stored text is json.Marshal of the folder path segments, so we
	// compare against the same encoding (literal segments, since the filter is a
	// folder path and folder names are authoritative). Postgres needs an explicit
	// jsonb cast for array equality; SQLite stores JSON as TEXT and compares
	// directly.
	if req.FolderID != nil {
		query = query.Where("folder_id = ?", *req.FolderID)
	}
	if req.CategoryDepth != nil {
		query = query.Where("depth = ?", *req.CategoryDepth)
	}
	if wantPath := types.TrimGazeFolderSegments(req.CategoryPath); len(wantPath) > 0 {
		if encoded, err := json.Marshal([]string(wantPath)); err == nil {
			switch r.gazeDialect() {
			case "postgres":
				query = query.Where("category_path::jsonb = ?::jsonb", string(encoded))
			case "sqlite":
				// StringArray.Value yields []byte, which SQLite stores as a BLOB;
				// a BLOB never compares equal to a TEXT bind, so compare the
				// text form instead.
				query = query.Where("CAST(category_path AS TEXT) = ?", string(encoded))
			default:
				query = query.Where("category_path = ?", string(encoded))
			}
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = r.applyGazePageListOrder(query, req.SortBy, req.SortOrder)

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Pagination
	query = query.Offset(offset).Limit(pageSize)

	var pages []*types.GazePage
	if err := query.Find(&pages).Error; err != nil {
		return nil, 0, err
	}
	return pages, total, nil
}

// ListByType retrieves all gaze pages of a given type within a knowledge base
func (r *gazePageRepository) ListByType(ctx context.Context, kbID string, pageType string) ([]*types.GazePage, error) {
	var pages []*types.GazePage
	if err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND page_type = ?", kbID, pageType).
		Order("updated_at DESC").
		Find(&pages).Error; err != nil {
		return nil, err
	}
	return pages, nil
}

// ListByTypeLight projects only the columns needed to render an index
// directory entry (slug, title, summary) and paginates by title ASC.
// This keeps the GET /gaze/index response cheap on KBs with tens of
// thousands of pages — the old path loaded every row including its TEXT
// content just to throw the content away on the way out.
//
// Archived pages are excluded. `limit` clamps to [1, 200]; `offset` is
// honored as-is. Returns the total non-archived count for the type
// alongside the page so the caller can render "showing N of M".
func (r *gazePageRepository) ListByTypeLight(
	ctx context.Context,
	kbID string,
	pageType string,
	limit int,
	offset int,
) ([]types.GazeIndexEntry, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	base := r.db.WithContext(ctx).
		Model(&types.GazePage{}).
		Where("knowledge_base_id = ? AND page_type = ? AND status <> ?",
			kbID, pageType, types.GazePageStatusArchived)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var entries []types.GazeIndexEntry
	if err := base.
		Select("slug", "title", "summary", "parent_slug", "category_path", "gaze_path", "depth", "sort_order").
		Order(r.gazeCategoryRankOrder()).
		Order("gaze_path ASC").
		Order("sort_order ASC").
		Order("title ASC").
		Limit(limit).
		Offset(offset).
		Scan(&entries).Error; err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// ListBySourceRef retrieves all gaze pages that reference a given source knowledge ID.
// Handles both old format ("knowledgeID") and new format ("knowledgeID|title") in source_refs JSON array.
func (r *gazePageRepository) ListBySourceRef(ctx context.Context, kbID string, sourceKnowledgeID string) ([]*types.GazePage, error) {
	// Build the JSON needle safely so arbitrary IDs cannot break out of the
	// quoted string (e.g. ids containing quotes or backslashes).
	needle, err := json.Marshal([]string{sourceKnowledgeID})
	if err != nil {
		return nil, fmt.Errorf("marshal source ref needle: %w", err)
	}

	// For the "knowledgeID|title" prefix form, match against the JSON-encoded
	// value: json.Marshal escapes special chars so the LIKE pattern is safe.
	prefix, err := json.Marshal(sourceKnowledgeID + "|")
	if err != nil {
		return nil, fmt.Errorf("marshal source ref prefix: %w", err)
	}
	// prefix is a JSON string including the surrounding quotes; e.g. "abc|".
	// We strip the trailing quote so LIKE can continue into the title portion.
	prefixStr := string(prefix)
	if len(prefixStr) >= 2 && prefixStr[len(prefixStr)-1] == '"' {
		prefixStr = prefixStr[:len(prefixStr)-1]
	}
	// Escape LIKE metacharacters in the already-JSON-escaped prefix, then wrap
	// with %…% to match anywhere in the serialized JSON array.
	likePattern := "%" + escapeLikePattern(prefixStr) + "%"

	var pages []*types.GazePage
	if err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND (source_refs @> ?::jsonb OR source_refs::text LIKE ?)",
			kbID,
			string(needle),
			likePattern,
		).
		Find(&pages).Error; err != nil {
		return nil, err
	}
	return pages, nil
}

// ListSlugsBySourceRef returns just the slugs of pages that reference the
// given knowledge id. Same predicate as ListBySourceRef (both forms
// "knowledgeID" and "knowledgeID|title"), but projected down to a single
// column so the gaze ingest pipeline doesn't have to load full rows when
// it only needs a "before" set of slugs.
//
// Backed by idx_gaze_pages_source_refs (GIN jsonb_path_ops) for the
// containment branch and idx_gaze_pages_source_refs_text for the legacy
// text-LIKE branch — both added in migration 000041.
func (r *gazePageRepository) ListSlugsBySourceRef(ctx context.Context, kbID string, sourceKnowledgeID string) ([]string, error) {
	needle, err := json.Marshal([]string{sourceKnowledgeID})
	if err != nil {
		return nil, fmt.Errorf("marshal source ref needle: %w", err)
	}
	prefix, err := json.Marshal(sourceKnowledgeID + "|")
	if err != nil {
		return nil, fmt.Errorf("marshal source ref prefix: %w", err)
	}
	prefixStr := string(prefix)
	if len(prefixStr) >= 2 && prefixStr[len(prefixStr)-1] == '"' {
		prefixStr = prefixStr[:len(prefixStr)-1]
	}
	likePattern := "%" + escapeLikePattern(prefixStr) + "%"

	var slugs []string
	if err := r.db.WithContext(ctx).
		Model(&types.GazePage{}).
		Where("knowledge_base_id = ? AND (source_refs @> ?::jsonb OR source_refs::text LIKE ?)",
			kbID,
			string(needle),
			likePattern,
		).
		Pluck("slug", &slugs).Error; err != nil {
		return nil, err
	}
	return slugs, nil
}

// ListBySlugs returns lightweight projections (slug, title, page_type,
// status, aliases, out_links) for the given slugs in one IN query.
// Used by gaze ingest's lazy fetcher path to resolve slug -> title /
// out-links during Map/Reduce without paying for a full ListAll scan.
//
// Empty input returns nil, nil. Slugs not present in the KB are silently
// dropped from the returned map (caller treats absent slugs as "no
// such page" — the same shape ListAll had via missing keys).
func (r *gazePageRepository) ListBySlugs(
	ctx context.Context,
	kbID string,
	slugs []string,
) (map[string]*types.GazePageLite, error) {
	if len(slugs) == 0 {
		return nil, nil
	}
	var rows []types.GazePageLite
	if err := r.db.WithContext(ctx).
		Model(&types.GazePage{}).
		Select("slug", "title", "page_type", "status", "aliases", "out_links").
		Where("knowledge_base_id = ? AND slug IN ?", kbID, slugs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]*types.GazePageLite, len(rows))
	for i := range rows {
		r := rows[i]
		out[r.Slug] = &r
	}
	return out, nil
}

// ListDistinctCategoryPaths returns the materialized paths of existing gaze
// folders (split into segments), ordered by path and capped at maxPaths. Used
// by the batch taxonomy planner as the candidate pool of folders to reuse
// (similarity preprocessing then narrows it per batch). The folder tree is the
// single source of truth, so this no longer scans page rows.
func (r *gazePageRepository) ListDistinctCategoryPaths(
	ctx context.Context,
	kbID string,
	maxPaths int,
) ([][]string, error) {
	if maxPaths <= 0 {
		maxPaths = 150
	}
	var paths []string
	if err := r.db.WithContext(ctx).
		Model(&types.GazeFolder{}).
		Where("knowledge_base_id = ? AND path <> ?", kbID, "").
		Order("path ASC").
		Limit(maxPaths).
		Pluck("path", &paths).Error; err != nil {
		return nil, err
	}
	out := make([][]string, 0, len(paths))
	for _, p := range paths {
		if seg := types.CleanGazeCategoryPath(strings.Split(p, "/")); len(seg) > 0 {
			out = append(out, seg)
		}
	}
	return out, nil
}

// --- Folder tree (gaze_folders) ---

// ErrGazeFolderNotFound is returned when a gaze folder is not found.
var ErrGazeFolderNotFound = errors.New("gaze folder not found")

// ErrGazeFolderConflict is returned when a sibling folder with the same name
// already exists under the same parent.
var ErrGazeFolderConflict = errors.New("gaze folder name conflict")

// ErrGazeFolderNotEmpty is returned when a folder still has a live page or
// child folder at the instant an atomic delete is attempted.
var ErrGazeFolderNotEmpty = errors.New("gaze folder is not empty")

func (r *gazePageRepository) CreateFolder(ctx context.Context, folder *types.GazeFolder) error {
	return r.db.WithContext(ctx).Create(folder).Error
}

func (r *gazePageRepository) GetFolderByID(ctx context.Context, kbID string, id string) (*types.GazeFolder, error) {
	var folder types.GazeFolder
	if err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND id = ?", kbID, id).
		First(&folder).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGazeFolderNotFound
		}
		return nil, err
	}
	return &folder, nil
}

func (r *gazePageRepository) GetChildFolderByName(
	ctx context.Context, kbID string, parentID string, name string,
) (*types.GazeFolder, error) {
	var folder types.GazeFolder
	if err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND parent_id = ? AND name = ?", kbID, parentID, name).
		First(&folder).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGazeFolderNotFound
		}
		return nil, err
	}
	return &folder, nil
}

func (r *gazePageRepository) ListChildFolders(
	ctx context.Context, kbID string, parentID string,
) ([]*types.GazeFolder, error) {
	var folders []*types.GazeFolder
	if err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND parent_id = ?", kbID, parentID).
		Order("sort_order ASC").
		Order("name ASC").
		Find(&folders).Error; err != nil {
		return nil, err
	}
	return folders, nil
}

func (r *gazePageRepository) ListAllFolders(ctx context.Context, kbID string) ([]*types.GazeFolder, error) {
	var folders []*types.GazeFolder
	if err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ?", kbID).
		Order("depth ASC").
		Order("path ASC").
		Find(&folders).Error; err != nil {
		return nil, err
	}
	return folders, nil
}

func (r *gazePageRepository) UpdateFolder(ctx context.Context, folder *types.GazeFolder) error {
	result := r.db.WithContext(ctx).
		Model(&types.GazeFolder{}).
		Where("id = ?", folder.ID).
		Updates(map[string]interface{}{
			"parent_id":  folder.ParentID,
			"name":       folder.Name,
			"path":       folder.Path,
			"depth":      folder.Depth,
			"sort_order": folder.SortOrder,
			"updated_at": folder.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGazeFolderNotFound
	}
	return nil
}

func (r *gazePageRepository) DeleteFolder(ctx context.Context, kbID string, id string) error {
	// Keep the emptiness test in the same SQL statement as the soft delete.
	// A page move or child-folder create can race the service's earlier checks;
	// a check-then-delete sequence would otherwise leave a dangling folder_id.
	result := r.db.WithContext(ctx).Exec(`
UPDATE gaze_folders
SET deleted_at = ?
WHERE knowledge_base_id = ? AND id = ? AND deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM gaze_pages
    WHERE knowledge_base_id = ? AND folder_id = ? AND deleted_at IS NULL
  )
  AND NOT EXISTS (
    SELECT 1 FROM gaze_folders AS child
    WHERE child.knowledge_base_id = ? AND child.parent_id = ? AND child.deleted_at IS NULL
  )`, time.Now(), kbID, id, kbID, id, kbID, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := r.db.WithContext(ctx).Model(&types.GazeFolder{}).
			Where("knowledge_base_id = ? AND id = ?", kbID, id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrGazeFolderNotFound
		}
		return ErrGazeFolderNotEmpty
	}
	return nil
}

func (r *gazePageRepository) CountPagesInFolder(ctx context.Context, kbID string, folderID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&types.GazePage{}).
		Where("knowledge_base_id = ? AND folder_id = ? AND status <> ?",
			kbID, folderID, types.GazePageStatusArchived).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *gazePageRepository) CountPagesByFolder(
	ctx context.Context, kbID string, pageTypes []string,
) (map[string]int64, error) {
	type folderCount struct {
		FolderID string
		Cnt      int64
	}
	var rows []folderCount
	q := r.db.WithContext(ctx).
		Model(&types.GazePage{}).
		Select("folder_id, COUNT(*) as cnt").
		Where("knowledge_base_id = ? AND status <> ?", kbID, types.GazePageStatusArchived)
	if len(pageTypes) > 0 {
		q = q.Where("page_type IN ?", pageTypes)
	}
	if err := q.Group("folder_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.FolderID] = row.Cnt
	}
	return out, nil
}

func (r *gazePageRepository) ListPagesByFolderIDs(
	ctx context.Context, kbID string, folderIDs []string,
) ([]*types.GazePage, error) {
	if len(folderIDs) == 0 {
		return nil, nil
	}
	var pages []*types.GazePage
	if err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND folder_id IN ?", kbID, folderIDs).
		Find(&pages).Error; err != nil {
		return nil, err
	}
	return pages, nil
}

// ListSummariesByKnowledgeIDs returns summary-page content keyed by the
// knowledge id that authored it. The page_type filter is applied first
// (only summary pages have content suitable for retract framing); within
// that subset we look at source_refs for either the bare knowledge id
// or the "knowledgeID|title" legacy form.
//
// Empty kids returns nil, nil. A knowledge id with no surviving summary
// page is silently absent from the result map.
//
// Used by reduceSlugUpdates' retract branch so it can frame "what did
// the now-departed sibling document contribute?" for the GazePageModify
// LLM call without needing to keep the whole batchCtx.SummaryContent
// map in memory ahead of time.
func (r *gazePageRepository) ListSummariesByKnowledgeIDs(
	ctx context.Context,
	kbID string,
	kids []string,
) (map[string]string, error) {
	if len(kids) == 0 {
		return nil, nil
	}

	// Build a JSONB containment-OR with one needle per knowledge id,
	// plus a single text-LIKE OR over the legacy prefix forms. The
	// containment branches each get their own GIN index probe; the
	// LIKE branch falls back to the text fulltext GIN.
	type row struct {
		Content    string            `gorm:"column:content"`
		SourceRefs types.StringArray `gorm:"column:source_refs"`
	}

	q := r.db.WithContext(ctx).
		Model(&types.GazePage{}).
		Select("content", "source_refs").
		Where("knowledge_base_id = ? AND page_type = ? AND status <> ?",
			kbID, types.GazePageTypeSummary, types.GazePageStatusArchived)

	// Build OR clauses without using overly-clever GORM tricks: assemble
	// raw SQL fragments + args. Keeping this defensive because source_refs
	// patterns include user-controlled knowledge ids.
	clauses := make([]string, 0, len(kids)*2)
	args := make([]interface{}, 0, len(kids)*2)
	for _, kid := range kids {
		if kid == "" {
			continue
		}
		needle, err := json.Marshal([]string{kid})
		if err != nil {
			return nil, fmt.Errorf("marshal kid needle: %w", err)
		}
		clauses = append(clauses, "source_refs @> ?::jsonb")
		args = append(args, string(needle))

		prefix, err := json.Marshal(kid + "|")
		if err != nil {
			return nil, fmt.Errorf("marshal kid prefix: %w", err)
		}
		prefixStr := string(prefix)
		if len(prefixStr) >= 2 && prefixStr[len(prefixStr)-1] == '"' {
			prefixStr = prefixStr[:len(prefixStr)-1]
		}
		clauses = append(clauses, "source_refs::text LIKE ?")
		args = append(args, "%"+escapeLikePattern(prefixStr)+"%")
	}
	if len(clauses) == 0 {
		return nil, nil
	}
	q = q.Where("("+strings.Join(clauses, " OR ")+")", args...)

	var rows []row
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}

	// Map each row's content to every kid in its source_refs (a single
	// summary may carry multiple sources after a previous merge / re-
	// ingest). Caller looks up by kid, so duplicates resolve to the
	// same content string.
	kidSet := make(map[string]struct{}, len(kids))
	for _, kid := range kids {
		if kid != "" {
			kidSet[kid] = struct{}{}
		}
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		for _, ref := range r.SourceRefs {
			refKID := ref
			if pipeIdx := strings.Index(ref, "|"); pipeIdx > 0 {
				refKID = ref[:pipeIdx]
			}
			if _, want := kidSet[refKID]; !want {
				continue
			}
			if _, exists := out[refKID]; !exists {
				out[refKID] = r.Content
			}
		}
	}
	return out, nil
}

// ExistsSlugs reports which of the given slugs are live (non-archived,
// non-deleted) in the KB. Used by cleanDeadLinks to validate out-link
// targets without loading the referenced pages' content. Slugs not
// present in the KB at all map to false; archived slugs also map to
// false so dead-link cleanup treats them as gone.
//
// Empty input returns nil, nil so callers can branch cheaply.
func (r *gazePageRepository) ExistsSlugs(
	ctx context.Context,
	kbID string,
	slugs []string,
) (map[string]bool, error) {
	if len(slugs) == 0 {
		return nil, nil
	}
	var live []string
	if err := r.db.WithContext(ctx).
		Model(&types.GazePage{}).
		Where("knowledge_base_id = ? AND slug IN ? AND status <> ?",
			kbID, slugs, types.GazePageStatusArchived).
		Pluck("slug", &live).Error; err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(slugs))
	for _, s := range slugs {
		out[s] = false
	}
	for _, s := range live {
		out[s] = true
	}
	return out, nil
}

// ListAllSlugs returns every non-archived page slug in the KB. Used by
// the lint pipeline to compute the "live slug set" for broken-link
// detection without paying for ListAll's full row materialization.
func (r *gazePageRepository) ListAllSlugs(
	ctx context.Context,
	kbID string,
) ([]string, error) {
	var slugs []string
	if err := r.db.WithContext(ctx).
		Model(&types.GazePage{}).
		Where("knowledge_base_id = ? AND status <> ?", kbID, types.GazePageStatusArchived).
		Pluck("slug", &slugs).Error; err != nil {
		return nil, err
	}
	return slugs, nil
}

// ListPagesCursor returns up to `limit` pages for the KB ordered by
// (knowledge_base_id, id) ascending, paginated by an opaque numeric
// cursor. The cursor is the stringified id of the last row from the
// previous page; "" starts from the beginning. Empty nextCursor =
// end-of-stream.
//
// Used by lint to walk the entire KB without ever holding the full
// page set in memory. `limit` is clamped to [1, 500].
func (r *gazePageRepository) ListPagesCursor(
	ctx context.Context,
	kbID string,
	cursor string,
	limit int,
) ([]*types.GazePage, string, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	q := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND status <> ?", kbID, types.GazePageStatusArchived).
		Order("id ASC").
		Limit(limit)
	if cursor != "" {
		q = q.Where("id > ?", cursor)
	}
	var pages []*types.GazePage
	if err := q.Find(&pages).Error; err != nil {
		return nil, "", err
	}
	nextCursor := ""
	if len(pages) == limit {
		nextCursor = pages[len(pages)-1].ID
	}
	return pages, nextCursor, nil
}

// ListByTypeRecent returns up to `limit` summary-typed pages ordered
// by updated_at DESC, projected to slug/title/summary. Used by the
// rebuildIndexPage first-time generation path — historically that
// loaded EVERY summary page and concatenated them into the prompt,
// which broke the LLM context window once a KB grew past a few
// thousand documents. The recent-N projection caps the prompt size
// at the cost of intro framing for very old documents (which are
// unlikely to be the most-relevant index introductions anyway).
//
// `limit` is clamped to [1, 1000]; 0 falls back to 200.
func (r *gazePageRepository) ListByTypeRecent(
	ctx context.Context,
	kbID string,
	pageType string,
	limit int,
) ([]types.GazeIndexEntry, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	var entries []types.GazeIndexEntry
	if err := r.db.WithContext(ctx).
		Model(&types.GazePage{}).
		Select("slug", "title", "summary").
		Where("knowledge_base_id = ? AND page_type = ? AND status <> ?",
			kbID, pageType, types.GazePageStatusArchived).
		Order("updated_at DESC").
		Limit(limit).
		Scan(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

// FindSimilarPages returns the top-k entity/concept pages whose lowercase
// title is most similar to the given query under PostgreSQL pg_trgm
// trigram similarity. Backed by idx_gaze_pages_title_trgm (GIN
// gin_trgm_ops, migration 000041). Used by the dedup pre-filter to
// surface candidate merge targets without loading every entity/concept
// page into Go.
//
// types is an optional page_type allow-list; empty means entity+concept.
// limit is clamped to [1, 50]. Pages whose title similarity is below
// 0.1 are dropped server-side via the `%` operator (which respects
// pg_trgm.similarity_threshold).
func (r *gazePageRepository) FindSimilarPages(
	ctx context.Context,
	kbID string,
	query string,
	pageTypes []string,
	limit int,
) ([]*types.GazePageLite, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if len(pageTypes) == 0 {
		pageTypes = []string{types.GazePageTypeEntity, types.GazePageTypeConcept}
	}

	q := strings.ToLower(strings.TrimSpace(query))

	var rows []types.GazePageLite
	if err := r.db.WithContext(ctx).
		Model(&types.GazePage{}).
		Select("slug, title, page_type, status, aliases, out_links, similarity(lower(title), ?) AS sim", q).
		Where("knowledge_base_id = ? AND page_type IN ? AND status <> ? AND lower(title) % ?",
			kbID, pageTypes, types.GazePageStatusArchived, q).
		Order("sim DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*types.GazePageLite, len(rows))
	for i := range rows {
		r := rows[i]
		out[i] = &r
	}
	return out, nil
}

func gazeNormalizedTitleSQL(db *gorm.DB) string {
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "sqlite" {
		// SQLite has no POSIX [[:space:]]. Strip the common separators that
		// model formatting drift actually produces; callers still re-check
		// with the Go identity fold so extras cannot leak through.
		return "lower(replace(replace(replace(replace(replace(replace(title, ' ', ''), char(9), ''), char(10), ''), char(13), ''), char(160), ''), char(12288), ''))"
	}
	return "regexp_replace(lower(title), '[[:space:]]+', '', 'g')"
}

const (
	gazeNormalizedTitleQueryChunk = 100
	gazeNormalizedTitleRowCap     = 500
)

func gazeNormalizedTitleLookupLimit(n int) int {
	if n <= 0 {
		return 0
	}
	limit := n * 8
	if limit < 50 {
		limit = 50
	}
	if limit > gazeNormalizedTitleRowCap {
		limit = gazeNormalizedTitleRowCap
	}
	return limit
}

func uniqNormalizedTitleIdentities(identities []string) []string {
	if len(identities) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(identities))
	out := make([]string, 0, len(identities))
	for _, identity := range identities {
		identity = strings.TrimSpace(identity)
		if identity == "" {
			continue
		}
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		out = append(out, identity)
	}
	return out
}

// FindPagesByNormalizedTitle returns non-archived pages of pageType whose
// whitespace-stripped, lowercased title equals identity.
func (r *gazePageRepository) FindPagesByNormalizedTitle(
	ctx context.Context,
	kbID, pageType, identity string,
) ([]*types.GazePageLite, error) {
	return r.FindPagesByNormalizedTitles(ctx, kbID, pageType, []string{identity})
}

// FindPagesByNormalizedTitles returns non-archived pages of pageType whose
// whitespace-stripped, lowercased title is in identities.
func (r *gazePageRepository) FindPagesByNormalizedTitles(
	ctx context.Context,
	kbID, pageType string,
	identities []string,
) ([]*types.GazePageLite, error) {
	identities = uniqNormalizedTitleIdentities(identities)
	if kbID == "" || pageType == "" || len(identities) == 0 {
		return nil, nil
	}

	normSQL := gazeNormalizedTitleSQL(r.db)
	out := make([]*types.GazePageLite, 0, len(identities))
	for start := 0; start < len(identities); start += gazeNormalizedTitleQueryChunk {
		end := start + gazeNormalizedTitleQueryChunk
		if end > len(identities) {
			end = len(identities)
		}
		chunk := identities[start:end]
		var rows []types.GazePageLite
		if err := r.db.WithContext(ctx).
			Model(&types.GazePage{}).
			Select("slug, title, page_type, status, aliases, out_links").
			Where("knowledge_base_id = ? AND page_type = ? AND status <> ? AND "+normSQL+" IN ?",
				kbID, pageType, types.GazePageStatusArchived, chunk).
			Order("slug ASC").
			Limit(gazeNormalizedTitleLookupLimit(len(chunk))).
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for i := range rows {
			row := rows[i]
			out = append(out, &row)
		}
	}
	return out, nil
}

// ListAll retrieves all non-archived gaze pages in a knowledge base.
func (r *gazePageRepository) ListAll(ctx context.Context, kbID string) ([]*types.GazePage, error) {
	var pages []*types.GazePage
	if err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND status <> ?", kbID, types.GazePageStatusArchived).
		Order("page_type ASC, title ASC").
		Find(&pages).Error; err != nil {
		return nil, err
	}
	return pages, nil
}

// ListRecentForSuggestions returns recent user-visible gaze pages across the given
// knowledge bases, used as a fallback source for agent suggested questions when
// the KB has no FAQ entries or AI-generated document questions (typical for
// Gaze-only KBs). Excludes the index page and archived pages.
func (r *gazePageRepository) ListRecentForSuggestions(
	ctx context.Context,
	tenantID uint64,
	kbIDs []string,
	limit int,
) ([]*types.GazePage, error) {
	if len(kbIDs) == 0 || limit <= 0 {
		return nil, nil
	}
	var pages []*types.GazePage
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Where("knowledge_base_id IN ?", kbIDs).
		Where("page_type <> ?", types.GazePageTypeIndex).
		Where("status = ?", types.GazePageStatusPublished).
		Where("title <> ''").
		Order("updated_at DESC").
		Limit(limit).
		Find(&pages).Error; err != nil {
		return nil, err
	}
	return pages, nil
}

// Delete soft-deletes a gaze page by knowledge base ID and slug
func (r *gazePageRepository) Delete(ctx context.Context, kbID string, slug string) error {
	result := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND slug = ?", kbID, slug).
		Delete(&types.GazePage{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGazePageNotFound
	}
	return nil
}

// DeleteByID soft-deletes a gaze page by ID
func (r *gazePageRepository) DeleteByID(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&types.GazePage{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGazePageNotFound
	}
	return nil
}

// escapeLikePattern escapes LIKE / ILIKE metacharacters so the returned string
// can be safely concatenated with % wildcards without unintended matches.
// Order matters: escape the backslash first, then the wildcards.
func escapeLikePattern(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)
	return replacer.Replace(s)
}

// Search performs case-insensitive POSIX regex search on gaze pages within a knowledge base.
// The query is interpreted as a PostgreSQL regular expression (via ~*).
//
// Results are ranked by where the query hit, highest-relevance first:
//
//	title    hit → rank 4 (most obvious intent: user typed what the page is called)
//	slug     hit → rank 3 (url-like identifiers, direct jump)
//	summary  hit → rank 2 (short authored abstract)
//	content  hit → rank 1 (body mention — often surfaces unrelated pages whose
//	                       prose merely mentions the query as trivia)
//
// Without this ranking, a user searching for "王新" on a 4万-page gaze will
// see pages like "华为" or "Index" ahead of the actual 王新 page just
// because they mention 王新 in their body and were updated more recently.
// updated_at stays as the tiebreaker so same-rank ties stay deterministic.
func (r *gazePageRepository) Search(ctx context.Context, kbID string, query string, limit int) ([]*types.GazePage, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	// CASE expression is evaluated per-row during SELECT; we order by the
	// alias so the DB only computes the rank once. Parameterized four
	// times with the same regex to avoid coupling to GORM's positional
	// arg rewriting quirks.
	rankExpr := "CASE " +
		"WHEN title ~* ? THEN 4 " +
		"WHEN slug ~* ? THEN 3 " +
		"WHEN summary ~* ? THEN 2 " +
		"WHEN content ~* ? THEN 1 " +
		"ELSE 0 END AS match_rank"

	var pages []*types.GazePage
	if err := r.db.WithContext(ctx).
		Select("*, "+rankExpr, query, query, query, query).
		Where("knowledge_base_id = ? AND (title ~* ? OR content ~* ? OR summary ~* ? OR slug ~* ?)",
			kbID, query, query, query, query).
		Where("status != ?", "archived").
		Order("match_rank DESC, updated_at DESC").
		Limit(limit).
		Find(&pages).Error; err != nil {
		return nil, err
	}
	return pages, nil
}

// CountByType returns page counts grouped by type for a knowledge base
func (r *gazePageRepository) CountByType(ctx context.Context, kbID string) (map[string]int64, error) {
	type result struct {
		PageType string
		Count    int64
	}
	var results []result
	if err := r.db.WithContext(ctx).
		Model(&types.GazePage{}).
		Select("page_type, count(*) as count").
		Where("knowledge_base_id = ? AND status <> ?", kbID, types.GazePageStatusArchived).
		Group("page_type").
		Scan(&results).Error; err != nil {
		return nil, err
	}

	counts := make(map[string]int64)
	for _, r := range results {
		counts[r.PageType] = r.Count
	}
	return counts, nil
}

// CountOrphans returns the number of pages with no inbound links
func (r *gazePageRepository) CountOrphans(ctx context.Context, kbID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&types.GazePage{}).
		Where("knowledge_base_id = ? AND status <> ?", kbID, types.GazePageStatusArchived).
		Where(r.gazeEmptyInLinksPredicate()).
		// Exclude the index page because it is naturally a root page.
		Where("page_type <> ?", types.GazePageTypeIndex).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
