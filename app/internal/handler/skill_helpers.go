package handler

import (
	"context"
	stderrors "errors"

	"github.com/gin-gonic/gin"

	"gaze/internal/application/service"
	apperrors "gaze/internal/errors"
	"gaze/internal/types"
)

type sandboxConfigService interface {
	Get(ctx context.Context, tenantID uint64, id string) (*types.TenantSandboxConfigEntity, error)
}

type skillSourceRequest struct {
	Source       string `json:"source"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Version      string `json:"version"`
	Instructions string `json:"instructions"`
}

func sandboxConfigTenantID(c *gin.Context) uint64 {
	return c.GetUint64(types.TenantIDContextKey.String())
}

func skillTooLargeError() error {
	return apperrors.NewBadRequestError("skill bundle is too large")
}

func skillSourceRequestTooLargeError() error {
	return apperrors.NewBadRequestError("skill source request is too large")
}

func skillJSONRequestTooLargeError() error {
	return apperrors.NewBadRequestError("skill request is too large")
}

func respondSkillServiceError(c *gin.Context, err error) {
	if stderrors.Is(err, service.ErrSkillBundleInvalid) {
		_ = c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	_ = c.Error(err)
}
