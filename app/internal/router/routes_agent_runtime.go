package router

import (
	"github.com/gin-gonic/gin"

	"gaze/internal/handler"
)

// RegisterAgentRuntimeRoutes exposes read-only agent discovery for external
// integrations. Authoring routes remain unregistered.
func RegisterAgentRuntimeRoutes(r *gin.RouterGroup, h *handler.AgentRuntimeHandler, g *apiKeyGuards) {
	agents := g.apiKeyGroup(r.Group("/agents"), apiKeyReadAgents(apiKeyChat(apiKeyFullAccess())))
	{
		agents.GET("", h.ListAgents)
		agents.GET("/:id", h.GetAgent)
		agents.GET("/:id/suggested-questions", h.SuggestedQuestions)
	}
}
