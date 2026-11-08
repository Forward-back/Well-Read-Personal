package types

import (
	"strings"
	"testing"
)

func TestAppendCustomPromptInstructions(t *testing.T) {
	t.Run("empty preserves prompt", func(t *testing.T) {
		if got := AppendCustomPromptInstructions("base", "  ", "gaze"); got != "base" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("appends bounded business guidance after base", func(t *testing.T) {
		got := AppendCustomPromptInstructions("base", " Focus on contracts. ", "gaze")
		if !strings.HasPrefix(got, "base\n\n<gaze_business_instructions>") {
			t.Fatalf("unexpected prefix: %q", got)
		}
		if !strings.Contains(got, "Focus on contracts.") || !strings.Contains(got, "不妨碍") {
			t.Fatalf("missing guidance or precedence rule: %q", got)
		}
	})
}

func TestValidateKnowledgeBasePromptInstructions(t *testing.T) {
	kb := &KnowledgeBase{
		ChunkingConfig: ChunkingConfig{TableMetadataInstructions: strings.Repeat("a", MaxCustomPromptInstructionsLength+1)},
	}
	if err := ValidateKnowledgeBasePromptInstructions(kb); err == nil {
		t.Fatal("expected length validation error")
	}
}

func TestNormalizeKnowledgeBasePromptInstructions(t *testing.T) {
	kb := &KnowledgeBase{
		VLMConfig: VLMConfig{CustomInstructions: "  focus labels  "},
	}
	NormalizeKnowledgeBasePromptInstructions(kb)
	if kb.VLMConfig.CustomInstructions != "focus labels" {
		t.Fatalf("got %q", kb.VLMConfig.CustomInstructions)
	}
}
