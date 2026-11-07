package chat

import (
	"encoding/json"
	"net/http"
	"strings"

	"gaze/internal/models/provider"
	"gaze/internal/types"
	"github.com/sashabaranov/go-openai"
)

// authCreds carries the credentials a providerAdapter needs to authenticate a
// raw HTTP request. APIKey covers the common Bearer case.
type authCreds struct {
	APIKey string
}

// providerAdapter captures everything provider-specific about an
// OpenAI-compatible chat backend. Every method has a sensible default on
// baseProvider, so a new provider is added by embedding baseProvider and
// overriding only the one or two methods that actually differ.
type providerAdapter interface {
	// Name is the provider this adapter handles.
	Name() provider.ProviderName
	// Matches reports whether this adapter applies to the given model name.
	// Used for sub-provider routing (e.g. Qwen thinking models within Aliyun).
	// Default: true.
	Matches(model string) bool
	// Thinking is how this provider encodes ChatOptions.Thinking. Default: none.
	Thinking() ThinkingStrategy
	// ShapeRequest applies in-place parameter quirks to the standard request
	// (stripping unsupported fields, pinning temperature, …). Default: noop.
	ShapeRequest(req *openai.ChatCompletionRequest, opts *ChatOptions, isStream bool)
	// TransformMessages rewrites the converted messages (e.g. downgrading
	// multi-content to plain text). Default: identity.
	TransformMessages(msgs []openai.ChatCompletionMessage) []openai.ChatCompletionMessage
	// Endpoint overrides the request URL. Empty string means the standard
	// "<baseURL>/chat/completions" handled by the caller. Default: "".
	Endpoint(baseURL, modelID string, isStream bool) string
	// Auth sets authentication headers on a raw HTTP request. Default: Bearer.
	Auth(req *http.Request, creds authCreds, body []byte)
	// ForceRawHTTP forces the raw HTTP path even when the body is standard
	// (needed by providers that must sign the exact request bytes). Default: false.
	ForceRawHTTP() bool
	// ExtractToolCallMetadata captures provider-specific state from a raw
	// OpenAI-compatible tool_call object. Default: nil.
	ExtractToolCallMetadata(raw json.RawMessage) types.ToolCallMetadata
	// InjectToolCallMetadata writes provider-specific state back into an outbound
	// OpenAI-compatible tool_call object. Default: noop.
	InjectToolCallMetadata(toolCall map[string]any, metadata types.ToolCallMetadata)
}

// baseProvider supplies the default behavior for every providerAdapter method.
// It is also the fallback returned by resolveProvider for unknown providers:
// Bearer auth, standard endpoint, no thinking, no request shaping.
type baseProvider struct{}

func (baseProvider) Name() provider.ProviderName                                    { return "" }
func (baseProvider) Matches(string) bool                                            { return true }
func (baseProvider) Thinking() ThinkingStrategy                                     { return noThinking{} }
func (baseProvider) ShapeRequest(*openai.ChatCompletionRequest, *ChatOptions, bool) {}
func (baseProvider) TransformMessages(msgs []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	return msgs
}
func (baseProvider) Endpoint(string, string, bool) string { return "" }
func (baseProvider) Auth(req *http.Request, creds authCreds, _ []byte) {
	req.Header.Set("Authorization", "Bearer "+creds.APIKey)
}
func (baseProvider) ForceRawHTTP() bool { return false }
func (baseProvider) ExtractToolCallMetadata(json.RawMessage) types.ToolCallMetadata {
	return nil
}
func (baseProvider) InjectToolCallMetadata(map[string]any, types.ToolCallMetadata) {}

// --- Aliyun Qwen thinking models: enable_thinking (always sent, forced off non-stream) ---

type qwenThinkingProvider struct{ baseProvider }

func (qwenThinkingProvider) Name() provider.ProviderName { return provider.ProviderAliyun }
func (qwenThinkingProvider) Matches(model string) bool   { return provider.IsQwenThinkingModel(model) }
func (qwenThinkingProvider) Thinking() ThinkingStrategy {
	return enableThinking{alwaysSend: true, disableOnNonStream: true}
}

// --- DeepSeek: does not support tool_choice ---

type deepseekProvider struct{ baseProvider }

func (deepseekProvider) Name() provider.ProviderName { return provider.ProviderDeepSeek }

// Native DeepSeek cache counters are not represented by go-openai v1.41.2;
// use the raw path so prompt_cache_hit_tokens/miss_tokens remain observable.
func (deepseekProvider) ForceRawHTTP() bool { return true }
func (deepseekProvider) ShapeRequest(req *openai.ChatCompletionRequest, opts *ChatOptions, _ bool) {
	if opts != nil && opts.ToolChoice != "" {
		req.ToolChoice = nil
	}
}

// --- Generic (vLLM / local deployments): thinking via chat_template_kwargs ---

type genericProvider struct{ baseProvider }

func (genericProvider) Name() provider.ProviderName { return provider.ProviderGeneric }
func (genericProvider) Thinking() ThinkingStrategy  { return chatTemplateKwargs{} }

// --- LKEAP: thinking object, except DeepSeek-R1 which takes no thinking field ---

type lkeapReasoningProvider struct{ baseProvider }

func (lkeapReasoningProvider) Name() provider.ProviderName { return provider.ProviderLKEAP }
func (lkeapReasoningProvider) Matches(model string) bool {
	return strings.Contains(strings.ToLower(model), "deepseek-r1")
}
func (lkeapReasoningProvider) Thinking() ThinkingStrategy { return noThinking{} }

type lkeapProvider struct{ baseProvider }

func (lkeapProvider) Name() provider.ProviderName { return provider.ProviderLKEAP }
func (lkeapProvider) Thinking() ThinkingStrategy  { return thinkingTypeField{} }

// --- Volcengine Ark: `{ "thinking": { "type": ... } }` ---

type volcengineProvider struct{ baseProvider }

func (volcengineProvider) Name() provider.ProviderName { return provider.ProviderVolcengine }
func (volcengineProvider) Thinking() ThinkingStrategy  { return thinkingTypeField{} }

// --- Moonshot: only temperature=1 is accepted for most models ---

type moonshotProvider struct{ baseProvider }

func (moonshotProvider) Name() provider.ProviderName { return provider.ProviderMoonshot }
func (moonshotProvider) ShapeRequest(req *openai.ChatCompletionRequest, _ *ChatOptions, _ bool) {
	req.Temperature = 1
	req.TopP = 0
}

// --- Gemini's OpenAI-compatible endpoint: tool calls round-trip extra_content ---

type geminiProvider struct{ baseProvider }

func (geminiProvider) Name() provider.ProviderName { return provider.ProviderGemini }

// The OpenAI-compatible Gemini endpoint hides thought signatures in
// tool_call.extra_content.google, which the standard SDK would drop.
func (geminiProvider) ForceRawHTTP() bool { return true }

func (geminiProvider) ExtractToolCallMetadata(raw json.RawMessage) types.ToolCallMetadata {
	var call struct {
		ExtraContent map[string]json.RawMessage `json:"extra_content"`
	}
	if json.Unmarshal(raw, &call) != nil || len(call.ExtraContent) == 0 {
		return nil
	}
	return types.ToolCallMetadata(call.ExtraContent)
}

func (geminiProvider) InjectToolCallMetadata(toolCall map[string]any, metadata types.ToolCallMetadata) {
	if len(toolCall) == 0 || len(metadata) == 0 {
		return
	}
	extra := make(map[string]any, len(metadata))
	for key, raw := range metadata {
		if len(raw) == 0 {
			continue
		}
		var value any
		if json.Unmarshal(raw, &value) == nil {
			extra[key] = value
		}
	}
	if len(extra) > 0 {
		toolCall["extra_content"] = extra
	}
}

// providerRegistry is ordered: more specific adapters (those with a real
// Matches predicate) must precede the generic catch-all for the same provider.
var providerRegistry = []providerAdapter{
	qwenThinkingProvider{},
	deepseekProvider{},
	lkeapReasoningProvider{},
	lkeapProvider{},
	volcengineProvider{},
	moonshotProvider{},
	geminiProvider{},
	genericProvider{},
}

// resolveProvider returns the adapter handling the given provider+model, or
// baseProvider{} (Bearer auth, standard endpoint, no thinking) when none matches.
func resolveProvider(name provider.ProviderName, model string) providerAdapter {
	for _, p := range providerRegistry {
		if p.Name() == name && p.Matches(model) {
			return p
		}
	}
	return baseProvider{}
}
