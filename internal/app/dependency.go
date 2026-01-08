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
	// It looks for a sequence starting with <, -, or = and ending with >.
	// It handles the "middle" part for labels.
	arrowRe := regexp.MustCompile(`\s*<?(?:--|==|-\.)(?:.*?)>`)

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
