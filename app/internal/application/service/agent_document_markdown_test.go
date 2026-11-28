package service

import (
	"strings"
	"testing"

	"gaze/internal/types"
	"github.com/stretchr/testify/require"
)

func textChunk(id string, index, start, end int, content string) *types.Chunk {
	return &types.Chunk{
		ID: id, KnowledgeID: "doc", ChunkIndex: index, StartAt: start, EndAt: end,
		Content: content, IsEnabled: true, ChunkType: types.ChunkTypeText,
	}
}

func TestAssembleChunksMarkdownTrimsOverlap(t *testing.T) {
	markdown, exact := assembleChunksMarkdown([]*types.Chunk{
		textChunk("a", 0, 0, 3, "abc"),
		textChunk("b", 1, 2, 6, "cdef"),
	})
	require.True(t, exact)
	require.Equal(t, "abcdef", markdown)
}

func TestAssembleChunksMarkdownInsertsGapNewlines(t *testing.T) {
	markdown, exact := assembleChunksMarkdown([]*types.Chunk{
		textChunk("a", 0, 0, 3, "abc"),
		textChunk("b", 1, 3, 4, "X"),
		textChunk("c", 2, 6, 8, "ef"),
	})
	require.True(t, exact)
	require.Equal(t, "abcX\n\nef", markdown)
}

func TestAssembleChunksMarkdownSkipsParentAndDisabled(t *testing.T) {
	parent := textChunk("p", 0, 0, 3, "abc")
	parent.ChunkType = types.ChunkTypeParentText
	disabled := textChunk("d", 1, 3, 6, "def")
	disabled.IsEnabled = false
	markdown, exact := assembleChunksMarkdown([]*types.Chunk{
		parent,
		disabled,
		textChunk("t", 2, 6, 9, "ghi"),
	})
	require.True(t, exact)
	require.Equal(t, "ghi", markdown)
}

func TestAssembleChunksMarkdownFallsBackOnBadOffsets(t *testing.T) {
	markdown, exact := assembleChunksMarkdown([]*types.Chunk{
		textChunk("a", 0, 0, 5, "abc"),
		textChunk("b", 1, 5, 8, "def"),
	})
	require.False(t, exact)
	require.Equal(t, "abc\n\ndef", markdown)
}

const sampleMarkdown = `# 标题

前言。

## 背景

背景正文。

### 细节

细节正文。

## 背景

第二个背景。

~~~go
# 代码里的井号不是标题
~~~
`

func TestLocateMarkdownSectionSkipsFencedCode(t *testing.T) {
	_, matches, err := locateMarkdownSection(sampleMarkdown, "背景", 2, 2)
	require.NoError(t, err)
	require.Len(t, matches, 2)
	require.Equal(t, "背景", matches[1].Title)
}

func TestLocateMarkdownSectionRequiresDisambiguation(t *testing.T) {
	_, _, err := locateMarkdownSection(sampleMarkdown, "背景", 0, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "heading_level")
	_, _, err = locateMarkdownSection(sampleMarkdown, "不存在", 0, 1)
	require.Error(t, err)
}

func TestApplySectionEditReplaceAppendPrependDelete(t *testing.T) {
	section, _, err := locateMarkdownSection(sampleMarkdown, "细节", 3, 1)
	require.NoError(t, err)

	replaced, err := applySectionEdit(sampleMarkdown, section, "replace", "新的细节正文。")
	require.NoError(t, err)
	require.Contains(t, replaced, "### 细节\n\n新的细节正文。")
	require.NotContains(t, replaced, "\n细节正文。\n")

	section, _, err = locateMarkdownSection(sampleMarkdown, "背景", 2, 1)
	require.NoError(t, err)
	prepended, err := applySectionEdit(sampleMarkdown, section, "prepend", "开头补充。")
	require.NoError(t, err)
	require.Contains(t, prepended, "## 背景\n\n开头补充。\n\n背景正文。")

	appended, err := applySectionEdit(sampleMarkdown, section, "append", "末尾补充。")
	require.NoError(t, err)
	require.Less(t, strings.Index(appended, "末尾补充。"), strings.Index(appended, "### 细节"), appended)

	deleted, err := applySectionEdit(sampleMarkdown, section, "delete", "")
	require.NoError(t, err)
	require.NotContains(t, deleted, "### 细节")
	require.Contains(t, deleted, "## 背景\n\n第二个背景。")
}

func TestApplySectionEditRejectsEmptyContent(t *testing.T) {
	section, _, err := locateMarkdownSection(sampleMarkdown, "细节", 3, 1)
	require.NoError(t, err)
	_, err = applySectionEdit(sampleMarkdown, section, "replace", "   ")
	require.Error(t, err)
	_, err = applySectionEdit(sampleMarkdown, section, "unknown", "x")
	require.Error(t, err)
}

func TestEstimateChangedLines(t *testing.T) {
	added, removed := estimateChangedLines("a\nb\nc", "a\nB\nc\nd")
	require.Equal(t, 2, added)
	require.Equal(t, 1, removed)
}
