package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apperrors "gaze/internal/errors"
	"gaze/internal/logger"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

// WebSearchRuntimeHandler exposes provider management for the personal edition.
// Direct chat uses tenant KV and never reads these entities; Agents reference
// them by ID.
type WebSearchRuntimeHandler struct {
	repo      interfaces.WebSearchProviderRepository
	service   interfaces.WebSearchProviderService
	search    interfaces.WebSearchService
	agentRepo interfaces.CustomAgentRepository
}

func NewWebSearchRuntimeHandler(
	repo interfaces.WebSearchProviderRepository,
	service interfaces.WebSearchProviderService,
	search interfaces.WebSearchService,
	agentRepo interfaces.CustomAgentRepository,
) *WebSearchRuntimeHandler {
	return &WebSearchRuntimeHandler{repo: repo, service: service, search: search, agentRepo: agentRepo}
}

func isPersonalWebSearchProvider(provider types.WebSearchProviderType) bool {
	switch provider {
	case types.WebSearchProviderTypeDeepSeek, types.WebSearchProviderTypeDuckDuckGo:
		return true
	default:
		return false
	}
}

type webSearchProviderView struct {
	ID          string                          `json:"id"`
	Name        string                          `json:"name"`
	Provider    types.WebSearchProviderType     `json:"provider"`
	Description string                          `json:"description,omitempty"`
	Parameters  webSearchProviderParametersView `json:"parameters"`
	IsDefault   bool                            `json:"is_default"`
	Credentials map[string]credentialView       `json:"credentials"`
	CreatedAt   any                             `json:"created_at,omitempty"`
	UpdatedAt   any                             `json:"updated_at,omitempty"`
}

type webSearchProviderParametersView struct {
	EngineID    string            `json:"engine_id,omitempty"`
	BaseURL     string            `json:"base_url,omitempty"`
	ProxyURL    string            `json:"proxy_url,omitempty"`
	ExtraConfig map[string]string `json:"extra_config,omitempty"`
}

type credentialView struct {
	Configured bool `json:"configured"`
}

func (h *WebSearchRuntimeHandler) providerView(p *types.WebSearchProviderEntity) webSearchProviderView {
	return webSearchProviderView{
		ID:          p.ID,
		Name:        p.Name,
		Provider:    p.Provider,
		Description: p.Description,
		Parameters: webSearchProviderParametersView{
			EngineID:    p.Parameters.EngineID,
			BaseURL:     p.Parameters.BaseURL,
			ProxyURL:    p.Parameters.ProxyURL,
			ExtraConfig: p.Parameters.ExtraConfig,
		},
		IsDefault:   p.IsDefault,
		Credentials: map[string]credentialView{"api_key": {Configured: p.Parameters.APIKey != ""}},
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func (h *WebSearchRuntimeHandler) ListProviders(c *gin.Context) {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	providers, err := h.repo.List(c.Request.Context(), tenantID)
	if err != nil {
		_ = c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	out := make([]webSearchProviderView, 0, len(providers))
	for _, p := range providers {
		// Personal edition exposes only the two providers the product supports.
		// Legacy bing/google rows still resolve for Agents that already point at
		// them, but they are not offered in the settings surface.
		if !isPersonalWebSearchProvider(p.Provider) {
			continue
		}
		out = append(out, h.providerView(p))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": out})
}

func (h *WebSearchRuntimeHandler) ListProviderTypes(c *gin.Context) {
	typesOut := make([]types.WebSearchProviderTypeInfo, 0, 2)
	for _, info := range types.GetWebSearchProviderTypes() {
		if isPersonalWebSearchProvider(types.WebSearchProviderType(info.ID)) {
			typesOut = append(typesOut, info)
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": typesOut})
}

type webSearchProviderWriteRequest struct {
	Name        string                             `json:"name"`
	Provider    types.WebSearchProviderType        `json:"provider"`
	Description string                             `json:"description"`
	Parameters  *types.WebSearchProviderParameters `json:"parameters"`
	IsDefault   bool                               `json:"is_default"`
}

func (h *WebSearchRuntimeHandler) CreateProvider(c *gin.Context) {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	var req webSearchProviderWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewBadRequestError("请求参数格式错误").WithDetails(err.Error()))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		_ = c.Error(apperrors.NewValidationError("请填写配置名称"))
		return
	}
	if !isPersonalWebSearchProvider(req.Provider) {
		_ = c.Error(apperrors.NewValidationError("仅支持 DeepSeek 与 DuckDuckGo"))
		return
	}
	entity := &types.WebSearchProviderEntity{
		TenantID:    tenantID,
		Name:        name,
		Provider:    req.Provider,
		Description: strings.TrimSpace(req.Description),
		IsDefault:   req.IsDefault,
	}
	if req.Parameters != nil {
		entity.Parameters = types.WebSearchProviderParameters{
			APIKey:      strings.TrimSpace(req.Parameters.APIKey),
			EngineID:    strings.TrimSpace(req.Parameters.EngineID),
			BaseURL:     strings.TrimSpace(req.Parameters.BaseURL),
			ProxyURL:    strings.TrimSpace(req.Parameters.ProxyURL),
			ExtraConfig: req.Parameters.ExtraConfig,
		}
	}
	if err := h.service.CreateProvider(c.Request.Context(), entity); err != nil {
		_ = c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.providerView(entity)})
}

func (h *WebSearchRuntimeHandler) UpdateProvider(c *gin.Context) {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	existing, ok := h.ownedProvider(c, tenantID, c.Param("id"))
	if !ok {
		return
	}
	var req webSearchProviderWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewBadRequestError("请求参数格式错误").WithDetails(err.Error()))
		return
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		existing.Name = name
	}
	existing.Description = strings.TrimSpace(req.Description)
	if req.Provider != "" {
		if !isPersonalWebSearchProvider(req.Provider) {
			_ = c.Error(apperrors.NewValidationError("仅支持 DeepSeek 与 DuckDuckGo"))
			return
		}
		existing.Provider = req.Provider
	}
	if req.Parameters != nil {
		existing.Parameters.EngineID = strings.TrimSpace(req.Parameters.EngineID)
		existing.Parameters.BaseURL = strings.TrimSpace(req.Parameters.BaseURL)
		existing.Parameters.ProxyURL = strings.TrimSpace(req.Parameters.ProxyURL)
		existing.Parameters.ExtraConfig = req.Parameters.ExtraConfig
	}
	existing.IsDefault = req.IsDefault
	if err := h.service.UpdateProvider(c.Request.Context(), existing); err != nil {
		_ = c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.providerView(existing)})
}

func (h *WebSearchRuntimeHandler) DeleteProvider(c *gin.Context) {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	id := c.Param("id")
	if _, ok := h.ownedProvider(c, tenantID, id); !ok {
		return
	}
	if names := h.agentsUsingProvider(c, tenantID, id); len(names) > 0 {
		_ = c.Error(apperrors.NewConflictError(
			"该配置正在被 Agent 使用：" + strings.Join(names, "、")))
		return
	}
	if err := h.service.DeleteProvider(c.Request.Context(), tenantID, id); err != nil {
		_ = c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type webSearchCredentialRequest struct {
	APIKey *string `json:"api_key"`
}

func (h *WebSearchRuntimeHandler) UpdateCredentials(c *gin.Context) {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if _, ok := h.ownedProvider(c, tenantID, c.Param("id")); !ok {
		return
	}
	var req webSearchCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewBadRequestError("请求参数格式错误").WithDetails(err.Error()))
		return
	}
	entity, err := h.service.UpdateProviderCredentials(c.Request.Context(), tenantID, c.Param("id"), req.APIKey)
	if err != nil {
		_ = c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.providerView(entity)})
}

func (h *WebSearchRuntimeHandler) ClearCredential(c *gin.Context) {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	field := c.Param("field")
	if _, ok := h.ownedProvider(c, tenantID, c.Param("id")); !ok {
		return
	}
	if err := h.service.ClearProviderCredential(c.Request.Context(), tenantID, c.Param("id"), field); err != nil {
		_ = c.Error(apperrors.NewValidationError(err.Error()))
		return
	}
	entity, err := h.repo.GetByID(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil || entity == nil {
		_ = c.Error(apperrors.NewInternalServerError("读取配置失败"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.providerView(entity)})
}

func (h *WebSearchRuntimeHandler) TestProvider(c *gin.Context) {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	entity, ok := h.ownedProvider(c, tenantID, c.Param("id"))
	if !ok {
		return
	}
	if h.search == nil {
		_ = c.Error(apperrors.NewInternalServerError("联网搜索服务未配置"))
		return
	}
	results, err := h.search.Search(c.Request.Context(), entity.ID, &types.WebSearchConfig{
		Provider:   string(entity.Provider),
		MaxResults: 3,
	}, "WellRead connectivity test")
	if err != nil {
		_ = c.Error(apperrors.NewBadRequestError("连接测试失败").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"results": len(results)}})
}

func (h *WebSearchRuntimeHandler) ownedProvider(
	c *gin.Context, tenantID uint64, id string,
) (*types.WebSearchProviderEntity, bool) {
	if strings.TrimSpace(id) == "" {
		_ = c.Error(apperrors.NewBadRequestError("缺少配置 ID"))
		return nil, false
	}
	entity, err := h.repo.GetByID(c.Request.Context(), tenantID, id)
	if err != nil {
		_ = c.Error(apperrors.NewInternalServerError(err.Error()))
		return nil, false
	}
	if entity == nil {
		_ = c.Error(apperrors.NewNotFoundError("联网搜索配置不存在"))
		return nil, false
	}
	return entity, true
}

func (h *WebSearchRuntimeHandler) agentsUsingProvider(
	c *gin.Context, tenantID uint64, providerID string,
) []string {
	if h.agentRepo == nil {
		return nil
	}
	agents, err := h.agentRepo.ListAgentsByTenantID(c.Request.Context(), tenantID)
	if err != nil {
		logger.Warnf(c.Request.Context(), "list agents for provider delete guard failed: %v", err)
		return nil
	}
	names := make([]string, 0)
	for _, agent := range agents {
		if agent != nil && agent.Config.WebSearchProviderID == providerID {
			names = append(names, agent.Name)
		}
	}
	return names
}
