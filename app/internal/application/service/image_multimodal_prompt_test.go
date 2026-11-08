package service

import (
	"context"
	"strings"
	"testing"

	"gaze/internal/types"
)

func TestBuildVLMCaptionPrompt(t *testing.T) {
	t.Run("uses Chinese and custom instructions", func(t *testing.T) {
		got := buildVLMCaptionPrompt(context.Background(), types.VLMConfig{
			DescriptionLanguage: "English",
			CustomInstructions:  "Focus on alarm codes.",
		})
		if !strings.Contains(got, "简体中文") || !strings.Contains(got, "Focus on alarm codes.") {
			t.Fatalf("unexpected prompt: %s", got)
		}
	})

	t.Run("always uses Chinese regardless of context language", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), types.LanguageContextKey, "ko-KR")
		got := buildVLMCaptionPrompt(ctx, types.VLMConfig{})
		if !strings.Contains(got, "简体中文") {
			t.Fatalf("unexpected prompt: %s", got)
		}
	})
}
