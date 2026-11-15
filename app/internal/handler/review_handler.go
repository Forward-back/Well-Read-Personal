package handler

import (
	"net/http"
	"strconv"

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

func (h *ReviewHandler) ListGroups(c *gin.Context) {
	tree, err := h.service.ListGroupTree(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tree})
}

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

func (h *ReviewHandler) DeleteGroup(c *gin.Context) {
	deleteCards := c.Query("delete_cards") == "true"
	if err := h.service.DeleteGroup(c.Request.Context(), c.Param("id"), deleteCards); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ReviewHandler) ListCards(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	includeSubtree := c.Query("include_subtree") != "false"
	filter := &types.ReviewCardFilter{
		GroupID:        c.Query("group_id"),
		IncludeSubtree: includeSubtree,
		Keyword:        c.Query("keyword"),
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

func (h *ReviewHandler) GetCard(c *gin.Context) {
	detail, err := h.service.GetCard(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": detail})
}

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

func (h *ReviewHandler) SetCardStatus(c *gin.Context) {
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	if err := h.service.SetCardStatus(c.Request.Context(), c.Param("id"), req.Status); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ReviewHandler) DeleteCard(c *gin.Context) {
	if err := h.service.DeleteCard(c.Request.Context(), c.Param("id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ReviewHandler) RestoreCard(c *gin.Context) {
	if err := h.service.RestoreCard(c.Request.Context(), c.Param("id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ReviewHandler) TodayQueue(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	queue, err := h.service.TodayQueue(c.Request.Context(), c.Query("group_id"), limit)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": queue})
}

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

func (h *ReviewHandler) MarkSourceUnavailable(c *gin.Context) {
	if err := h.service.MarkSourceUnavailable(c.Request.Context(), c.Param("id"), c.Param("source_id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

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

func (h *ReviewHandler) Stats(c *gin.Context) {
	days, _ := strconv.Atoi(c.Query("days"))
	data, err := h.service.Stats(c.Request.Context(), days)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *ReviewHandler) Export(c *gin.Context) {
	data, err := h.service.Export(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=wellread-review-export.json")
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

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

func (h *ReviewHandler) GetActiveSession(c *gin.Context) {
	state, err := h.service.GetActiveSession(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": state})
}

func (h *ReviewHandler) GetSession(c *gin.Context) {
	state, err := h.service.GetSession(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": state})
}

func (h *ReviewHandler) AbandonSession(c *gin.Context) {
	if err := h.service.AbandonSession(c.Request.Context(), c.Param("id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ReviewHandler) ListSessions(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	sessions, err := h.service.ListSessions(c.Request.Context(), limit)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sessions})
}

func (h *ReviewHandler) GetCardPerformance(c *gin.Context) {
	performance, err := h.service.GetCardPerformance(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": performance})
}
