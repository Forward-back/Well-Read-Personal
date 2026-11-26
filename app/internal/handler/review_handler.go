package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"gaze/internal/errors"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

// ReviewHandler exposes the personal spaced-repetition API. All routes are
// JWT-only; the API-key gate default-denies them.
type ReviewHandler struct {
	service interfaces.ReviewService
}

func NewReviewHandler(service interfaces.ReviewService) *ReviewHandler {
	return &ReviewHandler{service: service}
}

// ListGroups godoc
// @Summary      列出复习卡组
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/groups [get]
func (h *ReviewHandler) ListGroups(c *gin.Context) {
	tree, err := h.service.ListGroupTree(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tree})
}

// CreateGroup godoc
// @Summary      创建复习卡组
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/groups [post]
func (h *ReviewHandler) CreateGroup(c *gin.Context) {
	var req types.CreateReviewGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	group, err := h.service.CreateGroup(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": group})
}

// UpdateGroup godoc
// @Summary      更新复习卡组
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/groups/{id} [put]
func (h *ReviewHandler) UpdateGroup(c *gin.Context) {
	var req types.UpdateReviewGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	group, err := h.service.UpdateGroup(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": group})
}

// MoveGroup godoc
// @Summary      移动复习卡组
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/groups/{id}/move [post]
func (h *ReviewHandler) MoveGroup(c *gin.Context) {
	var req struct {
		ParentID string `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	if err := h.service.MoveGroup(c.Request.Context(), c.Param("id"), req.ParentID); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteGroup godoc
// @Summary      删除复习卡组
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/groups/{id} [delete]
func (h *ReviewHandler) DeleteGroup(c *gin.Context) {
	deleteCards := c.Query("delete_cards") == "true"
	if err := h.service.DeleteGroup(c.Request.Context(), c.Param("id"), deleteCards); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListCards godoc
// @Summary      列出复习卡片
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/cards [get]
func (h *ReviewHandler) ListCards(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	includeSubtree := c.Query("include_subtree") != "false"
	filter := &types.ReviewCardFilter{
		GroupID:        c.Query("group_id"),
		IncludeSubtree: includeSubtree,
		Keyword:        c.Query("keyword"),
		Tag:            c.Query("tag"),
		DueOnly:        c.Query("due_only") == "true",
		Limit:          limit,
		Offset:         offset,
	}
	if status := c.Query("status"); status != "" {
		filter.Statuses = []string{status}
	}
	cards, err := h.service.ListCards(c.Request.Context(), filter)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": cards})
}

// CreateCard godoc
// @Summary      创建复习卡片
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/cards [post]
func (h *ReviewHandler) CreateCard(c *gin.Context) {
	var req types.CreateReviewCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	card, err := h.service.CreateCard(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": card})
}

// GetCard godoc
// @Summary      获取复习卡片
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/cards/{id} [get]
func (h *ReviewHandler) GetCard(c *gin.Context) {
	detail, err := h.service.GetCard(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": detail})
}

// UpdateCard godoc
// @Summary      更新复习卡片
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/cards/{id} [put]
func (h *ReviewHandler) UpdateCard(c *gin.Context) {
	var req types.UpdateReviewCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	card, err := h.service.UpdateCard(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": card})
}

// MoveCard godoc
// @Summary      移动复习卡片
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/cards/{id}/move [post]
func (h *ReviewHandler) MoveCard(c *gin.Context) {
	var req struct {
		GroupID string `json:"group_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	if err := h.service.MoveCard(c.Request.Context(), c.Param("id"), req.GroupID); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// SetCardStatus godoc
// @Summary      设置卡片暂停状态
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/cards/{id}/status [post]
func (h *ReviewHandler) SetCardStatus(c *gin.Context) {
	var req struct {
		Status         string     `json:"status"`
		SuspendedUntil *time.Time `json:"suspended_until"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	var err error
	if req.SuspendedUntil != nil {
		err = h.service.SetCardStatusUntil(c.Request.Context(), c.Param("id"), req.Status, req.SuspendedUntil)
	} else {
		err = h.service.SetCardStatus(c.Request.Context(), c.Param("id"), req.Status)
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteCard godoc
// @Summary      删除复习卡片
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/cards/{id} [delete]
func (h *ReviewHandler) DeleteCard(c *gin.Context) {
	if err := h.service.DeleteCard(c.Request.Context(), c.Param("id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RestoreCard godoc
// @Summary      恢复复习卡片
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/cards/{id}/restore [post]
func (h *ReviewHandler) RestoreCard(c *gin.Context) {
	if err := h.service.RestoreCard(c.Request.Context(), c.Param("id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ResetCardSchedule godoc
// @Summary      重置卡片调度
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/cards/{id}/reset [post]
func (h *ReviewHandler) ResetCardSchedule(c *gin.Context) {
	card, err := h.service.ResetCardSchedule(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": card})
}

// BatchCards godoc
// @Summary      批量操作复习卡片
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/cards/batch [post]
func (h *ReviewHandler) BatchCards(c *gin.Context) {
	var req types.BatchReviewCardsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	result, err := h.service.BatchCards(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// Heatmap godoc
// @Summary      获取漏洞热力图
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/heatmap [get]
func (h *ReviewHandler) Heatmap(c *gin.Context) {
	opts := &types.ReviewHeatmapOptions{
		GroupID: c.Query("group_id"),
		Expand:  c.Query("expand") == "1" || strings.EqualFold(c.Query("expand"), "true"),
	}
	opts.RowLimit, _ = strconv.Atoi(c.Query("row_limit"))
	opts.ColumnLimit, _ = strconv.Atoi(c.Query("column_limit"))
	if raw := strings.TrimSpace(c.Query("row_group_ids")); raw != "" {
		for _, id := range strings.Split(raw, ",") {
			if id = strings.TrimSpace(id); id != "" {
				opts.RowGroupIDs = append(opts.RowGroupIDs, id)
			}
		}
	}
	data, err := h.service.Heatmap(c.Request.Context(), opts)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// ClusterDetail godoc
// @Summary      获取漏洞簇详情
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/clusters/detail [get]
func (h *ReviewHandler) ClusterDetail(c *gin.Context) {
	rowGroupID := strings.TrimSpace(c.Query("row_group_id"))
	if rowGroupID == "" {
		rowGroupID = strings.TrimSuffix(strings.TrimSuffix(c.Query("row_id"), ":virtual"), ":direct")
	}
	req := &types.ReviewClusterDetailRequest{
		ScopeGroupID: strings.TrimSpace(c.Query("scope_group_id")),
		RowGroupID:   rowGroupID,
		ColumnKey:    strings.TrimSpace(c.Query("column_key")),
		Direct:       c.Query("direct") == "1" || strings.EqualFold(c.Query("direct"), "true"),
	}
	data, err := h.service.ClusterDetail(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// TodayQueue godoc
// @Summary      获取今日复习队列
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/queue [get]
func (h *ReviewHandler) TodayQueue(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	queue, err := h.service.TodayQueue(c.Request.Context(), c.Query("group_id"), limit)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": queue})
}

// SubmitReview godoc
// @Summary      提交复习评分
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/reviews [post]
func (h *ReviewHandler) SubmitReview(c *gin.Context) {
	var req types.SubmitReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	card, err := h.service.SubmitReview(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": card})
}

// MarkSourceUnavailable godoc
// @Summary      标记复习来源不可用
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Param        source_id  path  string  true  "source_id"
// @Security     Bearer
// @Router       /review/cards/{id}/sources/{source_id}/unavailable [post]
func (h *ReviewHandler) MarkSourceUnavailable(c *gin.Context) {
	if err := h.service.MarkSourceUnavailable(c.Request.Context(), c.Param("id"), c.Param("source_id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// MergeCards godoc
// @Summary      合并复习卡片
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/cards/merge [post]
func (h *ReviewHandler) MergeCards(c *gin.Context) {
	var req types.MergeReviewCardsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	if err := h.service.MergeCards(c.Request.Context(), &req); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RebindSource godoc
// @Summary      重绑复习来源
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Param        source_id  path  string  true  "source_id"
// @Security     Bearer
// @Router       /review/cards/{id}/sources/{source_id} [put]
func (h *ReviewHandler) RebindSource(c *gin.Context) {
	var req types.RebindReviewCardSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	if err := h.service.RebindSource(c.Request.Context(), c.Param("id"), c.Param("source_id"), &req); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Stats godoc
// @Summary      获取复习统计
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/stats [get]
func (h *ReviewHandler) Stats(c *gin.Context) {
	days, _ := strconv.Atoi(c.Query("days"))
	data, err := h.service.Stats(c.Request.Context(), days)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// Export godoc
// @Summary      导出复习数据
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/export [get]
func (h *ReviewHandler) Export(c *gin.Context) {
	data, err := h.service.Export(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=wellread-review-export.json")
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// PreviewGeneration godoc
// @Summary      预览生成复习卡片
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/generations/preview [post]
func (h *ReviewHandler) PreviewGeneration(c *gin.Context) {
	var req types.GenerateReviewCardsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	preview, err := h.service.PreviewGeneration(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": preview})
}

// ImportGeneratedCards godoc
// @Summary      导入生成的复习卡片
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/generations/import [post]
func (h *ReviewHandler) ImportGeneratedCards(c *gin.Context) {
	var req types.ImportReviewCardsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	count, err := h.service.ImportGeneratedCards(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"imported": count}})
}

// PreviewCardSplit godoc
// @Summary      预览拆分卡片
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/cards/{id}/split/preview [post]
func (h *ReviewHandler) PreviewCardSplit(c *gin.Context) {
	var req types.GenerateCardVariantsRequest
	_ = c.ShouldBindJSON(&req)
	preview, err := h.service.PreviewCardSplit(c.Request.Context(), c.Param("id"), req.Count)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": preview})
}

// ApplyCardSplit godoc
// @Summary      应用拆分卡片
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/cards/{id}/split/apply [post]
func (h *ReviewHandler) ApplyCardSplit(c *gin.Context) {
	var req types.ApplyCardSplitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	result, err := h.service.ApplyCardSplit(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// PreviewCardAngles godoc
// @Summary      预览卡片新角度
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/cards/{id}/angles/preview [post]
func (h *ReviewHandler) PreviewCardAngles(c *gin.Context) {
	var req types.GenerateCardVariantsRequest
	_ = c.ShouldBindJSON(&req)
	preview, err := h.service.PreviewCardAngles(c.Request.Context(), c.Param("id"), req.Count)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": preview})
}

// RootCause godoc
// @Summary      分析漏洞根因
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/clusters/root-cause [post]
func (h *ReviewHandler) RootCause(c *gin.Context) {
	var req types.ReviewRootCauseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	result, err := h.service.RootCause(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// StartSession godoc
// @Summary      开始复习会话
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/sessions [post]
func (h *ReviewHandler) StartSession(c *gin.Context) {
	var req types.StartReviewSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	state, err := h.service.StartSession(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": state})
}

// GetActiveSession godoc
// @Summary      获取进行中的复习会话
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/sessions/active [get]
func (h *ReviewHandler) GetActiveSession(c *gin.Context) {
	state, err := h.service.GetActiveSession(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": state})
}

// GetSession godoc
// @Summary      获取复习会话
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/sessions/{id} [get]
func (h *ReviewHandler) GetSession(c *gin.Context) {
	state, err := h.service.GetSession(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": state})
}

// AbandonSession godoc
// @Summary      放弃复习会话
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/sessions/{id}/abandon [post]
func (h *ReviewHandler) AbandonSession(c *gin.Context) {
	if err := h.service.AbandonSession(c.Request.Context(), c.Param("id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListSessions godoc
// @Summary      列出复习会话
// @Tags         复习
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /review/sessions [get]
func (h *ReviewHandler) ListSessions(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	sessions, err := h.service.ListSessions(c.Request.Context(), limit)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sessions})
}

// GetCardPerformance godoc
// @Summary      获取卡片表现
// @Tags         复习
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /review/cards/{id}/performance [get]
func (h *ReviewHandler) GetCardPerformance(c *gin.Context) {
	performance, err := h.service.GetCardPerformance(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": performance})
}
