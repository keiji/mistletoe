package app

import (
	"strings"
	"testing"
)

func TestGenerateMistletoeBody(t *testing.T) {
	snapshot := "{}"
	filename := "test.json"
	urls := []string{"http://example.com/1", "http://example.com/2"}

	body := GenerateMistletoeBody(snapshot, filename, urls)

	if !strings.Contains(body, "## Mistletoe") {
		t.Error("Body missing header")
	}
	if !strings.Contains(body, "### snapshot") {
		t.Error("Body missing snapshot section")
	}
	if !strings.Contains(body, "### Related Pull Request(s)") {
		t.Error("Body missing related PRs section")
	}
	if !strings.Contains(body, "http://example.com/1") {
		t.Error("Body missing URL 1")
	}

	// Check separators
	lines := strings.Split(strings.TrimSpace(body), "\n")
	top := strings.TrimSpace(lines[0])
	bottom := strings.TrimSpace(lines[len(lines)-1])

	if len(top) < 4 || len(top) > 16 || len(top)%2 != 0 {
		t.Errorf("Invalid top separator length: %d", len(top))
	}

	expectedBottom := len(top)*2 + 1
	if len(bottom) != expectedBottom {
		t.Errorf("Invalid bottom separator length: got %d, want %d", len(bottom), expectedBottom)
	}
}

func TestEmbedMistletoeBody_Append(t *testing.T) {
	original := "Original Content"
	newBlock := "\n\n----\n## Mistletoe\n...\n---------\n"

	result := EmbedMistletoeBody(original, newBlock)

	if !strings.Contains(result, original) {
		t.Error("Original content lost")
	}
	if !strings.HasSuffix(result, newBlock) && !strings.HasSuffix(result, strings.TrimLeft(newBlock, "\n")) {
		// EmbedMistletoeBody might strip newlines between
		if !strings.Contains(result, newBlock) {
			t.Error("New block not appended correctly")
		}
	}
}

func TestEmbedMistletoeBody_Replace(t *testing.T) {
	// Construct an existing body with a block
	top := "----" // 4
	bottom := "---------" // 9
	oldBlock := "\n" + top + "\n## Mistletoe\nOLD CONTENT\n" + bottom + "\n"
	original := "Header\n" + oldBlock + "Footer"

	newBlock := "\n\n------\n## Mistletoe\nNEW CONTENT\n-------------\n" // 6 top, 13 bottom

	result := EmbedMistletoeBody(original, newBlock)

	if strings.Contains(result, "OLD CONTENT") {
		t.Error("Old content should be removed")
	}
	if !strings.Contains(result, "NEW CONTENT") {
		t.Error("New content should be present")
	}
	if !strings.HasPrefix(result, "Header") {
		t.Error("Header preserved")
	}
	if !strings.HasSuffix(result, "Footer") {
		t.Error("Footer preserved")
	}
}
