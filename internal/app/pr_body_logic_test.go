package app

import (
	"strings"
	"testing"
)

func TestEmbedMistletoeBody(t *testing.T) {
	newBlock := `
----
## Mistletoe
New Content
---------
`

	tests := []struct {
		name         string
		originalBody string
		newBlock     string
		wantContains string // The body should contain this
		wantMissing  string // The body should NOT contain this
	}{
		{
			name:         "Append to empty body",
			originalBody: "",
			newBlock:     newBlock,
			wantContains: "New Content",
		},
		{
			name:         "Append to existing body",
			originalBody: "Existing content",
			newBlock:     newBlock,
			wantContains: "Existing content",
		},
		{
			name: "Replace existing block",
			originalBody: `Existing content

----
## Mistletoe
Old Content
---------

Footer`,
			newBlock:     newBlock,
			wantContains: "New Content",
			wantMissing:  "Old Content",
		},
		{
			name: "Replace existing block with different separator length",
			originalBody: `Existing content

--------
## Mistletoe
Old Content
-----------------

Footer`,
			newBlock:     newBlock,
			wantContains: "New Content",
			wantMissing:  "Old Content",
		},
		{
			name: "Replace even if bottom separator is malformed (too short)",
			originalBody: `Existing content

----
## Mistletoe
Old Content
--------

Footer`,
			newBlock:     newBlock,
			wantContains: "New Content",
			wantMissing:  "Old Content",
		},
		{
			name: "Handle manual edits gracefully (text before header)",
			originalBody: `Existing content
Some manual text
## Mistletoe
Old Content
------
Footer`,
			newBlock:     newBlock,
			wantContains: "New Content",
			wantMissing:  "Old Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EmbedMistletoeBody(tt.originalBody, tt.newBlock)
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("EmbedMistletoeBody() = %v, want to contain %v", got, tt.wantContains)
			}
			if tt.wantMissing != "" && strings.Contains(got, tt.wantMissing) {
				t.Errorf("EmbedMistletoeBody() = %v, want NOT to contain %v", got, tt.wantMissing)
			}
			// For replacement cases, check if "Existing content" and "Footer" are preserved if they existed
			if strings.Contains(tt.originalBody, "Existing content") && !strings.Contains(got, "Existing content") {
				t.Errorf("EmbedMistletoeBody() lost original content")
			}
			if strings.Contains(tt.originalBody, "Footer") && !strings.Contains(got, "Footer") {
				t.Errorf("EmbedMistletoeBody() lost footer content")
			}
		})
	}
}
