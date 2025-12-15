package app

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestGenerateMistletoeBody(t *testing.T) {
	snapshot := `{"foo":"bar"}`
	snapshotID := "test-id"
	filename := "mistletoe-snapshot-test-id.json"
	urls := []string{"http://example.com/pr/1"}

	body := GenerateMistletoeBody(snapshot, filename, snapshotID, urls)

	if !strings.Contains(body, "## Mistletoe") {
		t.Error("Body missing Mistletoe header")
	}
	if !strings.Contains(body, snapshot) {
		t.Error("Body missing snapshot")
	}
	if !strings.Contains(body, filename) {
		t.Error("Body missing filename")
	}
	if !strings.Contains(body, urls[0]) {
		t.Error("Body missing related url")
	}

	// Check Base64 block
	encoded := base64.StdEncoding.EncodeToString([]byte(snapshot))
	expectedBase64Header := fmt.Sprintf("snapshot-%s-base64.txt", snapshotID)

	if !strings.Contains(body, expectedBase64Header) {
		t.Error("Body missing Base64 file header")
	}
	if !strings.Contains(body, encoded) {
		t.Error("Body missing Base64 content")
	}

	// Check separator logic roughly
	lines := strings.Split(strings.TrimSpace(body), "\n")
	top := lines[0]
	bottom := lines[len(lines)-1]

	n := len(top)
	m := len(bottom)

	var expectedM int
	if n%2 != 0 {
		expectedM = n*2 - 2
	} else {
		expectedM = n*2 - 1
	}

	if m != expectedM {
		t.Errorf("Separator length mismatch. Top=%d, Bottom=%d, ExpectedBottom=%d", n, m, expectedM)
	}
}

func TestEmbedMistletoeBody_Append(t *testing.T) {
	orig := "Original Body"
	block := "\n\n---\n## Mistletoe\nContent\n------\n"

	res := EmbedMistletoeBody(orig, block)
	if !strings.HasSuffix(res, block) {
		t.Error("Should append block")
	}
	if !strings.HasPrefix(res, orig) {
		t.Error("Should keep original")
	}
}

func TestEmbedMistletoeBody_Replace(t *testing.T) {
	// Original has a block
	orig := `Intro

----
## Mistletoe
OldContent
-------

Outro`

	newBlock := "\n\n---\n## Mistletoe\nNewContent\n---\n"
	res := EmbedMistletoeBody(orig, newBlock)

	if strings.Contains(res, "OldContent") {
		t.Error("Old content should be gone")
	}
	if !strings.Contains(res, "NewContent") {
		t.Error("New content should be present")
	}
	if !strings.HasPrefix(res, "Intro") {
		t.Error("Intro should be preserved")
	}
	if !strings.HasSuffix(res, "Outro") {
		t.Error("Outro should be preserved")
	}
}
