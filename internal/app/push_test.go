package app

import (
	conf "mistletoe/internal/config"
)

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlePush(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()

	// Remote
	remoteURL, _ := setupRemoteAndContent(t, 2)
	id1 := "repo1"
	repoPath := filepath.Join(tmpDir, id1)
	exec.Command("git", "clone", remoteURL, repoPath).Run()
	configureGitUser(t, repoPath)

	// Config
	master := "master"
	config := conf.Config{
		Repositories: &[]conf.Repository{
			{ID: &id1, URL: &remoteURL, Branch: &master},
		},
	}
	configPath := filepath.Join(tmpDir, "repos.json")
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

	runHandlePush := func(input string, args ...string) (stdout string, stderr string, code int) {
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

		fullArgs := append(args, "--file", configPath)
		cwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(cwd)

		handlePush(fullArgs, GlobalOptions{GitPath: "git"})

		stdout = stdoutBuf.String()
		stderr = stderrBuf.String()
		return
	}

	// Scenario 1: No Push Needed
	t.Run("NoPushNeeded", func(t *testing.T) {
		out, _, code := runHandlePush("", "--ignore-stdin")
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
		if !strings.Contains(out, "No repositories to push") {
			t.Errorf("Unexpected output: %s", out)
		}
	})

	// Scenario 2: Push Needed - Yes
	t.Run("PushYes", func(t *testing.T) {
		// Make commit
		fname := filepath.Join(repoPath, "new.txt")
		os.WriteFile(fname, []byte("new"), 0644)
		exec.Command("git", "-C", repoPath, "add", ".").Run()
		exec.Command("git", "-C", repoPath, "commit", "-m", "unpushed").Run()

		out, _, code := runHandlePush("y\n", "--ignore-stdin") // input piped
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
		if !strings.Contains(out, "Pushing repo1") {
			t.Errorf("Expected Pushing repo1. Got: %s", out)
		}

		// Verify remote
		cmdCheck := exec.Command("git", "-C", repoPath, "ls-remote", "origin", "HEAD")
		outCheck, _ := cmdCheck.Output()
		remoteSHA := strings.Fields(string(outCheck))[0]

		cmdLocal := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
		outLocal, _ := cmdLocal.Output()
		localSHA := strings.TrimSpace(string(outLocal))

		if remoteSHA != localSHA {
			t.Errorf("Remote SHA mismatch")
		}
	})

	// Scenario 3: Push Needed - No
	t.Run("PushNo", func(t *testing.T) {
		// New commit
		fname := filepath.Join(repoPath, "new2.txt")
		os.WriteFile(fname, []byte("new2"), 0644)
		exec.Command("git", "-C", repoPath, "add", ".").Run()
		exec.Command("git", "-C", repoPath, "commit", "-m", "unpushed2").Run()

		out, _, code := runHandlePush("n\n", "--ignore-stdin")
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
		if strings.Contains(out, "Pushing repo1") {
			t.Errorf("Should not push")
		}
	})

	// Scenario 4: Validation Fail (Conflict)
	t.Run("ValidationFail", func(t *testing.T) {
		// Create conflict
		// Remote update
		repoRemotePath := filepath.Join(t.TempDir(), "remote_clone")
		exec.Command("git", "clone", remoteURL, repoRemotePath).Run()
		configureGitUser(t, repoRemotePath)
		os.WriteFile(filepath.Join(repoRemotePath, "conflict.txt"), []byte("A"), 0644)
		exec.Command("git", "-C", repoRemotePath, "add", ".").Run()
		exec.Command("git", "-C", repoRemotePath, "commit", "-m", "A").Run()
		exec.Command("git", "-C", repoRemotePath, "push").Run()

		// Local update
		os.WriteFile(filepath.Join(repoPath, "conflict.txt"), []byte("B"), 0644)
		exec.Command("git", "-C", repoPath, "add", ".").Run()
		exec.Command("git", "-C", repoPath, "commit", "-m", "B").Run()

		// Push should detect conflict in ValidateStatusForAction
		// It prints to Stderr and exits 1
		out, stderr, code := runHandlePush("", "--ignore-stdin")
		if code != 1 {
			t.Errorf("expected exit code 1 for conflict, got %d", code)
		}
		if !strings.Contains(stderr, "has conflicts") {
			t.Logf("Stdout: %s\nStderr: %s", out, stderr)
		}
	})
}
