package service

import (
	"mime"
	"path"
	"strings"

	"gaze/internal/infrastructure/docparser"
)

var markdownImageFileExtensions = map[string]struct{}{
	"png": {}, "jpg": {}, "jpeg": {}, "gif": {}, "webp": {}, "bmp": {}, "tif": {}, "tiff": {},
}

func isMarkdownKnowledgeFileType(fileType string) bool {
	switch strings.ToLower(strings.TrimPrefix(strings.TrimSpace(fileType), ".")) {
	case "md", "markdown":
		return true
	default:
		return false
	}
}

// applyMarkdownImagePolicy enforces the Markdown image contract: images whose
// destinations are already provider URLs (imported from a zip) stay and become
// multimodal inputs; every other image reference (relative path, remote URL,
// data URI) is replaced with its alt text because a lone Markdown file cannot
// carry the referenced bytes.
func applyMarkdownImagePolicy(markdown string) (string, []docparser.StoredImage) {
	images := make([]docparser.StoredImage, 0)
	seen := make(map[string]struct{})
	updated := docparser.RewriteMarkdownImageTargets(markdown, func(target string) (string, bool) {
		if !docparser.IsProviderImageURL(target) {
			return "", false
		}
		clean := target
		if idx := strings.IndexAny(clean, "?#"); idx >= 0 {
			clean = clean[:idx]
		}
		ext := strings.TrimPrefix(strings.ToLower(path.Ext(clean)), ".")
		resourceRef := strings.HasPrefix(target, "resource://")
		if _, ok := markdownImageFileExtensions[ext]; !ok {
			if !resourceRef {
				return "", false
			}
			// resource:// handles are opaque and carry no extension; the
			// multimodal loader detects the real content type from bytes.
			ext = ""
		}
		if _, ok := seen[target]; !ok {
			seen[target] = struct{}{}
			mimeType := ""
			if ext != "" {
				mimeType = mime.TypeByExtension("." + ext)
			}
			images = append(images, docparser.StoredImage{
				OriginalRef: target,
				ServingURL:  target,
				MimeType:    mimeType,
			})
		}
		return target, true
	})
	return updated, images
}
