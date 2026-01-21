package app

import (
	conf "mistletoe/internal/config"
	"mistletoe/internal/sys"
)

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleSync(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()

	// Create remote bare repo
	remoteURL, contentDir := setupRemoteAndContent(t, 1)

	// Create clones
	repo1 := filepath.Join(tmpDir, "repo1")
	exec.Command("git", "clone", remoteURL, repo1).Run()
	configureGitUser(t, repo1)

	repo2 := filepath.Join(tmpDir, "repo2")
	exec.Command("git", "clone", remoteURL, repo2).Run()
	configureGitUser(t, repo2)

	// Config
	master := "master"
	repo1Rel := "repo1"
	repo2Rel := "repo2"
	config := conf.Config{
		Repositories: &[]conf.Repository{
			{URL: strPtr(remoteURL), ID: &repo1Rel, Branch: &master},
			{URL: strPtr(remoteURL), ID: &repo2Rel, Branch: &master},
		},
	}
	configPath := filepath.Join(tmpDir, "mstl.json")
	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0644)

	// Mock Stdout/Stderr/Stdin
	var stdoutBuf, stderrBuf bytes.Buffer
	originalStdout, originalStderr, originalStdin := sys.Stdout, sys.Stderr, sys.Stdin
	defer func() {
		sys.Stdout, sys.Stderr, sys.Stdin = originalStdout, originalStderr, originalStdin
	}()
	sys.Stdout = &stdoutBuf
	sys.Stderr = &stderrBuf

	runHandleSync := func(input string, args ...string) (stdout string, stderr string, err error) {
		stdoutBuf.Reset()
		stderrBuf.Reset()

		if input != "" {
			sys.Stdin = strings.NewReader(input)
		} else {
			sys.Stdin = strings.NewReader("")
		}

		// Ensure args has --file
		fullArgs := append(args, "--file", configPath)
		// Change CWD to tmpDir for relative path resolution
		cwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(cwd)

		err = handleSync(fullArgs, GlobalOptions{GitPath: "git"})

		stdout = stdoutBuf.String()
		stderr = stderrBuf.String()
		return
	}

	// Scenario 1: Clean Sync
	t.Run("CleanSync", func(t *testing.T) {
		out, _, err := runHandleSync("", "--ignore-stdin")
		if err != nil {
			t.Errorf("expected success, got error: %v", err)
		}
		if !strings.Contains(out, fmt.Sprintf("[%s] Skipping: Already up to date.", repo1Rel)) {
			t.Errorf("Expected Skipping repo1 output. Got: %s", out)
		}
	})

	// Scenario 2: Pull Needed
	t.Run("PullNeeded", func(t *testing.T) {
		// Update remote
		newFile := filepath.Join(contentDir, "file-new.txt")
		os.WriteFile(newFile, []byte("new content"), 0644)
		exec.Command("git", "-C", contentDir, "add", ".").Run()
		exec.Command("git", "-C", contentDir, "commit", "-m", "new commit").Run()
		exec.Command("git", "-C", contentDir, "push", "origin", "master").Run()

		out, _, _ := runHandleSync("", "--ignore-stdin")

		if !strings.Contains(out, "Updates available.") {
			t.Error("Expected pull needed message")
		}
		// Verify
		if _, err := os.Stat(filepath.Join(repo1, "file-new.txt")); os.IsNotExist(err) {
			t.Error("repo1 did not pull new file")
		}
	})

	// Scenario 3: Diverged (Merge)
	t.Run("Diverged", func(t *testing.T) {
		// Create diverge
		divergeFile := filepath.Join(contentDir, "file-diverge.txt")
		os.WriteFile(divergeFile, []byte("remote change"), 0644)
		exec.Command("git", "-C", contentDir, "add", ".").Run()
		exec.Command("git", "-C", contentDir, "commit", "-m", "Remote Diverge").Run()
		exec.Command("git", "-C", contentDir, "push", "origin", "master").Run()

		localDiverge := filepath.Join(repo1, "file-local.txt")
		os.WriteFile(localDiverge, []byte("local change"), 0644)
		exec.Command("git", "-C", repo1, "add", ".").Run()
		exec.Command("git", "-C", repo1, "commit", "-m", "Local Diverge").Run()

		// Input "merge"
		out, _, err := runHandleSync("merge\n", "--ignore-stdin")
		if err != nil {
			t.Errorf("expected success, got error: %v", err)
		}
		if !strings.Contains(out, "Syncing") {
			t.Log(out)
		}
	})

	// Scenario 4: Conflict (Fail)
	t.Run("Conflict", func(t *testing.T) {
		// Create conflict
		conflictFile := filepath.Join(contentDir, "conflict.txt")
		os.WriteFile(conflictFile, []byte("Version A"), 0644)
		exec.Command("git", "-C", contentDir, "add", ".").Run()
		exec.Command("git", "-C", contentDir, "commit", "-m", "Conflict A").Run()
		exec.Command("git", "-C", contentDir, "push", "origin", "master").Run()

		localConflict := filepath.Join(repo1, "conflict.txt")
		os.WriteFile(localConflict, []byte("Version B"), 0644)
		exec.Command("git", "-C", repo1, "add", ".").Run()
		exec.Command("git", "-C", repo1, "commit", "-m", "Conflict B").Run()

		// Pass --ignore-stdin explicitly
		out, stderr, err := runHandleSync("merge\n", "--ignore-stdin")
		if err == nil {
			t.Error("expected error for conflict, got nil")
		}
		// Check error message or stderr output
		if !strings.Contains(stderr, "Error pulling") && !strings.Contains(err.Error(), "Error pulling") {
			t.Logf("Stdout: %s\nStderr: %s\nError: %v", out, stderr, err)
		}
	})

	// Scenario 5: Flag Unknown
	t.Run("Flag_Unknown", func(t *testing.T) {
		out, _, err := runHandleSync("", "--unknown")
		if err == nil {
			t.Error("expected error, got nil")
		} else if !strings.Contains(err.Error(), "flag provided but not defined") {
			t.Errorf("Expected flag error. Got: %v", err)
		}
		_ = out
	})

	// Scenario 6: Flag Duplicate
	t.Run("Flag_Duplicate", func(t *testing.T) {
		out, _, err := runHandleSync("", "-j", "1", "--jobs", "2")
		if err == nil {
			t.Error("expected error, got nil")
		} else if !strings.Contains(err.Error(), "cannot be specified with different values") {
			t.Errorf("Expected duplicate flag error. Got: %v", err)
		}
		_ = out
	})

	// Scenario 7: Diverged (Yes Flag - Auto Merge)
	t.Run("Diverged_YesFlag", func(t *testing.T) {
		// Reset repo1 state to be diverged again
		// (Assuming we are reusing the setup, but repo1 was modified in previous tests.
		// It is safer to re-setup or ensure we are clean. The previous tests ran sequentially.)

		// Let's create a NEW repo/setup for this specific test to avoid side effects
		// or just perform new operations on the existing one if we track state.
		// "Diverged" test left repo1 in a synced state?
		// No, "Diverged" test ran "merge" and presumably succeeded?
		// It calls RunGitInteractive with "pull --no-rebase".
		// We need to create a new divergence.

		// Create a new file for divergence
		divergeFile2 := filepath.Join(contentDir, "file-diverge2.txt")
		os.WriteFile(divergeFile2, []byte("remote change 2"), 0644)
		exec.Command("git", "-C", contentDir, "add", ".").Run()
		exec.Command("git", "-C", contentDir, "commit", "-m", "Remote Diverge 2").Run()
		exec.Command("git", "-C", contentDir, "push", "origin", "master").Run()

		localDiverge2 := filepath.Join(repo1, "file-local2.txt")
		os.WriteFile(localDiverge2, []byte("local change 2"), 0644)
		exec.Command("git", "-C", repo1, "add", ".").Run()
		exec.Command("git", "-C", repo1, "commit", "-m", "Local Diverge 2").Run()

		// Run with -y (and no input needed)
		out, _, err := runHandleSync("", "-y", "--ignore-stdin")
		if err != nil {
			t.Errorf("expected success, got error: %v", err)
		}
		if !strings.Contains(out, "Using default strategy (merge) due to --yes flag") {
			t.Errorf("Expected auto-merge message. Got: %s", out)
		}
	})
}
