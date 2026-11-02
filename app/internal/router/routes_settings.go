package router

import (
	"gaze/internal/handler"

	"github.com/gin-gonic/gin"
)

// RegisterSettingsRoutes wires the single personal settings surface.
//
// These routes are deliberately JWT-only: the machine API key must never read
// or mint credentials, nor change configuration. The API-key gate denies any
// route that is not explicitly declared through apiKeyRoute/apiKeyGroup, so
// leaving these undeclared is the fail-closed choice.
func RegisterSettingsRoutes(r *gin.RouterGroup, h *handler.SettingsHandler) {
	settings := r.Group("/settings")
	{
		settings.GET("/kv/:key", h.GetTenantKV)
		settings.PUT("/kv/:key", h.UpdateTenantKV)

		settings.GET("/api-key", h.GetAPIKey)
		settings.POST("/api-key", h.CreateAPIKey)
		settings.POST("/api-key/reset", h.ResetAPIKey)
		settings.DELETE("/api-key", h.DeleteAPIKey)
	}
}
