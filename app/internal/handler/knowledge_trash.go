package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"gaze/internal/errors"
	"gaze/internal/types"
	secutils "gaze/internal/utils"
)

// ListKnowledgeTrash lists soft-deleted documents for the recycle bin.
// @Summary      回收站列表
// @Tags         知识库
// @Produce      json
// @Param        kb_id    query  string  false  "知识库 ID"
// @Param        keyword  query  string  false  "标题关键字"
// @Security     Bearer
// @Router       /knowledge/trash [get]
func (h *KnowledgeHandler) ListKnowledgeTrash(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Query("kb_id"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	result, err := h.agentDocs.ListTrash(ctx, kbID, c.Query("keyword"), &types.Pagination{Page: page, PageSize: pageSize})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// RestoreKnowledgeTrash restores one document from the recycle bin.
// @Summary      恢复回收站文档
// @Tags         知识库
// @Produce      json
// @Param        id  path  string  true  "文档 ID"
// @Security     Bearer
// @Router       /knowledge/{id}/restore [post]
func (h *KnowledgeHandler) RestoreKnowledgeTrash(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok {
		c.Error(errors.NewUnauthorizedError("未登录"))
		return
	}
	id := secutils.SanitizeForLog(c.Param("id"))
	row, err := h.knowledgeRepo.GetKnowledgeUnscoped(ctx, tenantID, id)
	if err != nil || row == nil {
		c.Error(errors.NewNotFoundError("文档不存在"))
		return
	}
	if _, _, err := h.requireKnowledgeWriteAccess(c, row.KnowledgeBaseID); err != nil {
		c.Error(err)
		return
	}
	restored, err := h.agentDocs.RestoreDocuments(ctx, []string{id})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"restored": restored}})
}

// PurgeKnowledgeTrash permanently deletes one trashed document.
// @Summary      彻底删除回收站文档
// @Tags         知识库
// @Produce      json
// @Param        id  path  string  true  "文档 ID"
// @Security     Bearer
// @Router       /knowledge/trash/{id} [delete]
func (h *KnowledgeHandler) PurgeKnowledgeTrash(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok {
		c.Error(errors.NewUnauthorizedError("未登录"))
		return
	}
	id := secutils.SanitizeForLog(c.Param("id"))
	row, err := h.knowledgeRepo.GetKnowledgeUnscoped(ctx, tenantID, id)
	if err != nil || row == nil {
		c.Error(errors.NewNotFoundError("文档不存在"))
		return
	}
	if _, _, err := h.requireKnowledgeWriteAccess(c, row.KnowledgeBaseID); err != nil {
		c.Error(err)
		return
	}
	purged, err := h.agentDocs.PurgeDocuments(ctx, []string{id})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"purged": purged}})
}
