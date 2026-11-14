package types

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanReviewExcerptStripsMarkdown(t *testing.T) {
	input := "## 标题\n![截图](../../images/a.png)\n- **重点**\n见 [文档](https://example.com)"
	got := CleanReviewExcerpt(input)
	assert.NotContains(t, got, "##")
	assert.NotContains(t, got, "![](")
	assert.NotContains(t, got, "**")
	assert.Contains(t, got, "标题")
	assert.Contains(t, got, "重点")
	assert.Contains(t, got, "文档")
}

func TestCleanReviewExcerptTruncatesLongText(t *testing.T) {
	got := CleanReviewExcerpt(strings.Repeat("字", 600))
	assert.LessOrEqual(t, len([]rune(got)), 501)
	assert.True(t, strings.HasSuffix(got, "…"))
}
