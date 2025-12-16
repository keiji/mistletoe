package app

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"
)

// GenerateMistletoeBody creates the structured body content.
// It accepts a map of all related PRs (RepoID -> URL), an optional dependency graph, and the raw dependency content.
func GenerateMistletoeBody(snapshotData string, snapshotFilename string, currentRepoID string, allPRs map[string]string, deps *DependencyGraph, dependencyContent string) string {
	// Seed random number generator
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate N: random number [4, 16]
	n := rng.Intn(13) + 4

	topSep := strings.Repeat("-", n)

	// Calculate bottom separator length
	// Base is n * 2.
	// If n is odd: n * 2 - 2
	// If n is even: n * 2 - 1
	var bottomLen int
	if n%2 != 0 {
		bottomLen = n*2 - 2
	} else {
		bottomLen = n*2 - 1
	}

	bottomSep := strings.Repeat("-", bottomLen)

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
	sb.WriteString("\n```\n\n")

	// Add Base64 encoded snapshot block
	sb.WriteString("```\n")
	sb.WriteString(base64.StdEncoding.EncodeToString([]byte(snapshotData)))
	sb.WriteString("\n```\n")
	sb.WriteString("</details>\n\n")

	// Add Dependency Graph Block if content is provided
	if dependencyContent != "" {
		sb.WriteString("<details>\n")
		sb.WriteString("<summary>dependencies.mmd</summary>\n\n")
		sb.WriteString("```mermaid\n")
		sb.WriteString(dependencyContent)
		if !strings.HasSuffix(dependencyContent, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("```\n")
		sb.WriteString("</details>\n\n")
	}

	sb.WriteString("### Related Pull Request(s)\n\n")

	// Filter out self
	targets := make(map[string]string)
	for id, url := range allPRs {
		if id != currentRepoID {
			targets[id] = url
		}
	}

	if deps == nil {
		var urls []string
		for _, u := range targets {
			urls = append(urls, u)
		}
		sort.Strings(urls)

		if len(urls) > 0 {
			for _, u := range urls {
				sb.WriteString(fmt.Sprintf(" * %s\n", u))
			}
		}
	} else {
		// Categorize
		var dependencies []string // Depends on
		var dependents []string   // Depended by
		var others []string

		// Prepare sets for fast lookup
		forwardDeps := make(map[string]bool)
		if list, ok := deps.Forward[currentRepoID]; ok {
			for _, id := range list {
				forwardDeps[id] = true
			}
		}

		reverseDeps := make(map[string]bool)
		if list, ok := deps.Reverse[currentRepoID]; ok {
			for _, id := range list {
				reverseDeps[id] = true
			}
		}

		for id, url := range targets {
			isDep := forwardDeps[id]
			isDeper := reverseDeps[id]

			if isDep {
				dependencies = append(dependencies, url)
			}
			if isDeper {
				dependents = append(dependents, url)
			}
			if !isDep && !isDeper {
				others = append(others, url)
			}
		}

		sort.Strings(dependencies)
		sort.Strings(dependents)
		sort.Strings(others)

		if len(dependencies) > 0 {
			sb.WriteString("#### Dependencies\n")
			for _, u := range dependencies {
				sb.WriteString(fmt.Sprintf(" * %s\n", u))
			}
			sb.WriteString("\n")
		}

		if len(dependents) > 0 {
			sb.WriteString("#### Dependents\n")
			for _, u := range dependents {
				sb.WriteString(fmt.Sprintf(" * %s\n", u))
			}
			sb.WriteString("\n")
		}

		if len(others) > 0 {
			sb.WriteString("#### Others\n")
			for _, u := range others {
				sb.WriteString(fmt.Sprintf(" * %s\n", u))
			}
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

	// Scan for header
	for i, line := range lines {
		if headerRe.MatchString(strings.TrimSpace(line)) {
			// Found header at i

			// 1. Determine Start Index (Top Separator)
			// Look at i-1. Is it dashes?
			startIdx = i // Default to start at header if no top separator found
			if i > 0 {
				prev := strings.TrimSpace(lines[i-1])
				// Allow dashes >= 3 to be flexible (MD HR is usually 3 chars)
				if len(prev) >= 3 && strings.Count(prev, "-") == len(prev) {
					startIdx = i - 1
				}
			}

			// 2. Determine End Index (Bottom Separator)
			// Scan from i+1
			for j := i + 1; j < len(lines); j++ {
				next := strings.TrimSpace(lines[j])
				// Allow dashes >= 3
				if len(next) >= 3 && strings.Count(next, "-") == len(next) {
					endIdx = j
					break
				}
			}

			if endIdx != -1 {
				break // Found complete block (Header + Bottom Separator)
			}
			// If we found header but no bottom separator, we reset startIdx because
			// we can't safely identify the end of the block.
			startIdx = -1
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
