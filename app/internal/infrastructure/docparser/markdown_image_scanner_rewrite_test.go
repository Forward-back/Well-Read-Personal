package docparser

import (
	"strings"
	"testing"
)

func TestRewriteMarkdownImageTargets(t *testing.T) {
	input := "before ![arch](img/a.png) mid ![doc](<docs/b.png> \"title\") after ![remote](https://x/y.png)"
	replaced := RewriteMarkdownImageTargets(input, func(target string) (string, bool) {
		switch target {
		case "img/a.png":
			return "local://images/a.png", true
		case "docs/b.png":
			return "local://images/b.png", true
		default:
			return "", false
		}
	})

	if !strings.Contains(replaced, "![arch](local://images/a.png)") {
		t.Fatalf("relative image not rewritten: %s", replaced)
	}
	if !strings.Contains(replaced, "![doc](<local://images/b.png> \"title\")") {
		t.Fatalf("angle destination/title not preserved: %s", replaced)
	}
	if !strings.Contains(replaced, "remote") || strings.Contains(replaced, "https://x/y.png") {
		t.Fatalf("unresolved image should fall back to alt text: %s", replaced)
	}
	if !strings.Contains(replaced, "before ") || !strings.Contains(replaced, " after ") {
		t.Fatalf("surrounding text lost: %s", replaced)
	}
}

func TestRewriteMarkdownImageTargetsKeepsUnmatchedText(t *testing.T) {
	input := "no images here\n"
	if got := RewriteMarkdownImageTargets(input, func(string) (string, bool) { return "", false }); got != input {
		t.Fatalf("expected input unchanged, got %q", got)
	}
}
