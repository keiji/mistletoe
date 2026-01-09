package app

import (
	conf "mistletoe/internal/config"
)

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func getHeadHash(t *testing.T, dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get HEAD hash in %s: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

func getCurrentBranch(t *testing.T, dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get current branch in %s: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

func TestInitRevision(t *testing.T) {
	// 1. Build mstl binary
	binPath := buildMstl(t)

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
	// Configure git user
	if err := exec.Command("git", "-C", contentDir, "config", "user.email", "test@example.com").Run(); err != nil {
		t.Fatalf("failed to configure user.email: %v", err)
	}
	if err := exec.Command("git", "-C", contentDir, "config", "user.name", "Test User").Run(); err != nil {
		t.Fatalf("failed to configure user.name: %v", err)
	}

	if err := exec.Command("git", "-C", contentDir, "remote", "add", "origin", remoteDir).Run(); err != nil {
		t.Fatalf("failed to add remote origin: %v", err)
	}

	// Create 3 commits
	var commits []string
	for i := 0; i < 3; i++ {
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
		commits = append(commits, getHeadHash(t, contentDir))
	}
	if err := exec.Command("git", "-C", contentDir, "push", "origin", "master").Run(); err != nil {
		t.Fatalf("failed to push to remote: %v", err)
	}

	// commits[0] -> first commit
	// commits[1] -> second commit
	// commits[2] -> third commit (master HEAD)

	repoURL := "file://" + remoteDir

	t.Run("Revision specified, no Branch", func(t *testing.T) {
		repoID := "repo-rev-only"
		configFile := filepath.Join(t.TempDir(), "repos.json")
		targetCommit := commits[1] // The middle commit
		config := conf.Config{
			Repositories: &[]conf.Repository{
				{URL: &repoURL, ID: &repoID, Revision: &targetCommit},
			},
		}
		configBytes, _ := json.Marshal(config)
		if err := os.WriteFile(configFile, configBytes, 0644); err != nil {
			t.Fatalf("failed to write config file: %v", err)
		}

		workDir := t.TempDir()
		cmd := exec.Command(binPath, "init", "--file", configFile)
		cmd.Dir = workDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mstl init failed: %v, output: %s", err, out)
		}

		targetRepo := filepath.Join(workDir, repoID)
		currentHash := getHeadHash(t, targetRepo)
		if currentHash != targetCommit {
			t.Errorf("expected hash %s, got %s", targetCommit, currentHash)
		}

		// Verify detached HEAD
		currentBranch := getCurrentBranch(t, targetRepo)
		if currentBranch != "HEAD" {
			t.Errorf("expected detached HEAD, got branch %s", currentBranch)
		}
	})

	t.Run("Revision specified, Branch specified", func(t *testing.T) {
		repoID := "repo-rev-branch"
		configFile := filepath.Join(t.TempDir(), "repos.json")
		targetCommit := commits[0] // The first commit
		targetBranch := "new-feature"
		config := conf.Config{
			Repositories: &[]conf.Repository{
				{URL: &repoURL, ID: &repoID, Revision: &targetCommit, Branch: &targetBranch},
			},
		}
		configBytes, _ := json.Marshal(config)
		if err := os.WriteFile(configFile, configBytes, 0644); err != nil {
			t.Fatalf("failed to write config file: %v", err)
		}

		workDir := t.TempDir()
		cmd := exec.Command(binPath, "init", "--file", configFile)
		cmd.Dir = workDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mstl init failed: %v, output: %s", err, out)
		}

		targetRepo := filepath.Join(workDir, repoID)
		currentHash := getHeadHash(t, targetRepo)
		if currentHash != targetCommit {
			t.Errorf("expected hash %s, got %s", targetCommit, currentHash)
		}

		currentBranch := getCurrentBranch(t, targetRepo)
		if currentBranch != targetBranch {
			t.Errorf("expected branch %s, got %s", targetBranch, currentBranch)
		}
	})

	t.Run("Validation: Branch already exists", func(t *testing.T) {
		repoID := "repo-validation"
		workDir := t.TempDir()

		// Setup existing repo with the branch
		targetRepo := filepath.Join(workDir, repoID)

		// Clone manually first
		if err := exec.Command("git", "clone", repoURL, targetRepo).Run(); err != nil {
			t.Fatalf("failed to clone: %v", err)
		}

		targetBranch := "existing-branch"
		if err := exec.Command("git", "-C", targetRepo, "checkout", "-b", targetBranch).Run(); err != nil {
			t.Fatalf("failed to create branch: %v", err)
		}

		configFile := filepath.Join(t.TempDir(), "repos.json")
		targetCommit := commits[0]
		config := conf.Config{
			Repositories: &[]conf.Repository{
				{URL: &repoURL, ID: &repoID, Revision: &targetCommit, Branch: &targetBranch},
			},
		}
		configBytes, _ := json.Marshal(config)
		if err := os.WriteFile(configFile, configBytes, 0644); err != nil {
			t.Fatalf("failed to write config file: %v", err)
		}

		cmd := exec.Command(binPath, "init", "--file", configFile)
		cmd.Dir = workDir

		// Expect success (using -B behavior)
		if err := cmd.Run(); err != nil {
			t.Fatalf("expected mstl init to succeed, but failed: %v", err)
		}

		// Verify state
		currentHash := getHeadHash(t, targetRepo)
		if currentHash != targetCommit {
			t.Errorf("expected hash %s, got %s", targetCommit, currentHash)
		}

		currentBranch := getCurrentBranch(t, targetRepo)
		if currentBranch != targetBranch {
			t.Errorf("expected branch %s, got %s", targetBranch, currentBranch)
		}
	})
}
