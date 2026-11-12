package service

import (
	"strings"
	"testing"
)

func TestApplyMarkdownImagePolicyKeepsProviderImages(t *testing.T) {
	input := "![a](local://images/a.png) ![b](img/b.png) ![c](https://example.com/c.png) ![d](minio://bucket/d.jpg)"
	updated, images := applyMarkdownImagePolicy(input)

	if len(images) != 2 {
		t.Fatalf("expected 2 provider images, got %d", len(images))
	}
	if !strings.Contains(updated, "local://images/a.png") || !strings.Contains(updated, "minio://bucket/d.jpg") {
		t.Fatalf("provider images removed: %s", updated)
	}
	if strings.Contains(updated, "img/b.png") || strings.Contains(updated, "example.com/c.png") {
		t.Fatalf("unresolved images not stripped: %s", updated)
	}
	if !strings.Contains(updated, "b") || !strings.Contains(updated, "c") {
		t.Fatalf("alt text should remain: %s", updated)
	}
}
