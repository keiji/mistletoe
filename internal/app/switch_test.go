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

// setupRepo creates a dummy git repository at the given path with a commit and remote.
func setupRepo(t *testing.T, path, remoteURL string) {
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
	runGit("remote", "add", "origin", remoteURL)

	// Create a commit so HEAD is valid (not unborn)
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

	// Change to tmpDir
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
	setupRepo(t, repo1, "repo1")
	setupRepo(t, repo2, "repo2")

	// Create a config file
	configPath := filepath.Join(tmpDir, "mstl.json")
	repo1Rel := "repo1"
	repo2Rel := "repo2"
	config := conf.Config{
		Repositories: &[]conf.Repository{
			{URL: strPtr("repo1"), ID: &repo1Rel},
			{URL: strPtr("repo2"), ID: &repo2Rel},
		},
	}
	configData, _ := json.Marshal(config)
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Mock Stdout/Stderr and osExit
	var stdoutBuf, stderrBuf bytes.Buffer
	originalStdout, originalStderr := Stdout, Stderr
	originalOsExit := osExit
	defer func() {
		Stdout, Stderr = originalStdout, originalStderr
		osExit = originalOsExit
	}()
	Stdout = &stdoutBuf
	Stderr = &stderrBuf

	// Helper to run handleSwitch with capture
	runHandleSwitch := func(args ...string) (stdout string, stderr string, code int) {
		stdoutBuf.Reset()
		stderrBuf.Reset()

		// Mock Stdin to empty
		Stdin = strings.NewReader("")

		osExit = func(c int) {
			code = c
			panic("os.Exit called")
		}

		defer func() {
			recover()
			stdout = stdoutBuf.String()
			stderr = stderrBuf.String()
		}()

		// Append --ignore-stdin
		fullArgs := append(args, "--ignore-stdin")
		handleSwitch(fullArgs, GlobalOptions{GitPath: "git"})

		stdout = stdoutBuf.String()
		stderr = stderrBuf.String()
		return
	}

	// Scenario 1: Switch to non-existent branch (fail)
	t.Run("Switch NonExistent Strict", func(t *testing.T) {
		_, stderr, code := runHandleSwitch("feature-branch", "--file", configPath)
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr, "missing in repositories") {
			t.Errorf("unexpected stderr: %s", stderr)
		}
	})

	// Scenario 2: Create branch (success)
	t.Run("Switch Create Success", func(t *testing.T) {
		out, _, code := runHandleSwitch("-v", "-c", "feature-branch", "--file", configPath)
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
		// Verify
		verifyBranch(t, repo1, "feature-branch")
		verifyBranch(t, repo2, "feature-branch")
		if !strings.Contains(out, "Creating and switching to branch") {
			t.Logf("Output: %s", out)
		}
	})

	// Scenario 3: Flexible ordering
	t.Run("Switch Flexible Ordering", func(t *testing.T) {
		_, _, code := runHandleSwitch("--file", configPath, "-c", "feature-branch-2")
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
		verifyBranch(t, repo1, "feature-branch-2")
		verifyBranch(t, repo2, "feature-branch-2")
	})

	// Scenario 4: Error - switch branch -c (Ambiguous / Invalid flag usage)
	t.Run("Switch Invalid Flag Position", func(t *testing.T) {
		_, stderr, code := runHandleSwitch("abranch", "-c", "--file", configPath)
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
		// flag package outputs to Stderr
		if !strings.Contains(stderr, "flag needs an argument") && !strings.Contains(stderr, "parse error") && !strings.Contains(stderr, "invalid") {
			// Note: flag.ContinueOnError means ParseFlagsFlexible returns error, and we print "Error parsing flags:"
			if !strings.Contains(stderr, "Error parsing flags") {
				t.Logf("Stderr: %s", stderr)
			}
		}
	})

	// Scenario 5: Error - switch -c branch extra
	t.Run("Switch Extra Args", func(t *testing.T) {
		_, stderr, code := runHandleSwitch("-c", "branch3", "extra", "--file", configPath)
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr, "Unexpected argument: extra") {
			t.Logf("Stderr: %s", stderr)
		}
	})

	// Scenario 6: Mixed - switch branch -c branch2
	t.Run("Switch Ambiguous Mixed", func(t *testing.T) {
		_, stderr, code := runHandleSwitch("branchA", "-c", "branchB", "--file", configPath)
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr, "Unexpected argument: branchA") {
			t.Logf("Stderr: %s", stderr)
		}
	})

	// Scenario 8: Jobs validation
	t.Run("Switch Jobs Invalid", func(t *testing.T) {
		_, stderr, code := runHandleSwitch("b", "-j", "0", "--file", configPath)
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr, "Jobs must be at least") {
			t.Logf("Stderr: %s", stderr)
		}
	})
}

func TestHandleSwitch_ConfigureUpstream(t *testing.T) {
	// 1. Setup Environment
	tmpDir, err := os.MkdirTemp("", "mstl-switch-upstream-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// 2. Setup Remote Bare Repo
	remotePath := filepath.Join(tmpDir, "remote.git")
	if err := exec.Command("git", "init", "--bare", remotePath).Run(); err != nil {
		t.Fatalf("failed to init remote: %v", err)
	}

	// 3. Setup Local Repo (cloned from remote)
	localPath := filepath.Join(tmpDir, "local")
	// Seed remote first
	seedPath := filepath.Join(tmpDir, "seed")
	if err := exec.Command("git", "clone", remotePath, seedPath).Run(); err != nil {
		t.Fatalf("failed to clone seed: %v", err)
	}
	// Commit and push main
	exec.Command("git", "-C", seedPath, "commit", "--allow-empty", "-m", "init").Run()
	exec.Command("git", "-C", seedPath, "push", "origin", "master").Run() // use master for simplicity

	// Create branch 'feature-up' on remote
	exec.Command("git", "-C", seedPath, "checkout", "-b", "feature-up").Run()
	exec.Command("git", "-C", seedPath, "commit", "--allow-empty", "-m", "feature").Run()
	exec.Command("git", "-C", seedPath, "push", "origin", "feature-up").Run()

	// Clone to local
	if err := exec.Command("git", "clone", remotePath, localPath).Run(); err != nil {
		t.Fatalf("failed to clone local: %v", err)
	}

	// 4. Create Config
	configPath := filepath.Join(tmpDir, "mstl.json")
	localRel := "local"
	// Use absolute path for URL because we are in tmpDir
	// But clone uses origin url from .git/config which matches remotePath
	// We need config to point to localPath

	// Wait, the config URL is just an identifier if ID is set, or used for matching.
	// But `ValidateRepositoriesIntegrity` checks if `remote.origin.url` matches config URL.
	// So we must use the correct URL.
	// Since we cloned from `remotePath` (absolute), `git remote get-url origin` will be absolute path to remote.git.
	// We need to use `file://` prefix usually for proper URL parsing validation if it checks schemes,
	// but strictly speaking standard git paths work too.
	// Let's verify what `git remote get-url origin` returns.
	out, _ := exec.Command("git", "-C", localPath, "remote", "get-url", "origin").CombinedOutput()
	remoteURL := strings.TrimSpace(string(out))

	config := conf.Config{
		Repositories: &[]conf.Repository{
			{URL: &remoteURL, ID: &localRel},
		},
	}
	configData, _ := json.Marshal(config)
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// 5. Mock Globals
	var stdoutBuf, stderrBuf bytes.Buffer
	originalStdout, originalStderr := Stdout, Stderr
	originalOsExit := osExit
	defer func() {
		Stdout, Stderr = originalStdout, originalStderr
		osExit = originalOsExit
	}()
	Stdout = &stdoutBuf
	Stderr = &stderrBuf

	osExit = func(c int) {
		panic(c)
	}

	// 6. Run HandleSwitch for 'feature-up'
	// Since it exists on remote, mstl should check it out and set upstream.
	// It does NOT exist locally yet.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panicked with %v. Stderr: %s", r, stderrBuf.String())
		}
	}()

	// We use -c because handleSwitch requires -c to create local branch even if remote exists?
	// Wait, standard `git checkout branch` (without -b) works if it matches a remote.
	// Let's check `switch.go` logic.
	// `handleSwitch` checks `branchExists` locally.
	// If `!create`: it checks `dirExists` (which is local branch existence). If missing, it errors.
	// So `mstl switch feature-up` fails if local branch doesn't exist, even if remote does.
	// Unless we use `-c`.
	// If we use `-c`, it enters the "Create mode" block.
	// Inside "Create mode":
	// If `exists` (local): checkout.
	// Else: checkout -b.

	// Issue: If we do `git checkout -b feature-up`, it creates a new branch.
	// Then `configureUpstreamIfSafe` is called.
	// It fetches `origin feature-up`.
	// Checks if `refs/remotes/origin/feature-up` exists.
	// Checks for conflicts.
	// If safe, `branch --set-upstream-to`.

	// So we must use `-c`.

	// Explicitly verify remote URL before config to match config URL
	out, err = exec.Command("git", "-C", localPath, "remote", "get-url", "origin").CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get remote url: %v", err)
	}
	// Verify that upstream is NOT set before running the command
	// This ensures the command actually performs the action.
	cmdPre := exec.Command("git", "-C", localPath, "config", "--get", "branch.feature-up.remote")
	if outPre, errPre := cmdPre.CombinedOutput(); errPre == nil && len(bytes.TrimSpace(outPre)) > 0 {
		t.Fatalf("upstream should not be set yet, got: %s", outPre)
	}

	// Mock Stdin
	Stdin = strings.NewReader("")

	args := []string{"-c", "feature-up", "--file", configPath, "--ignore-stdin"}
	handleSwitch(args, GlobalOptions{GitPath: "git"})

	// 7. Verify Upstream
	cmd := exec.Command("git", "-C", localPath, "config", "--get", "branch.feature-up.remote")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get config remote: %v", err)
	}
	if strings.TrimSpace(string(out)) != "origin" {
		t.Errorf("expected remote origin, got %s", string(out))
	}

	cmd = exec.Command("git", "-C", localPath, "config", "--get", "branch.feature-up.merge")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get config merge: %v", err)
	}
	if strings.TrimSpace(string(out)) != "refs/heads/feature-up" {
		t.Errorf("expected merge refs/heads/feature-up, got %s", string(out))
	}
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
