package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"gaze/internal/models/chat"
	"gaze/internal/types"
)

type streamingFakeChat struct{}

func (streamingFakeChat) Chat(context.Context, []chat.Message, *chat.ChatOptions) (*types.ChatResponse, error) {
	return &types.ChatResponse{Content: "non-stream"}, nil
}

func (streamingFakeChat) ChatStream(context.Context, []chat.Message, *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	ch := make(chan types.StreamResponse, 4)
	ch <- types.StreamResponse{ResponseType: types.ResponseTypeThinking, Content: "先核对 "}
	ch <- types.StreamResponse{ResponseType: types.ResponseTypeThinking, Content: "中断屏蔽字。"}
	ch <- types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Content: `{"ok":true}`}
	ch <- types.StreamResponse{
		ResponseType: types.ResponseTypeAnswer, Done: true,
		Usage: &types.TokenUsage{TotalTokens: 12},
	}
	close(ch)
	return ch, nil
}

func (streamingFakeChat) GetModelName() string { return "streaming-fake" }
func (streamingFakeChat) GetModelID() string   { return "streaming-fake" }

func TestReadingModelChatStreamsReasoning(t *testing.T) {
	var events []types.ReadingProgressEvent
	ctx := types.WithReadingProgress(context.Background(), func(event types.ReadingProgressEvent) {
		events = append(events, event)
	})
	svc := &readingRunService{}
	resp, err := svc.readingModelChat(ctx, streamingFakeChat{}, nil, &chat.ChatOptions{})
	require.NoError(t, err)
	require.Equal(t, `{"ok":true}`, resp.Content)
	require.Equal(t, "先核对 中断屏蔽字。", resp.ReasoningContent)
	require.Equal(t, 12, resp.Usage.TotalTokens)
	require.Len(t, events, 2)
	require.Equal(t, "reasoning", events[0].Type)
	require.Contains(t, events[0].Content, "核对")
}

func TestReadingModelChatUsesNonStreamWithoutSink(t *testing.T) {
	svc := &readingRunService{}
	resp, err := svc.readingModelChat(context.Background(), streamingFakeChat{}, nil, &chat.ChatOptions{})
	require.NoError(t, err)
	require.Equal(t, "non-stream", resp.Content)
}
