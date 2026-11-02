package router

import (
	"github.com/gin-gonic/gin"

	"gaze/internal/handler"
)

// RegisterMemoryRuntimeRoutes exposes the per-user "forget" action used by
// chat timelines. Full memory management stays disabled.
func RegisterMemoryRuntimeRoutes(r *gin.RouterGroup, h *handler.MemoryRuntimeHandler, g *apiKeyGuards) {
	memory := g.apiKeyGroup(r.Group("/memory"), apiKeyFullAccess())
	memory.DELETE("/items/:id", h.DeleteItem)
}
