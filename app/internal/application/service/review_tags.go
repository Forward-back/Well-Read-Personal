package service

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	werrors "gaze/internal/errors"
	"gaze/internal/types"
)

const reviewMaxTagsPerCard = 10

// normalizeReviewTags trims, rejects blanks, deduplicates case-insensitively
// and enforces the per-card tag limits. The first spelling wins.
func normalizeReviewTags(tags []string) ([]string, error) {
	seen := make(map[string]bool, len(tags))
	cleaned := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return nil, werrors.NewValidationError("标签不能为空")
		}
		if utf8.RuneCountInString(tag) > 32 {
			return nil, werrors.NewValidationError("单个标签不能超过 32 个字符")
		}
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, tag)
	}
	if len(cleaned) > reviewMaxTagsPerCard {
		return nil, werrors.NewValidationError("每张卡片最多 10 个标签")
	}
	return cleaned, nil
}

func reviewTagsJSON(tags []string) types.JSON {
	encoded, _ := json.Marshal(tags)
	return types.JSON(encoded)
}

func reviewTagsFromJSON(raw types.JSON) []string {
	if len(raw) == 0 {
		return nil
	}
	var tags []string
	if err := json.Unmarshal(raw, &tags); err != nil {
		return nil
	}
	return tags
}

// mergeReviewTags appends tags to an existing card tag list, keeping the union
// and enforcing the per-card cap.
func mergeReviewTags(existing []string, extra []string) ([]string, error) {
	merged := append(append([]string{}, existing...), extra...)
	return normalizeReviewTags(merged)
}
