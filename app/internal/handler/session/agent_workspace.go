package session

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"gaze/internal/errors"
	"gaze/internal/event"
	"gaze/internal/logger"
	"gaze/internal/types"
	secutils "gaze/internal/utils"
)

// CreateAgentWorkspaceSession opens an Agent-workspace session.
// @Summary      创建智能体会话
// @Tags         智能体工作区
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /agent-workspace/sessions [post]
func (h *Handler) CreateAgentWorkspaceSession(c *gin.Context) {
	var req types.CreateAgentWorkspaceSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	view, err := h.agentWorkspaceService.CreateSession(c.Request.Context(), &req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": view})
}

// ListAgentWorkspaceSessions lists workspace sessions only.
// @Summary      智能体会话列表
// @Tags         智能体工作区
// @Produce      json
// @Security     Bearer
// @Router       /agent-workspace/sessions [get]
func (h *Handler) ListAgentWorkspaceSessions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	views, total, err := h.agentWorkspaceService.ListSessions(
		c.Request.Context(), &types.Pagination{Page: page, PageSize: pageSize}, c.Query("keyword"),
	)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": views, "total": total, "page": page, "page_size": pageSize})
}

// GetAgentWorkspaceSession returns one workspace session.
// @Summary      智能体会话详情
// @Tags         智能体工作区
// @Produce      json
// @Security     Bearer
// @Router       /agent-workspace/sessions/{id} [get]
func (h *Handler) GetAgentWorkspaceSession(c *gin.Context) {
	view, err := h.agentWorkspaceService.GetSession(c.Request.Context(), secutils.SanitizeForLog(c.Param("id")))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}

// UpdateAgentWorkspaceSession patches title/model/scope.
// @Summary      更新智能体会话
// @Tags         智能体工作区
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /agent-workspace/sessions/{id} [put]
func (h *Handler) UpdateAgentWorkspaceSession(c *gin.Context) {
	var req types.UpdateAgentWorkspaceSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	view, err := h.agentWorkspaceService.UpdateSession(
		c.Request.Context(), secutils.SanitizeForLog(c.Param("id")), &req,
	)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}

// DeleteAgentWorkspaceSession removes a workspace session.
// @Summary      删除智能体会话
// @Tags         智能体工作区
// @Produce      json
// @Security     Bearer
// @Router       /agent-workspace/sessions/{id} [delete]
func (h *Handler) DeleteAgentWorkspaceSession(c *gin.Context) {
	if err := h.agentWorkspaceService.DeleteSession(c.Request.Context(), secutils.SanitizeForLog(c.Param("id"))); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListAgentWorkspaceMessages returns the message history (with agent_steps).
// @Summary      智能体会话消息
// @Tags         智能体工作区
// @Produce      json
// @Security     Bearer
// @Router       /agent-workspace/sessions/{id}/messages [get]
func (h *Handler) ListAgentWorkspaceMessages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "100"))
	messages, err := h.agentWorkspaceService.ListMessages(
		c.Request.Context(), secutils.SanitizeForLog(c.Param("id")), page, pageSize,
	)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"messages": messages}})
}

// AgentWorkspaceChat runs one workspace turn and streams agent events (SSE).
// @Summary      智能体对话（SSE）
// @Tags         智能体工作区
// @Accept       json
// @Produce      text/event-stream
// @Security     Bearer
// @Router       /agent-workspace/sessions/{id}/messages [post]
func (h *Handler) AgentWorkspaceChat(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := secutils.SanitizeForLog(c.Param("id"))
	var req types.AgentWorkspaceRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		c.Error(errors.NewBadRequestError("query 不能为空"))
		return
	}
	session, err := h.sessionService.GetOwnedSession(ctx, sessionID)
	if err != nil || session == nil || session.Surface != types.AgentWorkspaceSurface {
		c.Error(errors.NewNotFoundError("会话不存在"))
		return
	}

	requestID := uuid.NewString()
	if id := getRequestID(c); id != "" {
		requestID = id
	}
	userMessage, err := h.createUserMessage(ctx, sessionID, query, requestID, nil, nil, nil, "web", nil)
	if err != nil {
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	assistantMessage, err := h.createAssistantMessage(ctx, &types.Message{
		SessionID:   sessionID,
		Role:        "assistant",
		IsCompleted: false,
		RequestID:   requestID,
		ModelID:     session.SummaryModelID,
	})
	if err != nil {
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	eventBus := event.NewEventBus()
	asyncCtx, cancel := context.WithCancel(logger.CloneContext(ctx))
	receivedAt := time.Now()
	h.setupStreamHandler(asyncCtx, sessionID, assistantMessage.ID, requestID, session.TenantID, receivedAt, assistantMessage, eventBus)
	h.writeAgentQueryEvent(asyncCtx, sessionID, userMessage.ID, userMessage.CreatedAt, assistantMessage)

	go func() {
		defer cancel()
		if err := h.agentWorkspaceService.RunTurn(asyncCtx, session, assistantMessage.ID, query, eventBus); err != nil {
			logger.Errorf(asyncCtx, "agent workspace turn failed: %v", err)
			_ = eventBus.Emit(asyncCtx, event.Event{
				Type:      event.EventError,
				SessionID: sessionID,
				Data: event.ErrorData{
					Error:     err.Error(),
					Stage:     "agent_workspace",
					SessionID: sessionID,
				},
			})
		}
	}()

	setSSEHeaders(c)
	h.handleAgentEventsForSSE(c.Request.Context(), c, sessionID, assistantMessage.ID, requestID, eventBus, false, nil)
	h.completeAssistantMessage(context.WithoutCancel(ctx), assistantMessage, "", "")
}

// GetAgentPending returns one L2/L3 dry-run task with its preview.
// @Summary      待确认任务详情
// @Tags         智能体工作区
// @Produce      json
// @Security     Bearer
// @Router       /agent-workspace/pending/{id} [get]
func (h *Handler) GetAgentPending(c *gin.Context) {
	view, err := h.agentPendingService.Get(c.Request.Context(), secutils.SanitizeForLog(c.Param("id")))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}

// ConfirmAgentPending applies a pending L2/L3 change after owner confirmation.
// @Summary      确认并执行待确认任务
// @Tags         智能体工作区
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /agent-workspace/pending/{id}/confirm [post]
func (h *Handler) ConfirmAgentPending(c *gin.Context) {
	var req types.AgentConfirmRequest
	_ = c.ShouldBindJSON(&req)
	result, err := h.agentPendingService.Confirm(c.Request.Context(), secutils.SanitizeForLog(c.Param("id")), &req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// RejectAgentPending discards a pending L2/L3 change without writing anything.
// @Summary      驳回待确认任务
// @Tags         智能体工作区
// @Accept       json
// @Produce      json
// @Security     Bearer
// @Router       /agent-workspace/pending/{id}/reject [post]
func (h *Handler) RejectAgentPending(c *gin.Context) {
	var req types.AgentConfirmRequest
	_ = c.ShouldBindJSON(&req)
	view, err := h.agentPendingService.Reject(c.Request.Context(), secutils.SanitizeForLog(c.Param("id")), &req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}
