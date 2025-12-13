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

func TestHandleSync(t *testing.T) {
	// Setup binary
	binPath := buildGitc(t)

	// Create temp dir
	tmpDir := t.TempDir()

	// Create remote bare repo
	remoteURL, contentDir := setupRemoteAndContent(t, 1) // 1 commit
	// Create another commit on remote so clones are initially behind?
	// No, clone gets HEAD.

	// Let's create two local clones.
	repo1 := filepath.Join(tmpDir, "repo1")
	exec.Command("git", "clone", remoteURL, repo1).Run()
	configureGitUser(t, repo1)

	repo2 := filepath.Join(tmpDir, "repo2")
	exec.Command("git", "clone", remoteURL, repo2).Run()
	configureGitUser(t, repo2)

	// Config
	master := "master"
	config := Config{
		Repositories: &[]Repository{
			{URL: strPtr(remoteURL), ID: &repo1, Branch: &master},
			{URL: strPtr(remoteURL), ID: &repo2, Branch: &master},
		},
	}
	configPath := filepath.Join(tmpDir, "gitc.json")
	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0644)

	// Helper to run sync with input
	runSync := func(input string) (string, error) {
		cmd := exec.Command(binPath, "sync", "--file", configPath)
		if input != "" {
			cmd.Stdin = strings.NewReader(input + "\n")
		}
		// capture stdout and stderr
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// Scenario 1: Clean Sync (No updates needed)
	t.Run("CleanSync", func(t *testing.T) {
		out, err := runSync("")
		if err != nil {
			t.Fatalf("sync failed: %v, out: %s", err, out)
		}
		if !strings.Contains(out, fmt.Sprintf("Syncing %s...", repo1)) {
			t.Errorf("Expected Syncing repo1 output. Got: %s", out)
		}
		if !strings.Contains(out, fmt.Sprintf("Syncing %s...", repo2)) {
			t.Errorf("Expected Syncing repo2 output. Got: %s", out)
		}
	})

	// Scenario 2: Pull Needed (No conflict)
	t.Run("PullNeeded", func(t *testing.T) {
		// Push a new commit from contentDir (origin)
		// contentDir is the "local" repo used to populate remote.
		// remoteURL points to the bare repo.
		// setupRemoteAndContent returns contentDir which is pushed to origin.

		// Update contentDir and push
		newFile := filepath.Join(contentDir, "file-new.txt")
		os.WriteFile(newFile, []byte("new content"), 0644)
		exec.Command("git", "-C", contentDir, "add", ".").Run()
		exec.Command("git", "-C", contentDir, "commit", "-m", "new commit").Run()
		exec.Command("git", "-C", contentDir, "push", "origin", "master").Run() // Assuming master

		// Now repo1 and repo2 are behind.
		// Run sync with "merge" input
		out, err := runSync("merge")
		if err != nil {
			t.Fatalf("sync failed: %v, out: %s", err, out)
		}
		if !strings.Contains(out, "pullが必要なリポジトリがある") {
			t.Error("Expected pull needed message")
		}

		// Verify pull happened
		if _, err := os.Stat(filepath.Join(repo1, "file-new.txt")); os.IsNotExist(err) {
			t.Error("repo1 did not pull new file")
		}
		if _, err := os.Stat(filepath.Join(repo2, "file-new.txt")); os.IsNotExist(err) {
			t.Error("repo2 did not pull new file")
		}
	})

	// Scenario 3: Conflict
	t.Run("Conflict", func(t *testing.T) {
		// Create conflicting changes.
		// Push from contentDir
		conflictFile := filepath.Join(contentDir, "conflict.txt")
		os.WriteFile(conflictFile, []byte("Version A"), 0644)
		exec.Command("git", "-C", contentDir, "add", ".").Run()
		exec.Command("git", "-C", contentDir, "commit", "-m", "Conflict A").Run()
		exec.Command("git", "-C", contentDir, "push", "origin", "master").Run()

		// Local change in repo1
		localConflict := filepath.Join(repo1, "conflict.txt")
		os.WriteFile(localConflict, []byte("Version B"), 0644)
		exec.Command("git", "-C", repo1, "add", ".").Run()
		exec.Command("git", "-C", repo1, "commit", "-m", "Conflict B").Run()

		// Run sync
		out, err := runSync("merge") // Input shouldn't matter as it aborts before asking
		if err == nil {
			t.Fatal("expected error due to conflict")
		}
		if !strings.Contains(out, "コンフリクトしているので処理を中止する") {
			t.Error("Expected conflict message")
		}
	})
}
