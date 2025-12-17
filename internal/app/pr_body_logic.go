package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"
)

type relatedPRsJSON struct {
	Dependencies []string `json:"dependencies,omitempty"`
	Dependents   []string `json:"dependents,omitempty"`
	Others       []string `json:"others,omitempty"`
}

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

	// Filter out self
	targets := make(map[string]string)
	for id, url := range allPRs {
		if id != currentRepoID {
			targets[id] = url
		}
	}

	var relatedJSON relatedPRsJSON

	// Prepare lists
	var flatList []string
	var dependencies []string
	var dependents []string
	var others []string

	if deps == nil {
		for _, u := range targets {
			flatList = append(flatList, u)
		}
		sort.Strings(flatList)
		relatedJSON.Others = flatList
	} else {
		// Categorize
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

		relatedJSON.Dependencies = dependencies
		relatedJSON.Dependents = dependents
		relatedJSON.Others = others
	}

	// 1. Related Pull Request(s) Text
	sb.WriteString("### Related Pull Request(s)\n\n")

	if deps == nil {
		if len(flatList) > 0 {
			for _, u := range flatList {
				sb.WriteString(fmt.Sprintf(" * %s\n", u))
			}
		}
	} else {
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
	sb.WriteString("\n")

	// 2. Related Pull Request(s) JSON
	relatedFilename := strings.Replace(snapshotFilename, "snapshot", "related-pr", 1)
	sb.WriteString("<details>\n")
	sb.WriteString(fmt.Sprintf("<summary>%s</summary>\n\n", relatedFilename))
	sb.WriteString("```json\n")
	bytes, _ := json.MarshalIndent(relatedJSON, "", "    ")
	sb.WriteString(string(bytes))
	sb.WriteString("\n```\n")
	sb.WriteString("</details>\n\n")

	// 3. Snapshot
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

	// 4. Dependency Graph
	if dependencyContent != "" {
		// Calculate filename: replace "snapshot" -> "dependencies" and extension .json -> .mmd
		// snapshotFilename is like "mistletoe-snapshot-[identifier].json"
		depFilename := strings.Replace(snapshotFilename, "snapshot", "dependencies", 1)
		depFilename = strings.Replace(depFilename, ".json", ".mmd", 1)

		sb.WriteString("<details>\n")
		sb.WriteString(fmt.Sprintf("<summary>%s</summary>\n\n", depFilename))

		trimmed := strings.TrimSpace(dependencyContent)
		if strings.HasPrefix(trimmed, "```mermaid") {
			sb.WriteString(dependencyContent)
			if !strings.HasSuffix(dependencyContent, "\n") {
				sb.WriteString("\n")
			}
		} else {
			sb.WriteString("```mermaid\n")
			sb.WriteString(dependencyContent)
			if !strings.HasSuffix(dependencyContent, "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString("```\n")
		}
		sb.WriteString("</details>\n\n")
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
