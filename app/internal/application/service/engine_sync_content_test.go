package service

import (
	"encoding/json"
	"testing"

	"gaze/internal/types"
	"github.com/stretchr/testify/require"
)

func codeMetadata(t *testing.T, path string, start, end int) types.JSON {
	t.Helper()
	raw, err := json.Marshal(map[string]interface{}{
		"code": map[string]interface{}{
			"path":       path,
			"start_line": start,
			"end_line":   end,
		},
	})
	require.NoError(t, err)
	return types.JSON(raw)
}

func TestEngineChunkContentPrefixesCodeLocation(t *testing.T) {
	chunk := &types.Chunk{
		Content:  "def resolve(self):\n    return None",
		Metadata: codeMetadata(t, "rag/src/retrieval/chunk_store.py", 42, 57),
	}
	require.Equal(t,
		"rag/src/retrieval/chunk_store.py:42-57\ndef resolve(self):\n    return None",
		engineChunkContent(chunk))
}

func TestEngineChunkContentLeavesPlainChunksAlone(t *testing.T) {
	require.Equal(t, "plain text", engineChunkContent(&types.Chunk{Content: "plain text"}))
	require.Equal(t, "code without location", engineChunkContent(&types.Chunk{
		Content:  "code without location",
		Metadata: codeMetadata(t, "", 0, 0),
	}))
}
