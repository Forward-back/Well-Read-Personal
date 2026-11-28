package handler

import (
	"net/http"
	"strconv"
	"sync"
	"time"

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

// Create godoc
// @Summary      创建精读运行
// @Tags         精读
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /reading/runs [post]
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

// Get godoc
// @Summary      获取精读运行
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id} [get]
func (h *ReadingHandler) Get(c *gin.Context) {
	run, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

// GetActive godoc
// @Summary      获取进行中的精读运行
// @Tags         精读
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /reading/runs/active [get]
func (h *ReadingHandler) GetActive(c *gin.Context) {
	run, err := h.service.GetActive(c.Request.Context(), c.Query("session_id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

// Step godoc
// @Summary      推进精读步骤
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/step [post]
func (h *ReadingHandler) Step(c *gin.Context) {
	run, err := h.service.Step(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

// StepStream godoc
// @Summary      流式推进精读步骤
// @Description  通过 SSE 推送阶段、检索与模型思考过程；最终帧返回完整运行
// @Tags         精读
// @Accept       json
// @Produce      text/event-stream
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/step/stream [post]
func (h *ReadingHandler) StepStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	var writeMu sync.Mutex
	emit := func(event types.ReadingProgressEvent) {
		writeMu.Lock()
		defer writeMu.Unlock()
		c.SSEvent("message", event)
		c.Writer.Flush()
	}
	startedAt := time.Now()
	done := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				emit(types.ReadingProgressEvent{
					Type: "heartbeat", ElapsedMS: time.Since(startedAt).Milliseconds(),
				})
			}
		}
	}()

	ctx := types.WithReadingProgress(c.Request.Context(), emit)
	run, err := h.service.Step(ctx, c.Param("id"))
	close(done)
	<-writerDone
	if err != nil {
		emit(types.ReadingProgressEvent{Type: "error", Content: err.Error()})
		return
	}
	emit(types.ReadingProgressEvent{Type: "complete", Run: run, ElapsedMS: time.Since(startedAt).Milliseconds()})
}

// Feedback godoc
// @Summary      提交精读回答
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/feedback [post]
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

// JudgeQuestion godoc
// @Summary      判定精读问题
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Param        questionId  path  string  true  "questionId"
// @Security     Bearer
// @Router       /reading/runs/{id}/questions/{questionId}/judge [post]
func (h *ReadingHandler) JudgeQuestion(c *gin.Context) {
	run, err := h.service.JudgeQuestion(c.Request.Context(), c.Param("id"), c.Param("questionId"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

// ResolveContradiction godoc
// @Summary      处理精读矛盾
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Param        contradictionId  path  string  true  "contradictionId"
// @Security     Bearer
// @Router       /reading/runs/{id}/contradictions/{contradictionId}/resolve [post]
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

// GetReport godoc
// @Summary      获取精读报告
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/report [get]
func (h *ReadingHandler) GetReport(c *gin.Context) {
	report, err := h.service.GetReport(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": report})
}

// GetDocuments godoc
// @Summary      获取精读文档
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/documents [get]
func (h *ReadingHandler) GetDocuments(c *gin.Context) {
	documents, err := h.service.GetDocuments(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": documents})
}

// SaveReport godoc
// @Summary      保存精读报告
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/report [put]
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

// DeleteReport godoc
// @Summary      删除精读报告
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/report [delete]
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

// AddDocuments godoc
// @Summary      添加精读文档
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/add-documents [post]
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

// Stop godoc
// @Summary      暂停精读
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/stop [post]
func (h *ReadingHandler) Stop(c *gin.Context) {
	run, err := h.service.Stop(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

// Abandon godoc
// @Summary      终止精读
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/abandon [post]
func (h *ReadingHandler) Abandon(c *gin.Context) {
	run, err := h.service.Abandon(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

// Jump godoc
// @Summary      跳转精读章节
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/jump [post]
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

// UpdateUnderstanding godoc
// @Summary      更新理解确认
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/understanding [put]
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

// UpdateTerm godoc
// @Summary      更新精读术语
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Param        termId  path  string  true  "termId"
// @Security     Bearer
// @Router       /reading/runs/{id}/terms/{termId} [put]
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

// UpdateConclusion godoc
// @Summary      更新精读结论
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Param        conclusionId  path  string  true  "conclusionId"
// @Security     Bearer
// @Router       /reading/runs/{id}/conclusions/{conclusionId} [put]
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

// ListHistory godoc
// @Summary      获取精读历史
// @Tags         精读
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /reading/runs/history [get]
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

// ListDrafts godoc
// @Summary      列出精读草稿
// @Tags         精读
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /reading/drafts [get]
func (h *ReadingHandler) ListDrafts(c *gin.Context) {
	drafts, err := h.service.ListDrafts(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": drafts})
}

// CreateDraft godoc
// @Summary      创建精读草稿
// @Tags         精读
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /reading/drafts [post]
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

// UpdateDraft godoc
// @Summary      更新精读草稿
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        draftId  path  string  true  "draftId"
// @Security     Bearer
// @Router       /reading/drafts/{draftId} [put]
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

// DeleteDraft godoc
// @Summary      删除精读草稿
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        draftId  path  string  true  "draftId"
// @Security     Bearer
// @Router       /reading/drafts/{draftId} [delete]
func (h *ReadingHandler) DeleteDraft(c *gin.Context) {
	if err := h.service.DeleteDraft(c.Request.Context(), c.Param("draftId")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Resume godoc
// @Summary      继续精读
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/resume [post]
func (h *ReadingHandler) Resume(c *gin.Context) {
	run, err := h.service.Resume(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

// Export godoc
// @Summary      导出精读
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/export [get]
func (h *ReadingHandler) Export(c *gin.Context) {
	format := c.DefaultQuery("format", "markdown")
	report, err := h.service.Export(c.Request.Context(), c.Param("id"), format)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": report})
}

// ListSelfTests godoc
// @Summary      获取精读自测题
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/self-tests [get]
func (h *ReadingHandler) ListSelfTests(c *gin.Context) {
	tests, err := h.service.ListSelfTests(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tests})
}

// ImportSelfTests godoc
// @Summary      导入精读自测题
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/self-tests/import [post]
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

// SubmitFeynman godoc
// @Summary      提交费曼自述
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/feynman [post]
func (h *ReadingHandler) SubmitFeynman(c *gin.Context) {
	var req types.SubmitReadingFeynmanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	attempt, err := h.service.SubmitFeynman(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": attempt})
}

// ListFeynman godoc
// @Summary      列出费曼自述记录
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Param        section_id  query  string  false  "章节ID"
// @Security     Bearer
// @Router       /reading/runs/{id}/feynman [get]
func (h *ReadingHandler) ListFeynman(c *gin.Context) {
	attempts, err := h.service.ListFeynman(c.Request.Context(), c.Param("id"), c.Query("section_id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": attempts})
}

// CreateAnnotation godoc
// @Summary      创建原文批注
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Security     Bearer
// @Router       /reading/runs/{id}/annotations [post]
func (h *ReadingHandler) CreateAnnotation(c *gin.Context) {
	var req types.CreateReadingAnnotationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	annotation, err := h.service.CreateAnnotation(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": annotation})
}

// DeleteAnnotation godoc
// @Summary      删除原文批注
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Param        annotationId  path  string  true  "annotationId"
// @Security     Bearer
// @Router       /reading/runs/{id}/annotations/{annotationId} [delete]
func (h *ReadingHandler) DeleteAnnotation(c *gin.Context) {
	if err := h.service.DeleteAnnotation(c.Request.Context(), c.Param("id"), c.Param("annotationId")); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// CreateCardFromAnnotation godoc
// @Summary      从原文批注生成复习卡
// @Tags         精读
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "id"
// @Param        annotationId  path  string  true  "annotationId"
// @Security     Bearer
// @Router       /reading/runs/{id}/annotations/{annotationId}/card [post]
func (h *ReadingHandler) CreateCardFromAnnotation(c *gin.Context) {
	var req types.CreateReadingAnnotationCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	result, err := h.service.CreateCardFromAnnotation(c.Request.Context(), c.Param("id"), c.Param("annotationId"), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}
