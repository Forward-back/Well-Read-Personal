package router

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/dig"

	"gaze/internal/config"
	"gaze/internal/handler"
	"gaze/internal/handler/session"
	"gaze/internal/logger"
	"gaze/internal/middleware"
	"gaze/internal/tracing/langfuse"
	"gaze/internal/types/interfaces"

	_ "gaze/docs" // swagger docs
)

// RouterParams 路由参数
type RouterParams struct {
	dig.In

	Config                       *config.Config
	FileService                  interfaces.FileService
	UserService                  interfaces.UserService
	KBService                    interfaces.KnowledgeBaseService
	KnowledgeService             interfaces.KnowledgeService
	ChunkService                 interfaces.ChunkService
	SessionService               interfaces.SessionService
	MessageService               interfaces.MessageService
	ModelService                 interfaces.ModelService
	KBHandler                    *handler.KnowledgeBaseHandler
	KnowledgeHandler             *handler.KnowledgeHandler
	SettingsHandler              *handler.SettingsHandler
	TenantService                interfaces.TenantService
	TenantAPIKeyService          interfaces.TenantAPIKeyService
	AgentRuntimeHandler          *handler.AgentRuntimeHandler
	MemoryRuntimeHandler         *handler.MemoryRuntimeHandler
	WebSearchRuntimeHandler      *handler.WebSearchRuntimeHandler
	ChunkHandler                 *handler.ChunkHandler
	SessionHandler               *session.Handler
	MessageHandler               *handler.MessageHandler
	MessageSuggestionHandler     *handler.MessageSuggestionHandler
	ModelHandler                 *handler.ModelHandler
	ModelCredentialsHandler      *handler.ModelCredentialsHandler
	AuthHandler                  *handler.AuthHandler
	InitializationHandler        *handler.InitializationHandler
	SystemHandler                *handler.SystemHandler
	StorageBackendHandler        *handler.StorageBackendHandler
	StorageBackendResolver       interfaces.StorageBackendResolver
	ResourceCatalog              interfaces.ResourceCatalog
	TagHandler                   *handler.TagHandler
	SkillHandler                 *handler.SkillHandler
	RedisClient                  *redis.Client
	DataSourceHandler            *handler.DataSourceHandler
	DataSourceCredentialsHandler *handler.DataSourceCredentialsHandler
	GazePageHandler              *handler.GazePageHandler
	ReadingHandler               *handler.ReadingHandler
}

// NewRouter 创建新的路由
func NewRouter(params RouterParams) *gin.Engine {
	r := gin.New()
	r.ContextWithFallback = true

	// Trusted proxies: gin defaults to trusting ALL proxies, which makes
	// c.ClientIP() honor a client-supplied X-Forwarded-For. Public, unauthed
	// endpoints rate-limit per caller, so a spoofed XFF would trivially bypass
	// the limiter. Restrict to the fronting proxy network so
	// only the real client IP (appended by nginx) is returned. Configurable via
	// GAZE_TRUSTED_PROXIES (comma-separated CIDRs/IPs).
	if err := r.SetTrustedProxies(trustedProxies()); err != nil {
		logger.Errorf(context.Background(), "[Router] failed to set trusted proxies: %v", err)
	}

	// CORS 中间件应放在最前面。
	// 注意：通配符 AllowOrigins 下浏览器会拒绝一切带凭据（cookie）的跨域
	// 请求（CORS 规范禁止 "*" 与 credentials 组合），因此 AllowCredentials
	// 实际只对未来改为回显具体 Origin 时才生效；当前认证全部走显式的
	// Authorization / X-API-Key 头，不依赖 ambient 凭据。若引入 cookie
	// 认证，必须先把 AllowOrigins 换成受控清单。
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-API-Key", "X-Request-ID", "X-Tenant-ID", "X-Embed-Session", "X-External-User-ID", "X-External-User-Token"},
		ExposeHeaders:    []string{"Content-Length", "Access-Control-Allow-Origin"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 基础中间件（不需要认证）
	r.Use(middleware.RequestID())
	r.Use(middleware.Language())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.ErrorHandler())

	// 健康检查（不需要认证）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Swagger API 文档（仅在非生产环境下启用）
	// 通过 GIN_MODE 环境变量判断：release 模式下禁用 Swagger
	if gin.Mode() != gin.ReleaseMode {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler,
			ginSwagger.DefaultModelsExpandDepth(-1), // 默认折叠 Models
			ginSwagger.DocExpansion("list"),         // 展开模式: "list"(展开标签), "full"(全部展开), "none"(全部折叠)
			ginSwagger.DeepLinking(true),            // 启用深度链接
			ginSwagger.PersistAuthorization(true),   // 持久化认证信息
		))
	}

	// 前端静态文件（仅 Lite 版本内嵌前端）
	if handler.Edition == "lite" {
		serveFrontendStatic(r)
	}

	// Sandbox terminal WebSocket (self-authenticated via a short-lived
	// query ticket). Browsers cannot set auth headers on the WS handshake,
	// so this must precede the global Auth middleware.
	RegisterSandboxTerminalRoutes(r, params.SessionHandler)

	// 认证中间件
	r.Use(middleware.Auth(params.UserService, params.TenantAPIKeyService, params.TenantService))

	// 文件服务：统一代理本地/MinIO/COS/TOS存储后端（需要认证）
	serveFilesWithResources(r, params.FileService, params.StorageBackendResolver, params.ResourceCatalog)

	// Presigned file access: no auth required, signature-verified.
	servePresignedFiles(r, params.TenantService, params.StorageBackendResolver)

	// Diagnostic preview of presigned URLs (behind auth middleware).
	servePresignedPreview(r, params.StorageBackendResolver)

	// Langfuse observability — only active when LANGFUSE_* env vars are set.
	// The middleware is registered unconditionally; when disabled it's a no-op.
	r.Use(langfuse.GinMiddleware())

	// 需要认证的API路由
	v1 := r.Group("/api/v1")
	{
		// apiKeyGuards records the API-key policy of every route reachable
		// with X-API-Key. The personal edition exposes one MCP key, but the
		// gate still denies any route that was not explicitly declared.
		apiKeyGuards := newAPIKeyGuards()

		// API-key gate: single authority for X-API-Key principals. Runs
		// first on every /api/v1 route (JWT sessions pass straight
		// through) and denies any route not explicitly declared via the
		// apiKeyGroup helpers. Must be attached BEFORE the Register* calls
		// so that sub-groups inherit it.
		v1.Use(apiKeyGuards.apiKeyAuthorizer.Middleware())

		RegisterAuthRoutes(v1, params.AuthHandler, apiKeyGuards)
		RegisterSettingsRoutes(v1, params.SettingsHandler)
		RegisterKnowledgeBaseRoutes(v1, params.KBHandler, apiKeyGuards)
		RegisterKnowledgeTagRoutes(v1, params.TagHandler, apiKeyGuards)
		RegisterKnowledgeRoutes(v1, params.KnowledgeHandler, apiKeyGuards)
		RegisterAgentRuntimeRoutes(v1, params.AgentRuntimeHandler, apiKeyGuards)
		RegisterMemoryRuntimeRoutes(v1, params.MemoryRuntimeHandler, apiKeyGuards)
		RegisterWebSearchRuntimeRoutes(v1, params.WebSearchRuntimeHandler, apiKeyGuards)
		RegisterChunkRoutes(v1, params.ChunkHandler, apiKeyGuards)
		RegisterSessionRoutes(v1, params.SessionHandler, params.MessageSuggestionHandler, apiKeyGuards)
		RegisterChatRoutes(v1, params.SessionHandler, apiKeyGuards)
		RegisterMessageRoutes(v1, params.MessageHandler, apiKeyGuards)
		RegisterModelRoutes(v1, params.ModelHandler, params.ModelCredentialsHandler, apiKeyGuards)
		RegisterInitializationRoutes(v1, params.InitializationHandler, apiKeyGuards)
		params.SystemHandler.BindDeploymentCapabilities(deploymentCapabilitiesFromRouter(params))
		RegisterSystemRoutes(v1, params.SystemHandler, apiKeyGuards)
		RegisterStorageBackendRoutes(v1, params.StorageBackendHandler, apiKeyGuards)
		RegisterSkillRoutes(v1, params.SkillHandler, apiKeyGuards)
		RegisterDataSourceRoutes(v1, params.DataSourceHandler, params.DataSourceCredentialsHandler, apiKeyGuards)
		RegisterGazePageRoutes(v1, params.GazePageHandler, apiKeyGuards)
		RegisterChunkerDebugRoutes(v1, apiKeyGuards)
		RegisterReadingRoutes(v1, params.ReadingHandler, apiKeyGuards)

		// Fail fast if any declared API-key policy points at a route
		// template that does not actually exist (typo / path drift). A
		// stale template would silently 403 every API key on that route,
		// so we panic at startup instead of shipping a dead policy.
		apiKeyGuards.assertAPIKeyPoliciesMatchRoutes(r)
	}

	return r
}

// trustedProxies returns the proxy CIDRs/IPs whose X-Forwarded-For headers
// gin should trust when resolving the client IP. Defaults to loopback and
// private ranges (covers the bundled nginx in a container network); override
// with GAZE_TRUSTED_PROXIES (comma-separated). An explicit empty value
// disables proxy trust entirely so ClientIP() returns the direct peer.
func trustedProxies() []string {
	raw, ok := os.LookupEnv("GAZE_TRUSTED_PROXIES")
	if !ok {
		return []string{
			"127.0.0.0/8",
			"::1/128",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"fc00::/7",
		}
	}
	proxies := make([]string, 0)
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}
	return proxies
}
