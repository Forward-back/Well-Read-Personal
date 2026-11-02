package router

import (
	"github.com/gin-gonic/gin"

	"gaze/internal/handler"
)

// RegisterWebSearchRuntimeRoutes exposes the read-only provider list used by
// chat readiness checks. Provider CRUD remains disabled.
func RegisterWebSearchRuntimeRoutes(r *gin.RouterGroup, h *handler.WebSearchRuntimeHandler, g *apiKeyGuards) {
	providers := g.apiKeyGroup(r.Group("/web-search-providers"), apiKeyFullAccess())
	providers.GET("", h.ListProviders)
}
