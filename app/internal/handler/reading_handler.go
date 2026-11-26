package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"gaze/internal/errors"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

// ReadingHandler exposes the interactive document reading API.
type ReadingHandler struct {
	service interfaces.ReadingRunService
}

func NewReadingHandler(service interfaces.ReadingRunService) *ReadingHandler {
	return &ReadingHandler{service: service}
}

func (h *ReadingHandler) Create(c *gin.Context) {
	var req types.CreateReadingRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	run, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": run})
}

func (h *ReadingHandler) Get(c *gin.Context) {
	run, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func (h *ReadingHandler) GetActive(c *gin.Context) {
	run, err := h.service.GetActive(c.Request.Context(), c.Query("session_id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func (h *ReadingHandler) Step(c *gin.Context) {
	run, err := h.service.Step(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func (h *ReadingHandler) Feedback(c *gin.Context) {
	var req types.ReadingFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	run, err := h.service.Feedback(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func (h *ReadingHandler) JudgeQuestion(c *gin.Context) {
	run, err := h.service.JudgeQuestion(c.Request.Context(), c.Param("id"), c.Param("questionId"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func (h *ReadingHandler) ResolveContradiction(c *gin.Context) {
	var req types.ReadingContradictionResolutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	run, err := h.service.ResolveContradiction(
		c.Request.Context(), c.Param("id"), c.Param("contradictionId"), &req,
	)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func (h *ReadingHandler) GetReport(c *gin.Context) {
	report, err := h.service.GetReport(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": report})
}

func (h *ReadingHandler) GetDocuments(c *gin.Context) {
	documents, err := h.service.GetDocuments(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": documents})
}

func (h *ReadingHandler) SaveReport(c *gin.Context) {
	var req types.SaveReadingReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	report, err := h.service.SaveReport(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": report})
}

func (h *ReadingHandler) DeleteReport(c *gin.Context) {
	baseVersion := -1
	if raw := c.Query("base_version"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			_ = c.Error(errors.NewBadRequestError("base_version 必须是整数"))
			return
		}
		baseVersion = parsed
	}
	report, err := h.service.DeleteReport(c.Request.Context(), c.Param("id"), baseVersion)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": report})
}

func (h *ReadingHandler) MigrateLegacy(c *gin.Context) {
	limit := 500
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			_ = c.Error(errors.NewBadRequestError("limit 必须是正整数"))
			return
		}
		limit = parsed
	}
	migrated, err := h.service.MigrateLegacyFinishedRuns(c.Request.Context(), limit)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"migrated": migrated}})
}

func (h *ReadingHandler) AddDocuments(c *gin.Context) {
	var req types.AddReadingDocumentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	run, err := h.service.AddDocuments(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func (h *ReadingHandler) Stop(c *gin.Context) {
	run, err := h.service.Stop(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func (h *ReadingHandler) Abandon(c *gin.Context) {
	run, err := h.service.Abandon(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func (h *ReadingHandler) Jump(c *gin.Context) {
	var req types.JumpReadingSectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	run, err := h.service.Jump(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func (h *ReadingHandler) UpdateUnderstanding(c *gin.Context) {
	var req types.UpdateReadingUnderstandingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	run, err := h.service.UpdateUnderstanding(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func (h *ReadingHandler) UpdateTerm(c *gin.Context) {
	var req types.UpdateReadingNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	term, err := h.service.UpdateTerm(
		c.Request.Context(), c.Param("id"), c.Param("termId"), &req,
	)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": term})
}

func (h *ReadingHandler) UpdateConclusion(c *gin.Context) {
	var req types.UpdateReadingNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	conclusion, err := h.service.UpdateConclusion(
		c.Request.Context(), c.Param("id"), c.Param("conclusionId"), &req,
	)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": conclusion})
}

func (h *ReadingHandler) ListHistory(c *gin.Context) {
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			_ = c.Error(errors.NewBadRequestError("limit 必须是正整数"))
			return
		}
		limit = parsed
	}
	items, err := h.service.ListHistory(c.Request.Context(), limit)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func (h *ReadingHandler) ListDrafts(c *gin.Context) {
	drafts, err := h.service.ListDrafts(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": drafts})
}

func (h *ReadingHandler) CreateDraft(c *gin.Context) {
	var req types.ReadingDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	draft, err := h.service.CreateDraft(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": draft})
}

func (h *ReadingHandler) UpdateDraft(c *gin.Context) {
	var req types.ReadingDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	draft, err := h.service.UpdateDraft(c.Request.Context(), c.Param("draftId"), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": draft})
}

func (h *ReadingHandler) DeleteDraft(c *gin.Context) {
	if err := h.service.DeleteDraft(c.Request.Context(), c.Param("draftId")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ReadingHandler) Resume(c *gin.Context) {
	run, err := h.service.Resume(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func (h *ReadingHandler) Export(c *gin.Context) {
	format := c.DefaultQuery("format", "markdown")
	report, err := h.service.Export(c.Request.Context(), c.Param("id"), format)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": report})
}

func (h *ReadingHandler) ListSelfTests(c *gin.Context) {
	tests, err := h.service.ListSelfTests(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tests})
}

func (h *ReadingHandler) ImportSelfTests(c *gin.Context) {
	var req types.ImportReadingSelfTestsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	result, err := h.service.ImportSelfTests(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}
