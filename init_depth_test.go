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
	binPath := filepath.Join(t.TempDir(), "gitc")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build gitc: %v", err)
	}

	// 2. Setup Remote Repo
	remoteDir := t.TempDir()
	gitInit := exec.Command("git", "init", "--bare", remoteDir)
	if err := gitInit.Run(); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}

	// Create content repo to push to remote
	contentDir := t.TempDir()
	if err := exec.Command("git", "init", contentDir).Run(); err != nil {
		t.Fatalf("failed to init content repo: %v", err)
	}
	if err := exec.Command("git", "-C", contentDir, "config", "user.email", "test@example.com").Run(); err != nil {
		t.Fatalf("failed to configure user.email: %v", err)
	}
	if err := exec.Command("git", "-C", contentDir, "config", "user.name", "Test User").Run(); err != nil {
		t.Fatalf("failed to configure user.name: %v", err)
	}
	if err := exec.Command("git", "-C", contentDir, "remote", "add", "origin", remoteDir).Run(); err != nil {
		t.Fatalf("failed to add remote origin: %v", err)
	}

	// Create 5 commits
	for i := 0; i < 5; i++ {
		fname := filepath.Join(contentDir, "file.txt")
		if err := os.WriteFile(fname, []byte(fmt.Sprintf("commit %d", i)), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		if err := exec.Command("git", "-C", contentDir, "add", "file.txt").Run(); err != nil {
			t.Fatalf("failed to add file: %v", err)
		}
		if err := exec.Command("git", "-C", contentDir, "commit", "-m", fmt.Sprintf("commit %d", i)).Run(); err != nil {
			t.Fatalf("failed to commit: %v", err)
		}
	}
	if err := exec.Command("git", "-C", contentDir, "push", "origin", "master").Run(); err != nil {
		t.Fatalf("failed to push to remote: %v", err)
	}

	// 3. Setup Config
	// Use file:// protocol for --depth to work locally
	repoURL := "file://" + remoteDir
	repoID := "shallow-repo"
	configFile := filepath.Join(t.TempDir(), "repos.json")
	config := Config{
		Repositories: []Repository{
			{URL: repoURL, ID: &repoID, Branch: "master"},
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
