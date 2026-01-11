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


// setupRemoteAndContent from common_test.go is assumed to be available
// If not, we copy it here for self-contained test
// But common_test.go is in the same package, so it should be fine if we run `go test ./internal/app/...`

func TestStatusCmd(t *testing.T) {
	// Refactored to unit test logic using handleStatus

	// Create temp dir
	tmpDir := t.TempDir()

	// 1. Validation Error - Wrong Remote
	t.Run("Validation Error - Wrong Remote", func(t *testing.T) {
		repoID := "bad-repo"
		repoPath := filepath.Join(tmpDir, repoID)
		if err := os.Mkdir(repoPath, 0755); err != nil {
			t.Fatal(err)
		}
		exec.Command("git", "-C", repoPath, "init").Run()
		exec.Command("git", "-C", repoPath, "remote", "add", "origin", "https://example.com/wrong.git").Run()

		config := conf.Config{
			Repositories: &[]conf.Repository{
				{ID: &repoID, URL: strPtr("https://example.com/correct.git")},
			},
		}
		configFile := filepath.Join(tmpDir, "repos_bad.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		out, stderr, code := runHandleStatus(t, configFile, tmpDir)

		if code != 1 {
			t.Errorf("Expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr, "different remote origin") {
			t.Errorf("Expected error message, got: %s", stderr)
		}
		_ = out
	})

	// 2. Status Success - Synced and Unpushed
	t.Run("Status Success - Synced and Unpushed", func(t *testing.T) {
		remote1, _ := setupRemoteAndContent(t, 2)
		id1 := "repo1"
		repo1Path := filepath.Join(tmpDir, id1)
		exec.Command("git", "clone", remote1, repo1Path).Run()

		remote2, _ := setupRemoteAndContent(t, 2)
		id2 := "repo2"
		repo2Path := filepath.Join(tmpDir, id2)
		exec.Command("git", "clone", remote2, repo2Path).Run()

		configureGitUser(t, repo2Path)
		fname := filepath.Join(repo2Path, "new.txt")
		os.WriteFile(fname, []byte("new"), 0644)
		exec.Command("git", "-C", repo2Path, "add", ".").Run()
		exec.Command("git", "-C", repo2Path, "commit", "-m", "unpushed").Run()

		config := conf.Config{
			Repositories: &[]conf.Repository{
				{ID: &id1, URL: &remote1},
				{ID: &id2, URL: &remote2},
			},
		}
		configFile := filepath.Join(tmpDir, "repos_good.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		out, _, code := runHandleStatus(t, configFile, tmpDir)
		if code != 0 {
			t.Errorf("Expected exit code 0, got %d", code)
		}

		coloredUnpushed := "\033[32m>\033[0m"
		lines := strings.Split(out, "\n")
		for _, line := range lines {
			if strings.Contains(line, "repo1") && strings.Contains(line, coloredUnpushed) {
				t.Errorf("repo1 should not have unpushed commits")
			}
			if strings.Contains(line, "repo2") && !strings.Contains(line, coloredUnpushed) {
				t.Errorf("repo2 should have unpushed commits")
			}
		}
	})

	// 3. Status Success - Diverged (No Branch conf.Config)
	t.Run("Status Success - Diverged (No Branch conf.Config)", func(t *testing.T) {
		remoteDir, _ := setupRemoteAndContent(t, 1)

		repoID := "diverged-repo"
		localRepoPath := filepath.Join(tmpDir, repoID)
		exec.Command("git", "clone", remoteDir, localRepoPath).Run()

		otherClone := t.TempDir()
		exec.Command("git", "clone", remoteDir, otherClone).Run()
		configureGitUser(t, otherClone)
		exec.Command("git", "-C", otherClone, "commit", "--allow-empty", "-m", "Remote B").Run()
		exec.Command("git", "-C", otherClone, "push").Run()

		configureGitUser(t, localRepoPath)
		exec.Command("git", "-C", localRepoPath, "commit", "--allow-empty", "-m", "Local C").Run()
		exec.Command("git", "-C", localRepoPath, "fetch").Run()

		config := conf.Config{
			Repositories: &[]conf.Repository{
				{ID: &repoID, URL: &remoteDir},
			},
		}
		configFile := filepath.Join(tmpDir, "repos_diverged.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		out, _, code := runHandleStatus(t, configFile, tmpDir)
		if code != 0 {
			t.Errorf("Expected exit code 0, got %d", code)
		}

		coloredUnpushed := "\033[32m>\033[0m"
		coloredPullable := "\033[33m<\033[0m"

		if !strings.Contains(out, coloredUnpushed) {
			t.Errorf("Expected Diverged repo to show '>'")
		}
		if strings.Contains(out, coloredPullable) {
			t.Errorf("Did not expect '<'")
		}
	})

	// 4. Status Success - Pullable Only
	t.Run("Status Success - Pullable Only", func(t *testing.T) {
		remoteDir, _ := setupRemoteAndContent(t, 1)

		repoID := "pull-repo"
		localRepoPath := filepath.Join(tmpDir, repoID)
		exec.Command("git", "clone", remoteDir, localRepoPath).Run()

		otherClone := t.TempDir()
		exec.Command("git", "clone", remoteDir, otherClone).Run()
		configureGitUser(t, otherClone)
		exec.Command("git", "-C", otherClone, "commit", "--allow-empty", "-m", "Remote B").Run()
		exec.Command("git", "-C", otherClone, "push").Run()

		master := "master"
		config := conf.Config{
			Repositories: &[]conf.Repository{
				{ID: &repoID, URL: &remoteDir, Branch: &master},
			},
		}
		configFile := filepath.Join(tmpDir, "repos_pull.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		out, _, code := runHandleStatus(t, configFile, tmpDir)
		if code != 0 {
			t.Errorf("Expected exit code 0, got %d", code)
		}

		coloredPullable := "\033[33m<\033[0m"
		if !strings.Contains(out, coloredPullable) {
			t.Errorf("Expected '<'")
		}
	})

	// 5. Status Success - Diverged with conf.Config
	t.Run("Status Success - Diverged with conf.Config", func(t *testing.T) {
		remoteDir, _ := setupRemoteAndContent(t, 1)

		repoID := "pd-repo"
		localRepoPath := filepath.Join(tmpDir, repoID)
		exec.Command("git", "clone", remoteDir, localRepoPath).Run()

		otherClone := t.TempDir()
		exec.Command("git", "clone", remoteDir, otherClone).Run()
		configureGitUser(t, otherClone)
		exec.Command("git", "-C", otherClone, "commit", "--allow-empty", "-m", "Remote B").Run()
		exec.Command("git", "-C", otherClone, "push").Run()

		configureGitUser(t, localRepoPath)
		exec.Command("git", "-C", localRepoPath, "commit", "--allow-empty", "-m", "Local C").Run()
		exec.Command("git", "-C", localRepoPath, "fetch").Run()

		master := "master"
		config := conf.Config{
			Repositories: &[]conf.Repository{
				{ID: &repoID, URL: &remoteDir, Branch: &master},
			},
		}
		configFile := filepath.Join(tmpDir, "repos_pd.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		out, _, code := runHandleStatus(t, configFile, tmpDir)
		if code != 0 {
			t.Errorf("Expected exit code 0, got %d", code)
		}

		coloredDiverged := "\033[32m>\033[0m\033[33m<\033[0m"
		if !strings.Contains(out, coloredDiverged) {
			t.Errorf("Expected '><'")
		}
	})

	// 6. Status Success - Pullable with Conflict
	t.Run("Status Success - Pullable with Conflict", func(t *testing.T) {
		remoteDir, _ := setupRemoteAndContent(t, 1)

		repoID := "conflict-repo"
		localRepoPath := filepath.Join(tmpDir, repoID)
		exec.Command("git", "clone", remoteDir, localRepoPath).Run()

		otherClone := t.TempDir()
		exec.Command("git", "clone", remoteDir, otherClone).Run()
		configureGitUser(t, otherClone)
		os.WriteFile(filepath.Join(otherClone, "file-0.txt"), []byte("Remote Change"), 0644)
		exec.Command("git", "-C", otherClone, "commit", "-am", "Remote Change").Run()
		exec.Command("git", "-C", otherClone, "push").Run()

		configureGitUser(t, localRepoPath)
		os.WriteFile(filepath.Join(localRepoPath, "file-0.txt"), []byte("Local Change"), 0644)
		exec.Command("git", "-C", localRepoPath, "commit", "-am", "Local Change").Run()

		master := "master"
		config := conf.Config{
			Repositories: &[]conf.Repository{
				{ID: &repoID, URL: &remoteDir, Branch: &master},
			},
		}
		configFile := filepath.Join(tmpDir, "repos_conflict.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		out, _, code := runHandleStatus(t, configFile, tmpDir)
		if code != 0 {
			t.Errorf("Expected exit code 0, got %d", code)
		}

		coloredConflict := "\033[33m!\033[0m"
		coloredUnpushed := "\033[32m>\033[0m"

		if !strings.Contains(out, coloredConflict) {
			t.Errorf("Expected '!'")
		}
		if !strings.Contains(out, coloredUnpushed) {
			t.Errorf("Expected '>'")
		}
	})
}

func runHandleStatus(t *testing.T, configFile, workDir string) (string, string, int) {
	var stdoutBuf, stderrBuf bytes.Buffer
	originalStdout, originalStderr := Stdout, Stderr
	originalOsExit := osExit
	defer func() {
		Stdout, Stderr = originalStdout, originalStderr
		osExit = originalOsExit
	}()
	Stdout = &stdoutBuf
	Stderr = &stderrBuf

	exitCode := 0
	osExit = func(code int) {
		exitCode = code
		panic("os.Exit called")
	}
	defer func() { recover() }()

	cwd, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(cwd)

	handleStatus([]string{"--file", configFile}, GlobalOptions{GitPath: "git"})

	return stdoutBuf.String(), stderrBuf.String(), exitCode
}
