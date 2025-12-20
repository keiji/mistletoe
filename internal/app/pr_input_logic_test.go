package app

import (
	"strings"
	"testing"
)

func TestParsePrTitleBody(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedTitle string
		expectedBody  string
	}{
		{
			name:          "Empty Input",
			input:         "",
			expectedTitle: "",
			expectedBody:  "",
		},
		{
			name:          "Single Line Short",
			input:         "Simple Title",
			expectedTitle: "Simple Title",
			expectedBody:  "",
		},
		{
			name: "Standard Format (Title, Empty, Body)",
			input: `My PR Title

This is the body.
It has multiple lines.`,
			expectedTitle: "My PR Title",
			expectedBody: `This is the body.
It has multiple lines.`,
		},
		{
			name: "No Separator (Title, Body)",
			input: `My PR Title
This is the body immediately.`,
			expectedTitle: "My PR Title",
			expectedBody:  "This is the body immediately.",
		},
		{
			name: "Long Title (Overflow)",
			input: func() string {
				// Create a string > 256 chars
				return strings.Repeat("A", 300) + "\nAnd some body"
			}(),
			expectedTitle: strings.Repeat("A", 253) + "...",
			expectedBody:  strings.Repeat("A", 300) + "\nAnd some body",
		},
		{
			name: "Long Title (Exact Limit)",
			input: func() string {
				// Exact 256 chars
				return strings.Repeat("A", 256) + "\nBody"
			}(),
			// Should NOT truncate if it's exactly 256
			expectedTitle: strings.Repeat("A", 256),
			expectedBody:  "Body",
		},
		{
			name: "Long Title (Limit + 1)",
			input: func() string {
				return strings.Repeat("A", 257) + "\nBody"
			}(),
			expectedTitle: strings.Repeat("A", 253) + "...",
			expectedBody:  strings.Repeat("A", 257) + "\nBody",
		},
		{
			name: "Multi-byte characters (Japanese)",
			input: `日本語のタイトル

本文です。`,
			expectedTitle: "日本語のタイトル",
			expectedBody:  "本文です。",
		},
		{
			name: "Multi-byte characters Long Title",
			input: func() string {
				// 300 Japanese characters
				return strings.Repeat("あ", 300) + "\n本文"
			}(),
			expectedTitle: strings.Repeat("あ", 253) + "...",
			expectedBody:  strings.Repeat("あ", 300) + "\n本文",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, body := ParsePrTitleBody(tt.input)
			if title != tt.expectedTitle {
				t.Errorf("Title mismatch.\nExpected: %q\nGot:      %q", tt.expectedTitle, title)
			}
			if body != tt.expectedBody {
				t.Errorf("Body mismatch.\nExpected: %q\nGot:      %q", tt.expectedBody, body)
			}
		})
	}
}
