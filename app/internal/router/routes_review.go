package router

import (
	"github.com/gin-gonic/gin"

	"gaze/internal/handler"
)

// RegisterReviewRoutes mounts the personal spaced-repetition API. These
// endpoints are JWT-only; the API-key gate default-denies undeclared routes.
func RegisterReviewRoutes(r *gin.RouterGroup, h *handler.ReviewHandler) {
	if h == nil {
		return
	}
	review := r.Group("/review")
	{
		review.GET("/groups", h.ListGroups)
		review.POST("/groups", h.CreateGroup)
		review.PUT("/groups/:id", h.UpdateGroup)
		review.POST("/groups/:id/move", h.MoveGroup)
		review.DELETE("/groups/:id", h.DeleteGroup)

		review.GET("/cards", h.ListCards)
		review.GET("/cards/:id/performance", h.GetCardPerformance)
		review.POST("/cards/merge", h.MergeCards)
		review.POST("/cards/batch", h.BatchCards)
		review.POST("/cards", h.CreateCard)
		review.GET("/cards/:id", h.GetCard)
		review.PUT("/cards/:id", h.UpdateCard)
		review.POST("/cards/:id/move", h.MoveCard)
		review.POST("/cards/:id/reset", h.ResetCardSchedule)
		review.POST("/cards/:id/status", h.SetCardStatus)
		review.DELETE("/cards/:id", h.DeleteCard)
		review.POST("/cards/:id/restore", h.RestoreCard)
		review.POST("/cards/:id/sources/:source_id/unavailable", h.MarkSourceUnavailable)
		review.PUT("/cards/:id/sources/:source_id", h.RebindSource)

		review.GET("/heatmap", h.Heatmap)
		review.GET("/clusters/detail", h.ClusterDetail)

		review.GET("/queue", h.TodayQueue)
		review.POST("/sessions", h.StartSession)
		review.GET("/sessions", h.ListSessions)
		review.GET("/sessions/active", h.GetActiveSession)
		review.GET("/sessions/:id", h.GetSession)
		review.POST("/sessions/:id/abandon", h.AbandonSession)
		review.POST("/reviews", h.SubmitReview)
		review.GET("/stats", h.Stats)
		review.POST("/generations/preview", h.PreviewGeneration)
		review.POST("/generations/import", h.ImportGeneratedCards)
		review.GET("/export", h.Export)
	}
}
