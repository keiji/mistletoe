package app

import (
	"strings"
	"unicode/utf8"
)

// ParsePrTitleBody parses the user input into a PR title and body.
// logic:
// 1. If line 1 length > PrTitleMaxLength:
//    Title = Line 1 truncated to (PrTitleMaxLength-3) + "..."
//    Body = Full Input
// 2. If line 1 is followed by an empty line:
//    Title = Line 1
//    Body = Line 3+
// 3. Otherwise:
//    Title = Line 1
//    Body = Full Input
func ParsePrTitleBody(input string) (string, string) {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	lines := strings.Split(input, "\n")

	if len(lines) == 0 {
		return "", ""
	}

	line1 := lines[0]
	// utf8.RuneCountInString is better for character count than len()
	if utf8.RuneCountInString(line1) > PrTitleMaxLength {
		runes := []rune(line1)
		title := string(runes[:PrTitleMaxLength-3]) + "..."
		return title, input
	}

	if len(lines) > 1 {
		if strings.TrimSpace(lines[1]) == "" {
			// Case: Line 1, Empty Line, Body
			body := strings.Join(lines[2:], "\n")
			return line1, body
		}
		// Case: Line 1, Line 2... (No empty separator)
		// Body = Full Input
		return line1, input
	}

	// Single line
	return line1, ""
}
