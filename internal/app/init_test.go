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

func TestPerformInit(t *testing.T) {
	// Setup a "remote" bare repo
	remoteDir, err := os.MkdirTemp("", "mstl-test-remote")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(remoteDir)

	// Initialize bare repo
	if _, err := RunGit(remoteDir, "git", false, "init", "--bare"); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}
	// We need to push something to it so it has HEAD
	// Create a seeder repo
	seedDir, err := os.MkdirTemp("", "mstl-test-seed")
	if err != nil {
		t.Fatalf("failed to create seed dir: %v", err)
	}
	defer os.RemoveAll(seedDir)
	if _, err := RunGit(seedDir, "git", false, "init"); err != nil {
		t.Fatalf("failed to init seed repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "README.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := RunGit(seedDir, "git", false, "add", "README.md"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	if _, err := RunGit(seedDir, "git", false, "commit", "-m", "Initial"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	// Force master branch to avoid init.defaultBranch issues (main vs master)
	if _, err := RunGit(seedDir, "git", false, "branch", "-M", "master"); err != nil {
		t.Fatalf("failed to rename branch to master: %v", err)
	}
	if _, err := RunGit(seedDir, "git", false, "remote", "add", "origin", remoteDir); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}
	if _, err := RunGit(seedDir, "git", false, "push", "origin", "master"); err != nil {
		t.Fatalf("failed to push: %v", err)
	}
	// Update HEAD in bare repo
	if _, err := RunGit(remoteDir, "git", false, "symbolic-ref", "HEAD", "refs/heads/master"); err != nil {
		t.Fatalf("failed to set HEAD: %v", err)
	}

	// Prepare workspace
	workDir, err := os.MkdirTemp("", "mstl-test-work")
	if err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}
	defer os.RemoveAll(workDir)

	repoID := "repo1"
	repoURL := "file://" + remoteDir // Use file protocol for cloning
	branch := "master"

	repos := []Repository{
		{
			ID:     &repoID,
			URL:    &repoURL,
			Branch: &branch,
		},
	}

	// Run PerformInit
	err = PerformInit(repos, workDir, "git", 1, 0, false)
	if err != nil {
		t.Fatalf("PerformInit failed: %v", err)
	}

	// Verify clone
	targetDir := filepath.Join(workDir, repoID)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Fatalf("target dir %s not created", targetDir)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "README.md")); os.IsNotExist(err) {
		t.Fatalf("README.md not found in cloned repo")
	}

	// Verify idempotency (run again)
	err = PerformInit(repos, workDir, "git", 1, 0, false)
	if err != nil {
		t.Fatalf("PerformInit (idempotency) failed: %v", err)
	}
}
