package app

import (
	conf "mistletoe/internal/config"
)

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"os/exec"
)

// We need to mock RunGit or the underlying exec.Command to test status logic without real git repos?
// Or we can create real git repos. Creating real repos is safer for status logic testing.
// See common_test.go for helpers.

func TestValidateRepositoriesIntegrity(t *testing.T) {
	// Setup workspace
	tmpDir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(tmpDir)

	repoID := "repo1"
	repoURL := "https://example.com/repo1.git"
	otherURL := "https://example.com/other.git"

	tests := []struct {
		name    string
		setup   func()
		repos   []conf.Repository
		wantErr bool
	}{
		{
			name: "Dir does not exist (Skipped)",
			setup: func() {
				// No dir
			},
			repos: []conf.Repository{
				{ID: &repoID, URL: &repoURL},
			},
			wantErr: false,
		},
		{
			name: "Dir exists but not a git repo",
			setup: func() {
				os.Mkdir(repoID, 0755)
			},
			repos: []conf.Repository{
				{ID: &repoID, URL: &repoURL},
			},
			wantErr: true,
		},
		{
			name: "Target exists but is file",
			setup: func() {
				os.WriteFile(repoID, []byte("file"), 0644)
			},
			repos: []conf.Repository{
				{ID: &repoID, URL: &repoURL},
			},
			wantErr: true,
		},
		{
			name: "Git repo with correct remote",
			setup: func() {
				createDummyGitRepo(t, repoID, repoURL)
			},
			repos: []conf.Repository{
				{ID: &repoID, URL: &repoURL},
			},
			wantErr: false,
		},
		{
			name: "Git repo with wrong remote",
			setup: func() {
				createDummyGitRepo(t, repoID, otherURL)
			},
			repos: []conf.Repository{
				{ID: &repoID, URL: &repoURL},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.RemoveAll(repoID) // Clean up
			if tt.setup != nil {
				tt.setup()
			}
			config := conf.Config{Repositories: &tt.repos}
			err := ValidateRepositoriesIntegrity(&config, "git", false)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRepositoriesIntegrity() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
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

	// Create "remote" repo
	createDummyGitRepo(t, remoteDir, "origin-url-ignored")
	configureGitUser(t, remoteDir) // Needed to commit
	// Add a commit to remote
	exec.Command("git", "-C", remoteDir, "commit", "--allow-empty", "-m", "remote-init").Run()

	// Local repo cloning remote
	localDir := filepath.Join(tmpDir, "local")
	exec.Command("git", "clone", remoteDir, localDir).Run()
	configureGitUser(t, localDir)

	// conf.Config
	id := "local"
	url := remoteDir // Use file path as URL
	branch := "master" // git init default is master usually in these tests unless configured
	// Check branch name
	out, _ := exec.Command("git", "-C", localDir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if strings.TrimSpace(string(out)) == "main" {
		branch = "main"
	}

	repo1 := conf.Repository{ID: &id, URL: &url, Branch: &branch}
	config1 := conf.Config{Repositories: &[]conf.Repository{repo1}}

	rows1 := CollectStatus(&config1, 1, "git", false, false)
	if len(rows1) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(rows1))
	}
	if rows1[0].HasUnpushed || rows1[0].IsPullable {
		t.Errorf("Expected clean status, got Unpushed=%v Pullable=%v", rows1[0].HasUnpushed, rows1[0].IsPullable)
	}

	// 2. Unpushed (Ahead)
	exec.Command("git", "-C", localDir, "commit", "--allow-empty", "-m", "local-commit").Run()
	rows2 := CollectStatus(&config1, 1, "git", false, false)
	if !rows2[0].HasUnpushed {
		t.Error("Expected Unpushed=true")
	}

	// 3. Pullable (Behind)
	// Reset local to match remote, then add commit to remote
	exec.Command("git", "-C", localDir, "reset", "--hard", "origin/"+branch).Run()
	exec.Command("git", "-C", remoteDir, "commit", "--allow-empty", "-m", "remote-commit").Run()
	// Fetch in local so it knows about it
	exec.Command("git", "-C", localDir, "fetch").Run()

	rows3 := CollectStatus(&config1, 1, "git", false, false)
	if !rows3[0].IsPullable {
		t.Error("Expected IsPullable=true")
	}

	// 4. Diverged (Unpushed + Pullable) - BUT wait, CollectStatus logic depends on Branch config
	// Add local commit again
	exec.Command("git", "-C", localDir, "commit", "--allow-empty", "-m", "local-diverged").Run()

	// We need config to point to specific branch for Pullable check
	repo := conf.Repository{ID: &id, URL: &url, Branch: &branch}
	config := conf.Config{Repositories: &[]conf.Repository{repo}}

	rows := CollectStatus(&config, 1, "git", false, false)
	if !rows[0].HasUnpushed {
		t.Error("Expected HasUnpushed=true (Diverged)")
	}
	if !rows[0].IsPullable {
		t.Error("Expected IsPullable=true (Diverged)")
	}
}

func TestValidateStatusForAction(t *testing.T) {
	// Mock osExit
	oldOsExit := osExit
	defer func() { osExit = oldOsExit }()

	var exitCode int
	osExit = func(code int) {
		exitCode = code
	}

	tests := []struct {
		name          string
		rows          []StatusRow
		checkPullable bool
		wantExit      bool
	}{
		{
			name:          "Clean",
			rows:          []StatusRow{{Repo: "r1", HasConflict: false, BranchName: "main"}},
			checkPullable: true,
			wantExit:      false,
		},
		{
			name:          "Conflict",
			rows:          []StatusRow{{Repo: "r1", HasConflict: true, BranchName: "main"}},
			checkPullable: false,
			wantExit:      true,
		},
		{
			name:          "Detached HEAD",
			rows:          []StatusRow{{Repo: "r1", HasConflict: false, BranchName: "HEAD"}},
			checkPullable: false,
			wantExit:      true,
		},
		{
			name:          "Behind (Check=True)",
			rows:          []StatusRow{{Repo: "r1", IsPullable: true, BranchName: "main"}},
			checkPullable: true,
			wantExit:      true,
		},
		{
			name:          "Behind (Check=False)",
			rows:          []StatusRow{{Repo: "r1", IsPullable: true, BranchName: "main"}},
			checkPullable: false,
			wantExit:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode = 0 // reset
			ValidateStatusForAction(tt.rows, tt.checkPullable)
			if tt.wantExit && exitCode == 0 {
				t.Error("Expected exit(1), got 0")
			}
			if !tt.wantExit && exitCode != 0 {
				t.Errorf("Expected exit(0), got %d", exitCode)
			}
		})
	}
}
