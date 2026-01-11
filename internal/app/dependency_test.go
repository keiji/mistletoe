package app

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLoadDependencies(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "deps-*.md")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `
graph TD
    A --> B
    B --> C
    D
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	validIDs := []string{"A", "B", "C", "D"}

	graph, err := LoadDependencies(tmpFile.Name(), validIDs)
	if err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	// Verify graph structure
	expectedForward := map[string][]string{
		"A": {"B"},
		"B": {"C"},
	}

	// We only check what we expect. D has no dependencies.
	for k, v := range expectedForward {
		if !reflect.DeepEqual(graph.Forward[k], v) {
			t.Errorf("Forward[%s] = %v, want %v", k, graph.Forward[k], v)
		}
	}
}

func TestLoadDependencies_FileNotFound(t *testing.T) {
	_, err := LoadDependencies("non_existent_file.md", []string{})
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

func TestParseDependencies_Complex(t *testing.T) {
	content := `
graph TD
    %% This is a comment
    A[Service A] -->|calls| B(Service B)
    B -.-> C{Database}
    C == return ==> B
    D <--> E
`
	validIDs := []string{"A", "B", "C", "D", "E"}

	graph, err := ParseDependencies(content, validIDs)
	if err != nil {
		t.Fatalf("ParseDependencies failed: %v", err)
	}

	// The logic extracts just the ID part, so A[Service A] becomes A.
	// A -> B
	if !contains(graph.Forward["A"], "B") {
		t.Errorf("A should depend on B. Forward[A] = %v", graph.Forward["A"])
	}
	// B -> C
	if !contains(graph.Forward["B"], "C") {
		t.Errorf("B should depend on C. Forward[B] = %v", graph.Forward["B"])
	}
	// C -> B (== return ==>)
	if !contains(graph.Forward["C"], "B") {
		t.Errorf("C should depend on B. Forward[C] = %v", graph.Forward["C"])
	}
	// D <--> E
	if !contains(graph.Forward["D"], "E") {
		t.Errorf("D should depend on E. Forward[D] = %v", graph.Forward["D"])
	}
	if !contains(graph.Forward["E"], "D") {
		t.Errorf("E should depend on D. Forward[E] = %v", graph.Forward["E"])
	}
}

func TestParseDependencies_InvalidID(t *testing.T) {
	content := `A --> Z`
	validIDs := []string{"A", "B"} // Z is missing

	_, err := ParseDependencies(content, validIDs)
	if err == nil {
		t.Error("expected error for invalid ID 'Z', got nil")
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func TestFilterDependencyContent(t *testing.T) {
	input := `
graph TD
    A --> B
    B --> C
    C --> D
    D --> A
    A --> P
    P --> B
    X --- Y
    Y --- Z
    Z --- P
    P[Private Repo]
    A[Public A]
`
	validIDs := []string{"A", "B", "C", "D", "X", "Y", "Z"} // P is private

	got := FilterDependencyContent(input, validIDs)

	// Verify P is completely removed
	if strings.Contains(got, "P") {
		// "P" might appear in "Public A" label? No "Public A" has P.
		// "Private Repo" has P.
		// We should be careful.
		// "A --> P" contains P.
		// "P --> B" contains P.
		// "Z --- P" contains P.
		// "P[Private Repo]" contains P.

		// If implementation is correct, lines with ID "P" should be gone.
		// However, "P" inside a label of another node (e.g. A[Project P]) is allowed.
		// In this test case, no valid ID uses P in label except "Public A" which has P.
		// "A[Public A]" line should remain.

		// Let's check specific lines removal.
	}

	if !strings.Contains(got, "A --> B") {
		t.Error("Missing A --> B")
	}
	if !strings.Contains(got, "X --- Y") {
		t.Error("Missing X --- Y")
	}
	if strings.Contains(got, "A --> P") {
		t.Error("Should not contain A --> P")
	}
	if strings.Contains(got, "P --> B") {
		t.Error("Should not contain P --> B")
	}
	if strings.Contains(got, "Z --- P") {
		t.Error("Should not contain Z --- P")
	}
	if strings.Contains(got, "P[Private Repo]") {
		t.Error("Should not contain node definition for P")
	}
	if !strings.Contains(got, "A[Public A]") {
		t.Error("Should contain node definition for A")
	}
}

func TestFilterDependencyContent_Chained(t *testing.T) {
	input := `
graph TD
    A --> B --> C
    A --> P --> C
`
	validIDs := []string{"A", "B", "C"} // P is private

	// Chained arrows are tricky.
	// The current logic works line by line and extracts 2 IDs.
	// If the line has more than 2 IDs, the current ParseDependencies/FilterDependencyContent logic
	// (which finds ONE arrow match) might only see the first pair.
	// E.g. "A --> B --> C". arrowRe finds "-->". Left is "A", Right is "B --> C".
	// RightID extraction might extract "B".
	// So it sees A -> B.
	// It doesn't process B -> C.

	// If the implementation is simplistic (just checks left and right of first arrow),
	// "A --> P --> C" might be seen as A -> P. And thus filtered out.
	// "A --> B --> C" might be seen as A -> B. And preserved.

	// Let's verify what we get.
	got := FilterDependencyContent(input, validIDs)

	if !strings.Contains(got, "A --> B --> C") {
		t.Error("Should preserve A --> B --> C")
	}
	if strings.Contains(got, "A --> P --> C") {
		t.Error("Should filter A --> P --> C")
	}
}
