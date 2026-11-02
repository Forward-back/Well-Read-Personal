package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gaze/internal/handler"
)

// RegisterChunkerDebugRoutes wires the read-only chunker preview endpoint
// used by the KB editor's debug panel. Stateless — uses no service deps.
func RegisterChunkerDebugRoutes(r *gin.RouterGroup, g *apiKeyGuards) {
	g.apiKeyRoute(r, http.MethodPost, "/chunker/preview", apiKeyRetrieve(apiKeyIngest(apiKeyFullAccess())), handler.PreviewChunking)
}

// RegisterChunkRoutes 注册分块相关的路由
func RegisterChunkRoutes(r *gin.RouterGroup, handler *handler.ChunkHandler, g *apiKeyGuards) {
	chunks := g.apiKeyGroup(r.Group("/chunks"), apiKeyIngest(apiKeyFullAccess()))
	chunkRead := chunks.With(apiKeyRetrieve(apiKeyFullAccess()))
	{
		chunkRead.GET("/:knowledge_id", handler.ListKnowledgeChunks)
		chunkRead.GET("/by-id/:id", handler.GetChunkByIDOnly)
		chunkRead.GET("/:knowledge_id/:id/revisions", handler.ListChunkRevisions)
		chunks.DELETE("/:knowledge_id/:id", handler.DeleteChunk)
		chunks.DELETE("/:knowledge_id", handler.DeleteChunksByKnowledgeID)
		chunks.PUT("/:knowledge_id/:id", handler.UpdateChunk)
		chunks.POST("/:knowledge_id/:id/revert", handler.RevertChunk)
		chunks.DELETE("/by-id/:id/questions", handler.DeleteGeneratedQuestion)
		chunks.PUT("/by-id/:id/questions", handler.UpsertGeneratedQuestion)
		chunks.POST("/by-id/:id/questions/regenerate", handler.RegenerateGeneratedQuestions)
	}
}

// RegisterKnowledgeRoutes 注册知识相关的路由
func RegisterKnowledgeRoutes(r *gin.RouterGroup, handler *handler.KnowledgeHandler, g *apiKeyGuards) {
	kb := g.apiKeyGroup(r.Group("/knowledge-bases/:id/knowledge"), apiKeyIngest(apiKeyFullAccess()))
	kbRead := kb.With(apiKeyRetrieve(apiKeyFullAccess()))
	{
		kb.POST("/file", handler.CreateKnowledgeFromFile)
		kb.POST("/url", handler.CreateKnowledgeFromURL)
		kb.POST("/manual", handler.CreateManualKnowledge)
		kbRead.GET("", handler.ListKnowledge)
		kbRead.GET("/folders", handler.ListKnowledgeFolders)
		kb.POST("/folders", handler.CreateKnowledgeFolder)
		kb.PUT("/folders/:folder_id", handler.UpdateKnowledgeFolder)
		kb.DELETE("/folders/:folder_id", handler.DeleteKnowledgeFolder)
		kb.With(apiKeyFullAccess()).DELETE("", handler.ClearKnowledgeBaseContents)
	}

	kgrp := r.Group("/knowledge")
	k := g.apiKeyGroup(kgrp, apiKeyIngest(apiKeyFullAccess()))
	kRead := k.With(apiKeyRetrieve(apiKeyFullAccess()))
	{
		kRead.GET("/batch", handler.GetKnowledgeBatch)
		kRead.GET("/:id", handler.GetKnowledge)
		kRead.GET("/:id/stages", handler.GetKnowledgeSpans)
		kRead.GET("/:id/spans", handler.GetKnowledgeSpans)
		k.DELETE("/:id", handler.DeleteKnowledge)
		k.PUT("/:id", handler.UpdateKnowledge)
		k.POST("/:id/regenerate-summary", handler.RegenerateKnowledgeSummary)
		k.PUT("/manual/:id", handler.UpdateManualKnowledge)
		k.POST("/:id/reparse", handler.ReparseKnowledge)
		k.POST("/:id/cancel-parse", handler.CancelKnowledgeParse)
		kRead.GET("/:id/download", handler.DownloadKnowledgeFile)
		kRead.GET("/:id/preview", handler.PreviewKnowledgeFile)
		k.PUT("/image/:id/:chunk_id", handler.UpdateImageInfo)
		kRead.GET("/search", handler.SearchKnowledge)
		kRead.GET("/move/progress/:task_id", handler.GetKnowledgeMoveProgress)
		k.PUT("/tags", handler.UpdateKnowledgeTagBatch)
		k.POST("/batch-reparse", handler.BatchReparseKnowledge)
		k.POST("/batch-delete", handler.BatchDeleteKnowledge)
		k.POST("/folder", handler.MoveKnowledgeToFolder)
		k.POST("/move", handler.MoveKnowledge)
	}
}

// RegisterKnowledgeBaseRoutes 注册知识库相关的路由
func RegisterKnowledgeBaseRoutes(r *gin.RouterGroup, handler *handler.KnowledgeBaseHandler, g *apiKeyGuards) {
	kbgrp := r.Group("/knowledge-bases")
	kb := g.apiKeyGroup(kbgrp, apiKeyRetrieve(apiKeyFullAccess()))
	kbManagement := kb.With(apiKeyManageKnowledgeBases(apiKeyFullAccess()))
	{
		kbManagement.POST("", handler.CreateKnowledgeBase)
		kb.GET("", handler.ListKnowledgeBases)
		kb.GET("/:id", handler.GetKnowledgeBase)
		kbManagement.PUT("/:id", handler.UpdateKnowledgeBase)
		kbManagement.DELETE("/:id", handler.DeleteKnowledgeBase)
		kb.PUT("/:id/pin", handler.TogglePinKnowledgeBase)
		kb.POST("/:id/hybrid-search", handler.HybridSearch)
		kb.GET("/:id/hybrid-search", handler.HybridSearch)
		kbManagement.POST("/copy", handler.CopyKnowledgeBase)
		kbManagement.POST("/:id/duplicate", handler.DuplicateKnowledgeBase)
		kb.With(apiKeyRetrieve(apiKeyManageKnowledgeBases(apiKeyFullAccess()))).
			GET("/copy/progress/:task_id", handler.GetKBCloneProgress)
		kb.GET("/:id/move-targets", handler.ListMoveTargets)
	}
}

// RegisterKnowledgeTagRoutes 注册知识库标签相关路由。
func RegisterKnowledgeTagRoutes(r *gin.RouterGroup, tagHandler *handler.TagHandler, g *apiKeyGuards) {
	if tagHandler == nil {
		return
	}
	kbTags := g.apiKeyGroup(r.Group("/knowledge-bases/:id/tags"), apiKeyIngest(apiKeyFullAccess()))
	kbTagsRead := kbTags.With(apiKeyRetrieve(apiKeyFullAccess()))
	{
		kbTagsRead.GET("", tagHandler.ListTags)
		kbTags.POST("", tagHandler.CreateTag)
		kbTags.PUT("/:tag_id", tagHandler.UpdateTag)
		kbTags.DELETE("/:tag_id", tagHandler.DeleteTag)
	}
}

// RegisterGazePageRoutes registers gaze page related routes.
func RegisterGazePageRoutes(r *gin.RouterGroup, gazeHandler *handler.GazePageHandler, g *apiKeyGuards) {
	gaze := g.apiKeyGroup(r.Group("/knowledgebase/:kb_id/gaze"), apiKeyIngest(apiKeyFullAccess()))
	gazeRead := gaze.With(apiKeyRetrieve(apiKeyFullAccess()))
	{
		gazeRead.GET("/pages", gazeHandler.ListPages)
		gaze.POST("/pages", gazeHandler.CreatePage)
		gaze.PUT("/move-page", gazeHandler.MovePage)
		gazeRead.GET("/pages/*slug", gazeHandler.GetPage)
		gaze.PUT("/pages/*slug", gazeHandler.UpdatePage)
		gaze.DELETE("/pages/*slug", gazeHandler.DeletePage)

		gazeRead.GET("/revisions/*slug", gazeHandler.ListRevisions)
		gaze.POST("/revert", gazeHandler.RevertPage)

		gazeRead.GET("/folders", gazeHandler.ListFolders)
		gaze.POST("/folders", gazeHandler.CreateFolder)
		gaze.PUT("/folders/:folder_id", gazeHandler.UpdateFolder)
		gaze.DELETE("/folders/:folder_id", gazeHandler.DeleteFolder)

		gazeRead.GET("/index", gazeHandler.GetIndex)
		gazeRead.GET("/graph", gazeHandler.GetGraph)
		gazeRead.GET("/stats", gazeHandler.GetStats)
		gazeRead.GET("/search", gazeHandler.SearchPages)
		gaze.POST("/rebuild-links", gazeHandler.RebuildLinks)
		gaze.POST("/outdated-sync", gazeHandler.SyncOutdated)
	}
}
