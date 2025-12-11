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

// Helper to init a bare repo and a content repo, returns remote URL (file://)
func setupRemoteAndContent(t *testing.T, commitCount int) (string, string) {
	remoteDir := t.TempDir()
	if err := exec.Command("git", "init", "--bare", remoteDir).Run(); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}

	contentDir := t.TempDir()
	if err := exec.Command("git", "init", contentDir).Run(); err != nil {
		t.Fatalf("failed to init content repo: %v", err)
	}
	if err := exec.Command("git", "-C", contentDir, "config", "user.email", "test@example.com").Run(); err != nil {
		t.Fatalf("failed to configure user.email: %v", err)
	}
	if err := exec.Command("git", "-C", contentDir, "config", "user.name", "Test User").Run(); err != nil {
		t.Fatalf("failed to configure user.name: %v", err)
	}

	// Normalize path for Windows compatibility if needed, though usually / is fine in git URLs
	// But file:// URL needs absolute path
	remoteURL := "file://" + filepath.ToSlash(remoteDir)

	if err := exec.Command("git", "-C", contentDir, "remote", "add", "origin", remoteURL).Run(); err != nil {
		t.Fatalf("failed to add remote origin: %v", err)
	}

	for i := 0; i < commitCount; i++ {
		fname := filepath.Join(contentDir, fmt.Sprintf("file-%d.txt", i))
		if err := os.WriteFile(fname, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		exec.Command("git", "-C", contentDir, "add", ".").Run()
		exec.Command("git", "-C", contentDir, "commit", "-m", fmt.Sprintf("commit %d", i)).Run()
	}

	if commitCount > 0 {
		if err := exec.Command("git", "-C", contentDir, "push", "origin", "master").Run(); err != nil {
			t.Fatalf("failed to push: %v", err)
		}
	}

	return remoteURL, contentDir
}

func configureGitUser(t *testing.T, dir string) {
	if err := exec.Command("git", "-C", dir, "config", "user.email", "test@example.com").Run(); err != nil {
		t.Fatalf("failed to config user.email in %s: %v", dir, err)
	}
	if err := exec.Command("git", "-C", dir, "config", "user.name", "Test User").Run(); err != nil {
		t.Fatalf("failed to config user.name in %s: %v", dir, err)
	}
}

func TestStatusCmd(t *testing.T) {
	// 1. Build gitc binary
	binPath := filepath.Join(t.TempDir(), "gitc")
	// On Windows usually ends with .exe, but go build handles output name.
	if os.PathSeparator == '\\' {
		binPath += ".exe"
	}

	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build gitc: %v", err)
	}

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
			Repositories: []Repository{
				{ID: &repoID, URL: "https://example.com/correct.git"},
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
			Repositories: []Repository{
				{ID: &id1, URL: remote1},
				{ID: &id2, URL: remote2},
			},
		}
		configFile := filepath.Join(workDir, "repos.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		cmd := exec.Command(binPath, "status", "-f", configFile)
		cmd.Dir = workDir

		// Set GIT_AUTHOR_NAME etc for the test run if needed, but we already committed
		// status just reads.

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("status command failed: %v\nOutput: %s", err, string(out))
		}
		output := string(out)

		// Check table content
		// Repo1: Unpushed "No"
		// Repo2: Unpushed "Yes"

		// We expect output to contain:
		// | repo1 | ... | No |
		// | repo2 | ... | Yes |

		// Note: The table might format it nicely.
		// We check for presence of substrings.

		if !strings.Contains(output, "repo1") {
			t.Errorf("Output missing repo1")
		}
		if !strings.Contains(output, "repo2") {
			t.Errorf("Output missing repo2")
		}

		// We can't easily parse ASCII table in regex without being loose,
		// but we can check if "Yes" appears.
		// Since repo1 is "No" and repo2 is "Yes", both should appear.
		// However, "No" appears twice? Or just check rough alignment.

		lines := strings.Split(output, "\n")
		foundRepo1 := false
		foundRepo2 := false

		for _, line := range lines {
			if strings.Contains(line, "repo1") {
				foundRepo1 = true
				if strings.Contains(line, "Yes") {
					t.Errorf("repo1 should not have unpushed commits: %s", line)
				}
			}
			if strings.Contains(line, "repo2") {
				foundRepo2 = true
				if !strings.Contains(line, "Yes") {
					t.Errorf("repo2 SHOULD have unpushed commits: %s", line)
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

	t.Run("Status Success - Diverged", func(t *testing.T) {
		workDir := t.TempDir()
		remoteDir, _ := setupRemoteAndContent(t, 1) // Remote has commit A

		// Setup local repo "diverged-repo"
		repoID := "diverged-repo"
		localRepoPath := filepath.Join(workDir, repoID)
		exec.Command("git", "clone", remoteDir, localRepoPath).Run()

		// Create divergence
		// 1. Update remote separately
		// We can do this by cloning to another temp dir, committing, and pushing
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

		// Config
		config := Config{
			Repositories: []Repository{
				{ID: &repoID, URL: remoteDir},
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

		// Expect "Yes" for unpushed because local has commit C which is not in remote (A-B)
		if !strings.Contains(output, "Yes") {
			t.Errorf("Expected Diverged repo to show 'Yes' for unpushed, but got output:\n%s", output)
		}
	})
}
