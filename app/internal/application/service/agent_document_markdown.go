package service

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gaze/internal/types"
)

const agentDocumentMaxRunes = 200000

// assembleChunksMarkdown stitches enabled text chunks in chunk_index order,
// trimming overlap using rune offsets. exact=false means the offsets were
// inconsistent and the caller received a plain "\n\n" join instead.
func assembleChunksMarkdown(chunks []*types.Chunk) (string, bool) {
	texts := make([]*types.Chunk, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk == nil || !chunk.IsEnabled || chunk.ChunkType != types.ChunkTypeText {
			continue
		}
		texts = append(texts, chunk)
	}
	if len(texts) == 0 {
		return "", true
	}
	sort.Slice(texts, func(i, j int) bool { return texts[i].ChunkIndex < texts[j].ChunkIndex })
	var builder strings.Builder
	prevEnd := -1
	for _, chunk := range texts {
		runes := []rune(chunk.Content)
		start, end := chunk.StartAt, chunk.EndAt
		if start < 0 || end <= start || end-start != len(runes) {
			return joinTextChunks(texts), false
		}
		if prevEnd >= 0 && start > prevEnd {
			if start-prevEnd >= 2 {
				builder.WriteString("\n\n")
			} else {
				builder.WriteString("\n")
			}
		}
		trim := 0
		if prevEnd > start {
			trim = prevEnd - start
		}
		if trim >= len(runes) {
			return joinTextChunks(texts), false
		}
		builder.WriteString(string(runes[trim:]))
		if end > prevEnd {
			prevEnd = end
		}
	}
	return builder.String(), true
}

func joinTextChunks(chunks []*types.Chunk) string {
	parts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		text := strings.TrimSpace(chunk.Content)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func normalizeHeadingTitle(value string) string {
	text := strings.TrimSpace(value)
	text = strings.TrimRight(text, "#")
	text = strings.TrimSpace(text)
	return strings.Join(strings.Fields(text), " ")
}

type markdownSection struct {
	Level     int
	Title     string
	Start     int // rune offset of the heading line
	BodyStart int // rune offset right after the heading line
	BodyEnd   int // rune offset of the next heading of any level (direct body end)
	End       int // rune offset where the next same-or-higher heading starts
	LineNo    int
}

var markdownHeadingPattern = regexp.MustCompile(`^(#{1,6})[ \t]+(.+?)[ \t]*#*[ \t]*$`)

func parseMarkdownSections(markdown string) []markdownSection {
	runes := []rune(markdown)
	sections := make([]markdownSection, 0)
	lineStart := 0
	lineNo := 0
	inFence := false
	fenceMark := ""
	for i := 0; i <= len(runes); i++ {
		if i < len(runes) && runes[i] != '\n' {
			continue
		}
		lineNo++
		line := string(runes[lineStart:i])
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~"):
			mark := trimmed[:3]
			if !inFence {
				inFence = true
				fenceMark = mark
			} else if mark == fenceMark {
				inFence = false
			}
		case !inFence:
			if match := markdownHeadingPattern.FindStringSubmatch(line); match != nil {
				bodyStart := i + 1
				if bodyStart > len(runes) {
					bodyStart = len(runes)
				}
				sections = append(sections, markdownSection{
					Level:     len(match[1]),
					Title:     normalizeHeadingTitle(match[2]),
					Start:     lineStart,
					BodyStart: bodyStart,
					BodyEnd:   len(runes),
					End:       len(runes),
					LineNo:    lineNo,
				})
			}
		}
		lineStart = i + 1
	}
	for idx := range sections {
		if idx+1 < len(sections) {
			// Body edits (replace/append/prepend) stop at the next heading of
			// any level so child subsections survive; delete removes the whole
			// subtree via End.
			sections[idx].BodyEnd = sections[idx+1].Start
		}
		for next := idx + 1; next < len(sections); next++ {
			if sections[next].Level <= sections[idx].Level {
				sections[idx].End = sections[next].Start
				break
			}
		}
	}
	return sections
}

// locateMarkdownSection finds one heading scope. occurrence defaults to 1;
// ambiguous headings must be disambiguated with level + occurrence.
func locateMarkdownSection(markdown, heading string, level, occurrence int) (*markdownSection, []markdownSection, error) {
	want := normalizeHeadingTitle(strings.TrimLeft(strings.TrimSpace(heading), "#"))
	if want == "" {
		return nil, nil, fmt.Errorf("heading 不能为空")
	}
	sections := parseMarkdownSections(markdown)
	matches := make([]markdownSection, 0, 1)
	for _, section := range sections {
		if section.Title != want {
			continue
		}
		if level > 0 && section.Level != level {
			continue
		}
		matches = append(matches, section)
	}
	if len(matches) == 0 {
		return nil, nil, fmt.Errorf("未找到标题为「%s」的章节", want)
	}
	if occurrence <= 0 {
		occurrence = 1
	}
	if occurrence > len(matches) {
		lines := make([]string, 0, len(matches))
		for _, match := range matches {
			lines = append(lines, fmt.Sprintf("第 %d 行(#%d)", match.LineNo, match.Level))
		}
		return nil, nil, fmt.Errorf(
			"标题「%s」共匹配 %d 处（%s），occurrence 超出范围",
			want, len(matches), strings.Join(lines, "、"),
		)
	}
	if len(matches) > 1 && level == 0 {
		lines := make([]string, 0, len(matches))
		for _, match := range matches {
			lines = append(lines, fmt.Sprintf("第 %d 行(#%d)", match.LineNo, match.Level))
		}
		return nil, nil, fmt.Errorf(
			"标题「%s」有 %d 处（%s），请用 heading_level 与 occurrence 指定目标",
			want, len(matches), strings.Join(lines, "、"),
		)
	}
	chosen := matches[occurrence-1]
	return &chosen, matches, nil
}

func applySectionEdit(markdown string, section *markdownSection, action, content string) (string, error) {
	runes := []rune(markdown)
	body := strings.TrimSpace(content)
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "delete":
		out := string(runes[:section.Start]) + string(runes[section.End:])
		return finalizeMarkdown(out), nil
	case "replace", "append", "prepend":
		if body == "" {
			return "", fmt.Errorf("action=%s 时 content 不能为空", action)
		}
		var builder strings.Builder
		switch strings.ToLower(strings.TrimSpace(action)) {
		case "replace":
			builder.WriteString(strings.TrimRight(string(runes[:section.BodyStart]), "\n"))
			builder.WriteString("\n\n")
			builder.WriteString(body)
			builder.WriteString("\n")
			tail := strings.TrimLeft(string(runes[section.BodyEnd:]), "\n")
			if tail != "" {
				builder.WriteString("\n")
				builder.WriteString(tail)
			}
		case "prepend":
			builder.WriteString(string(runes[:section.BodyStart]))
			builder.WriteString("\n")
			builder.WriteString(body)
			builder.WriteString("\n\n")
			builder.WriteString(string(runes[section.BodyStart:]))
		case "append":
			builder.WriteString(strings.TrimRight(string(runes[:section.BodyEnd]), "\n"))
			builder.WriteString("\n\n")
			builder.WriteString(body)
			builder.WriteString("\n")
			tail := strings.TrimLeft(string(runes[section.BodyEnd:]), "\n")
			if tail != "" {
				builder.WriteString("\n")
				builder.WriteString(tail)
			}
		}
		return finalizeMarkdown(builder.String()), nil
	default:
		return "", fmt.Errorf("不支持的 action：%s（可选 replace/append/prepend/delete）", action)
	}
}

func finalizeMarkdown(markdown string) string {
	text := strings.ReplaceAll(markdown, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimRight(text, "\n") + "\n"
}

// estimateChangedLines counts added/removed lines between two markdown texts.
func estimateChangedLines(oldText, newText string) (added, removed int) {
	counts := map[string]int{}
	for _, line := range strings.Split(oldText, "\n") {
		counts[line]--
	}
	for _, line := range strings.Split(newText, "\n") {
		counts[line]++
	}
	for _, delta := range counts {
		if delta > 0 {
			added += delta
		} else {
			removed -= delta
		}
	}
	return added, removed
}
