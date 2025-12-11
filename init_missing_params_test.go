package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func getHeadHashLocal(t *testing.T, dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get HEAD hash in %s: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

func getCurrentBranchLocal(t *testing.T, dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get current branch in %s: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

func TestInit_MissingBranchAndRevision(t *testing.T) {
	// 1. Build gitc binary
	binPath := filepath.Join(t.TempDir(), "gitc")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

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

	// Create content
	contentDir := t.TempDir()
	if err := exec.Command("git", "init", contentDir).Run(); err != nil {
		t.Fatalf("failed to init content repo: %v", err)
	}
	// Config user
	exec.Command("git", "-C", contentDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", contentDir, "config", "user.name", "Test User").Run()
	exec.Command("git", "-C", contentDir, "remote", "add", "origin", remoteDir).Run()

	// Commit
	os.WriteFile(filepath.Join(contentDir, "file.txt"), []byte("content"), 0644)
	exec.Command("git", "-C", contentDir, "add", "file.txt").Run()
	exec.Command("git", "-C", contentDir, "commit", "-m", "initial").Run()

	// Get branch name before push
	defaultBranch := getCurrentBranchLocal(t, contentDir)

	if err := exec.Command("git", "-C", contentDir, "push", "origin", defaultBranch).Run(); err != nil {
		t.Fatalf("failed to push to remote: %v", err)
	}

	repoURL := "file://" + remoteDir

	t.Run("Missing Branch and Revision", func(t *testing.T) {
		repoID := "repo-defaults"
		configFile := filepath.Join(t.TempDir(), "repos.json")

		// Config with NO Branch and NO Revision
		// We use Repository struct but set fields to defaults
		// When marshaled, omitempty will remove them
		config := Config{
			Repositories: []Repository{
				{URL: repoURL, ID: &repoID}, // Branch and Revision are ""
			},
		}

		data, err := json.Marshal(config)
		if err != nil {
			t.Fatalf("json marshal failed: %v", err)
		}

		// Verify strict absence of fields in JSON
		var rawConfig map[string][]map[string]interface{}
		json.Unmarshal(data, &rawConfig)
		repoMap := rawConfig["repositories"][0]
		if _, ok := repoMap["branch"]; ok {
			t.Error("expected branch to be omitted")
		}
		if _, ok := repoMap["revision"]; ok {
			t.Error("expected revision to be omitted")
		}

		if err := os.WriteFile(configFile, data, 0644); err != nil {
			t.Fatalf("failed to write config file: %v", err)
		}

		workDir := t.TempDir()
		cmd := exec.Command(binPath, "init", "--file", configFile)
		cmd.Dir = workDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("gitc init failed: %v, output: %s", err, out)
		}

		targetRepo := filepath.Join(workDir, repoID)

		// Should be on default branch
		currentBranch := getCurrentBranchLocal(t, targetRepo)
		if currentBranch != defaultBranch {
			t.Errorf("expected branch %s, got %s", defaultBranch, currentBranch)
		}
	})
}
