package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gaze/internal/handler"
)

// RegisterModelRoutes wires model provider and credential management.
func RegisterModelRoutes(
	r *gin.RouterGroup,
	handler *handler.ModelHandler,
	credHandler *handler.ModelCredentialsHandler,
	g *apiKeyGuards,
) {
	models := g.apiKeyGroup(r.Group("/models"), apiKeyManageModels(apiKeyFullAccess()))
	{
		models.GET("/providers", handler.ListModelProviders)
		models.POST("", handler.CreateModel)
		models.GET("", handler.ListModels)
		models.POST("/:id/debug", handler.DebugModel)
		models.GET("/:id", handler.GetModel)
		models.PUT("/:id", handler.UpdateModel)
		models.DELETE("/:id", handler.DeleteModel)
		models.PUT("/:id/credentials", credHandler.Put)
		models.DELETE("/:id/credentials/:field", credHandler.DeleteField)
	}
}

func RegisterInitializationRoutes(r *gin.RouterGroup, handler *handler.InitializationHandler, g *apiKeyGuards) {
	g.apiKeyRoute(r, http.MethodGet, "/initialization/config/:kbId",
		apiKeyRetrieve(apiKeyFullAccess()), handler.GetCurrentConfigByKB)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/initialize/:kbId",
		apiKeyManageKnowledgeBases(apiKeyFullAccess()), handler.InitializeByKB)
	g.apiKeyRoute(r, http.MethodPut, "/initialization/config/:kbId",
		apiKeyManageKnowledgeBases(apiKeyFullAccess()), handler.UpdateKBConfig)

	g.apiKeyRoute(r, http.MethodGet, "/initialization/ollama/status", apiKeyManageModels(apiKeyFullAccess()), handler.CheckOllamaStatus)
	g.apiKeyRoute(r, http.MethodGet, "/initialization/ollama/models", apiKeyManageModels(apiKeyFullAccess()), handler.ListOllamaModels)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/ollama/models/check", apiKeyManageModels(apiKeyFullAccess()), handler.CheckOllamaModels)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/ollama/models/download", apiKeyManageModels(apiKeyFullAccess()), handler.DownloadOllamaModel)
	g.apiKeyRoute(r, http.MethodGet, "/initialization/ollama/download/progress/:taskId", apiKeyManageModels(apiKeyFullAccess()), handler.GetDownloadProgress)
	g.apiKeyRoute(r, http.MethodGet, "/initialization/ollama/download/tasks", apiKeyManageModels(apiKeyFullAccess()), handler.ListDownloadTasks)

	g.apiKeyRoute(r, http.MethodPost, "/initialization/remote/check", apiKeyManageModels(apiKeyFullAccess()), handler.CheckRemoteModel)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/embedding/test", apiKeyManageModels(apiKeyFullAccess()), handler.TestEmbeddingModel)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/rerank/check", apiKeyManageModels(apiKeyFullAccess()), handler.CheckRerankModel)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/asr/check", apiKeyManageModels(apiKeyFullAccess()), handler.CheckASRModel)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/multimodal/test", apiKeyManageModels(apiKeyFullAccess()), handler.TestMultimodalFunction)

	g.apiKeyRoute(r, http.MethodPost, "/initialization/extract/text-relation", apiKeyManageModels(apiKeyFullAccess()), handler.ExtractTextRelations)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/extract/fabri-tag", apiKeyManageModels(apiKeyFullAccess()), handler.FabriTag)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/extract/fabri-text", apiKeyManageModels(apiKeyFullAccess()), handler.FabriText)
}

// RegisterStorageBackendRoutes manages concrete object/file storage instances.
func RegisterStorageBackendRoutes(r *gin.RouterGroup, h *handler.StorageBackendHandler, g *apiKeyGuards) {
	backends := g.apiKeyGroup(r.Group("/storage-backends"), apiKeyManageStorageBackends(apiKeyFullAccess()))
	{
		backends.GET("/types", h.Types)
		backends.POST("/test", h.TestRaw)
		backends.POST("", h.Create)
		backends.GET("", h.List)
		backends.GET("/:id", h.Get)
		backends.PUT("/:id", h.Update)
		backends.DELETE("/:id", h.Delete)
		backends.POST("/:id/test", h.TestByID)
		backends.PUT("/:id/default", h.SetDefault)
	}
}

// RegisterDataSourceRoutes 注册数据源相关的路由
func RegisterDataSourceRoutes(
	r *gin.RouterGroup,
	handler *handler.DataSourceHandler,
	credHandler *handler.DataSourceCredentialsHandler,
	g *apiKeyGuards,
) {
	ds := g.apiKeyGroup(r.Group("/datasource"), apiKeyManageDataSources(apiKeyFullAccess()))
	{
		ds.GET("/types", handler.GetAvailableConnectors)
		ds.POST("/validate-credentials", handler.ValidateCredentials)

		ds.POST("", handler.CreateDataSource)
		ds.GET("", handler.ListDataSources)
		ds.GET("/:id", handler.GetDataSource)
		ds.PUT("/:id", handler.UpdateDataSource)
		ds.DELETE("/:id", handler.DeleteDataSource)

		ds.PUT("/:id/credentials", credHandler.Put)
		ds.DELETE("/:id/credentials/:field", credHandler.DeleteField)

		ds.POST("/:id/validate", handler.ValidateConnection)
		ds.GET("/:id/resources", handler.ListAvailableResources)
		ds.POST("/:id/resource-ancestors", handler.ResolveResourceAncestors)

		ds.POST("/:id/sync", handler.ManualSync)
		ds.POST("/:id/pause", handler.PauseDataSource)
		ds.POST("/:id/resume", handler.ResumeDataSource)

		ds.GET("/:id/logs", handler.GetSyncLogs)
		ds.GET("/logs/:log_id", handler.GetSyncLog)
	}
}
