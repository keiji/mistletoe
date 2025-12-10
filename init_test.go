package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Helper to create a dummy git repo with a remote
func createDummyGitRepo(t *testing.T, dir, remoteURL string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create dir %s: %v", dir, err)
	}

	cmds := [][]string{
		{"init"},
		{"remote", "add", "origin", remoteURL},
	}

	for _, args := range cmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to run git %v in %s: %v", args, dir, err)
		}
	}
}

func TestValidateEnvironment(t *testing.T) {
	// Create a temporary directory for the test workspace
	tmpDir, err := os.MkdirTemp("", "gitc-test-env")
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
				{URL: repoURL, ID: &repoID},
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
				{URL: repoURL, ID: &repoID},
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
				{URL: repoURL, ID: &repoID},
			},
			wantErr: true,
		},
		{
			name: "Existing git repo with correct remote",
			setup: func() {
				createDummyGitRepo(t, repoID, repoURL)
			},
			repos: []Repository{
				{URL: repoURL, ID: &repoID},
			},
			wantErr: false,
		},
		{
			name: "Existing git repo with wrong remote",
			setup: func() {
				createDummyGitRepo(t, repoID, "https://github.com/other/repo.git")
			},
			repos: []Repository{
				{URL: repoURL, ID: &repoID},
			},
			wantErr: true,
		},
		{
			name: "Existing file (not dir)",
			setup: func() {
				if err := os.WriteFile(repoID, []byte("file"), 0644); err != nil {
					t.Fatalf("failed to create file: %v", err)
				}
			},
			repos: []Repository{
				{URL: repoURL, ID: &repoID},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Cleanup from previous run
			os.RemoveAll(repoID)

			tt.setup()

			err := validateEnvironment(tt.repos)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEnvironment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRepositories_Duplicates(t *testing.T) {
	id1 := "repo1"
	id2 := "repo2"

	tests := []struct {
		name    string
		repos   []Repository
		wantErr bool
	}{
		{
			name: "No duplicates",
			repos: []Repository{
				{ID: &id1, URL: "http://example.com/1.git"},
				{ID: &id2, URL: "http://example.com/2.git"},
			},
			wantErr: false,
		},
		{
			name: "Duplicates",
			repos: []Repository{
				{ID: &id1, URL: "http://example.com/1.git"},
				{ID: &id1, URL: "http://example.com/2.git"},
			},
			wantErr: true,
		},
		{
			name: "Nil IDs (ignored)",
			repos: []Repository{
				{ID: nil, URL: "http://example.com/1.git"},
				{ID: nil, URL: "http://example.com/2.git"},
				{ID: &id1, URL: "http://example.com/3.git"},
			},
			wantErr: false,
		},
		{
			name: "Nil ID and matching string ID (no collision)",
			repos: []Repository{
				{ID: nil, URL: "http://example.com/1.git"},
				{ID: &id1, URL: "http://example.com/2.git"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateRepositories(tt.repos); (err != nil) != tt.wantErr {
				t.Errorf("validateRepositories() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetRepoDir(t *testing.T) {
	id := "custom-dir"
	tests := []struct {
		name     string
		repo     Repository
		expected string
	}{
		{
			name:     "With ID",
			repo:     Repository{ID: &id, URL: "https://github.com/foo/bar.git"},
			expected: "custom-dir",
		},
		{
			name:     "Without ID, standard git",
			repo:     Repository{ID: nil, URL: "https://github.com/foo/bar.git"},
			expected: "bar",
		},
		{
			name:     "Without ID, no .git",
			repo:     Repository{ID: nil, URL: "https://github.com/foo/baz"},
			expected: "baz",
		},
		{
			name:     "Without ID, trailing slash",
			repo:     Repository{ID: nil, URL: "https://github.com/foo/qux/"},
			expected: "qux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRepoDir(tt.repo)
			if got != tt.expected {
				t.Errorf("getRepoDir() = %v, want %v", got, tt.expected)
			}
		})
	}
}
