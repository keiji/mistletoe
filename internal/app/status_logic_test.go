package app

import (
	"os"
	"os/exec"
	"testing"
)

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
		repos   []Repository
		wantErr bool
	}{
		{
			name: "Dir does not exist (Skipped)",
			setup: func() {
				// No dir
			},
			repos: []Repository{
				{ID: &repoID, URL: &repoURL},
			},
			wantErr: false,
		},
		{
			name: "Dir exists but not a git repo",
			setup: func() {
				os.Mkdir(repoID, 0755)
			},
			repos: []Repository{
				{ID: &repoID, URL: &repoURL},
			},
			wantErr: true,
		},
		{
			name: "Target exists but is file",
			setup: func() {
				os.WriteFile(repoID, []byte("file"), 0644)
			},
			repos: []Repository{
				{ID: &repoID, URL: &repoURL},
			},
			wantErr: true,
		},
		{
			name: "Git repo with correct remote",
			setup: func() {
				createDummyGitRepo(t, repoID, repoURL)
			},
			repos: []Repository{
				{ID: &repoID, URL: &repoURL},
			},
			wantErr: false,
		},
		{
			name: "Git repo with wrong remote",
			setup: func() {
				createDummyGitRepo(t, repoID, otherURL)
			},
			repos: []Repository{
				{ID: &repoID, URL: &repoURL},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.RemoveAll(repoID)
			tt.setup()

			config := Config{Repositories: &tt.repos}
			err := ValidateRepositoriesIntegrity(&config, "git") // Assuming git is in path
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRepositoriesIntegrity() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCollectStatus(t *testing.T) {
	tmpDir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(tmpDir)

	// 1. Synced Repo
	remote1, _ := setupRemoteAndContent(t, 2)
	id1 := "repo-synced"
	exec.Command("git", "clone", remote1, id1).Run()

	// 2. Unpushed Repo (Ahead)
	remote2, _ := setupRemoteAndContent(t, 2)
	id2 := "repo-ahead"
	exec.Command("git", "clone", remote2, id2).Run()
	// Commit locally
	configureGitUser(t, id2)
	exec.Command("git", "-C", id2, "commit", "--allow-empty", "-m", "Ahead").Run()

	// 3. Pullable Repo (Behind)
	remote3, _ := setupRemoteAndContent(t, 2)
	id3 := "repo-behind"
	exec.Command("git", "clone", remote3, id3).Run()
	// Push to remote from elsewhere
	other3 := t.TempDir()
	exec.Command("git", "clone", remote3, other3).Run()
	configureGitUser(t, other3)
	exec.Command("git", "-C", other3, "commit", "--allow-empty", "-m", "Remote Ahead").Run()
	exec.Command("git", "-C", other3, "push").Run()

	// 4. Diverged (Unpushed + Pullable) - BUT wait, CollectStatus logic depends on Branch config
	// If Branch matches current, we get IsPullable.
	remote4, _ := setupRemoteAndContent(t, 2)
	id4 := "repo-diverged"
	exec.Command("git", "clone", remote4, id4).Run()
	// Remote change
	other4 := t.TempDir()
	exec.Command("git", "clone", remote4, other4).Run()
	configureGitUser(t, other4)
	exec.Command("git", "-C", other4, "commit", "--allow-empty", "-m", "Remote Div").Run()
	exec.Command("git", "-C", other4, "push").Run()
	// Local change
	configureGitUser(t, id4)
	exec.Command("git", "-C", id4, "commit", "--allow-empty", "-m", "Local Div").Run()
	// Need fetch to know about remote
	exec.Command("git", "-C", id4, "fetch").Run()

	master := "master"
	config := Config{
		Repositories: &[]Repository{
			{ID: &id1, URL: &remote1, Branch: &master},
			{ID: &id2, URL: &remote2, Branch: &master},
			{ID: &id3, URL: &remote3, Branch: &master},
			{ID: &id4, URL: &remote4, Branch: &master},
		},
	}

	rows := CollectStatus(&config, 1, "git")

	if len(rows) != 4 {
		t.Fatalf("Expected 4 rows, got %d", len(rows))
	}

	// Verify ID1: Synced
	r1 := findRow(rows, id1)
	if r1.HasUnpushed || r1.IsPullable {
		t.Errorf("Repo1 should be synced. Unpushed=%v, Pullable=%v", r1.HasUnpushed, r1.IsPullable)
	}

	// Verify ID2: Ahead
	r2 := findRow(rows, id2)
	if !r2.HasUnpushed {
		t.Errorf("Repo2 should be Unpushed")
	}
	if r2.IsPullable {
		t.Errorf("Repo2 should NOT be Pullable")
	}

	// Verify ID3: Behind
	r3 := findRow(rows, id3)
	if r3.HasUnpushed {
		t.Errorf("Repo3 should NOT be Unpushed")
	}
	if !r3.IsPullable {
		t.Errorf("Repo3 should be Pullable")
	}

	// Verify ID4: Diverged
	r4 := findRow(rows, id4)
	if !r4.HasUnpushed {
		t.Errorf("Repo4 should be Unpushed")
	}
	if !r4.IsPullable {
		t.Errorf("Repo4 should be Pullable")
	}
}

func findRow(rows []StatusRow, repoID string) StatusRow {
	for _, r := range rows {
		if r.Repo == repoID {
			return r
		}
	}
	return StatusRow{}
}
