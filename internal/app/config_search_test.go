package app

import (
	"fmt"
	"os"
	"os/exec"
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

func TestSearchParentConfig_ParentFound_SwitchContext(t *testing.T) {
	// 1. Setup Workspace Dir (Parent of repo)
	workspaceDir, err := os.MkdirTemp("", "mstl-test-workspace")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workspaceDir)

	// 2. Create .mstl/config.json in workspace
	// Define a repo 'repo-a' which we will run from.
	repoName := "repo-a"
	repoUrl := "https://example.com/repo-a.git"

	configContent := fmt.Sprintf(`{
		"repositories": [
			{
				"id": "%s",
				"url": "%s"
			}
		]
	}`, repoName, repoUrl)

	mstlDir := filepath.Join(workspaceDir, ".mstl")
	if err := os.Mkdir(mstlDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(mstlDir, "config.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Create repo-a inside workspace
	repoPath := filepath.Join(workspaceDir, repoName)
	if err := os.Mkdir(repoPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Init git in repo-a
	runGitCmd(t, repoPath, "init")
	runGitCmd(t, repoPath, "config", "user.email", "test@example.com")
	runGitCmd(t, repoPath, "config", "user.name", "Test User")
	runGitCmd(t, repoPath, "remote", "add", "origin", repoUrl)

	// 4. Run SearchParentConfig from inside repo-a
	// Save original CWD to restore later
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	// Change to repo-a
	if err := os.Chdir(repoPath); err != nil {
		t.Fatal(err)
	}

	// Call SearchParentConfig with yesFlag=true
	// It should find workspace/.mstl/config.json and switch CWD to workspaceDir
	foundPath, err := SearchParentConfig(DefaultConfigFile, nil, "git", true)
	if err != nil {
		t.Fatalf("SearchParentConfig failed: %v", err)
	}

	// 5. Verification
	// Verify path returned is absolute path to workspace/.mstl/config.json
	expectedConfigPath := filepath.Join(workspaceDir, DefaultConfigFile)
	if foundPath != expectedConfigPath {
		t.Errorf("Expected config path %s, got %s", expectedConfigPath, foundPath)
	}

	// Verify CWD switched
	currentWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Resolve symlinks just in case
	evalWorkspaceDir, _ := filepath.EvalSymlinks(workspaceDir)
	evalCurrentWd, _ := filepath.EvalSymlinks(currentWd)

	if evalCurrentWd != evalWorkspaceDir {
		t.Errorf("Expected CWD to switch to %s, but is %s", evalWorkspaceDir, evalCurrentWd)
	}
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		t.Fatalf("git command %v failed in %s: %v", args, dir, err)
	}
}
