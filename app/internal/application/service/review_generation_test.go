package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGeneratedReviewCardsStripsCodeFence(t *testing.T) {
	cards, err := parseGeneratedReviewCards("```json\n[{\"question\":\"什么是栈？\",\"answer\":\"后进先出。\",\"source_index\":1}]\n```")
	require.NoError(t, err)
	require.Len(t, cards, 1)
	assert.Equal(t, "什么是栈？", cards[0].Question)
	assert.Equal(t, 1, cards[0].SourceIndex)
}

func TestParseGeneratedReviewCardsRejectsPlainText(t *testing.T) {
	_, err := parseGeneratedReviewCards("没有 JSON")
	require.Error(t, err)
}

func TestBuildReviewGenerationPromptIncludesSources(t *testing.T) {
	prompt := buildReviewGenerationPrompt(5, []sampledReviewChunk{
		{title: "数据结构", content: "栈是后进先出的线性表。"},
	})
	assert.Contains(t, prompt, "5 张")
	assert.Contains(t, prompt, "【资料1】数据结构")
	assert.Contains(t, prompt, "栈是后进先出的线性表")
}
