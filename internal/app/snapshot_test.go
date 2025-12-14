package app

import (
	"encoding/json"
	"os/exec"
	"testing"
)

func TestGenerateSnapshot(t *testing.T) {
	// Setup 2 repos
	remoteURL1, repoDir1 := setupRemoteAndContent(t, 1)
	remoteURL2, repoDir2 := setupRemoteAndContent(t, 1)

	// Since GenerateSnapshot uses GetRepoDir which returns ID if present.
	// We use absolute paths as IDs for testing.
	id1 := repoDir1
	id2 := repoDir2

	config := Config{
		Repositories: &[]Repository{
			{ID: &id1, URL: &remoteURL1},
			{ID: &id2, URL: &remoteURL2},
		},
	}

	// We need git path
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal("git not found")
	}

	data, identifier, err := GenerateSnapshot(&config, gitPath)
	if err != nil {
		t.Fatalf("GenerateSnapshot failed: %v", err)
	}

	// Verify identifier is SHA256 (64 hex chars)
	if len(identifier) != 64 {
		t.Errorf("Identifier length %d, expected 64", len(identifier))
	}

	// Verify JSON
	var snapshotConfig Config
	if err := json.Unmarshal(data, &snapshotConfig); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}

	if len(*snapshotConfig.Repositories) != 2 {
		t.Errorf("Expected 2 repos, got %d", len(*snapshotConfig.Repositories))
	}

	// Check content
	for _, r := range *snapshotConfig.Repositories {
		if r.Revision == nil || *r.Revision == "" {
			t.Errorf("Revision missing for %s", *r.ID)
		}
		// Branch should be master (default from setupRemoteAndContent)
		if r.Branch == nil || *r.Branch != "master" {
			t.Errorf("Expected branch master, got %v", r.Branch)
		}
	}
}

func TestGenerateSnapshot_Subset(t *testing.T) {
	// Setup 1 repo
	remoteURL1, repoDir1 := setupRemoteAndContent(t, 1)

	// Config has 2 repos, one missing
	id1 := repoDir1
	id2 := "/path/to/missing/repo"
	url2 := "https://example.com/missing.git"

	config := Config{
		Repositories: &[]Repository{
			{ID: &id1, URL: &remoteURL1},
			{ID: &id2, URL: &url2},
		},
	}

	gitPath, _ := exec.LookPath("git")
	data, _, err := GenerateSnapshot(&config, gitPath)
	if err != nil {
		t.Fatalf("GenerateSnapshot failed: %v", err)
	}

	var snapshotConfig Config
	if err := json.Unmarshal(data, &snapshotConfig); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}

	// Should only have 1 repo
	if len(*snapshotConfig.Repositories) != 1 {
		t.Errorf("Expected 1 repo, got %d", len(*snapshotConfig.Repositories))
	}
	if *(*snapshotConfig.Repositories)[0].ID != id1 {
		t.Errorf("Expected repo %s, got %s", id1, *(*snapshotConfig.Repositories)[0].ID)
	}
}
