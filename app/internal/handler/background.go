package handler

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"gaze/internal/application/service"
	"gaze/internal/errors"
	"gaze/internal/logger"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

// BackgroundHandler exposes the owner's shell background settings. All routes
// are owner-session only; no API-key route declares them, so the gate denies
// MCP keys.
type BackgroundHandler struct {
	backgroundService interfaces.BackgroundService
}

func NewBackgroundHandler(backgroundService interfaces.BackgroundService) *BackgroundHandler {
	return &BackgroundHandler{backgroundService: backgroundService}
}

func (h *BackgroundHandler) currentUserID(c *gin.Context) (string, bool) {
	user, ok := c.Request.Context().Value(types.UserContextKey).(*types.User)
	if !ok || user == nil {
		c.Error(errors.NewUnauthorizedError("Failed to get user information"))
		return "", false
	}
	return user.ID, true
}

// ListBackgrounds godoc
// @Summary      获取当前用户的背景列表
// @Description  返回当前选中的背景（default / builtin:N / upload:<id>）与已上传背景元数据，不含文件内容。
// @Tags         背景
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "背景列表"
// @Failure      401  {object}  errors.AppError         "未登录"
// @Failure      500  {object}  errors.AppError         "读取失败"
// @Security     Bearer
// @Router       /auth/me/backgrounds [get]
func (h *BackgroundHandler) ListBackgrounds(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		return
	}
	result, err := h.backgroundService.ListBackgrounds(c.Request.Context(), userID)
	if err != nil {
		logger.Errorf(c.Request.Context(), "Failed to list backgrounds: %v", err)
		c.Error(errors.NewInternalServerError("Failed to list backgrounds").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// UploadBackground godoc
// @Summary      上传背景图片
// @Description  原图上传，支持 JPEG/PNG/WebP，单张不超过 10MB，最多 10 张；上传成功后自动选中该背景。
// @Tags         背景
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "背景图片"
// @Success      200  {object}  map[string]interface{}  "上传结果"
// @Failure      400  {object}  errors.AppError         "格式、大小或数量不合法"
// @Failure      401  {object}  errors.AppError         "未登录"
// @Security     Bearer
// @Router       /auth/me/backgrounds [post]
func (h *BackgroundHandler) UploadBackground(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxBackgroundUploadBytes+(1<<20))
	file, err := c.FormFile("file")
	if err != nil {
		c.Error(errors.NewValidationError("Background file is required").WithDetails(err.Error()))
		return
	}
	bg, err := h.backgroundService.UploadBackground(c.Request.Context(), userID, file)
	if err != nil {
		logger.Warnf(c.Request.Context(), "Background upload rejected: %v", err)
		c.Error(backgroundError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"background": bg,
			"choice":     "upload:" + bg.ID,
		},
	})
}

// DeleteBackground godoc
// @Summary      删除上传的背景
// @Description  删除指定上传背景及其文件；若该背景正在使用，则回退到默认草地背景。
// @Tags         背景
// @Produce      json
// @Param        id  path  string  true  "背景 ID"
// @Success      200  {object}  map[string]interface{}  "删除成功"
// @Failure      404  {object}  errors.AppError         "背景不存在"
// @Security     Bearer
// @Router       /auth/me/backgrounds/{id} [delete]
func (h *BackgroundHandler) DeleteBackground(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		return
	}
	if err := h.backgroundService.DeleteBackground(c.Request.Context(), userID, c.Param("id")); err != nil {
		if stderrors.Is(err, service.ErrBackgroundNotFound) {
			c.Error(errors.NewNotFoundError("Background not found"))
			return
		}
		logger.Errorf(c.Request.Context(), "Failed to delete background: %v", err)
		c.Error(errors.NewInternalServerError("Failed to delete background").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// SetBackgroundChoice godoc
// @Summary      选择背景
// @Description  保存选中的背景：default、builtin:1..5 或 upload:<id>。
// @Tags         背景
// @Accept       json
// @Produce      json
// @Param        request  body  object{choice=string}  true  "背景选择"
// @Success      200  {object}  map[string]interface{}  "保存成功"
// @Failure      400  {object}  errors.AppError         "选择不合法"
// @Security     Bearer
// @Router       /auth/me/background [put]
func (h *BackgroundHandler) SetBackgroundChoice(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		return
	}
	var req struct {
		Choice string `json:"choice" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid background choice").WithDetails(err.Error()))
		return
	}
	if err := h.backgroundService.SetBackgroundChoice(c.Request.Context(), userID, req.Choice); err != nil {
		if stderrors.Is(err, service.ErrBackgroundChoice) {
			c.Error(errors.NewValidationError("Invalid background choice"))
			return
		}
		logger.Errorf(c.Request.Context(), "Failed to set background choice: %v", err)
		c.Error(errors.NewInternalServerError("Failed to set background choice").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"choice": strings.TrimSpace(req.Choice)}})
}

// GetBackgroundFile godoc
// @Summary      读取上传的背景文件
// @Description  需要 Bearer 登录；SVG 不支持，仅返回 JPEG/PNG/WebP 原图。
// @Tags         背景
// @Produce      image/jpeg
// @Param        id  path  string  true  "背景 ID"
// @Success      200  {file}  binary  "图片内容"
// @Failure      404  {object}  errors.AppError  "背景不存在"
// @Security     Bearer
// @Router       /auth/me/backgrounds/{id}/file [get]
func (h *BackgroundHandler) GetBackgroundFile(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		return
	}
	reader, bg, err := h.backgroundService.OpenBackground(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		if stderrors.Is(err, service.ErrBackgroundNotFound) {
			c.Error(errors.NewNotFoundError("Background not found"))
			return
		}
		logger.Errorf(c.Request.Context(), "Failed to open background: %v", err)
		c.Error(errors.NewInternalServerError("Failed to read background"))
		return
	}
	defer reader.Close()

	c.Header("Cache-Control", "private, max-age=86400")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", bg.DisplayName))
	c.DataFromReader(http.StatusOK, bg.SizeBytes, bg.MimeType, reader, nil)
}

func backgroundError(err error) error {
	switch {
	case stderrors.Is(err, service.ErrBackgroundTooLarge):
		return errors.NewValidationError("Background image exceeds 10 MB")
	case stderrors.Is(err, service.ErrBackgroundType):
		return errors.NewValidationError("Background must be a JPEG, PNG or WebP image")
	case stderrors.Is(err, service.ErrBackgroundLimit):
		return errors.NewValidationError("At most 10 uploaded backgrounds are allowed")
	default:
		return errors.NewInternalServerError("Failed to upload background").WithDetails(err.Error())
	}
}
