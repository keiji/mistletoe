package app

import (
	conf "mistletoe/internal/config"
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

	// Mock Stdout/Stderr/Stdin/osExit
	var stdoutBuf, stderrBuf bytes.Buffer
	originalStdout, originalStderr, originalStdin := Stdout, Stderr, Stdin
	originalOsExit := osExit
	defer func() {
		Stdout, Stderr, Stdin = originalStdout, originalStderr, originalStdin
		osExit = originalOsExit
	}()
	Stdout = &stdoutBuf
	Stderr = &stderrBuf

	runHandleSync := func(input string, args ...string) (stdout string, stderr string, code int) {
		stdoutBuf.Reset()
		stderrBuf.Reset()

		if input != "" {
			Stdin = strings.NewReader(input)
		} else {
			Stdin = strings.NewReader("")
		}

		osExit = func(c int) {
			code = c
			panic("os.Exit called")
		}
		defer func() {
			recover()
			stdout = stdoutBuf.String()
			stderr = stderrBuf.String()
		}()

		// Ensure args has --file
		fullArgs := append(args, "--file", configPath)
		// Change CWD to tmpDir for relative path resolution
		cwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(cwd)

		handleSync(fullArgs, GlobalOptions{GitPath: "git"})

		stdout = stdoutBuf.String()
		stderr = stderrBuf.String()
		return
	}

	// Scenario 1: Clean Sync
	t.Run("CleanSync", func(t *testing.T) {
		out, _, code := runHandleSync("", "--ignore-stdin")
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
		// Logic change: Clean sync now skips pulling
		if !strings.Contains(out, fmt.Sprintf("Skipping %s: Already up to date.", repo1Rel)) {
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
		out, _, code := runHandleSync("merge\n", "--ignore-stdin")
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
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
		out, stderr, code := runHandleSync("merge\n", "--ignore-stdin")
		if code != 1 {
			t.Errorf("expected exit code 1 for conflict, got %d", code)
		}
		if !strings.Contains(stderr, "Error pulling") {
			t.Logf("Stdout: %s\nStderr: %s", out, stderr)
		}
	})
}
