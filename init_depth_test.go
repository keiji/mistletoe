package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to check commit count in a git repo
func checkCommitCount(t *testing.T, dir string, expected int) {
	cmd := exec.Command("git", "-C", dir, "rev-list", "--count", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to count commits in %s: %v", dir, err)
	}
	count := strings.TrimSpace(string(out))
	if count != fmt.Sprintf("%d", expected) {
		t.Errorf("expected %d commits, got %s", expected, count)
	}
}

// TestHandleInitDepth tests the --depth flag integration.
// Since handleInit calls os.Exit on error and performs complex side effects,
// we will integration test it by building the binary and running it.
// This is cleaner than refactoring handleInit to be testable in the short term.
func TestHandleInitDepth(t *testing.T) {
	// 1. Build gitc binary
	binPath := buildGitc(t)

	// 2. Setup Remote Repo
	repoURL, _ := setupRemoteAndContent(t, 5)

	// 3. Setup Config
	// Use file:// protocol for --depth to work locally
	repoID := "shallow-repo"
	configFile := filepath.Join(t.TempDir(), "repos.json")
	master := "master"
	config := Config{
		Repositories: &[]Repository{
			{URL: &repoURL, ID: &repoID, Branch: &master},
		},
	}
	configBytes, _ := json.Marshal(config)
	if err := os.WriteFile(configFile, configBytes, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// 4. Run gitc init --depth 2
	workDir := t.TempDir()
	cmd := exec.Command(binPath, "init", "--file", configFile, "--depth", "2")
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("gitc init failed: %v", err)
	}

	// 5. Verify Depth
	targetRepo := filepath.Join(workDir, repoID)
	checkCommitCount(t, targetRepo, 2)
}
