package app

import (
	conf "mistletoe/internal/config"
)

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSnapshotFromStatus(t *testing.T) {
	// Setup
	repoID := "repo1"
	repoURL := "https://example.com/repo1.git"
	branch := "feature/abc"
	revision := "abcdef1234567890"

	config := &conf.Config{
		Repositories: &[]conf.Repository{
			{
				ID:     &repoID,
				URL:    &repoURL,
				Branch: &branch,
			},
		},
	}

	statusRows := []StatusRow{
		{
			Repo:          repoID,
			BranchName:    branch,
			LocalHeadFull: revision,
			// Other fields ignored by snapshot logic
		},
	}

	// Execute
	data, identifier, err := GenerateSnapshotFromStatus(config, statusRows)
	if err != nil {
		t.Fatalf("GenerateSnapshotFromStatus failed: %v", err)
	}

	// Verify Identifier
	if identifier == "" {
		t.Error("identifier is empty")
	}

	// Verify JSON content
	var snapshotConfig conf.Config
	if err := json.Unmarshal(data, &snapshotConfig); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(*snapshotConfig.Repositories) != 1 {
		t.Errorf("expected 1 repo, got %d", len(*snapshotConfig.Repositories))
	}

	r := (*snapshotConfig.Repositories)[0]
	if *r.ID != repoID {
		t.Errorf("expected ID %s, got %s", repoID, *r.ID)
	}
	if *r.URL != repoURL {
		t.Errorf("expected URL %s, got %s", repoURL, *r.URL)
	}
	if *r.Revision != revision {
		t.Errorf("expected Revision %s, got %s", revision, *r.Revision)
	}
	if *r.Branch != branch {
		t.Errorf("expected Branch %s, got %s", branch, *r.Branch)
	}
}

func TestGenerateSnapshotVerbose(t *testing.T) {
	tmpDir := t.TempDir()

	repoName := "myrepo"
	repoDir := filepath.Join(tmpDir, repoName)
	remoteURL := "https://example.com/myrepo.git"
	// branchName is not explicitly checked but created implicitly by git init

	// Create git repo
	createDummyGitRepo(t, repoDir, remoteURL)
	configureGitUser(t, repoDir)

	// Create a commit so we have a revision
	RunGit(repoDir, "git", false, "commit", "--allow-empty", "-m", "initial")

	// Get the revision
	rev, _ := RunGit(repoDir, "git", false, "rev-parse", "HEAD")

	// Create conf.Config pointing to this repo
	configID := repoName // using dirname as ID for simplicity
	configURL := remoteURL
	config := &conf.Config{
		Repositories: &[]conf.Repository{
			{
				ID:  &configID,
				URL: &configURL,
			},
		},
	}

	// Change CWD to tmpDir so GenerateSnapshotVerbose finds the repo by relative path
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Execute
	data, identifier, err := GenerateSnapshotVerbose(config, "git", false)
	if err != nil {
		t.Fatalf("GenerateSnapshotVerbose failed: %v", err)
	}

	if identifier == "" {
		t.Error("identifier is empty")
	}

	// Verify JSON
	var snapshotConfig conf.Config
	if err := json.Unmarshal(data, &snapshotConfig); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(*snapshotConfig.Repositories) != 1 {
		t.Errorf("expected 1 repo, got %d", len(*snapshotConfig.Repositories))
	}

	r := (*snapshotConfig.Repositories)[0]
	if *r.ID != repoName {
		t.Errorf("expected ID %s, got %s", repoName, *r.ID)
	}
	if *r.Revision != rev {
		t.Errorf("expected Revision %s, got %s", rev, *r.Revision)
	}
}
