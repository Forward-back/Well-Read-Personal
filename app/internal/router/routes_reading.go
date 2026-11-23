package router

import (
	"github.com/gin-gonic/gin"

	"gaze/internal/handler"
)

// RegisterReadingRoutes mounts the interactive document reading API. These
// endpoints are human, session-scoped, read-only over KB content, and are not
// exposed to API keys (the API-key gate default-denies undeclared routes).
func RegisterReadingRoutes(r *gin.RouterGroup, h *handler.ReadingHandler, g *apiKeyGuards) {
	if h == nil {
		return
	}
	reading := r.Group("/reading")
	{
		reading.POST("/runs", h.Create)
		reading.GET("/runs/active", h.GetActive)
		reading.GET("/runs/:id", h.Get)
		reading.POST("/runs/:id/step", h.Step)
		reading.POST("/runs/:id/feedback", h.Feedback)
		reading.POST("/runs/:id/questions/:questionId/judge", h.JudgeQuestion)
		reading.POST("/runs/:id/contradictions/:contradictionId/resolve", h.ResolveContradiction)
		reading.GET("/runs/:id/report", h.GetReport)
		reading.PUT("/runs/:id/report", h.SaveReport)
		reading.DELETE("/runs/:id/report", h.DeleteReport)
		reading.GET("/runs/:id/documents", h.GetDocuments)
		reading.POST("/runs/:id/add-documents", h.AddDocuments)
		reading.POST("/runs/:id/stop", h.Stop)
		reading.POST("/runs/:id/abandon", h.Abandon)
		reading.POST("/runs/:id/resume", h.Resume)
		reading.POST("/runs/:id/jump", h.Jump)
		reading.PUT("/runs/:id/understanding", h.UpdateUnderstanding)
		reading.PUT("/runs/:id/terms/:termId", h.UpdateTerm)
		reading.PUT("/runs/:id/conclusions/:conclusionId", h.UpdateConclusion)
		reading.GET("/runs/:id/export", h.Export)
		reading.GET("/drafts", h.ListDrafts)
		reading.POST("/drafts", h.CreateDraft)
		reading.PUT("/drafts/:draftId", h.UpdateDraft)
		reading.DELETE("/drafts/:draftId", h.DeleteDraft)
	}
}
