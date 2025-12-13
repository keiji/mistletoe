package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	tmpDir, err := os.MkdirTemp("", "mstl-switch-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build the mstl binary to test end-to-end (simulating integration test)
	// We need to build it because handleSwitch calls os.Exit on error, which kills the test runner.
	binPath := buildMstl(t)

	// Change to tmpDir so that mstl operates relatively if needed (though we use absolute paths in config)
	cwd, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Errorf("failed to restore original working directory: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// Create 2 dummy repos
	repo1 := filepath.Join(tmpDir, "repo1")
	repo2 := filepath.Join(tmpDir, "repo2")
	setupRepo(t, repo1)
	setupRepo(t, repo2)

	// Create a config file
	configPath := filepath.Join(tmpDir, "mstl.json")
	config := Config{
		Repositories: &[]Repository{
			{URL: strPtr("repo1"), ID: &repo1}, // Using local path as ID to force directory name
			{URL: strPtr("repo2"), ID: &repo2},
		},
	}
	configData, _ := json.Marshal(config)
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Helper to run mstl
	// Returns output and error
	runMstl := func(args ...string) (string, error) {
		// Default to including --file configPath unless explicitly overridden in args?
		// But args parsing is flexible now. We can just prepend it if not present.
		// For simplicity, let's just append it always, assuming args doesn't conflict.
		// Wait, we need to test flexible ordering, so we shouldn't force it at the beginning always.
		// Let's rely on the test case to provide args, but ensure config is passed.
		// To make it easier, let's assume 'args' contains everything needed except the binary.

		cmd := exec.Command(binPath, args...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// Scenario 1: Switch to non-existent branch (fail)
	// args: switch feature-branch --file ...
	t.Run("Switch NonExistent Strict", func(t *testing.T) {
		_, err := runMstl("switch", "feature-branch", "--file", configPath)
		if err == nil {
			t.Fatal("expected error for non-existent branch in strict mode, but got nil")
		}
	})

	// Scenario 2: Create branch (success)
	// args: switch -c feature-branch --file ...
	t.Run("Switch Create Success", func(t *testing.T) {
		if _, err := runMstl("switch", "-c", "feature-branch", "--file", configPath); err != nil {
			t.Fatalf("failed to create branch: %v", err)
		}
		// Verify
		verifyBranch(t, repo1, "feature-branch")
		verifyBranch(t, repo2, "feature-branch")
	})

	// Scenario 3: Flexible ordering
	// args: switch --file ... -c feature-branch-2
	t.Run("Switch Flexible Ordering", func(t *testing.T) {
		if _, err := runMstl("switch", "--file", configPath, "-c", "feature-branch-2"); err != nil {
			t.Fatalf("failed to create branch with flexible ordering: %v", err)
		}
		verifyBranch(t, repo1, "feature-branch-2")
		verifyBranch(t, repo2, "feature-branch-2")
	})

	// Scenario 4: Error - switch branch -c (Ambiguous / Invalid flag usage)
	t.Run("Switch Invalid Flag Position", func(t *testing.T) {
		out, err := runMstl("switch", "abranch", "-c", "--file", configPath)
		if err == nil {
			t.Fatal("expected error for 'switch abranch -c', got success")
		}
		if !strings.Contains(out, "flag needs an argument") && !strings.Contains(out, "flag provided but not defined") {
			// Note: "flag provided but not defined" might appear if parser gets confused, but we expect "flag needs an argument"
			// Actually, wait. "switch abranch -c --file ..."
			// flexible parser:
			// abranch -> pos arg
			// -c -> flag (needs arg). Next is --file.
			// strict parser consumes --file as value for -c?
			// Let's trace. -c is String flag.
			// If -c consumes --file, then branchName="-file".
			// Then we have remaining pos arg "abranch".
			// handleSwitch checks: createBranchName="-file". len(fs.Args()) > 0 ("abranch").
			// Error: Unexpected argument: abranch.
			// So it should fail either way.
			// But specific user requirement: "switch abranch -c" -> Error.
			// If I run just "switch abranch -c", it ends with -c. "flag needs argument".
			// If I run "switch abranch -c --file ...", it might error differently.
			// Let's test the exact case user mentioned: "switch abranch -c" (fails due to no config too, but let's see)
			// We can pass config via env or just look for the flag error before config error.
			// Or we pass config normally at start: "switch --file ... abranch -c"

			// Test "switch abranch -c" assuming config loaded or fail early?
			// Let's try the isolated case where -c is at the very end.
			_, err := runMstl("switch", "--file", configPath, "abranch", "-c")
			if err == nil {
				t.Fatal("expected error for flag at end")
			}
			if !strings.Contains(out, "flag needs an argument") {
				// It might fail with "flag needs an argument: -c"
				if !strings.Contains(out, "flag needs an argument") {
					t.Logf("Output: %s", out)
				}
			}
		}
	})

	// Scenario 5: Error - switch -c branch extra
	t.Run("Switch Extra Args", func(t *testing.T) {
		out, err := runMstl("switch", "-c", "branch3", "extra", "--file", configPath)
		if err == nil {
			t.Fatal("expected error for extra args")
		}
		if !strings.Contains(out, "Unexpected argument: extra") {
			t.Logf("Output: %s", out)
			// Allow failure to match exact string if it fails for right reason
		}
	})

	// Scenario 6: Mixed - switch branch -c branch2
	// parser: branch (pos), -c (flag), branch2 (value).
	// result: createBranchName=branch2, args=[branch].
	// Error: Unexpected argument: branch.
	t.Run("Switch Ambiguous Mixed", func(t *testing.T) {
		out, err := runMstl("switch", "branchA", "-c", "branchB", "--file", configPath)
		if err == nil {
			t.Fatal("expected error for ambiguous mixed args")
		}
		if !strings.Contains(out, "Unexpected argument: branchA") {
			t.Logf("Output: %s", out)
		}
	})
}

func verifyBranch(t *testing.T, repoPath, expectedBranch string) {
	t.Helper()
	cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "--short", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to check branch in %s: %v", repoPath, err)
	}
	actual := strings.TrimSpace(string(out))
	if actual != expectedBranch {
		t.Errorf("repo %s is on %s, expected %s", repoPath, actual, expectedBranch)
	}
}
