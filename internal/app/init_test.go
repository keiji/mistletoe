package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateEnvironment(t *testing.T) {
	// Create a temporary directory for the test workspace
	tmpDir, err := os.MkdirTemp("", "mstl-test-env")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to the temp dir so the relative path logic in validateEnvironment works
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	repoID := "repo1"
	repoURL := "https://github.com/example/repo1.git"

	tests := []struct {
		name    string
		setup   func() // Function to set up the environment state
		repos   []Repository
		wantErr bool
	}{
		{
			name: "Clean state (dir does not exist)",
			setup: func() {
				// No setup needed, dir shouldn't exist
			},
			repos: []Repository{
				{URL: &repoURL, ID: &repoID},
			},
			wantErr: false,
		},
		{
			name: "Existing empty directory",
			setup: func() {
				if err := os.Mkdir(repoID, 0755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}
			},
			repos: []Repository{
				{URL: &repoURL, ID: &repoID},
			},
			wantErr: false,
		},
		{
			name: "Existing non-empty non-git directory",
			setup: func() {
				if err := os.Mkdir(repoID, 0755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(repoID, "file.txt"), []byte("content"), 0644); err != nil {
					t.Fatalf("failed to create file: %v", err)
				}
			},
			repos: []Repository{
				{URL: &repoURL, ID: &repoID},
			},
			wantErr: true,
		},
		{
			name: "Existing git repo with correct remote",
			setup: func() {
				createDummyGitRepo(t, repoID, repoURL)
			},
			repos: []Repository{
				{URL: &repoURL, ID: &repoID},
			},
			wantErr: false,
		},
		{
			name: "Existing git repo with wrong remote",
			setup: func() {
				createDummyGitRepo(t, repoID, "https://github.com/other/repo.git")
			},
			repos: []Repository{
				{URL: &repoURL, ID: &repoID},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Cleanup from previous run if any (though we use unique IDs mostly, re-using repoID here)
			os.RemoveAll(repoID)

			if tt.setup != nil {
				tt.setup()
			}

			// Pass false for verbose
			err := validateEnvironment(tt.repos, "", "git", false)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEnvironment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
