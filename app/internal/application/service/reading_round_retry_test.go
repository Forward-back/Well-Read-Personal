package service

import (
	"context"
	"testing"

	"gaze/internal/models/chat"
	"gaze/internal/types"
	"github.com/stretchr/testify/require"
)

type scriptedChatModel struct {
	responses []*types.ChatResponse
	calls     [][]chat.Message
}

func (m *scriptedChatModel) Chat(
	_ context.Context, messages []chat.Message, _ *chat.ChatOptions,
) (*types.ChatResponse, error) {
	m.calls = append(m.calls, append([]chat.Message(nil), messages...))
	index := len(m.calls) - 1
	if index >= len(m.responses) {
		index = len(m.responses) - 1
	}
	return m.responses[index], nil
}

func (m *scriptedChatModel) ChatStream(
	context.Context, []chat.Message, *chat.ChatOptions,
) (<-chan types.StreamResponse, error) {
	return nil, nil
}

func (m *scriptedChatModel) GetModelName() string { return "scripted" }
func (m *scriptedChatModel) GetModelID() string   { return "scripted" }

func TestCallReadingRoundModelRetryKeepsReasoningContent(t *testing.T) {
	model := &scriptedChatModel{responses: []*types.ChatResponse{
		{Content: "not json", ReasoningContent: "first reasoning"},
		{Content: `{"understanding":"ok","questions":[],"answers":[],"conclusions":[],"terms":[],"contradictions":[],"search_queries":[]}`},
	}}
	svc := &readingRunService{}
	messages := []chat.Message{
		{Role: "system", Content: "rules"},
		{Role: "user", Content: "read this"},
	}

	parsed, _, reasoning, _, err := svc.callReadingRoundModel(context.Background(), model, messages, "standard")
	require.NoError(t, err)
	require.Equal(t, "ok", parsed.Understanding)
	require.Contains(t, reasoning, "first reasoning")
	require.Len(t, model.calls, 2)

	assistant := model.calls[1][2]
	require.Equal(t, "assistant", assistant.Role)
	require.Equal(t, "not json", assistant.Content)
	require.Equal(t, "first reasoning", assistant.ReasoningContent)
}

func TestCallReadingRoundModelRetryOmitsEmptyAssistantTurn(t *testing.T) {
	model := &scriptedChatModel{responses: []*types.ChatResponse{
		{Content: "", ReasoningContent: ""},
		{Content: `{"understanding":"ok","questions":[],"answers":[],"conclusions":[],"terms":[],"contradictions":[],"search_queries":[]}`},
	}}
	svc := &readingRunService{}
	messages := []chat.Message{
		{Role: "system", Content: "rules"},
		{Role: "user", Content: "read this"},
	}

	_, _, _, _, err := svc.callReadingRoundModel(context.Background(), model, messages, "standard")
	require.NoError(t, err)
	require.Len(t, model.calls, 2)
	for _, message := range model.calls[1] {
		if message.Role == "assistant" {
			t.Fatalf("empty assistant turn must not be sent: %+v", message)
		}
	}
}
