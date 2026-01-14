package app

import (
	"os"
	"path/filepath"
	"testing"
)

// Mock Scanln input
var mockScanlnInput string

func init() {
	// Override Stdin for tests if needed, but since we use Scanln which reads from os.Stdin,
	// mocking it is tricky without replacing os.Stdin.
	// We can use a pipe.
}

func TestSearchParentConfig_CurrentDirDefaults(t *testing.T) {
	// Test case: Config exists in current directory. Should return it.
	tmpDir, err := os.MkdirTemp("", "mstl-test-search-config")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, DefaultConfigFile)
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change into tmpDir
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	got, err := SearchParentConfig(DefaultConfigFile, nil, "git", false)
	if err != nil {
		t.Errorf("SearchParentConfig returned error: %v", err)
	}
	if got != DefaultConfigFile {
		t.Errorf("SearchParentConfig = %s, want %s", got, DefaultConfigFile)
	}
}

func TestSearchParentConfig_ExplicitFile(t *testing.T) {
	// Test case: Explicit file provided. Should return it immediately.
	got, err := SearchParentConfig("custom.json", nil, "git", false)
	if err != nil {
		t.Errorf("SearchParentConfig returned error: %v", err)
	}
	if got != "custom.json" {
		t.Errorf("SearchParentConfig = %s, want custom.json", got)
	}
}

func TestSearchParentConfig_Stdin(t *testing.T) {
	// Test case: Stdin provided. Should return input path immediately.
	got, err := SearchParentConfig(DefaultConfigFile, []byte("{}"), "git", false)
	if err != nil {
		t.Errorf("SearchParentConfig returned error: %v", err)
	}
	if got != DefaultConfigFile {
		t.Errorf("SearchParentConfig = %s, want %s", got, DefaultConfigFile)
	}
}

func TestSearchParentConfig_NoGit(t *testing.T) {
	// Test case: Not in git repo. Should return default (to fail later).
	// We assume tmpDir is not a git repo.
	tmpDir, err := os.MkdirTemp("", "mstl-test-no-git")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Mock git to fail or return false
	// Since we are using real git in tests (usually), and tmpDir is fresh, it's not a git repo.
	// Unless the environment has git setup in parent.
	// To be safe, we can mock RunGit or assume environment is clean.
	// For now, let's assume real execution.

	got, err := SearchParentConfig(DefaultConfigFile, nil, "git", false)
	if err != nil {
		// It might fail if git command is not found, but we expect it to return DefaultConfigFile regardless of error inside
		// unless unexpected error.
	}
	if got != DefaultConfigFile {
		t.Errorf("SearchParentConfig = %s, want %s", got, DefaultConfigFile)
	}
}
