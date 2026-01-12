// Package app implements the core application logic.
package app


import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// DependencyGraph holds dependency information between repositories.
type DependencyGraph struct {
	// Forward maps a repo ID to the list of repos it depends on.
	// Key depends on Values.
	Forward map[string][]string
	// Reverse maps a repo ID to the list of repos that depend on it.
	// Key is depended on by Values.
	Reverse map[string][]string
}

// LoadDependencies reads a Markdown file containing a Mermaid graph,
// parses the dependencies, and validates that all nodes correspond to valid repository IDs.
func LoadDependencies(filepath string, validIDs []string) (*DependencyGraph, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read dependency file: %w", err)
	}

	return ParseDependencies(string(content), validIDs)
}

// ParseDependencies parses the Mermaid graph content.
func ParseDependencies(content string, validIDs []string) (*DependencyGraph, error) {
	validIDMap := make(map[string]bool)
	for _, id := range validIDs {
		validIDMap[id] = true
	}

	graph := &DependencyGraph{
		Forward: make(map[string][]string),
		Reverse: make(map[string][]string),
	}

	scanner := bufio.NewScanner(strings.NewReader(content))

	// Regex to split by arrows.
	// Supports:
	// 1. Simple arrows: -->, ---, -.->, ==>
	// 2. Labeled arrows: -- text -->, == text ==>, -. text .->
	// This regex is simplified to catch the arrow part.
	// It looks for a sequence starting with <, -, or = and ending with >? (optional > for ---)
	// Actually --- does not end with >.
	// Let's broaden the regex to support ---
	// \s*<?(?:--|==|-\.)(?:.*)(?:>|-)
	// But simple --- is just ---.
	// Original regex: `\s*<?(?:--|==|-\.)(?:.*?)>`
	//
	// New strategy: Match any sequence that looks like an arrow.
	// Starts with <, -, =. Contains -, =, . Ends with >, -.
	// Simplification: Just look for the connector patterns.
	// -->, ---, -.->, ==>
	// Use alternation to handle arrows ending in > (greedy priority) vs - (fallback).
	arrowRe := regexp.MustCompile(`\s*(?:<?(?:--|==|-\.)(?:.*?)>|<?(?:--|==|-\.)(?:.*?)-)`)

	// Regex to extract ID: start of string, take valid chars
	// Valid mstl IDs: ^[a-zA-Z0-9._-]+$
	idRe := regexp.MustCompile(`^([a-zA-Z0-9._-]+)`)

	lineNum := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNum++
		if line == "" || strings.HasPrefix(line, "%%") || strings.HasPrefix(line, "graph ") || strings.HasPrefix(line, "flowchart ") || strings.HasPrefix(line, "```") {
			continue
		}

		// Find arrow location
		loc := arrowRe.FindStringIndex(line)
		if loc == nil {
			continue
		}

		arrowStr := strings.TrimSpace(line[loc[0]:loc[1]])

		leftRaw := strings.TrimSpace(line[:loc[0]])
		rightRaw := strings.TrimSpace(line[loc[1]:])

		// Handle labels attached to the right side if they were not consumed by arrowRe fully.
		// For example `-->|label| B`.
		// arrowRe above `\s*<?(?:--|==|-\.)(?:.*?)>` catches ` -->` but maybe not `|label|`.
		// Actually, `-->` matches `(?:--|==|-\.)(?:.*?)>`. `-->` starts with `--`, middle is empty, ends with `>`.

		// If rightRaw starts with `|`, it's a label like `|text| ID`.
		if strings.HasPrefix(rightRaw, "|") {
			pipeIdx := strings.Index(rightRaw[1:], "|")
			if pipeIdx != -1 {
				// pipeIdx is index relative to rightRaw[1:], so actual end index is 1 + pipeIdx + 1
				endIdx := pipeIdx + 2
				rightRaw = strings.TrimSpace(rightRaw[endIdx:])
			}
		}

		leftID := extractID(leftRaw, idRe)
		rightID := extractID(rightRaw, idRe)

		if leftID == "" || rightID == "" {
			continue
		}

		// Validation
		if !validIDMap[leftID] {
			return nil, fmt.Errorf("line %d: repository ID '%s' not found in configuration", lineNum, leftID)
		}
		if !validIDMap[rightID] {
			return nil, fmt.Errorf("line %d: repository ID '%s' not found in configuration", lineNum, rightID)
		}

		// Forward: A -> B
		addDependency(graph, leftID, rightID)

		// Check if mutual: starts with `<`
		if strings.HasPrefix(arrowStr, "<") {
			// B -> A
			addDependency(graph, rightID, leftID)
		}
	}

	return graph, nil
}

func extractID(raw string, re *regexp.Regexp) string {
	matches := re.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func addDependency(g *DependencyGraph, from, to string) {
	// Check duplicates
	for _, existing := range g.Forward[from] {
		if existing == to {
			return
		}
	}

	g.Forward[from] = append(g.Forward[from], to)
	g.Reverse[to] = append(g.Reverse[to], from)
}

// FilterDependencyContent filters the Mermaid graph content, removing lines
// that reference invalid repository IDs (e.g. private repositories).
func FilterDependencyContent(content string, validIDs []string) string {
	validIDMap := make(map[string]bool)
	for _, id := range validIDs {
		validIDMap[id] = true
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))

	// Regex matches simplified arrows: -->, ---, -.->, ==>
	// It captures the arrow including potential text in the middle.
	arrowRe := regexp.MustCompile(`\s*(?:<?(?:--|==|-\.)(?:.*?)>|<?(?:--|==|-\.)(?:.*?)-)`)
	// Regex for valid IDs (matches what extractID uses)
	idRe := regexp.MustCompile(`^([a-zA-Z0-9._-]+)`)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Pass through structural lines and empty lines
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") || strings.HasPrefix(trimmed, "graph ") || strings.HasPrefix(trimmed, "flowchart ") || strings.HasPrefix(trimmed, "```") {
			sb.WriteString(line + "\n")
			continue
		}

		// Split line by arrows to get all node parts
		parts := arrowRe.Split(trimmed, -1)

		allValid := true
		for _, part := range parts {
			part = strings.TrimSpace(part)
			// Handle labels attached to the part (e.g., `|label| B` or `B`)
			// If split results in part starting with label pipe, remove it.
			if strings.HasPrefix(part, "|") {
				pipeIdx := strings.Index(part[1:], "|")
				if pipeIdx != -1 {
					part = strings.TrimSpace(part[pipeIdx+2:])
				}
			}

			id := extractID(part, idRe)
			if id != "" {
				// If we found an ID, it must be in the valid map.
				if !validIDMap[id] {
					allValid = false
					break
				}
			}
		}

		if allValid {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}
