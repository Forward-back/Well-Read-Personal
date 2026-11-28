package router

import (
	"github.com/gin-gonic/gin"

	"gaze/internal/handler/session"
)

// RegisterAgentWorkspaceRoutes mounts the Agent-workspace API. These are human
// session-scoped endpoints; API keys are default-denied by the apiKey gate.
func RegisterAgentWorkspaceRoutes(r *gin.RouterGroup, h *session.Handler) {
	if h == nil {
		return
	}
	workspace := r.Group("/agent-workspace")
	{
		workspace.POST("/sessions", h.CreateAgentWorkspaceSession)
		workspace.GET("/sessions", h.ListAgentWorkspaceSessions)
		workspace.GET("/sessions/:id", h.GetAgentWorkspaceSession)
		workspace.PUT("/sessions/:id", h.UpdateAgentWorkspaceSession)
		workspace.DELETE("/sessions/:id", h.DeleteAgentWorkspaceSession)
		workspace.GET("/sessions/:id/messages", h.ListAgentWorkspaceMessages)
		workspace.POST("/sessions/:id/messages", h.AgentWorkspaceChat)
	}
}
