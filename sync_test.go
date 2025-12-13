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
	binPath := buildMstl(t)

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
	configPath := filepath.Join(tmpDir, "mstl.json")
	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0644)

	// Helper to run sync with input
	runSync := func(input string, extraArgs ...string) (string, error) {
		args := []string{"sync", "--file", configPath}
		args = append(args, extraArgs...)
		cmd := exec.Command(binPath, args...)
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

	// Scenario 1b: Clean Sync with Parallel (Check flag parsing)
	t.Run("CleanSyncParallel", func(t *testing.T) {
		out, err := runSync("", "-p", "2")
		if err != nil {
			t.Fatalf("sync failed: %v, out: %s", err, out)
		}
		// Output should be similar
		if !strings.Contains(out, fmt.Sprintf("Syncing %s...", repo1)) {
			t.Errorf("Expected Syncing repo1 output. Got: %s", out)
		}
	})

	// Scenario 2: Pull Needed (No conflict, Fast Forward)
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
		// Run sync with empty input (expecting no prompt for fast-forward)
		out, err := runSync("")
		if err != nil {
			t.Fatalf("sync failed: %v, out: %s", err, out)
		}
		if !strings.Contains(out, "Updates available.") {
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

	// Scenario 3: Diverged (Merge Needed)
	t.Run("Diverged", func(t *testing.T) {
		// Create a diverged state.
		// Remote has new commit.
		divergeFile := filepath.Join(contentDir, "file-diverge.txt")
		os.WriteFile(divergeFile, []byte("remote change"), 0644)
		exec.Command("git", "-C", contentDir, "add", ".").Run()
		exec.Command("git", "-C", contentDir, "commit", "-m", "Remote Diverge").Run()
		exec.Command("git", "-C", contentDir, "push", "origin", "master").Run()

		// Local repo1 has new commit (different)
		localDiverge := filepath.Join(repo1, "file-local.txt")
		os.WriteFile(localDiverge, []byte("local change"), 0644)
		exec.Command("git", "-C", repo1, "add", ".").Run()
		exec.Command("git", "-C", repo1, "commit", "-m", "Local Diverge").Run()

		// Run sync with "merge" input (it should prompt)
		out, err := runSync("merge")
		if err != nil {
			t.Fatalf("sync failed: %v, out: %s", err, out)
		}

		// Verify it pulled and merged
		if _, err := os.Stat(filepath.Join(repo1, "file-diverge.txt")); os.IsNotExist(err) {
			t.Error("repo1 did not pull remote file")
		}
	})

	// Scenario 4: Conflict
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
		// Expect failure due to conflict (exit 1)
		out, err := runSync("merge")
		if err == nil {
			t.Fatal("Expected sync to fail due to conflict, but it succeeded")
		}

		if strings.Contains(out, fmt.Sprintf("Skipping %s due to detected conflict.", repo1)) {
			t.Errorf("Did NOT expect skipping message for repo1. Got: %s", out)
		}

		if !strings.Contains(out, fmt.Sprintf("Syncing %s...", repo1)) {
			t.Errorf("Expected Syncing message for repo1. Got: %s", out)
		}

		if !strings.Contains(out, fmt.Sprintf("Error pulling %s", repo1)) {
			t.Errorf("Expected Error pulling message for repo1. Got: %s", out)
		}
	})
}
