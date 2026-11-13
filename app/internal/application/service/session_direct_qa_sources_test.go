package service

import (
	"strings"
	"testing"

	"gaze/internal/types"
	"github.com/stretchr/testify/require"
)

func TestFormatWebSearchSourcesDedupesURLs(t *testing.T) {
	out := formatWebSearchSources([]*types.WebSearchResult{
		{Title: "first", URL: "https://a.example/1"},
		{Title: "duplicate", URL: "https://a.example/1"},
		{Title: "", URL: "https://b.example/2"},
		nil,
		{Title: "empty", URL: "  "},
	})
	require.Contains(t, out, "1. [first](https://a.example/1)")
	require.Contains(t, out, "2. [https://b.example/2](https://b.example/2)")
	require.NotContains(t, out, "duplicate")
	require.Equal(t, 2, strings.Count(out, "](https://"))
}
