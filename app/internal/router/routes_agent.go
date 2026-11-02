package router

import (
	"github.com/gin-gonic/gin"

	"gaze/internal/handler"
)

// RegisterSkillRoutes registers the personal skill library routes.
func RegisterSkillRoutes(
	r *gin.RouterGroup,
	skillHandler *handler.SkillHandler,
	g *apiKeyGuards,
) {
	skills := r.Group("/skills")
	{
		skills.GET("", skillHandler.ListSkills)
		skills.GET("/selectable", skillHandler.ListSelectableCatalog)
		skills.GET("/catalog", skillHandler.ListCatalog)
	}
	catalogWrite := g.apiKeyGroup(r.Group("/skills/catalog"), apiKeyFullAccess())
	{
		catalogWrite.POST("", skillHandler.RegisterCatalog)
		catalogWrite.POST("/:id/install", skillHandler.InstallCatalog)
		catalogWrite.GET("/:id/files", skillHandler.ListCatalogFiles)
		catalogWrite.GET("/:id/files/content", skillHandler.GetCatalogFile)
		catalogWrite.PUT("/:id/enabled", skillHandler.SetCatalogEnabled)
		catalogWrite.PUT("/:id", skillHandler.UpdateCatalog)
		catalogWrite.DELETE("/:id", skillHandler.DeleteCatalog)
	}
}
