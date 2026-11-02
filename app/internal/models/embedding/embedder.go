package embedding

import (
	"context"
	"fmt"
	"strings"

	"gaze/internal/logger"
	"gaze/internal/models/provider"
	"gaze/internal/models/utils/ollama"
	"gaze/internal/tracing/langfuse"
	"gaze/internal/types"
)

// Embedder defines the interface for text vectorization
type Embedder interface {
	// Embed converts text to vector
	Embed(ctx context.Context, text string) ([]float32, error)

	// BatchEmbed converts multiple texts to vectors in batch
	BatchEmbed(ctx context.Context, texts []string) ([][]float32, error)

	// GetModelName returns the model name
	GetModelName() string

	// GetDimensions returns the vector dimensions
	GetDimensions() int

	// GetModelID returns the model ID
	GetModelID() string

	EmbedderPooler
}

type EmbedderPooler interface {
	BatchEmbedWithPool(ctx context.Context, model Embedder, texts []string) ([][]float32, error)
}

// EmbedderType represents the embedder type
type EmbedderType string

// Config represents the embedder configuration
type Config struct {
	Source                    types.ModelSource `json:"source"`
	BaseURL                   string            `json:"base_url"`
	ModelName                 string            `json:"model_name"`
	APIKey                    string            `json:"api_key"`
	TruncatePromptTokens      int               `json:"truncate_prompt_tokens"`
	Dimensions                int               `json:"dimensions"`
	SupportsDimensionOverride bool              `json:"supports_dimension_override"`
	ModelID                   string            `json:"model_id"`
	Provider                  string            `json:"provider"`
	// MaxConcurrency caps concurrent background calls to this model; 0 falls
	// back to the process-wide default (see limiter.GateN).
	MaxConcurrency int               `json:"max_concurrency"`
	ExtraConfig    map[string]string `json:"extra_config"`
	// CustomHeaders 允许在调用远程 API 时附加自定义 HTTP 请求头（类似 OpenAI Python SDK 的 extra_headers）。
	CustomHeaders map[string]string `json:"custom_headers"`
	AppSecret     string            // 加密值，工厂函数调用方传入，使用前已解密
}

// ConfigFromModel 根据 types.Model 构造 embedding.Config。
// 生产路径（从 DB 拉起）和测试连接路径（临时表单）共享这份映射。
func ConfigFromModel(m *types.Model) Config {
	if m == nil {
		return Config{}
	}
	return Config{
		Source:                    m.Source,
		BaseURL:                   m.Parameters.BaseURL,
		APIKey:                    m.Parameters.APIKey,
		ModelID:                   m.ID,
		ModelName:                 m.Name,
		Dimensions:                m.Parameters.EmbeddingParameters.Dimension,
		SupportsDimensionOverride: m.Parameters.EmbeddingParameters.SupportsDimensionOverride,
		TruncatePromptTokens:      m.Parameters.EmbeddingParameters.TruncatePromptTokens,
		Provider:                  m.Parameters.Provider,
		MaxConcurrency:            m.Parameters.MaxConcurrency,
		ExtraConfig:               m.Parameters.ExtraConfig,
		CustomHeaders:             m.Parameters.CustomHeaders,
	}
}

// NewEmbedder creates an embedder based on the configuration
func NewEmbedder(config Config, pooler EmbedderPooler, ollamaService *ollama.OllamaService) (Embedder, error) {
	e, err := newEmbedder(config, pooler, ollamaService)
	if err != nil {
		return e, err
	}
	if setter, ok := e.(interface{ SetSupportsDimensionOverride(bool) }); ok {
		setter.SetSupportsDimensionOverride(config.SupportsDimensionOverride)
	}
	// Innermost: gate the real provider round-trips (including the per-sub-batch
	// pool callbacks) before debug/langfuse wrap for logging/tracing. See
	// concurrencyEmbedder for why this sits below the observability decorators.
	e = wrapEmbeddingConcurrency(e, config.MaxConcurrency)
	if logger.LLMDebugEnabled() {
		e = &debugEmbedder{inner: e}
	}
	if langfuse.GetManager().Enabled() {
		e = &langfuseEmbedder{inner: e}
	}
	return e, nil
}

func newEmbedder(config Config, pooler EmbedderPooler, ollamaService *ollama.OllamaService) (Embedder, error) {
	var embedder Embedder
	var err error
	switch {
	case strings.ToLower(string(config.Source)) == string(types.ModelSourceLocal):
		embedder, err = NewOllamaEmbedder(config.BaseURL,
			config.ModelName, config.TruncatePromptTokens, config.Dimensions, config.ModelID, pooler, ollamaService)
		return embedder, err
	case provider.IsOpenAICompatibleSource(config.Source):
		// OpenAI-compatible embedder covers all remote providers.
		openaiEmb, oErr := NewOpenAIEmbedder(config.APIKey,
			config.BaseURL,
			config.ModelName,
			config.TruncatePromptTokens,
			config.Dimensions,
			config.ModelID,
			pooler)
		if openaiEmb != nil {
			openaiEmb.SetCustomHeaders(config.CustomHeaders)
		}
		embedder, err = openaiEmb, oErr
		return embedder, err
	default:
		return nil, fmt.Errorf("unsupported embedder source: %s", config.Source)
	}
}
