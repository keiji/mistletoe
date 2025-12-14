package app

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

// GenerateMistletoeBody creates the structured body content.
func GenerateMistletoeBody(snapshotData string, snapshotFilename string, relatedURLs []string) string {
	// Seed random number generator
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 1. Generate N: random even number [4, 16]
	// (0..6)*2 + 4 => 0+4=4, 12+4=16
	n := rng.Intn(7)*2 + 4

	topSep := strings.Repeat("-", n)
	bottomSep := strings.Repeat("-", n*2+1)

	var sb strings.Builder
	sb.WriteString("\n\n")
	sb.WriteString(topSep + "\n")
	sb.WriteString("## Mistletoe\n")
	sb.WriteString("This content is auto-generated. Manual edits may be lost.\n\n")

	sb.WriteString("### snapshot\n\n")
	sb.WriteString("<details>\n")
	sb.WriteString(fmt.Sprintf("<summary>%s</summary>\n\n", snapshotFilename))
	sb.WriteString("```json\n")
	sb.WriteString(snapshotData)
	sb.WriteString("\n```\n")
	sb.WriteString("</details>\n\n")

	sb.WriteString("### Related Pull Request(s)\n\n")
	if len(relatedURLs) > 0 {
		for _, u := range relatedURLs {
			sb.WriteString(fmt.Sprintf(" * %s\n", u))
		}
	}

	sb.WriteString(bottomSep + "\n")
	return sb.String()
}

// EmbedMistletoeBody replaces existing Mistletoe block or appends new one.
func EmbedMistletoeBody(originalBody, newBlock string) string {
	lines := strings.Split(originalBody, "\n")
	startIdx := -1
	endIdx := -1

	// Regex for Mistletoe header (any level)
	headerRe := regexp.MustCompile(`^#+\s+Mistletoe`)

	// Scan for start
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check if it matches 4-16 dashes, even number
		if len(trimmed) >= 4 && len(trimmed) <= 16 && len(trimmed)%2 == 0 && strings.Count(trimmed, "-") == len(trimmed) {
			// Check next line for header
			if i+1 < len(lines) && headerRe.MatchString(strings.TrimSpace(lines[i+1])) {
				startIdx = i
				// Calculate expected bottom length
				expectedBottomLen := len(trimmed)*2 + 1

				// Scan for end
				for j := i + 2; j < len(lines); j++ {
					t2 := strings.TrimSpace(lines[j])
					if len(t2) == expectedBottomLen && strings.Count(t2, "-") == len(t2) {
						endIdx = j
						break
					}
				}
				if endIdx != -1 {
					break // Found complete block
				}
				// If we found start but no end, we reset startIdx to continue searching or stop?
				// To be safe, if we don't find a matching bottom, we assume it's not a valid block or corrupted.
				startIdx = -1
			}
		}
	}

	if startIdx != -1 && endIdx != -1 {
		pre := lines[:startIdx]
		post := lines[endIdx+1:]

		preStr := strings.TrimRight(strings.Join(pre, "\n"), "\n")
		postStr := strings.TrimLeft(strings.Join(post, "\n"), "\n")

		// Construct result. newBlock already has leading/trailing newlines/spacing usually.
		// We want to ensure clean separation.
		return preStr + newBlock + postStr
	}

	// Not found, append
	return strings.TrimRight(originalBody, "\n") + newBlock
}
