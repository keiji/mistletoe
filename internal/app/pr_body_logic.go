package app

import (
	conf "mistletoe/internal/config"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

// RelatedPRsJSON is the struct for related PRs JSON.
type RelatedPRsJSON struct {
	Dependencies []string `json:"dependencies,omitempty"`
	Dependents   []string `json:"dependents,omitempty"`
	Others       []string `json:"others,omitempty"`
}

type relatedPRsJSON = RelatedPRsJSON

// GenerateMistletoeBody creates the structured body content.
// It accepts a map of all related PRs (RepoID -> []PrInfo), an optional dependency graph, and the raw dependency content.
func GenerateMistletoeBody(snapshotData string, snapshotFilename string, currentRepoID string, allPRs map[string][]PrInfo, deps *DependencyGraph, dependencyContent string) string {
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
	targets := make(map[string][]PrInfo)
	for id, items := range allPRs {
		if id != currentRepoID {
			targets[id] = items
		}
	}

	var relatedJSON relatedPRsJSON

	// Prepare lists
	var flatList []PrInfo
	var dependencies []PrInfo
	var dependents []PrInfo
	var others []PrInfo

	if deps == nil {
		for _, items := range targets {
			flatList = append(flatList, items...)
		}
		SortPrs(flatList)
		var urls []string
		for _, item := range flatList {
			urls = append(urls, item.URL)
		}
		relatedJSON.Others = urls
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

		for id, items := range targets {
			isDep := forwardDeps[id]
			isDeper := reverseDeps[id]

			if isDep {
				dependencies = append(dependencies, items...)
			}
			if isDeper {
				dependents = append(dependents, items...)
			}
			if !isDep && !isDeper {
				others = append(others, items...)
			}
		}

		SortPrs(dependencies)
		SortPrs(dependents)
		SortPrs(others)

		extractURLs := func(items []PrInfo) []string {
			var us []string
			for _, i := range items {
				us = append(us, i.URL)
			}
			return us
		}

		relatedJSON.Dependencies = extractURLs(dependencies)
		relatedJSON.Dependents = extractURLs(dependents)
		relatedJSON.Others = extractURLs(others)
	}

	// 1. Related Pull Request(s) Text
	sb.WriteString("### Related Pull Request(s)\n\n")

	if deps == nil {
		if len(flatList) > 0 {
			for _, item := range flatList {
				sb.WriteString(fmt.Sprintf(" * %s\n", item.URL))
			}
		} else {
			sb.WriteString("None\n")
		}
	} else {
		if len(dependencies) > 0 {
			sb.WriteString("#### Dependencies\n")
			for _, item := range dependencies {
				sb.WriteString(fmt.Sprintf(" * %s\n", item.URL))
			}
			sb.WriteString("\n")
		}

		if len(dependents) > 0 {
			sb.WriteString("#### Used by\n")
			for _, item := range dependents {
				sb.WriteString(fmt.Sprintf(" * %s\n", item.URL))
			}
			sb.WriteString("\n")
		}

		if len(others) > 0 {
			if len(dependencies) == 0 && len(dependents) == 0 {
				for _, item := range others {
					sb.WriteString(fmt.Sprintf(" * %s\n", item.URL))
				}
			} else {
				sb.WriteString("#### Related to\n")
				for _, item := range others {
					sb.WriteString(fmt.Sprintf(" * %s\n", item.URL))
				}
			}
		}

		if len(dependencies) == 0 && len(dependents) == 0 && len(others) == 0 {
			sb.WriteString("None\n")
		}
	}
	sb.WriteString("\n")

	// 2. Snapshot
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

	// 3. Dependency Graph
	if dependencyContent != "" {
		// Calculate filename: replace "snapshot" -> "dependencies" and extension .json -> .mmd
		// snapshotFilename is like "mistletoe-snapshot-[identifier].json"
		depFilename := strings.Replace(snapshotFilename, "snapshot", "dependencies", 1)
		depFilename = strings.Replace(depFilename, ".json", ".md", 1)

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

	// 4. Related Pull Request(s) JSON
	relatedFilename := strings.Replace(snapshotFilename, "snapshot", "related-pr", 1)
	sb.WriteString("<details>\n")
	sb.WriteString(fmt.Sprintf("<summary>%s</summary>\n\n", relatedFilename))
	sb.WriteString("```json\n")
	bytes, _ := json.MarshalIndent(relatedJSON, "", "    ")
	sb.WriteString(string(bytes))
	sb.WriteString("\n```\n")
	sb.WriteString("</details>\n\n")

	sb.WriteString(bottomSep + "\n")
	return sb.String()
}

// GeneratePlaceholderMistletoeBody creates a placeholder body content.
func GeneratePlaceholderMistletoeBody() string {
	// Seed random number generator
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate N: random number [4, 16]
	n := rng.Intn(13) + 4

	topSep := strings.Repeat("-", n)

	// Calculate bottom separator length
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
	sb.WriteString("**Work in progress...** (Generation of snapshot and links is pending)\n")
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

// ParseMistletoeBlock extracts the JSON blocks from a Mistletoe-formatted body.
// It returns the decoded snapshot (as a conf.Config struct), raw Related PRs JSON, the raw dependency graph content, and true if Mistletoe block was found.
// If not found, returns nil, nil, "", false.
// If found but data missing/invalid, returns error.
func ParseMistletoeBlock(body string) (*conf.Config, []byte, string, bool) {
	lines := strings.Split(body, "\n")
	startIdx := -1
	endIdx := -1

	headerRe := regexp.MustCompile(`^#+\s+Mistletoe`)

	// 1. Locate Block
	for i, line := range lines {
		if headerRe.MatchString(strings.TrimSpace(line)) {
			startIdx = i
			if i > 0 {
				prev := strings.TrimSpace(lines[i-1])
				if len(prev) >= 3 && strings.Count(prev, "-") == len(prev) {
					startIdx = i - 1
				}
			}
			for j := i + 1; j < len(lines); j++ {
				next := strings.TrimSpace(lines[j])
				if len(next) >= 3 && strings.Count(next, "-") == len(next) {
					endIdx = j
					break
				}
			}
			if endIdx != -1 {
				break
			}
			startIdx = -1
		}
	}

	if startIdx == -1 || endIdx == -1 {
		return nil, nil, "", false
	}

	blockContent := strings.Join(lines[startIdx:endIdx+1], "\n")

	// 2. Extract Snapshot (conf.Config)
	// Look for snapshot filename (mistletoe-snapshot-...) in <summary>
	// Then parse the JSON code block inside that details block.

	detailsRe := regexp.MustCompile(`(?s)<details>(.*?)</details>`)
	matches := detailsRe.FindAllStringSubmatch(blockContent, -1)

	var snapshotConfig *conf.Config
	var relatedPrJSON []byte
	var dependencyContent string

	for _, m := range matches {
		content := m[1]
		if strings.Contains(content, "mistletoe-snapshot-") {
			// Extract JSON
			jsonRe := regexp.MustCompile(`(?s)\x60\x60\x60json\s*(.*?)\s*\x60\x60\x60`)
			jsonMatch := jsonRe.FindStringSubmatch(content)
			if len(jsonMatch) > 1 {
				rawJSON := jsonMatch[1]
				// Decode
				if cfg, err := conf.ParseConfig([]byte(rawJSON)); err == nil {
					snapshotConfig = cfg
				}
			}
		}
		if strings.Contains(content, "mistletoe-related-pr-") {
			// Extract JSON
			jsonRe := regexp.MustCompile(`(?s)\x60\x60\x60json\s*(.*?)\s*\x60\x60\x60`)
			jsonMatch := jsonRe.FindStringSubmatch(content)
			if len(jsonMatch) > 1 {
				relatedPrJSON = []byte(jsonMatch[1])
			}
		}
		if strings.Contains(content, "mistletoe-dependencies-") {
			// Extract Mermaid
			// Look for ```mermaid ... ```
			// Or just the content if no language specified? But Generate uses ```mermaid.
			codeRe := regexp.MustCompile(`(?s)\x60\x60\x60(?:mermaid)?\s*(.*?)\s*\x60\x60\x60`)
			codeMatch := codeRe.FindStringSubmatch(content)
			if len(codeMatch) > 1 {
				dependencyContent = codeMatch[1]
			}
		}
	}

	// Even if data is missing, we found the block structure.
	// But the caller logic uses `found` to mean "safe to overwrite".
	// If it's a mangled block, we should probably still consider it "found" (so we can repair it).
	// So return true.
	return snapshotConfig, relatedPrJSON, dependencyContent, true
}
