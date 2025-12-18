package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// We need to mock RunGit or the underlying exec.Command to test status logic without real git repos?
// Or we can create real git repos. Creating real repos is safer for status logic testing.
// See common_test.go for helpers.

func TestValidateRepositoriesIntegrity(t *testing.T) {
	// ... (Existing test logic using real repos)
	// We'll reuse the pattern from init_test.go if possible or just fix the call

	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Create valid repo
	repoDir := filepath.Join(tmpDir, "repo1")
	// Note: createDummyGitRepo accepts (t, dir, remoteURL) based on common_test.go in memory but error says signature mismatch.
	// Wait, status_logic_test.go error says: have (T, string), want (T, string, string)
	// So createDummyGitRepo expects remoteURL as 3rd arg.
	createDummyGitRepo(t, repoDir, "https://example.com/repo1.git")

	url := "https://example.com/repo1.git"
	id := "repo1"
	repo := Repository{ID: &id, URL: &url}
	repos := []Repository{repo}
	config := Config{Repositories: &repos}

	// Test Success
	if err := ValidateRepositoriesIntegrity(&config, "git", false); err != nil {
		t.Errorf("Expected success, got %v", err)
	}

	// Test Failure (Mismatch URL)
	badUrl := "https://example.com/other.git"
	badRepo := Repository{ID: &id, URL: &badUrl}
	badConfig := Config{Repositories: &[]Repository{badRepo}}
	if err := ValidateRepositoriesIntegrity(&badConfig, "git", false); err == nil {
		t.Error("Expected failure for mismatched URL, got nil")
	}
}

func TestCollectStatus(t *testing.T) {
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// 1. Clean State (Up-to-date)
	// We need a remote to compare against. We can fake it by having two local repos acting as one remote to another.
	remoteDir := filepath.Join(tmpDir, "remote")
	// Use remoteDir as URL for itself just to init it, or create bare?
	// createDummyGitRepo creates normal repo.
	// Let's use setupRemoteAndContent from common_test.go which does bare repo + content repo.
	// But that returns URL and contentDir.
	// Let's try to use createDummyGitRepo manually.

	// Create "remote" repo
	createDummyGitRepo(t, remoteDir, "origin-url-ignored")
	configureGitUser(t, remoteDir) // Needed to commit
	// Add a commit to remote
	exec.Command("git", "-C", remoteDir, "commit", "--allow-empty", "-m", "remote-init").Run()

	// Local repo cloning remote
	localDir := filepath.Join(tmpDir, "local")
	exec.Command("git", "clone", remoteDir, localDir).Run()
	configureGitUser(t, localDir)

	// Config
	id := "local"
	url := remoteDir // Use file path as URL
	branch := "master" // git init default is master usually in these tests unless configured
	// Check branch name
	out, _ := exec.Command("git", "-C", localDir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if strings.TrimSpace(string(out)) == "main" {
		branch = "main"
	}

	repo1 := Repository{ID: &id, URL: &url, Branch: &branch}
	config1 := Config{Repositories: &[]Repository{repo1}}

	rows1 := CollectStatus(&config1, 1, "git", false)
	if len(rows1) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(rows1))
	}
	if rows1[0].HasUnpushed || rows1[0].IsPullable {
		t.Errorf("Expected clean status, got Unpushed=%v Pullable=%v", rows1[0].HasUnpushed, rows1[0].IsPullable)
	}

	// 2. Unpushed (Ahead)
	exec.Command("git", "-C", localDir, "commit", "--allow-empty", "-m", "local-commit").Run()
	rows2 := CollectStatus(&config1, 1, "git", false)
	if !rows2[0].HasUnpushed {
		t.Error("Expected Unpushed=true")
	}

	// 3. Pullable (Behind)
	// Reset local to match remote, then add commit to remote
	exec.Command("git", "-C", localDir, "reset", "--hard", "origin/"+branch).Run()
	exec.Command("git", "-C", remoteDir, "commit", "--allow-empty", "-m", "remote-commit").Run()
	// Fetch in local so it knows about it
	exec.Command("git", "-C", localDir, "fetch").Run()

	rows3 := CollectStatus(&config1, 1, "git", false)
	if !rows3[0].IsPullable {
		t.Error("Expected IsPullable=true")
	}

	// 4. Diverged (Unpushed + Pullable) - BUT wait, CollectStatus logic depends on Branch config
	// Add local commit again
	exec.Command("git", "-C", localDir, "commit", "--allow-empty", "-m", "local-diverged").Run()

	// We need config to point to specific branch for Pullable check
	repo := Repository{ID: &id, URL: &url, Branch: &branch}
	config := Config{Repositories: &[]Repository{repo}}

	rows := CollectStatus(&config, 1, "git", false)
	if !rows[0].HasUnpushed {
		t.Error("Expected HasUnpushed=true (Diverged)")
	}
	if !rows[0].IsPullable {
		t.Error("Expected IsPullable=true (Diverged)")
	}
}
