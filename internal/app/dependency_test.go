package app

import (
	"os"
	"reflect"
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

func TestParseDependencies_InvalidLeftID(t *testing.T) {
	content := `Z --> B`
	validIDs := []string{"A", "B"} // Z is missing

	_, err := ParseDependencies(content, validIDs)
	if err == nil {
		t.Error("expected error for invalid left ID 'Z', got nil")
	}
}

func TestParseDependencies_Malformed(t *testing.T) {
	// These lines should be ignored because IDs cannot be extracted
	content := `
graph TD
    A -->
    --> B
    % --> B
    A --> %
`
	validIDs := []string{"A", "B"}
	graph, err := ParseDependencies(content, validIDs)
	if err != nil {
		t.Fatalf("ParseDependencies failed: %v", err)
	}

	if len(graph.Forward) > 0 {
		t.Errorf("expected empty graph, got %v", graph.Forward)
	}
}

func TestParseDependencies_Duplicate(t *testing.T) {
	content := `
graph TD
    A --> B
    A --> B
`
	validIDs := []string{"A", "B"}
	graph, err := ParseDependencies(content, validIDs)
	if err != nil {
		t.Fatalf("ParseDependencies failed: %v", err)
	}

	if len(graph.Forward["A"]) != 1 {
		t.Errorf("expected 1 dependency for A, got %d", len(graph.Forward["A"]))
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
