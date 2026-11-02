package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gaze/internal/handler"
)

// RegisterAuthRoutes registers the personal edition's authentication routes.
// Registration, invitations, OIDC and workspace switching do not exist here:
// the owner is seeded from the environment on first start.
func RegisterAuthRoutes(r *gin.RouterGroup, handler *handler.AuthHandler, g *apiKeyGuards) {
	r.POST("/auth/login", handler.Login)
	r.POST("/auth/refresh", handler.RefreshToken)
	r.GET("/auth/validate", handler.ValidateToken)
	r.POST("/auth/logout", handler.Logout)
	// auth/me returns only the caller's own identity/profile, so it is safe
	// for any valid API key. Chat clients / MCP call it to discover "who am I".
	g.apiKeyRoute(r, http.MethodGet, "/auth/me", apiKeyAny(), handler.GetCurrentUser)
	r.POST("/auth/change-password", handler.ChangePassword)
}

// RegisterSystemRoutes registers system information and infrastructure routes.
func RegisterSystemRoutes(
	r *gin.RouterGroup,
	handler *handler.SystemHandler,
	g *apiKeyGuards,
) {
	systemRoutes := g.apiKeyGroup(r.Group("/system"), apiKeyManageVectorStores(apiKeyFullAccess()))
	{
		systemRoutes.With(apiKeyAny()).GET("/capabilities", handler.GetDeploymentCapabilities)
		systemRoutes.GET("/info", handler.GetSystemInfo)
		systemRoutes.GET("/parser-engines", handler.ListParserEngines)
		systemRoutes.POST("/parser-engines/check", handler.CheckParserEngines)
		systemRoutes.POST("/docreader/reconnect", handler.ReconnectDocReader)
		systemRoutes.GET("/storage-engine-status", handler.GetStorageEngineStatus)
		systemRoutes.POST("/storage-engine-check", handler.CheckStorageEngine)
		systemRoutes.POST("/sandbox-check", handler.CheckSandboxConfig)
	}
}
