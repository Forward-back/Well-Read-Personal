package handler

import (
	"context"
	stderrors "errors"
	"net/http"
	"strconv"
	"strings"

	"gaze/internal/application/repository"
	"gaze/internal/application/service"
	"gaze/internal/errors"
	"gaze/internal/logger"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	secutils "gaze/internal/utils"
	"github.com/gin-gonic/gin"
)

// GazePageHandler handles HTTP requests for gaze page operations
type GazePageHandler struct {
	gazeService   interfaces.GazePageService
	kbService     interfaces.KnowledgeBaseService
	memoryService interfaces.MemoryService
}

// NewGazePageHandler creates a new gaze page handler
func NewGazePageHandler(
	gazeService interfaces.GazePageService,
	kbService interfaces.KnowledgeBaseService,
	memoryService interfaces.MemoryService,
) *GazePageHandler {
	return &GazePageHandler{
		gazeService:   gazeService,
		kbService:     kbService,
		memoryService: memoryService,
	}
}

// validateGazeKB validates that the KB exists and is a gaze type
func (h *GazePageHandler) validateGazeKB(c *gin.Context) (string, uint64, error) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("kb_id"))
	tenantID := c.GetUint64(types.TenantIDContextKey.String())

	if kbID == "" {
		return "", 0, errors.NewBadRequestError("Knowledge base ID is required")
	}

	kb, err := h.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		return "", 0, errors.NewNotFoundError("Knowledge base not found")
	}

	if !kb.IsGazeEnabled() {
		return "", 0, errors.NewBadRequestError("Gaze feature is not enabled for this knowledge base")
	}

	return kbID, tenantID, nil
}

// getSlugParam extracts and cleans the slug from gin's wildcard path param
func getSlugParam(c *gin.Context) string {
	slug := c.Param("slug")
	// gin wildcard params include a leading "/"
	slug = strings.TrimPrefix(slug, "/")
	return strings.TrimSpace(slug)
}

// ListPages godoc
// @Summary      List gaze pages
// @Description  List gaze pages with optional filtering and pagination
// @Tags         Gaze
// @Produce      json
// @Param        kb_id      path      string  true   "Knowledge base ID"
// @Param        page_type  query     string  false  "Filter by page type; comma-separated for multiple (e.g. entity,concept)"
// @Param        status     query     string  false  "Filter by status"
// @Param        query      query     string  false  "Full-text search"
// @Param        page       query     int     false  "Page number"
// @Param        page_size  query     int     false  "Page size"
// @Param        sort_by    query     string  false  "Sort field"
// @Param        sort_order query     string  false  "Sort order (asc/desc)"
// @Success      200  {object}  types.GazePageListResponse
// @Failure      400  {object}  errors.AppError
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/pages [get]
func (h *GazePageHandler) ListPages(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	categoryPath := parseGazeCategoryPath(c.Query("category_path"))
	// folder_id is an exact placement filter. An explicitly-present but empty
	// value means "root" (folder_id = ''); an absent param means "no filter".
	var folderID *string
	if raw, ok := c.GetQuery("folder_id"); ok {
		raw = strings.TrimSpace(raw)
		folderID = &raw
	}
	var categoryDepth *int
	if raw := c.Query("category_depth"); raw != "" {
		if depth, parseErr := strconv.Atoi(raw); parseErr == nil && depth >= 0 {
			categoryDepth = &depth
		}
	}

	req := &types.GazePageListRequest{
		KnowledgeBaseID: kbID,
		PageType:        c.Query("page_type"),
		Status:          c.Query("status"),
		Query:           c.Query("query"),
		FolderID:        folderID,
		CategoryPath:    types.StringArray(categoryPath),
		CategoryDepth:   categoryDepth,
		Page:            page,
		PageSize:        pageSize,
		SortBy:          c.DefaultQuery("sort_by", "updated_at"),
		SortOrder:       c.DefaultQuery("sort_order", "desc"),
	}

	resp, err := h.gazeService.ListPages(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListFolders godoc
// @Summary      List gaze folders
// @Description  Retrieve the direct child folders of a parent folder (parent_id empty = root level), each with its page count and a has-children flag for the directory tree.
// @Tags         Gaze
// @Produce      json
// @Param        kb_id     path   string  true   "Knowledge base ID"
// @Param        parent_id query  string  false  "Parent folder id (empty = root)"
// @Success      200  {object}  types.GazeFolderListResponse
// @Failure      400  {object}  errors.AppError
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/folders [get]
func (h *GazePageHandler) ListFolders(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	parentID := strings.TrimSpace(c.Query("parent_id"))
	var pageTypes []string
	if raw := strings.TrimSpace(c.Query("page_types")); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if p := strings.TrimSpace(part); p != "" {
				pageTypes = append(pageTypes, p)
			}
		}
	}
	folders, err := h.gazeService.ListChildFolders(c.Request.Context(), kbID, parentID, pageTypes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if folders == nil {
		folders = []types.GazeFolderNode{}
	}
	c.JSON(http.StatusOK, types.GazeFolderListResponse{ParentID: parentID, Folders: folders})
}

// CreateFolder godoc
// @Summary      Create a gaze folder
// @Description  Create a new (initially empty) directory node under parent_id
// @Tags         Gaze
// @Accept       json
// @Produce      json
// @Param        kb_id  path  string                       true  "Knowledge base ID"
// @Param        folder body  types.GazeFolderCreateRequest true  "Folder data"
// @Success      201  {object}  types.GazeFolder
// @Failure      400  {object}  errors.AppError
// @Failure      409  {object}  errors.AppError
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/folders [post]
func (h *GazePageHandler) CreateFolder(c *gin.Context) {
	kbID, tenantID, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var req types.GazeFolderCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}
	folder, err := h.gazeService.CreateFolder(c.Request.Context(), kbID, tenantID, strings.TrimSpace(req.ParentID), req.Name)
	if err != nil {
		writeGazeFolderError(c, err)
		return
	}
	c.JSON(http.StatusCreated, folder)
}

// UpdateFolder godoc
// @Summary      Rename or move a gaze folder
// @Description  Rename and/or reparent a folder; the whole subtree's paths and the affected pages' cached paths are recomputed
// @Tags         Gaze
// @Accept       json
// @Produce      json
// @Param        kb_id     path  string                        true  "Knowledge base ID"
// @Param        folder_id path  string                        true  "Folder ID"
// @Param        folder    body  types.GazeFolderUpdateRequest true  "Folder update"
// @Success      200  {object}  types.GazeFolder
// @Failure      400  {object}  errors.AppError
// @Failure      404  {object}  errors.AppError
// @Failure      409  {object}  errors.AppError
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/folders/{folder_id} [put]
func (h *GazePageHandler) UpdateFolder(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	folderID := secutils.SanitizeForLog(c.Param("folder_id"))
	if folderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Folder ID is required"})
		return
	}
	var req types.GazeFolderUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}
	folder, err := h.gazeService.RenameOrMoveFolder(
		c.Request.Context(), kbID, folderID, req.Name, strings.TrimSpace(req.ParentID), req.MoveParent)
	if err != nil {
		writeGazeFolderError(c, err)
		return
	}
	c.JSON(http.StatusOK, folder)
}

// DeleteFolder godoc
// @Summary      Delete an empty gaze folder
// @Description  Delete a folder that has no pages and no child folders
// @Tags         Gaze
// @Param        kb_id     path  string  true  "Knowledge base ID"
// @Param        folder_id path  string  true  "Folder ID"
// @Success      204
// @Failure      400  {object}  errors.AppError
// @Failure      404  {object}  errors.AppError
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/folders/{folder_id} [delete]
func (h *GazePageHandler) DeleteFolder(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	folderID := secutils.SanitizeForLog(c.Param("folder_id"))
	if folderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Folder ID is required"})
		return
	}
	if err := h.gazeService.DeleteFolder(c.Request.Context(), kbID, folderID); err != nil {
		writeGazeFolderError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// MovePage godoc
// @Summary      Move a gaze page into a folder
// @Description  Relocate a page (identified by slug in the body) into a folder (folder_id empty = root); the page's cached category path is recomputed
// @Tags         Gaze
// @Accept       json
// @Produce      json
// @Param        kb_id  path  string  true  "Knowledge base ID"
// @Param        move   body  types.GazePageMoveRequest true "Move target"
// @Success      200  {object}  types.GazePage
// @Failure      404  {object}  errors.AppError
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/move-page [put]
func (h *GazePageHandler) MovePage(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var req types.GazePageMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page slug is required"})
		return
	}
	page, err := h.gazeService.MovePage(c.Request.Context(), kbID, slug, strings.TrimSpace(req.FolderID))
	if err != nil {
		writeGazeFolderError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

// writeGazeFolderError maps folder/page service errors to HTTP status codes.
func writeGazeFolderError(c *gin.Context, err error) {
	switch {
	case stderrors.Is(err, repository.ErrGazeFolderNotFound), stderrors.Is(err, repository.ErrGazePageNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case stderrors.Is(err, repository.ErrGazeFolderConflict), stderrors.Is(err, repository.ErrGazeFolderNotEmpty):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func parseGazeCategoryPath(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// CreatePage godoc
// @Summary      Create a gaze page
// @Description  Create a new gaze page in the knowledge base
// @Tags         Gaze
// @Accept       json
// @Produce      json
// @Param        kb_id  path  string          true  "Knowledge base ID"
// @Param        page   body  types.GazePage  true  "Gaze page data"
// @Success      201  {object}  types.GazePage
// @Failure      400  {object}  errors.AppError
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/pages [post]
func (h *GazePageHandler) CreatePage(c *gin.Context) {
	kbID, tenantID, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var page types.GazePage
	if err := c.ShouldBindJSON(&page); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	page.KnowledgeBaseID = kbID
	page.TenantID = tenantID
	page.PageType = strings.TrimSpace(page.PageType)
	page.Status = strings.TrimSpace(page.Status)
	if page.PageType != "" && !types.IsValidGazePageType(page.PageType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page_type: " + page.PageType})
		return
	}
	if page.Status != "" && !types.IsValidGazePageStatus(page.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status: " + page.Status})
		return
	}

	ctx := types.WithGazeEditSource(c.Request.Context(), types.GazeEditSourceUser)
	created, err := h.gazeService.CreatePage(ctx, &page)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.recordManualGazeActivity(ctx, created, "manual_create")
	c.JSON(http.StatusCreated, created)
}

// recordManualGazeActivity projects a manual page mutation directly into the
// knowledge-base activity feed. Activity recording is best-effort and must not
// fail the edit itself.
func (h *GazePageHandler) recordManualGazeActivity(
	ctx context.Context, page *types.GazePage, action string,
) {
	if page == nil {
		return
	}
}

// GetPage godoc
// @Summary      Get a gaze page by slug
// @Description  Retrieve a gaze page by its slug
// @Tags         Gaze
// @Produce      json
// @Param        kb_id  path  string  true  "Knowledge base ID"
// @Param        slug   path  string  true  "Page slug"
// @Success      200  {object}  types.GazePage
// @Failure      404  {object}  errors.AppError
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/pages/{slug} [get]
func (h *GazePageHandler) GetPage(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slug := getSlugParam(c)
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page slug is required"})
		return
	}

	page, err := h.gazeService.GetPageBySlug(c.Request.Context(), kbID, slug)
	if err != nil {
		if stderrors.Is(err, repository.ErrGazePageNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Gaze page not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, page)
}

// UpdatePage godoc
// @Summary      Update a gaze page
// @Description  Partially update a gaze page by slug. Absent fields keep
// @Description  their stored value. When `version` is > 0 it acts as an
// @Description  optimistic-lock guard: a mismatch with the stored version
// @Description  returns 409 together with the current version so the client
// @Description  can reload and re-apply.
// @Tags         Gaze
// @Accept       json
// @Produce      json
// @Param        kb_id  path  string                       true  "Knowledge base ID"
// @Param        slug   path  string                       true  "Page slug"
// @Param        page   body  types.GazePageUpdateRequest  true  "Fields to update"
// @Success      200  {object}  types.GazePage
// @Failure      400  {object}  errors.AppError
// @Failure      404  {object}  errors.AppError
// @Failure      409  {object}  errors.AppError
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/pages/{slug} [put]
func (h *GazePageHandler) UpdatePage(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slug := getSlugParam(c)
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page slug is required"})
		return
	}

	var req types.GazePageUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	ctx := types.WithGazeEditSource(c.Request.Context(), types.GazeEditSourceUser)

	existing, err := h.gazeService.GetPageBySlug(ctx, kbID, slug)
	if err != nil {
		if stderrors.Is(err, repository.ErrGazePageNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Gaze page not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if req.Version > 0 && req.Version != existing.Version {
		c.JSON(http.StatusConflict, gin.H{
			"error":           "Gaze page was modified by someone else",
			"current_version": existing.Version,
		})
		return
	}

	// Merge the provided fields onto the stored page so absent fields keep
	// their value — the service's UpdatePage semantics are "full intended
	// state", and sending it a half-empty struct would clear real data.
	page := *existing
	if req.Title != nil {
		page.Title = strings.TrimSpace(*req.Title)
	}
	if req.Content != nil {
		page.Content = *req.Content
	}
	if req.Summary != nil {
		page.Summary = *req.Summary
	}
	if req.PageType != nil {
		page.PageType = strings.TrimSpace(*req.PageType)
		if !types.IsValidGazePageType(page.PageType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page_type: " + page.PageType})
			return
		}
	}
	if req.Status != nil {
		page.Status = strings.TrimSpace(*req.Status)
		if !types.IsValidGazePageStatus(page.Status) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status: " + page.Status})
			return
		}
	}
	if req.Aliases != nil {
		page.Aliases = *req.Aliases
	}

	updated, err := h.gazeService.UpdatePage(ctx, &page)
	if err != nil {
		switch {
		case stderrors.Is(err, repository.ErrGazePageNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Gaze page not found"})
		case stderrors.Is(err, repository.ErrGazePageConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "Gaze page was modified by someone else"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	if updated.Version != existing.Version {
		h.recordManualGazeActivity(ctx, updated, "manual_edit")
	}
	c.JSON(http.StatusOK, updated)
}

// ListRevisions godoc
// @Summary      List gaze page revisions
// @Description  Returns the stored historical snapshots for a page, newest
// @Description  first (content omitted), plus the current version. Passing
// @Description  `version` returns that single snapshot with full content.
// @Tags         Gaze
// @Produce      json
// @Param        kb_id    path   string  true   "Knowledge base ID"
// @Param        slug     path   string  true   "Page slug"
// @Param        version  query  int     false  "Return this single revision with content"
// @Param        limit    query  int     false  "Page size (default 50, max 200)"
// @Param        offset   query  int     false  "Offset into the newest-first list"
// @Success      200  {object}  types.GazePageRevisionListResponse
// @Failure      404  {object}  errors.AppError
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/revisions/{slug} [get]
func (h *GazePageHandler) ListRevisions(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slug := getSlugParam(c)
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page slug is required"})
		return
	}

	ctx := c.Request.Context()

	if raw := c.Query("version"); raw != "" {
		version, parseErr := strconv.Atoi(raw)
		if parseErr != nil || version < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid version"})
			return
		}
		rev, err := h.gazeService.GetRevision(ctx, kbID, slug, version)
		if err != nil {
			if stderrors.Is(err, repository.ErrGazePageNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Gaze page revision not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, rev)
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}

	resp, err := h.gazeService.ListRevisions(ctx, kbID, slug, limit, offset)
	if err != nil {
		if stderrors.Is(err, repository.ErrGazePageNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Gaze page not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// RevertPage godoc
// @Summary      Revert a gaze page to an earlier revision
// @Description  Rolls the page (slug in the body, like move-page) back to
// @Description  the content of the given stored revision. Applied as a
// @Description  regular edit: the pre-revert state is snapshotted and the
// @Description  version advances.
// @Tags         Gaze
// @Accept       json
// @Produce      json
// @Param        kb_id   path  string                       true  "Knowledge base ID"
// @Param        revert  body  types.GazePageRevertRequest  true  "Revert target"
// @Success      200  {object}  types.GazePage
// @Failure      400  {object}  errors.AppError
// @Failure      404  {object}  errors.AppError
// @Failure      409  {object}  errors.AppError
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/revert [post]
func (h *GazePageHandler) RevertPage(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req types.GazePageRevertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page slug is required"})
		return
	}
	if req.Version < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid version"})
		return
	}

	ctx := c.Request.Context()
	updated, err := h.gazeService.RevertPageToVersion(ctx, kbID, slug, req.Version)
	if err != nil {
		switch {
		case stderrors.Is(err, repository.ErrGazePageNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Gaze page or revision not found"})
		case stderrors.Is(err, repository.ErrGazePageConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "Gaze page was modified by someone else"})
		case stderrors.Is(err, service.ErrGazeRevertToCurrentVersion):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	h.recordManualGazeActivity(ctx, updated, "revert")
	c.JSON(http.StatusOK, updated)
}

// DeletePage godoc
// @Summary      Delete a gaze page
// @Description  Soft-delete a gaze page by slug
// @Tags         Gaze
// @Param        kb_id  path  string  true  "Knowledge base ID"
// @Param        slug   path  string  true  "Page slug"
// @Success      204
// @Failure      404  {object}  errors.AppError
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/pages/{slug} [delete]
func (h *GazePageHandler) DeletePage(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slug := getSlugParam(c)
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page slug is required"})
		return
	}

	ctx := c.Request.Context()
	// Load first so the log entry can carry the page title after deletion.
	page, _ := h.gazeService.GetPageBySlug(ctx, kbID, slug)

	if err := h.gazeService.DeletePage(ctx, kbID, slug); err != nil {
		if stderrors.Is(err, repository.ErrGazePageNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Gaze page not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.recordManualGazeActivity(ctx, page, "manual_delete")
	c.Status(http.StatusNoContent)
}

// GetIndex godoc
// @Summary      Get gaze index view
// @Description  Returns the gaze index as intro text plus per-type paginated
// @Description  directory groups. The heavy directory markdown that used to
// @Description  live in gaze_pages.content was replaced with this structured
// @Description  response so a KB with tens of thousands of pages no longer
// @Description  materializes megabytes of TEXT on every index open.
// @Tags         Gaze
// @Produce      json
// @Param        kb_id   path   string  true   "Knowledge base ID"
// @Param        types   query  string  false  "Comma-separated page types (default: all content types)"
// @Param        limit   query  int     false  "Per-group window size, 1-200 (default 50)"
// @Param        cursor  query  string  false  "Opaque offset cursor from previous response"
// @Success      200  {object}  types.GazeIndexResponse
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/index [get]
func (h *GazePageHandler) GetIndex(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var pageTypes []string
	if raw := c.Query("types"); raw != "" {
		for _, t := range strings.Split(raw, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				pageTypes = append(pageTypes, t)
			}
		}
	}

	limit := 50
	if raw := c.Query("limit"); raw != "" {
		if v, convErr := strconv.Atoi(raw); convErr == nil && v > 0 {
			limit = v
		}
	}

	resp, err := h.gazeService.GetIndexView(c.Request.Context(), kbID, pageTypes, limit, c.Query("cursor"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Graph query parameter bounds. The defaults cap an `overview` request at
// 500 nodes — comfortably renderable in the frontend's hand-rolled SVG
// force simulation — while the hard max of 2000 is the upper bound a
// power user can opt into before rendering gets choppy. Ego depth is
// capped at 3 hops because the node population grows super-linearly with
// depth and wider searches are better served by repeated ego jumps.
const (
	gazeGraphDefaultLimit = 500
	gazeGraphMaxLimit     = 2000
	gazeGraphMaxDepth     = 3
	gazeGraphDefaultDepth = 1
)

// GetGraph godoc
// @Summary      Get gaze link graph
// @Description  Returns a slice of the gaze link graph for visualization. Supports
// @Description  `mode=overview` (top-N most-connected pages, default) and
// @Description  `mode=ego` (BFS neighborhood of a center slug) to keep response
// @Description  size tractable for knowledge bases with tens of thousands of pages.
// @Tags         Gaze
// @Produce      json
// @Param        kb_id   path  string  true   "Knowledge base ID"
// @Param        mode    query string  false  "overview (default) | ego"
// @Param        center  query string  false  "Center slug for ego mode"
// @Param        depth   query int     false  "Ego BFS depth (1-3, default 1)"
// @Param        types   query string  false  "Comma-separated page_type allow-list"
// @Param        limit   query int     false  "Max nodes to return (default 500, max 2000)"
// @Success      200  {object}  types.GazeGraphData
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/graph [get]
func (h *GazePageHandler) GetGraph(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mode := strings.TrimSpace(c.Query("mode"))
	if mode == "" {
		mode = types.GazeGraphModeOverview
	}
	if mode != types.GazeGraphModeOverview && mode != types.GazeGraphModeEgo {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be 'overview' or 'ego'"})
		return
	}

	center := strings.TrimSpace(c.Query("center"))
	if mode == types.GazeGraphModeEgo && center == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "center is required when mode=ego"})
		return
	}

	depth := gazeGraphDefaultDepth
	if v := c.Query("depth"); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "depth must be a positive integer"})
			return
		}
		if parsed > gazeGraphMaxDepth {
			parsed = gazeGraphMaxDepth
		}
		depth = parsed
	}

	limit := gazeGraphDefaultLimit
	if v := c.Query("limit"); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer"})
			return
		}
		if parsed > gazeGraphMaxLimit {
			parsed = gazeGraphMaxLimit
		}
		limit = parsed
	}

	var typesFilter []string
	if v := strings.TrimSpace(c.Query("types")); v != "" {
		for _, t := range strings.Split(v, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				typesFilter = append(typesFilter, t)
			}
		}
	}

	req := &types.GazeGraphRequest{
		KnowledgeBaseID: kbID,
		Mode:            mode,
		Center:          center,
		Depth:           depth,
		Types:           typesFilter,
		Limit:           limit,
	}
	if h.memoryService != nil {
		req.FamiliarKnowledgeIDs = h.memoryService.FamiliarKnowledgeIDs(c.Request.Context())
	}

	graph, err := h.gazeService.GetGraph(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, graph)
}

// GetStats godoc
// @Summary      Get gaze statistics
// @Description  Returns aggregate statistics about the gaze
// @Tags         Gaze
// @Produce      json
// @Param        kb_id  path  string  true  "Knowledge base ID"
// @Success      200  {object}  types.GazeStats
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/stats [get]
func (h *GazePageHandler) GetStats(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stats, err := h.gazeService.GetStats(c.Request.Context(), kbID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// SearchPages godoc
// @Summary      Search gaze pages
// @Description  Full-text search over gaze pages
// @Tags         Gaze
// @Produce      json
// @Param        kb_id  path   string  true   "Knowledge base ID"
// @Param        q      query  string  true   "Search query"
// @Param        limit  query  int     false  "Max results (default 10)"
// @Success      200  {array}  types.GazePage
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/search [get]
func (h *GazePageHandler) SearchPages(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query 'q' is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	pages, err := h.gazeService.SearchPages(c.Request.Context(), kbID, query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pages": pages})
}

// RebuildLinks godoc
// @Summary      Rebuild gaze links
// @Description  Re-parse all pages and rebuild bidirectional link references
// @Tags         Gaze
// @Param        kb_id  path  string  true  "Knowledge base ID"
// @Success      200  {object}  map[string]string
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/rebuild-links [post]
func (h *GazePageHandler) RebuildLinks(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.gazeService.RebuildLinks(c.Request.Context(), kbID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Links rebuilt successfully"})
}

// SyncOutdated godoc
// @Summary      同步版本过期标记
// @Description  按文号分组 KB 的 summary 页, 同文号旧页标 outdated + superseded_by_slug
// @Tags         gaze
// @Security     Bearer
// @Router       /knowledgebase/{kb_id}/gaze/outdated-sync [post]
func (h *GazePageHandler) SyncOutdated(c *gin.Context) {
	kbID, _, err := h.validateGazeKB(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	marked, err := h.gazeService.SyncOutdated(c.Request.Context(), kbID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"marked": marked})
}
