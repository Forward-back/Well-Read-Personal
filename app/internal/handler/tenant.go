package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"gaze/internal/config"
	"gaze/internal/errors"
	"gaze/internal/logger"
	"gaze/internal/models/chat"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	secutils "gaze/internal/utils"
)

// SettingsHandler exposes the personal edition's settings surface. Tenant
// storage remains the single internal workspace that owns models, knowledge
// bases and configuration; callers never see or switch workspaces.
type SettingsHandler struct {
	service       interfaces.TenantService
	apiKeyService interfaces.TenantAPIKeyService
	kbService     interfaces.KnowledgeBaseService
	config        *config.Config
	modelService  interfaces.ModelService
}

func NewSettingsHandler(
	service interfaces.TenantService,
	apiKeyService interfaces.TenantAPIKeyService,
	kbService interfaces.KnowledgeBaseService,
	config *config.Config,
	modelService interfaces.ModelService,
) *SettingsHandler {
	return &SettingsHandler{
		service:       service,
		apiKeyService: apiKeyService,
		kbService:     kbService,
		config:        config,
		modelService:  modelService,
	}
}

// personalAPIKeyName is the fixed label of the single machine credential. The
// personal edition has exactly one key and never exposes naming, scopes, KB
// allow-lists or capabilities.
const personalAPIKeyName = "MCP 只读密钥"

type personalAPIKeyResponse struct {
	ID         uint64     `json:"id"`
	Name       string     `json:"name"`
	MaskedKey  string     `json:"masked_key"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type personalAPIKeyCreateResponse struct {
	personalAPIKeyResponse
	Token string `json:"token"`
}

// GetAPIKey returns the single active machine credential, masked. When no key
// exists the response carries exists=false and a null key.
func (h *SettingsHandler) GetAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, err := h.currentTenantID(c)
	if err != nil {
		c.Error(err)
		return
	}
	key, err := h.activePersonalAPIKey(ctx, tenantID)
	if err != nil {
		c.Error(errors.NewInternalServerError("Failed to load API key").WithDetails(err.Error()))
		return
	}
	if key == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"exists": false}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"exists": true,
		"key":    personalAPIKeyForResponse(key),
	}})
}

// CreateAPIKey generates the single read-only key. Creating a second key is
// rejected; callers must reset the existing one instead.
func (h *SettingsHandler) CreateAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, err := h.currentTenantID(c)
	if err != nil {
		c.Error(err)
		return
	}
	existing, err := h.activePersonalAPIKey(ctx, tenantID)
	if err != nil {
		c.Error(errors.NewInternalServerError("Failed to load API key").WithDetails(err.Error()))
		return
	}
	if existing != nil {
		c.Error(errors.NewConflictError("An API key already exists; reset it instead"))
		return
	}
	result, err := h.apiKeyService.CreateAPIKey(ctx, interfaces.TenantAPIKeyCreateRequest{
		TenantID: tenantID,
		Name:     personalAPIKeyName,
		Preset:   types.APIKeyPresetRead,
	})
	if err != nil {
		c.Error(errors.NewInternalServerError("Failed to create API key").WithDetails(err.Error()))
		return
	}
	response := personalAPIKeyForResponse(result.APIKey)
	response.MaskedKey = maskAPIKey(result.Token)
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": personalAPIKeyCreateResponse{
			personalAPIKeyResponse: response,
			Token:                  result.Token,
		},
	})
}

// ResetAPIKey revokes every active key and issues a fresh read-only one.
func (h *SettingsHandler) ResetAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, err := h.currentTenantID(c)
	if err != nil {
		c.Error(err)
		return
	}
	if err := h.revokeAllPersonalAPIKeys(ctx, tenantID); err != nil {
		c.Error(errors.NewInternalServerError("Failed to revoke API key").WithDetails(err.Error()))
		return
	}
	result, err := h.apiKeyService.CreateAPIKey(ctx, interfaces.TenantAPIKeyCreateRequest{
		TenantID: tenantID,
		Name:     personalAPIKeyName,
		Preset:   types.APIKeyPresetRead,
	})
	if err != nil {
		c.Error(errors.NewInternalServerError("Failed to create API key").WithDetails(err.Error()))
		return
	}
	response := personalAPIKeyForResponse(result.APIKey)
	response.MaskedKey = maskAPIKey(result.Token)
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": personalAPIKeyCreateResponse{
			personalAPIKeyResponse: response,
			Token:                  result.Token,
		},
	})
}

// DeleteAPIKey revokes every active machine credential.
func (h *SettingsHandler) DeleteAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, err := h.currentTenantID(c)
	if err != nil {
		c.Error(err)
		return
	}
	if err := h.revokeAllPersonalAPIKeys(ctx, tenantID); err != nil {
		c.Error(errors.NewInternalServerError("Failed to revoke API key").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *SettingsHandler) currentTenantID(c *gin.Context) (uint64, error) {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		return 0, errors.NewUnauthorizedError("Authentication required")
	}
	return tenantID, nil
}

func (h *SettingsHandler) activePersonalAPIKey(ctx context.Context, tenantID uint64) (*types.TenantAPIKey, error) {
	keys, err := h.apiKeyService.ListAPIKeys(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, nil
	}
	return keys[0], nil
}

func (h *SettingsHandler) revokeAllPersonalAPIKeys(ctx context.Context, tenantID uint64) error {
	keys, err := h.apiKeyService.ListAPIKeys(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if key == nil {
			continue
		}
		if err := h.apiKeyService.RevokeAPIKey(ctx, tenantID, key.ID); err != nil {
			return err
		}
	}
	return nil
}

func personalAPIKeyForResponse(key *types.TenantAPIKey) personalAPIKeyResponse {
	if key == nil {
		return personalAPIKeyResponse{}
	}
	return personalAPIKeyResponse{
		ID:         key.ID,
		Name:       key.Name,
		MaskedKey:  maskAPIKey(key.APIKey),
		LastUsedAt: key.LastUsedAt,
		CreatedAt:  key.CreatedAt,
	}
}

// maskAPIKey keeps a short prefix/suffix so the owner can identify the key
// without exposing a usable credential.
func maskAPIKey(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) <= 12 {
		return strings.Repeat("*", len(token))
	}
	return token[:6] + "..." + token[len(token)-4:]
}

// GetTenantKV returns a settings partition by key.
func (h *SettingsHandler) GetTenantKV(c *gin.Context) {
	key := secutils.SanitizeForLog(c.Param("key"))

	switch key {
	case "web-search-config":
		h.GetTenantWebSearchConfig(c)
	case "prompt-templates":
		h.GetPromptTemplates(c)
	case "parser-engine-config":
		h.GetTenantParserEngineConfig(c)
	case "storage-engine-config":
		h.GetTenantStorageEngineConfig(c)
	case "chat-history-config":
		h.GetTenantChatHistoryConfig(c)
	case "retrieval-config":
		h.GetTenantRetrievalConfig(c)
	case "memory-config":
		h.GetTenantMemoryConfig(c)
	case "reading-config":
		tenant, _ := types.TenantInfoFromContext(c.Request.Context())
		if tenant == nil {
			c.Error(errors.NewInternalServerError("Workspace is empty"))
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"reading_model_id": tenant.ReadingModelID,
		}})
	case "direct-config":
		tenant, _ := types.TenantInfoFromContext(c.Request.Context())
		if tenant == nil {
			c.Error(errors.NewInternalServerError("Workspace is empty"))
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"direct_model_id": tenant.DirectModelID,
		}})
	default:
		logger.Info(c.Request.Context(), "KV key not supported", "key", key)
		c.Error(errors.NewBadRequestError("unsupported key"))
	}
}

// UpdateTenantKV updates a settings partition by key.
func (h *SettingsHandler) UpdateTenantKV(c *gin.Context) {
	key := secutils.SanitizeForLog(c.Param("key"))

	switch key {
	case "prompt-templates":
		h.updatePromptTemplates(c)
	case "web-search-config":
		h.updateTenantWebSearchConfigInternal(c)
	case "parser-engine-config":
		h.updateTenantParserEngineConfigInternal(c)
	case "storage-engine-config":
		h.updateTenantStorageEngineConfigInternal(c)
	case "chat-history-config":
		h.updateTenantChatHistoryConfigInternal(c)
	case "retrieval-config":
		h.updateTenantRetrievalConfigInternal(c)
	case "memory-config":
		h.updateTenantMemoryConfigInternal(c)
	case "reading-config":
		h.updateTenantReadingConfigInternal(c)
	case "direct-config":
		h.updateTenantDirectConfigInternal(c)
	default:
		logger.Info(c.Request.Context(), "KV key not supported", "key", key)
		c.Error(errors.NewBadRequestError("unsupported key"))
	}
}

func (h *SettingsHandler) updateTenantWebSearchConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()

	var cfg types.WebSearchConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	cfg = *types.MergeWebSearchConfigForUpdate(&cfg, tenant.WebSearchConfig)

	if cfg.MaxResults < 1 || cfg.MaxResults > 50 {
		c.Error(errors.NewBadRequestError("max_results must be between 1 and 50"))
		return
	}

	tenant.WebSearchConfig = &cfg
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to update workspace: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update workspace web search config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    h.webSearchConfigForResponse(ctx, updatedTenant),
		"message": "Web search configuration updated successfully",
	})
}

func (h *SettingsHandler) GetTenantWebSearchConfig(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start getting tenant web search config")
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	logger.Infof(ctx, "Tenant web search config retrieved successfully, Tenant ID: %d", tenant.ID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    h.webSearchConfigForResponse(ctx, tenant),
	})
}

// webSearchConfigForResponse adds direct-mode fallback availability so the
// composer can show the tenant-KV web search toggle without provider entities.
func (h *SettingsHandler) webSearchConfigForResponse(ctx context.Context, tenant *types.Tenant) *types.WebSearchConfig {
	out := types.WebSearchConfigForResponse(tenant.WebSearchConfig, true)
	if out == nil {
		return nil
	}
	out.DirectFallbackAvailable = h.directModelKeyAvailable(ctx, tenant.DirectModelID)
	return out
}

func (h *SettingsHandler) directModelKeyAvailable(ctx context.Context, modelID string) bool {
	if h.modelService == nil || strings.TrimSpace(modelID) == "" {
		return false
	}
	model, err := h.modelService.GetModelByID(ctx, modelID)
	if err != nil || model == nil {
		return false
	}
	return strings.TrimSpace(model.Parameters.APIKey) != ""
}

func (h *SettingsHandler) GetTenantParserEngineConfig(c *gin.Context) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	data := types.ParserEngineConfigForResponse(tenant.ParserEngineConfig, true)
	if data == nil {
		data = &types.ParserEngineConfig{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

func (h *SettingsHandler) updateTenantParserEngineConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()
	var cfg types.ParserEngineConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	merged := types.MergeParserEngineConfigForUpdate(&cfg, tenant.ParserEngineConfig)
	if err := validateParserEngineOutboundURLs(merged); err != nil {
		c.Error(errors.NewValidationError(err.Error()))
		return
	}
	tenant.ParserEngineConfig = merged
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update workspace parser engine config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types.ParserEngineConfigForResponse(updatedTenant.ParserEngineConfig, true),
		"message": "解析引擎配置已更新",
	})
}

func (h *SettingsHandler) GetTenantStorageEngineConfig(c *gin.Context) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	data := types.StorageEngineConfigForResponse(tenant.StorageEngineConfig, true)
	if data == nil {
		data = &types.StorageEngineConfig{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

func (h *SettingsHandler) updateTenantStorageEngineConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()
	var cfg types.StorageEngineConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.DefaultProvider))
	if provider == "" {
		provider = firstAllowedStorageProvider()
	}
	if provider == "" {
		c.Error(errors.NewBadRequestError("No storage provider is allowed by STORAGE_ALLOW_LIST"))
		return
	}
	if !isStorageProviderAllowed(provider) {
		c.Error(errors.NewBadRequestError("Storage provider is not allowed by STORAGE_ALLOW_LIST"))
		return
	}
	cfg.DefaultProvider = provider
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	merged := types.MergeStorageEngineConfigForUpdate(&cfg, tenant.StorageEngineConfig)
	tenant.StorageEngineConfig = merged
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update workspace storage engine config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types.StorageEngineConfigForResponse(updatedTenant.StorageEngineConfig, true),
		"message": "存储引擎配置已更新",
	})
}

// GetPromptTemplates returns the built-in prompt templates, localized for the
// caller's language.
func (h *SettingsHandler) GetPromptTemplates(c *gin.Context) {
	templates := h.config.PromptTemplates
	if templates == nil {
		templates = &config.PromptTemplatesConfig{}
	}

	lang := types.LanguageFromContextOrDefault(c.Request.Context())

	localized := &config.PromptTemplatesConfig{
		SystemPrompt:         config.LocalizeTemplates(templates.SystemPrompt, lang),
		ContextTemplate:      config.LocalizeTemplates(templates.ContextTemplate, lang),
		Rewrite:              config.LocalizeTemplates(templates.Rewrite, lang),
		Fallback:             config.LocalizeTemplates(templates.Fallback, lang),
		GenerateSessionTitle: templates.GenerateSessionTitle,
		GenerateSummary:      templates.GenerateSummary,
		KeywordsExtraction:   templates.KeywordsExtraction,
		AgentSystemPrompt:    config.LocalizeTemplates(templates.AgentSystemPrompt, lang),
		IntentPrompts:        config.LocalizeTemplates(templates.IntentPrompts, lang),
	}
	if override := h.config.AgentSystemPromptOverride(); override != "" {
		localized.AgentSystemPrompt = []config.PromptTemplate{{
			ID:      "personal_global_override",
			Name:    "Global assistant prompt",
			Content: override,
			Default: true,
			Mode:    "rag",
		}}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    localized,
	})
}

// updatePromptTemplates saves the owner's global assistant prompt override.
// An empty string removes the override and restores the built-in template.
func (h *SettingsHandler) updatePromptTemplates(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		AgentSystemPrompt string `json:"agent_system_prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	if len(req.AgentSystemPrompt) > 64*1024 {
		c.Error(errors.NewValidationError("prompt is too long"))
		return
	}

	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	tenant.AgentSystemPrompt = strings.TrimSpace(req.AgentSystemPrompt)
	if _, err := h.service.UpdateTenant(ctx, tenant); err != nil {
		c.Error(errors.NewInternalServerError("Failed to save prompt").WithDetails(err.Error()))
		return
	}
	h.config.SetAgentSystemPromptOverride(tenant.AgentSystemPrompt)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"agent_system_prompt": tenant.AgentSystemPrompt,
			"is_custom":           tenant.AgentSystemPrompt != "",
		},
	})
}

func (h *SettingsHandler) GetTenantChatHistoryConfig(c *gin.Context) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	data := tenant.ChatHistoryConfig
	if data == nil {
		data = &types.ChatHistoryConfig{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

func (h *SettingsHandler) updateTenantChatHistoryConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()

	var req types.ChatHistoryConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	existing := tenant.ChatHistoryConfig

	cfg := &types.ChatHistoryConfig{
		Enabled:          req.Enabled,
		EmbeddingModelID: req.EmbeddingModelID,
		KnowledgeBaseID:  "",
	}

	if existing != nil && existing.KnowledgeBaseID != "" {
		if existing.EmbeddingModelID == req.EmbeddingModelID {
			cfg.KnowledgeBaseID = existing.KnowledgeBaseID
		} else {
			logger.Infof(ctx, "Embedding model changed from %s to %s, will create new chat history KB", existing.EmbeddingModelID, req.EmbeddingModelID)
		}
	}

	if cfg.Enabled && cfg.EmbeddingModelID != "" && cfg.KnowledgeBaseID == "" {
		kb := &types.KnowledgeBase{
			Name:             "__chat_history__",
			Type:             types.KnowledgeBaseTypeDocument,
			IsTemporary:      true,
			Description:      "Auto-managed knowledge base for chat history message indexing",
			EmbeddingModelID: cfg.EmbeddingModelID,
		}
		createdKB, err := h.kbService.CreateKnowledgeBase(ctx, kb)
		if err != nil {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to create chat history knowledge base").WithDetails(err.Error()))
			return
		}
		cfg.KnowledgeBaseID = createdKB.ID
		logger.Infof(ctx, "Auto-created chat history KB: id=%s, embedding_model=%s", createdKB.ID, cfg.EmbeddingModelID)
	}

	tenant.ChatHistoryConfig = cfg
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update chat history config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedTenant.ChatHistoryConfig,
		"message": "Chat history configuration updated successfully",
	})
}

func (h *SettingsHandler) GetTenantRetrievalConfig(c *gin.Context) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	data := tenant.RetrievalConfig
	if data == nil {
		data = &types.RetrievalConfig{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

func (h *SettingsHandler) updateTenantRetrievalConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()

	var cfg types.RetrievalConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	if cfg.VectorThreshold < 0 || cfg.VectorThreshold > 1 {
		c.Error(errors.NewBadRequestError("vector_threshold must be between 0 and 1"))
		return
	}
	if cfg.KeywordThreshold < 0 || cfg.KeywordThreshold > 1 {
		c.Error(errors.NewBadRequestError("keyword_threshold must be between 0 and 1"))
		return
	}
	if cfg.RerankThreshold < -10 || cfg.RerankThreshold > 10 {
		c.Error(errors.NewBadRequestError("rerank_threshold must be between -10 and 10"))
		return
	}
	if cfg.EmbeddingTopK < 0 || cfg.EmbeddingTopK > 200 {
		c.Error(errors.NewBadRequestError("embedding_top_k must be between 0 and 200"))
		return
	}
	if cfg.RerankTopK < 0 || cfg.RerankTopK > 200 {
		c.Error(errors.NewBadRequestError("rerank_top_k must be between 0 and 200"))
		return
	}

	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	tenant.RetrievalConfig = &cfg
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update retrieval config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedTenant.RetrievalConfig,
		"message": "Retrieval configuration updated successfully",
	})
}

func (h *SettingsHandler) GetTenantMemoryConfig(c *gin.Context) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	data := tenant.MemoryConfig
	if data == nil {
		// Memory stays off by default: it retains what the owner says across
		// sessions, so it must be opted into.
		data = &types.MemoryConfig{}
	}
	data.Normalize()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

func (h *SettingsHandler) updateTenantMemoryConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()

	var cfg types.MemoryConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	if cfg.WriteMode != "" &&
		cfg.WriteMode != types.MemoryWriteExplicitOnly &&
		cfg.WriteMode != types.MemoryWriteAuto {
		c.Error(errors.NewBadRequestError("write_mode must be explicit_only or auto"))
		return
	}
	if cfg.MaxItems < 0 || cfg.MaxItems > 2000 {
		c.Error(errors.NewBadRequestError("max_items must be between 0 and 2000"))
		return
	}
	if cfg.ExtractDelaySeconds < 0 || cfg.ExtractDelaySeconds > types.MaxMemoryExtractDelaySeconds {
		c.Error(errors.NewBadRequestError(fmt.Sprintf(
			"extract_delay_seconds must be between 0 and %d", types.MaxMemoryExtractDelaySeconds)))
		return
	}
	if cfg.ExtractMinIntervalSeconds < 0 ||
		cfg.ExtractMinIntervalSeconds > types.MaxMemoryExtractMinIntervalSeconds {
		c.Error(errors.NewBadRequestError(fmt.Sprintf(
			"extract_min_interval_seconds must be between 0 and %d",
			types.MaxMemoryExtractMinIntervalSeconds)))
		return
	}
	if len(cfg.EmbeddingModelID) > 64 {
		c.Error(errors.NewBadRequestError("embedding_model_id is too long"))
		return
	}
	if cfg.InterestThreshold < 0 || cfg.InterestThreshold > types.MaxMemoryInterestThreshold {
		c.Error(errors.NewBadRequestError(fmt.Sprintf(
			"interest_threshold must be between 1 and %d", types.MaxMemoryInterestThreshold)))
		return
	}
	if len([]rune(cfg.ExtractInstructions)) > types.MaxMemoryExtractInstructionsRunes {
		c.Error(errors.NewBadRequestError(fmt.Sprintf(
			"extract_instructions must be at most %d characters",
			types.MaxMemoryExtractInstructionsRunes)))
		return
	}
	cfg.Normalize()

	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	tenant.MemoryConfig = &cfg
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update memory config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedTenant.MemoryConfig,
		"message": "Memory configuration updated successfully",
	})
}

func validateParserEngineOutboundURLs(cfg *types.ParserEngineConfig) error {
	if cfg == nil {
		return nil
	}
	if endpoint := strings.TrimSpace(cfg.MinerUEndpoint); endpoint != "" {
		if err := secutils.ValidateURLForSSRF(endpoint); err != nil {
			return fmt.Errorf("mineru_endpoint failed SSRF validation: %v", err)
		}
	}
	if vlmURL := strings.TrimSpace(cfg.MinerUVLMServerURL); vlmURL != "" {
		if err := secutils.ValidateURLForSSRF(vlmURL); err != nil {
			return fmt.Errorf("mineru_vlm_server_url failed SSRF validation: %v", err)
		}
	}
	if odlURL := strings.TrimSpace(cfg.ODLHybridURL); odlURL != "" {
		if err := secutils.ValidateURLForSSRF(odlURL); err != nil {
			return fmt.Errorf("odl_hybrid_url failed SSRF validation: %v", err)
		}
	}
	if endpoint := strings.TrimSpace(cfg.PaddleOCRVLEndpoint); endpoint != "" {
		if err := secutils.ValidateURLForSSRF(endpoint); err != nil {
			return fmt.Errorf("paddleocr_vl_endpoint failed SSRF validation: %v", err)
		}
	}
	return nil
}

func (h *SettingsHandler) updateTenantReadingConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		ReadingModelID string `json:"reading_model_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	modelID := strings.TrimSpace(req.ReadingModelID)
	if modelID != "" {
		if h.modelService == nil {
			c.Error(errors.NewInternalServerError("模型服务未配置"))
			return
		}
		model, err := h.modelService.GetModelByID(ctx, modelID)
		if err != nil || model == nil {
			c.Error(errors.NewBadRequestError("精读模型不存在"))
			return
		}
		if model.Type != types.ModelTypeKnowledgeQA {
			c.Error(errors.NewBadRequestError("精读模型必须是 KnowledgeQA 类型"))
			return
		}
		if chat.EffectiveThinkingControl(chat.ConfigFromModel(model)) == "none" {
			c.Error(errors.NewBadRequestError("精读模型必须开启思考模式"))
			return
		}
	}
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		c.Error(errors.NewInternalServerError("Workspace is empty"))
		return
	}
	tenant.ReadingModelID = modelID
	updated, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"reading_model_id": updated.ReadingModelID,
	}})
}

func (h *SettingsHandler) updateTenantDirectConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		DirectModelID string `json:"direct_model_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	modelID := strings.TrimSpace(req.DirectModelID)
	if modelID != "" {
		if h.modelService == nil {
			c.Error(errors.NewInternalServerError("模型服务未配置"))
			return
		}
		model, err := h.modelService.GetModelByID(ctx, modelID)
		if err != nil || model == nil {
			c.Error(errors.NewBadRequestError("直连模型不存在"))
			return
		}
		if model.Type != types.ModelTypeKnowledgeQA {
			c.Error(errors.NewBadRequestError("直连模型必须是 KnowledgeQA 类型"))
			return
		}
		if chat.EffectiveThinkingControl(chat.ConfigFromModel(model)) == "none" {
			c.Error(errors.NewBadRequestError("直连模型必须开启思考模式"))
			return
		}
	}
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		c.Error(errors.NewInternalServerError("Workspace is empty"))
		return
	}
	tenant.DirectModelID = modelID
	updated, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"direct_model_id": updated.DirectModelID,
	}})
}
