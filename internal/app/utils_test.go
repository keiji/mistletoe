package app

import (
	"os/exec"
	"testing"
)

// TestRunGit_Real tests RunGit against the real git command (if available)
// or mocks it if we want unit isolation.
func TestRunGit(t *testing.T) {
	// Simple test to ensure RunGit calls exec correctly
	// We'll assume git is available in the environment since this project depends on it.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	out, err := RunGit("", "git", false, "--version")
	if err != nil {
		t.Fatalf("RunGit failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("RunGit returned empty output")
	}
}

func TestResolveCommonValues(t *testing.T) {
	// Placeholder to preserve file structure if needed, or we can leave it empty/minimal.
}
