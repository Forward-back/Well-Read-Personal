package router

import (
	"github.com/gin-gonic/gin"

	"gaze/internal/handler"
)

// RegisterWebSearchRuntimeRoutes exposes web search provider management. The
// personal edition supports DeepSeek and DuckDuckGo only; Agents reference
// these entities, while direct chat reads tenant KV instead.
func RegisterWebSearchRuntimeRoutes(r *gin.RouterGroup, h *handler.WebSearchRuntimeHandler, g *apiKeyGuards) {
	providers := g.apiKeyGroup(r.Group("/web-search-providers"), apiKeyFullAccess())
	providers.GET("", h.ListProviders)
	providers.GET("/types", h.ListProviderTypes)
	providers.POST("", h.CreateProvider)
	providers.PUT("/:id", h.UpdateProvider)
	providers.DELETE("/:id", h.DeleteProvider)
	providers.PUT("/:id/credentials", h.UpdateCredentials)
	providers.DELETE("/:id/credentials/:field", h.ClearCredential)
	providers.POST("/:id/test", h.TestProvider)
}
