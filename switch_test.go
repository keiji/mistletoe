package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupRepo creates a dummy git repository at the given path with a commit.
func setupRepo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = path
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
	}

	runGit("init")
	// Make sure we are on master or main depending on git version default, or just create a commit.
	// We'll commit a file.
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "initial commit")
}

func TestHandleSwitch(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "gitc-switch-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to tmpDir so that gitc operates relatively if needed (though we use absolute paths in config)
	cwd, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Errorf("failed to restore original working directory: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// Build the gitc binary to test end-to-end (simulating integration test)
	// We need to build it because handleSwitch calls os.Exit on error, which kills the test runner.
	// Alternatively, we could refactor handleSwitch to return error, but for now we follow the pattern
	// or create a subprocess.
	// Looking at memory, existing tests use build.
	binPath := filepath.Join(tmpDir, "gitc")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = cwd // Build from the source root
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build gitc: %v\nOutput: %s", err, out)
	}

	// Create 2 dummy repos
	repo1 := filepath.Join(tmpDir, "repo1")
	repo2 := filepath.Join(tmpDir, "repo2")
	setupRepo(t, repo1)
	setupRepo(t, repo2)

	// Create a config file
	configPath := filepath.Join(tmpDir, "gitc.json")
	config := Config{
		Repositories: []Repository{
			{URL: "repo1", ID: &repo1}, // Using local path as ID to force directory name
			{URL: "repo2", ID: &repo2},
		},
	}
	configData, _ := json.Marshal(config)
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Helper to run gitc
	runGitc := func(args ...string) error {
		// Prepend config file arg
		finalArgs := append([]string{"--file", configPath}, args...)
		cmd := exec.Command(binPath, finalArgs...)
		// Output is not captured unless error, for debugging
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("gitc failed: %v, output: %s", err, out)
		}
		return nil
	}

	runGitcExpectError := func(args ...string) {
		err := runGitc(args...)
		if err == nil {
			t.Fatalf("expected error for args %v, but got nil", args)
		}
	}

	// Scenario 1: Switch to non-existent branch (fail)
	runGitcExpectError("switch", "feature-branch")

	// Scenario 2: Create branch (success)
	if err := runGitc("switch", "-c", "feature-branch"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	// Verify both repos are on feature-branch
	verifyBranch := func(repoPath, expectedBranch string) {
		cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "--short", "HEAD")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("failed to check branch in %s: %v", repoPath, err)
		}
		actual := string(out[:len(out)-1]) // remove newline
		if actual != expectedBranch {
			t.Errorf("repo %s is on %s, expected %s", repoPath, actual, expectedBranch)
		}
	}
	verifyBranch(repo1, "feature-branch")
	verifyBranch(repo2, "feature-branch")

	// Scenario 3: Switch back to master (success, exists)
	// 'master' typically exists from setupRepo
	// Note: 'git init' creates 'master' or 'main'. setupRepo creates 'initial commit'.
	// Let's verify what the default branch is.
	// Since we are creating logic, let's just create another branch on repo1 manually to simulate partial state?
	// No, let's assume 'master' (or whatever default) exists.
	// Let's find out the default branch name first.
	getDefaultBranch := func(repoPath string) string {
		cmd := exec.Command("git", "-C", repoPath, "branch", "--show-current")
		out, _ := cmd.Output()
		return string(out[:len(out)-1]) // trim newline
	}
	defaultBranch := getDefaultBranch(repo1)

	if err := runGitc("switch", defaultBranch); err != nil {
		t.Fatalf("failed to switch back to default branch: %v", err)
	}
	verifyBranch(repo1, defaultBranch)
	verifyBranch(repo2, defaultBranch)

	// Scenario 4: Partial existence with -c
	// Create branch 'partial' on repo1 only manually
	if err := exec.Command("git", "-C", repo1, "branch", "partial").Run(); err != nil {
		t.Fatalf("failed to create partial branch manually: %v", err)
	}

	// Run switch -c partial. Should skip create on repo1, create on repo2.
	if err := runGitc("switch", "-c", "partial"); err != nil {
		t.Fatalf("failed to switch/create partial branch: %v", err)
	}
	verifyBranch(repo1, "partial")
	verifyBranch(repo2, "partial")
}
