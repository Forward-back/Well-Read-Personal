package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gaze/internal/errors"
	"gaze/internal/logger"
	"gaze/internal/types"
	secutils "gaze/internal/utils"
)

// GetKnowledgeContent returns a document rendered as markdown, live or at one
// archived version.
// @Summary      获取文档正文（可指定版本）
// @Tags         知识库
// @Produce      json
// @Param        id          path   string  true   "文档 ID"
// @Param        version_id  query  string  false  "版本 ID"
// @Security     Bearer
// @Router       /knowledge/{id}/content [get]
func (h *KnowledgeHandler) GetKnowledgeContent(c *gin.Context) {
	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		c.Error(errors.NewBadRequestError("Knowledge ID cannot be empty"))
		return
	}
	if _, _, err := h.resolveKnowledgeAndValidateKBAccess(c, id); err != nil {
		c.Error(err)
		return
	}
	view, err := h.agentDocs.GetDocumentContent(c.Request.Context(), id, c.Query("version_id"))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}

// ListDocVersions lists the snapshot chain of a document.
// @Summary      文档版本列表
// @Tags         知识库
// @Produce      json
// @Param        id  path  string  true  "文档 ID"
// @Security     Bearer
// @Router       /knowledge/{id}/versions [get]
func (h *KnowledgeHandler) ListDocVersions(c *gin.Context) {
	knowledge, effCtx, err := h.resolveKnowledgeAndValidateKBAccess(c, secutils.SanitizeForLog(c.Param("id")))
	if err != nil {
		c.Error(err)
		return
	}
	tenantID, ok := types.TenantIDFromContext(effCtx)
	if !ok {
		c.Error(errors.NewUnauthorizedError("未登录"))
		return
	}
	versions, err := h.docVersions.ListVersions(effCtx, tenantID, knowledge.KnowledgeBaseID, knowledge.ID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"versions": versions}})
}

// GetDocVersion returns one archived snapshot with its chunk rows.
// @Summary      文档版本详情
// @Tags         知识库
// @Produce      json
// @Param        id          path  string  true  "文档 ID"
// @Param        version_id  path  string  true  "版本 ID"
// @Security     Bearer
// @Router       /knowledge/{id}/versions/{version_id} [get]
func (h *KnowledgeHandler) GetDocVersion(c *gin.Context) {
	knowledge, effCtx, err := h.resolveKnowledgeAndValidateKBAccess(c, secutils.SanitizeForLog(c.Param("id")))
	if err != nil {
		c.Error(err)
		return
	}
	tenantID, ok := types.TenantIDFromContext(effCtx)
	if !ok {
		c.Error(errors.NewUnauthorizedError("未登录"))
		return
	}
	version, chunks, err := h.docVersions.GetVersion(
		effCtx, tenantID, knowledge.KnowledgeBaseID, knowledge.ID,
		secutils.SanitizeForLog(c.Param("version_id")),
	)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"version": version, "chunks": chunks}})
}

// CompareDocVersions returns two snapshots with markdown joins for the diff UI.
// @Summary      文档版本对比
// @Tags         知识库
// @Produce      json
// @Param        id   path  string  true  "文档 ID"
// @Param        from query string  true  "起始版本 ID"
// @Param        to   query string  true  "目标版本 ID"
// @Security     Bearer
// @Router       /knowledge/{id}/versions/compare [get]
func (h *KnowledgeHandler) CompareDocVersions(c *gin.Context) {
	knowledge, effCtx, err := h.resolveKnowledgeAndValidateKBAccess(c, secutils.SanitizeForLog(c.Param("id")))
	if err != nil {
		c.Error(err)
		return
	}
	from := secutils.SanitizeForLog(c.Query("from"))
	to := secutils.SanitizeForLog(c.Query("to"))
	if from == "" || to == "" {
		c.Error(errors.NewBadRequestError("from 与 to 不能为空"))
		return
	}
	tenantID, ok := types.TenantIDFromContext(effCtx)
	if !ok {
		c.Error(errors.NewUnauthorizedError("未登录"))
		return
	}
	compare, err := h.docVersions.CompareVersions(effCtx, tenantID, knowledge.KnowledgeBaseID, knowledge.ID, from, to)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": compare})
}

// RollbackDocVersion restores a historical snapshot as a new rollback version
// and triggers engine ingest.
// @Summary      回滚文档版本
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id          path  string  true  "文档 ID"
// @Param        version_id  path  string  true  "版本 ID"
// @Security     Bearer
// @Router       /knowledge/{id}/versions/{version_id}/rollback [post]
func (h *KnowledgeHandler) RollbackDocVersion(c *gin.Context) {
	knowledge, effCtx, err := h.resolveKnowledgeAndValidateKBAccess(c, secutils.SanitizeForLog(c.Param("id")))
	if err != nil {
		c.Error(err)
		return
	}
	tenantID, ok := types.TenantIDFromContext(effCtx)
	if !ok {
		c.Error(errors.NewUnauthorizedError("未登录"))
		return
	}
	if _, _, err := h.requireKnowledgeWriteAccess(c, knowledge.KnowledgeBaseID); err != nil {
		c.Error(err)
		return
	}
	var req types.RollbackDocVersionRequest
	_ = c.ShouldBindJSON(&req)
	actor, _ := types.UserIDFromContext(effCtx)
	version, err := h.docVersions.Rollback(
		effCtx, tenantID, knowledge.KnowledgeBaseID, knowledge.ID,
		secutils.SanitizeForLog(c.Param("version_id")), actor, req.Comment,
	)
	if err != nil {
		c.Error(err)
		return
	}
	if h.engineSync != nil {
		h.engineSync.PublishKnowledgeChange(effCtx, tenantID, knowledge.ID)
	}
	syncStatus := knowledge.EngineSyncStatus
	if refreshed, err := h.kgService.GetKnowledgeByID(effCtx, knowledge.ID); err == nil && refreshed != nil {
		syncStatus = refreshed.EngineSyncStatus
	}
	logger.Infof(effCtx, "knowledge version rolled back: %s -> %s", knowledge.ID, version.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"version":            version,
		"engine_sync_status": syncStatus,
	}})
}
