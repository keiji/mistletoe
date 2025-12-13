package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusCmd(t *testing.T) {
	// 1. Build gitc binary
	binPath := buildGitc(t)

	t.Run("Validation Error - Wrong Remote", func(t *testing.T) {
		workDir := t.TempDir()

		// Create a repo manually in workDir with wrong remote
		repoID := "bad-repo"
		repoPath := filepath.Join(workDir, repoID)
		if err := os.Mkdir(repoPath, 0755); err != nil {
			t.Fatal(err)
		}
		exec.Command("git", "-C", repoPath, "init").Run()
		exec.Command("git", "-C", repoPath, "remote", "add", "origin", "https://example.com/wrong.git").Run()

		// Config expects correct URL
		config := Config{
			Repositories: &[]Repository{
				{ID: &repoID, URL: strPtr("https://example.com/correct.git")},
			},
		}
		configFile := filepath.Join(workDir, "repos.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		cmd := exec.Command(binPath, "status", "-f", configFile)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()

		// Expect failure
		if err == nil {
			t.Errorf("Expected status to fail due to wrong remote, but it succeeded")
		}
		output := string(out)
		if !strings.Contains(output, "different remote origin") {
			t.Errorf("Expected error message about different remote origin, got: %s", output)
		}
	})

	t.Run("Status Success - Synced and Unpushed", func(t *testing.T) {
		workDir := t.TempDir()

		// Repo 1: Synced
		remote1, _ := setupRemoteAndContent(t, 2)
		id1 := "repo1"
		// Clone it into workDir
		exec.Command("git", "clone", remote1, filepath.Join(workDir, id1)).Run()

		// Repo 2: Unpushed (Ahead)
		remote2, _ := setupRemoteAndContent(t, 2)
		id2 := "repo2"
		repo2Path := filepath.Join(workDir, id2)
		exec.Command("git", "clone", remote2, repo2Path).Run()

		// Add new commit to Repo2 locally
		configureGitUser(t, repo2Path)
		fname := filepath.Join(repo2Path, "new.txt")
		os.WriteFile(fname, []byte("new"), 0644)
		exec.Command("git", "-C", repo2Path, "add", ".").Run()
		if err := exec.Command("git", "-C", repo2Path, "commit", "-m", "unpushed commit").Run(); err != nil {
			t.Fatalf("failed to commit in repo2: %v", err)
		}

		// Config
		config := Config{
			Repositories: &[]Repository{
				{ID: &id1, URL: &remote1},
				{ID: &id2, URL: &remote2},
			},
		}
		configFile := filepath.Join(workDir, "repos.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		cmd := exec.Command(binPath, "status", "-f", configFile)
		cmd.Dir = workDir

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("status command failed: %v\nOutput: %s", err, string(out))
		}
		output := string(out)

		// Check table content
		// Repo1: Synced "-"
		// Repo2: Unpushed ">" (Green)

		coloredUnpushed := "\033[32m>\033[0m"

		if !strings.Contains(output, "repo1") {
			t.Errorf("Output missing repo1")
		}
		if !strings.Contains(output, "repo2") {
			t.Errorf("Output missing repo2")
		}

		lines := strings.Split(output, "\n")
		foundRepo1 := false
		foundRepo2 := false

		for _, line := range lines {
			if strings.Contains(line, "repo1") {
				foundRepo1 = true
				if strings.Contains(line, coloredUnpushed) {
					t.Errorf("repo1 should not have unpushed commits: %s", line)
				}
			}
			if strings.Contains(line, "repo2") {
				foundRepo2 = true
				if !strings.Contains(line, coloredUnpushed) {
					t.Errorf("repo2 SHOULD have unpushed commits (Green >): %s", line)
				}
			}
		}

		if !foundRepo1 {
			t.Error("repo1 row not found")
		}
		if !foundRepo2 {
			t.Error("repo2 row not found")
		}
	})

	t.Run("Status Success - Diverged (No Branch Config)", func(t *testing.T) {
		workDir := t.TempDir()
		remoteDir, _ := setupRemoteAndContent(t, 1) // Remote has commit A

		// Setup local repo "diverged-repo"
		repoID := "diverged-repo"
		localRepoPath := filepath.Join(workDir, repoID)
		exec.Command("git", "clone", remoteDir, localRepoPath).Run()

		// Create divergence
		// 1. Update remote separately
		otherClone := t.TempDir()
		exec.Command("git", "clone", remoteDir, otherClone).Run()
		configureGitUser(t, otherClone)
		if err := exec.Command("git", "-C", otherClone, "commit", "--allow-empty", "-m", "Remote Commit B").Run(); err != nil {
			t.Fatalf("failed to commit in otherClone: %v", err)
		}
		if err := exec.Command("git", "-C", otherClone, "push").Run(); err != nil {
			t.Fatalf("failed to push from otherClone: %v", err)
		}

		// 2. Commit locally in "diverged-repo" (it currently has A, now adds C)
		configureGitUser(t, localRepoPath)
		if err := exec.Command("git", "-C", localRepoPath, "commit", "--allow-empty", "-m", "Local Commit C").Run(); err != nil {
			t.Fatalf("failed to commit in localRepoPath: %v", err)
		}

		// Important: Fetch so local has remote objects (B)
		exec.Command("git", "-C", localRepoPath, "fetch").Run()

		// Config (No Branch specified)
		config := Config{
			Repositories: &[]Repository{
				{ID: &repoID, URL: &remoteDir},
			},
		}
		configFile := filepath.Join(workDir, "repos.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		cmd := exec.Command(binPath, "status", "-f", configFile)
		cmd.Dir = workDir

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("status command failed: %v\nOutput: %s", err, string(out))
		}
		output := string(out)

		coloredUnpushed := "\033[32m>\033[0m"
		coloredPullable := "\033[33m<\033[0m"

		if !strings.Contains(output, coloredUnpushed) {
			t.Errorf("Expected Diverged repo to show '>' (Green) for unpushed, but got output:\n%s", output)
		}
		if strings.Contains(output, coloredPullable) {
			t.Errorf("Did not expect '<' because Branch is not configured")
		}
	})

	t.Run("Status Success - Pullable Only", func(t *testing.T) {
		workDir := t.TempDir()
		remoteDir, _ := setupRemoteAndContent(t, 1)

		repoID := "pull-repo"
		localRepoPath := filepath.Join(workDir, repoID)
		exec.Command("git", "clone", remoteDir, localRepoPath).Run()

		// Remote gets B
		otherClone := t.TempDir()
		exec.Command("git", "clone", remoteDir, otherClone).Run()
		configureGitUser(t, otherClone)
		exec.Command("git", "-C", otherClone, "commit", "--allow-empty", "-m", "Remote B").Run()
		exec.Command("git", "-C", otherClone, "push").Run()

		master := "master"
		config := Config{
			Repositories: &[]Repository{
				{ID: &repoID, URL: &remoteDir, Branch: &master},
			},
		}
		configFile := filepath.Join(workDir, "repos.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		cmd := exec.Command(binPath, "status", "-f", configFile)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("status failed: %v", err)
		}
		output := string(out)

		// Expect < (Yellow)
		coloredPullable := "\033[33m<\033[0m"
		if !strings.Contains(output, coloredPullable) {
			t.Errorf("Expected '<' in yellow, got:\n%s", output)
		}
	})

	t.Run("Status Success - Diverged with Config", func(t *testing.T) {
		workDir := t.TempDir()
		remoteDir, _ := setupRemoteAndContent(t, 1) // Remote commit A

		// Setup local
		repoID := "pd-repo"
		localRepoPath := filepath.Join(workDir, repoID)
		exec.Command("git", "clone", remoteDir, localRepoPath).Run()

		// Diverge
		// Remote gets B
		otherClone := t.TempDir()
		exec.Command("git", "clone", remoteDir, otherClone).Run()
		configureGitUser(t, otherClone)
		exec.Command("git", "-C", otherClone, "commit", "--allow-empty", "-m", "Remote B").Run()
		exec.Command("git", "-C", otherClone, "push").Run()

		// Local gets C
		configureGitUser(t, localRepoPath)
		exec.Command("git", "-C", localRepoPath, "commit", "--allow-empty", "-m", "Local C").Run()

		// Fetch so Unpushed check works
		exec.Command("git", "-C", localRepoPath, "fetch").Run()

		master := "master"
		config := Config{
			Repositories: &[]Repository{
				{ID: &repoID, URL: &remoteDir, Branch: &master},
			},
		}
		configFile := filepath.Join(workDir, "repos.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		cmd := exec.Command(binPath, "status", "-f", configFile)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("status failed: %v %s", err, string(out))
		}
		output := string(out)

		// Expect > (Green) and < (Yellow)
		coloredDiverged := "\033[32m>\033[0m\033[33m<\033[0m"

		if !strings.Contains(output, coloredDiverged) {
             t.Errorf("Expected '><' (Green then Yellow), got:\n%s", output)
		}
	})
}
