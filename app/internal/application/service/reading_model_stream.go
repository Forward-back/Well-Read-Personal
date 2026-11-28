package service

import (
	"context"
	"fmt"
	"strings"

	"gaze/internal/models/chat"
	"gaze/internal/types"
)

// readingModelChat uses the streaming model path when a progress sink is
// attached, preserving the non-streaming path for all existing callers/tests.
// The returned value is assembled into the same ChatResponse shape callers
// already expect.
func (s *readingRunService) readingModelChat(
	ctx context.Context, model chat.Chat, messages []chat.Message, opts *chat.ChatOptions,
) (*types.ChatResponse, error) {
	if !types.HasReadingProgress(ctx) {
		return model.Chat(ctx, messages, opts)
	}
	stream, err := model.ChatStream(ctx, messages, opts)
	if err != nil {
		return nil, err
	}
	var content strings.Builder
	var reasoning strings.Builder
	var usage types.TokenUsage
	finishReason := ""
	for response := range stream {
		switch response.ResponseType {
		case types.ResponseTypeThinking:
			if strings.TrimSpace(response.Content) == "" {
				continue
			}
			reasoning.WriteString(response.Content)
			types.EmitReadingProgress(ctx, types.ReadingProgressEvent{
				Type: "reasoning", Content: response.Content,
			})
		case types.ResponseTypeAnswer:
			if response.Content != "" {
				content.WriteString(response.Content)
			}
		case types.ResponseTypeError:
			if response.Content != "" {
				return nil, fmt.Errorf("%s", response.Content)
			}
			return nil, fmt.Errorf("模型流式调用失败")
		}
		if response.Usage != nil {
			usage = *response.Usage
		}
		if response.FinishReason != "" {
			finishReason = response.FinishReason
		}
	}
	return &types.ChatResponse{
		Content:          content.String(),
		ReasoningContent: reasoning.String(),
		Usage:            usage,
		FinishReason:     finishReason,
	}, nil
}

func emitReadingPhase(ctx context.Context, phase, status string, section *types.ReadingSection, round int) {
	event := types.ReadingProgressEvent{Type: "phase", Phase: phase, Status: status, Round: round}
	if section != nil {
		event.SectionID = section.ID
		event.SectionTitle = section.Title
		event.SectionHeading = section.Heading
	}
	types.EmitReadingProgress(ctx, event)
}
