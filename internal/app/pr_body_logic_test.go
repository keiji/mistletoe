package app

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateMistletoeBody(t *testing.T) {
	snapshot := `{"foo":"bar"}`
	filename := "mistletoe-snapshot-test-id.json"

	currentID := "repo-a"
	allPRs := map[string][]string{
		"repo-a": {"http://example.com/pr/a"},
		"repo-b": {"http://example.com/pr/b"},
	}

	// Test without dependencies (legacy/default behavior)
	body := GenerateMistletoeBody(snapshot, filename, currentID, allPRs, nil, "")

	if !strings.Contains(body, "## Mistletoe") {
		t.Error("Body missing Mistletoe header")
	}

	// Check Order: Related Text -> Snapshot -> Related JSON
	relatedTextIdx := strings.Index(body, "### Related Pull Request(s)")
	snapshotIdx := strings.Index(body, "### snapshot")
	relatedJSONIdx := strings.Index(body, "mistletoe-related-pr-")

	if relatedTextIdx == -1 {
		t.Error("Body missing Related Pull Request(s) section")
	}
	if snapshotIdx == -1 {
		t.Error("Body missing snapshot section")
	}
	if relatedJSONIdx == -1 {
		t.Error("Body missing related PR JSON section")
	}
	if relatedTextIdx > snapshotIdx {
		t.Error("Related Pull Request(s) should be above snapshot")
	}
	if snapshotIdx > relatedJSONIdx {
		t.Error("Snapshot should be above Related PR JSON")
	}

	if !strings.Contains(body, snapshot) {
		t.Error("Body missing snapshot")
	}
	if !strings.Contains(body, filename) {
		t.Error("Body missing snapshot filename")
	}

	// Check Related JSON
	relatedFilename := strings.Replace(filename, "snapshot", "related-pr", 1)
	if !strings.Contains(body, relatedFilename) {
		t.Error("Body missing related pr filename")
	}
	if !strings.Contains(body, "http://example.com/pr/b") {
		t.Error("Body missing related url")
	}
	if strings.Contains(body, "http://example.com/pr/a") {
		t.Error("Body should not contain self url")
	}

	// Check JSON content (flat list -> others)
	if !strings.Contains(body, `"others":`) {
		t.Error("Body missing others key in JSON")
	}

	// Check Base64 block
	encoded := base64.StdEncoding.EncodeToString([]byte(snapshot))
	if !strings.Contains(body, encoded) {
		t.Error("Body missing Base64 content")
	}

	// Check separator logic roughly
	lines := strings.Split(strings.TrimSpace(body), "\n")
	top := lines[0]
	bottom := lines[len(lines)-1]

	n := len(top)
	m := len(bottom)

	var expectedM int
	if n%2 != 0 {
		expectedM = n*2 - 2
	} else {
		expectedM = n*2 - 1
	}

	if m != expectedM {
		t.Errorf("Separator length mismatch. Top=%d, Bottom=%d, ExpectedBottom=%d", n, m, expectedM)
	}
}

func TestGenerateMistletoeBody_WithDependencies(t *testing.T) {
	snapshot := `{"foo":"bar"}`
	filename := "test.json"
	currentID := "repo-main"

	allPRs := map[string][]string{
		"repo-main":  {"url-main"},
		"repo-dep1":  {"url-dep1"},
		"repo-dep2":  {"url-dep2"}, // Depended by main
		"repo-lib":   {"url-lib"},  // Main depends on lib
		"repo-other": {"url-other"},
	}

	deps := &DependencyGraph{
		Forward: map[string][]string{
			"repo-main": {"repo-lib"},
		},
		Reverse: map[string][]string{
			"repo-main": {"repo-dep2"},
		},
	}
	// repo-dep1 is not in graph -> Others

	body := GenerateMistletoeBody(snapshot, filename, currentID, allPRs, deps, "")

	if !strings.Contains(body, "#### Dependencies") {
		t.Error("Missing Dependencies section")
	}
	if !strings.Contains(body, "url-lib") {
		t.Error("Missing url-lib in body")
	}

	if !strings.Contains(body, "#### Dependents") {
		t.Error("Missing Dependents section")
	}
	if !strings.Contains(body, "url-dep2") {
		t.Error("Missing url-dep2 in body")
	}

	if !strings.Contains(body, "#### Others") {
		t.Error("Missing Others section")
	}
	if !strings.Contains(body, "url-other") {
		t.Error("Missing url-other in body")
	}
	if !strings.Contains(body, "url-dep1") {
		t.Error("Missing url-dep1 in body")
	}

	if strings.Contains(body, "url-main") {
		t.Error("Should not contain self url")
	}

	// Verify JSON structure keys exist
	if !strings.Contains(body, `"dependencies":`) {
		t.Error("Missing dependencies key in JSON")
	}
	if !strings.Contains(body, `"dependents":`) {
		t.Error("Missing dependents key in JSON")
	}
	if !strings.Contains(body, `"others":`) {
		t.Error("Missing others key in JSON")
	}
}

func TestGenerateMistletoeBody_WithDependencyContent(t *testing.T) {
	snapshot := "{}"
	filename := "mistletoe-snapshot-test-id.json"
	depContent := "graph TD\nA-->B"
	body := GenerateMistletoeBody(snapshot, filename, "A", nil, nil, depContent)

	if !strings.Contains(body, "<summary>mistletoe-dependencies-test-id.mmd</summary>") {
		t.Error("Missing dependencies summary with correct format")
	}
	if !strings.Contains(body, "```mermaid\n"+depContent) {
		t.Error("Missing mermaid block content")
	}
}

func TestGenerateMistletoeBody_WithDependencyContent_AlreadyWrapped(t *testing.T) {
	snapshot := "{}"
	filename := "mistletoe-snapshot-test-id.json"
	depContent := "```mermaid\ngraph TD\nA-->B\n```"
	body := GenerateMistletoeBody(snapshot, filename, "A", nil, nil, depContent)

	if !strings.Contains(body, "<summary>mistletoe-dependencies-test-id.mmd</summary>") {
		t.Error("Missing dependencies summary with correct format")
	}
	if !strings.Contains(body, depContent) {
		t.Error("Missing content")
	}
	if strings.Contains(body, "```mermaid\n```mermaid") {
		t.Error("Double wrapping detected")
	}
}

func TestEmbedMistletoeBody_Append(t *testing.T) {
	orig := "Original Body"
	block := "\n\n---\n## Mistletoe\nContent\n------\n"

	res := EmbedMistletoeBody(orig, block)
	if !strings.HasSuffix(res, block) {
		t.Error("Should append block")
	}
	if !strings.HasPrefix(res, orig) {
		t.Error("Should keep original")
	}
}

func TestEmbedMistletoeBody_Replace(t *testing.T) {
	// Original has a block
	orig := `Intro

----
## Mistletoe
OldContent
-------

Outro`

	newBlock := "\n\n---\n## Mistletoe\nNewContent\n---\n"
	res := EmbedMistletoeBody(orig, newBlock)

	if strings.Contains(res, "OldContent") {
		t.Error("Old content should be gone")
	}
	if !strings.Contains(res, "NewContent") {
		t.Error("New content should be present")
	}
	if !strings.HasPrefix(res, "Intro") {
		t.Error("Intro should be preserved")
	}
	if !strings.HasSuffix(res, "Outro") {
		t.Error("Outro should be preserved")
	}
}

func TestDependencyCategorization_Verification(t *testing.T) {
	// 1. Setup the scenario from the user prompt
	// Graph:
	// mstl1 -.-> mstl2
	// mstl2 --> mstl3
	// mstl1 --> mstl3

	mermaid := `graph TD
    mstl1["frontend"] -.-> mstl2[backend]
    mstl2 --> mstl3("common")
    mstl1 --> mstl3`

	repoIDs := []string{"mstl1", "mstl2", "mstl3"}

	// Parse the graph to get DependencyGraph
	deps, err := ParseDependencies(mermaid, repoIDs)
	if err != nil {
		t.Fatalf("Failed to parse mermaid: %v", err)
	}

	// 2. Mock PR URLs
	allPRs := map[string][]string{
		"mstl1": {"https://github.com/org/mstl1/pull/1"},
		"mstl2": {"https://github.com/org/mstl2/pull/2"},
		"mstl3": {"https://github.com/org/mstl3/pull/3"},
	}

	// 3. Define Expectations
	tests := []struct {
		RepoID             string
		ExpectDependencies []string // Should appear in Dependencies section
		ExpectDependents   []string // Should appear in Dependents section
		ExpectOthers       []string // Should appear in Others section
	}{
		{
			RepoID: "mstl1",
			// Dependencies: mstl2, mstl3
			ExpectDependencies: []string{"mstl2/pull/2", "mstl3/pull/3"},
			ExpectDependents:   []string{},
		},
		{
			RepoID: "mstl2",
			// Dependencies: mstl3
			// Dependents: mstl1
			ExpectDependencies: []string{"mstl3/pull/3"},
			ExpectDependents:   []string{"mstl1/pull/1"},
		},
		{
			RepoID: "mstl3",
			// Dependents: mstl1, mstl2
			ExpectDependencies: []string{},
			ExpectDependents:   []string{"mstl1/pull/1", "mstl2/pull/2"},
		},
	}

	// 4. Verify each repository
	for _, tc := range tests {
		t.Run(tc.RepoID, func(t *testing.T) {
			body := GenerateMistletoeBody("{}", "snapshot.json", tc.RepoID, allPRs, deps, mermaid)

			// Helper to extract sections
			checkSection := func(name string, expectedURLs []string) {
				// Normalize to ensure we are searching correctly
				// The body uses "#### Name"
				header := "#### " + name
				if len(expectedURLs) == 0 {
					if strings.Contains(body, header) {
						t.Errorf("Section '%s' should not be present, but header found.", name)
					}
					return
				}

				if !strings.Contains(body, header) {
					t.Errorf("Section '%s' missing.", name)
					return
				}

				// Check that each URL is present
				for _, url := range expectedURLs {
					if !strings.Contains(body, url) {
						t.Errorf("In section '%s', missing URL: %s", name, url)
					}
				}
			}

			checkSection("Dependencies", tc.ExpectDependencies)
			checkSection("Dependents", tc.ExpectDependents)
			checkSection("Others", tc.ExpectOthers)

			// Additional check: Ensure items are not miscategorized.
			if len(tc.ExpectDependencies) > 0 && len(tc.ExpectDependents) > 0 {
				depIdx := strings.Index(body, "#### Dependencies")
				deperIdx := strings.Index(body, "#### Dependents")

				if depIdx > deperIdx {
					t.Error("Dependencies section should come before Dependents")
				}

				for _, u := range tc.ExpectDependencies {
					uIdx := strings.Index(body, u)
					if uIdx > deperIdx {
						t.Errorf("URL %s (Dependency) appears after Dependents header", u)
					}
				}

				for _, u := range tc.ExpectDependents {
					uIdx := strings.Index(body, u)
					if uIdx < deperIdx {
						t.Errorf("URL %s (Dependent) appears before Dependents header", u)
					}
				}
			}
		})
	}
}
