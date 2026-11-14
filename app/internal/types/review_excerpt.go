package types

import (
	"regexp"
	"strings"
)

var (
	reviewHTMLCommentRE = regexp.MustCompile(`(?s)<!--.*?-->`)
	reviewImageRE       = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reviewLinkRE        = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	reviewFenceRE       = regexp.MustCompile("(?m)^\\s*```.*$")
	reviewHeadingRE     = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s*`)
	reviewListRE        = regexp.MustCompile(`(?m)^\s*[-*+]\s+`)
	reviewQuoteRE       = regexp.MustCompile(`(?m)^\s*>\s?`)
	reviewHTMLTagRE     = regexp.MustCompile(`<[^>]+>`)
	reviewBlankLinesRE  = regexp.MustCompile(`\n{3,}`)
)

// CleanReviewExcerpt turns source material into the plain-text evidence shown
// on a card back. Images and links are dropped; their visible text survives.
func CleanReviewExcerpt(text string) string {
	cleaned := strings.TrimSpace(text)
	cleaned = reviewHTMLCommentRE.ReplaceAllString(cleaned, "")
	cleaned = reviewImageRE.ReplaceAllString(cleaned, "")
	cleaned = reviewLinkRE.ReplaceAllString(cleaned, "$1")
	cleaned = reviewFenceRE.ReplaceAllString(cleaned, "")
	cleaned = reviewHeadingRE.ReplaceAllString(cleaned, "")
	cleaned = reviewListRE.ReplaceAllString(cleaned, "")
	cleaned = reviewQuoteRE.ReplaceAllString(cleaned, "")
	cleaned = reviewHTMLTagRE.ReplaceAllString(cleaned, "")
	cleaned = strings.ReplaceAll(cleaned, "**", "")
	cleaned = strings.ReplaceAll(cleaned, "`", "")
	cleaned = reviewBlankLinesRE.ReplaceAllString(cleaned, "\n\n")
	cleaned = strings.TrimSpace(cleaned)
	runes := []rune(cleaned)
	if len(runes) > 500 {
		return string(runes[:500]) + "…"
	}
	return cleaned
}
