package app

import (
	"strings"
	"testing"
)

func TestGenerateMistletoeBody(t *testing.T) {
	// Generate multiple times to cover random range
	for i := 0; i < 50; i++ {
		body := GenerateMistletoeBody("{}", "file.json", nil)
		lines := strings.Split(body, "\n")
		// Filter out empty lines caused by \n\n splits
		var nonEmpty []string
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				nonEmpty = append(nonEmpty, l)
			}
		}

		// The first non-empty line should be top separator
		// The last non-empty line should be bottom separator
		// We expect structure:
		// Top Sep
		// ## Mistletoe
		// ...
		// Bottom Sep

		if len(nonEmpty) < 2 {
			t.Fatalf("Generated body too short: %s", body)
		}

		topSep := nonEmpty[0]
		bottomSep := nonEmpty[len(nonEmpty)-1]

		// Check basic validity (all dashes)
		if strings.Trim(topSep, "-") != "" {
			t.Errorf("Top separator not all dashes: %q", topSep)
		}
		if strings.Trim(bottomSep, "-") != "" {
			t.Errorf("Bottom separator not all dashes: %q", bottomSep)
		}

		n := len(topSep)
		bottomLen := len(bottomSep)

		// Verify N range [4, 16]
		if n < 4 || n > 16 {
			t.Errorf("Top separator length %d out of range [4, 16]", n)
		}

		// Verify logic
		// If n is odd: bottom = n*2 - 2
		// If n is even: bottom = n*2 - 1
		var expectedBottom int
		if n%2 != 0 {
			expectedBottom = n*2 - 2
		} else {
			expectedBottom = n*2 - 1
		}

		if bottomLen != expectedBottom {
			t.Errorf("For top length %d (odd=%v), expected bottom %d, got %d", n, n%2 != 0, expectedBottom, bottomLen)
		}
	}
}

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
