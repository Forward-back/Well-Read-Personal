package chat

import (
	"context"
	"encoding/json"
	"testing"

	"gaze/internal/models/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatOptionsCompletionBudget(t *testing.T) {
	assert.Zero(t, (*ChatOptions)(nil).CompletionBudget())
	assert.Zero(t, (&ChatOptions{}).CompletionBudget())
	assert.Equal(t, 128, (&ChatOptions{MaxTokens: 128}).CompletionBudget())
	assert.Equal(t, 256, (&ChatOptions{MaxCompletionTokens: 256}).CompletionBudget())
	assert.Equal(t, 256, (&ChatOptions{MaxTokens: 128, MaxCompletionTokens: 256}).CompletionBudget())
}

func TestBuildChatCompletionRequest_OneWireTokenField(t *testing.T) {
	messages := []Message{{Role: "user", Content: "hello"}}

	t.Run("volcengine both aliases send only max_completion_tokens", func(t *testing.T) {
		c := newOutboundChat(t, string(provider.ProviderVolcengine), "doubao-seed-2-0-mini", nil)
		req := c.BuildChatCompletionRequest(messages, &ChatOptions{
			MaxTokens: 2048, MaxCompletionTokens: 4096,
		}, false)
		assert.Equal(t, 4096, req.MaxCompletionTokens)
		assert.Zero(t, req.MaxTokens)

		body, err := json.Marshal(req)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"max_completion_tokens":4096`)
		assert.NotContains(t, string(body), `"max_tokens"`)
	})

	t.Run("deepseek sends max_tokens", func(t *testing.T) {
		c := newOutboundChat(t, string(provider.ProviderDeepSeek), "deepseek-chat", nil)
		req := c.shapedRequest(messages, &ChatOptions{
			MaxTokens: 2048, MaxCompletionTokens: 4096,
		}, false)
		assert.Equal(t, 4096, req.MaxTokens)
		assert.Zero(t, req.MaxCompletionTokens)

		body, err := json.Marshal(req)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"max_tokens":4096`)
		assert.NotContains(t, string(body), "max_completion_tokens")
	})

	t.Run("lkeap sends max_tokens", func(t *testing.T) {
		c := newOutboundChat(t, string(provider.ProviderLKEAP), "deepseek-v3.1", nil)
		req := c.BuildChatCompletionRequest(messages, &ChatOptions{MaxCompletionTokens: 4096}, false)
		assert.Equal(t, 4096, req.MaxTokens)
		assert.Zero(t, req.MaxCompletionTokens)

		body, err := json.Marshal(req)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"max_tokens":4096`)
		assert.NotContains(t, string(body), "max_completion_tokens")
	})

	t.Run("generic vLLM sends max_tokens from MaxTokens-only callers", func(t *testing.T) {
		c := newOutboundChat(t, string(provider.ProviderGeneric), "qwen3", nil)
		req := c.BuildChatCompletionRequest(messages, &ChatOptions{MaxTokens: 2048}, false)
		assert.Equal(t, 2048, req.MaxTokens)
		assert.Zero(t, req.MaxCompletionTokens)
	})

	t.Run("aliyun dashscope keeps max_completion_tokens", func(t *testing.T) {
		c := newOutboundChat(t, string(provider.ProviderAliyun), "qwen-plus", nil)
		req := c.BuildChatCompletionRequest(messages, &ChatOptions{MaxTokens: 2048}, false)
		assert.Zero(t, req.MaxTokens)
		assert.Equal(t, 2048, req.MaxCompletionTokens)
	})

	t.Run("openai gpt-4o sends max_completion_tokens", func(t *testing.T) {
		c := newOutboundChat(t, string(provider.ProviderGeneric), "gpt-4o", nil)
		req := c.BuildChatCompletionRequest(messages, &ChatOptions{MaxTokens: 128}, false)
		assert.Zero(t, req.MaxTokens)
		assert.Equal(t, 128, req.MaxCompletionTokens)
	})
}

func TestBuildOutbound_OneWireTokenField(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hello"}}

	t.Run("volcengine thinking keeps only max_completion_tokens", func(t *testing.T) {
		c := newOutboundChat(t, string(provider.ProviderVolcengine), "doubao-seed-2-0-mini", nil)
		body, _, useRaw, err := c.buildOutbound(context.Background(), msgs, &ChatOptions{
			MaxTokens: 2048, MaxCompletionTokens: 4096, Thinking: ptrBool(true),
		}, true)
		require.NoError(t, err)
		require.True(t, useRaw)
		js := mustJSON(t, body)
		assert.Contains(t, js, `"thinking"`)
		assert.Contains(t, js, `"max_completion_tokens":4096`)
		assert.NotContains(t, js, `"max_tokens"`)
	})

	t.Run("lkeap thinking keeps only max_tokens", func(t *testing.T) {
		c := newOutboundChat(t, string(provider.ProviderLKEAP), "deepseek-v3.1", nil)
		body, _, useRaw, err := c.buildOutbound(context.Background(), msgs, &ChatOptions{
			MaxCompletionTokens: 4096, Thinking: ptrBool(false),
		}, true)
		require.NoError(t, err)
		require.True(t, useRaw)
		js := mustJSON(t, body)
		assert.Contains(t, js, `"thinking"`)
		assert.Contains(t, js, `"max_tokens":4096`)
		assert.NotContains(t, js, "max_completion_tokens")
	})
}

func TestOllamaBuildChatRequestUsesCompletionBudget(t *testing.T) {
	c := &OllamaChat{modelName: "llama3"}
	req := c.buildChatRequest(
		[]Message{{Role: "user", Content: "hi"}},
		&ChatOptions{MaxCompletionTokens: 512},
		false,
	)
	assert.Equal(t, 512, req.Options["num_predict"])
}

func TestBuildLangfuseModelParamsUsesCompletionBudget(t *testing.T) {
	params := buildLangfuseModelParams(&ChatOptions{MaxTokens: 128, MaxCompletionTokens: 256})
	assert.Equal(t, 256, params["max_completion_tokens"])
	_, hasLegacy := params["max_tokens"]
	assert.False(t, hasLegacy)
}
