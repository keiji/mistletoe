package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestValidateEnvironment(t *testing.T) {
	// Setup a temporary directory structure
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Create a valid git repo
	gitRepo := filepath.Join(tmpDir, "valid-repo")
	os.Mkdir(gitRepo, 0755)
	exec.Command("git", "init", gitRepo).Run()
	// Set remote origin
	exec.Command("git", "-C", gitRepo, "remote", "add", "origin", "https://github.com/example/valid.git").Run()

	// Create a non-git directory
	nonGitDir := filepath.Join(tmpDir, "non-git")
	os.Mkdir(nonGitDir, 0755)
	os.WriteFile(filepath.Join(nonGitDir, "file.txt"), []byte("content"), 0644)

	// Create an empty directory
	emptyDir := filepath.Join(tmpDir, "empty")
	os.Mkdir(emptyDir, 0755)

	validURL := "https://github.com/example/valid.git"
	invalidURL := "https://github.com/example/invalid.git"
	validID := "valid-repo"
	nonGitID := "non-git"
	emptyID := "empty"
	newID := "new-repo"

	tests := []struct {
		name    string
		repos   []Repository
		wantErr bool
	}{
		{
			name: "Valid environment - existing correct repo",
			repos: []Repository{
				{ID: &validID, URL: &validURL},
			},
			wantErr: false,
		},
		{
			name: "Invalid environment - existing repo wrong remote",
			repos: []Repository{
				{ID: &validID, URL: &invalidURL},
			},
			wantErr: true,
		},
		{
			name: "Invalid environment - non-git directory not empty",
			repos: []Repository{
				{ID: &nonGitID, URL: &validURL},
			},
			wantErr: true,
		},
		{
			name: "Valid environment - empty directory (will be cloned into)",
			repos: []Repository{
				{ID: &emptyID, URL: &validURL},
			},
			wantErr: false,
		},
		{
			name: "Valid environment - directory does not exist",
			repos: []Repository{
				{ID: &newID, URL: &validURL},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Change to the temp dir so the relative path logic in validateEnvironment works
			os.Chdir(tmpDir)
			// Pass false for verbose
			err := validateEnvironment(tt.repos, "git", false)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEnvironment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
