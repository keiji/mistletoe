package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyRevisionsUnchanged(t *testing.T) {
	// Setup a real git repo
	remoteURL, contentDir := setupRemoteAndContent(t, 1)

	// Get the initial revision (HEAD)
	out, err := exec.Command("git", "-C", contentDir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}
	initialHead := strings.TrimSpace(string(out))

	// Construct Config
	repoID := filepath.Base(contentDir)
	baseDir := filepath.Dir(contentDir)

	config := &Config{
		Repositories: &[]Repository{
			{
				ID:  strPtr(repoID),
				URL: strPtr(remoteURL),
			},
		},
		BaseDir: baseDir,
	}

	// Construct StatusRow matching the initial state
	rows := []StatusRow{
		{
			Repo:          repoID, // Repo name in map matches the ID
			LocalHeadFull: initialHead,
		},
	}

	// Case 1: No Change
	t.Run("NoChange", func(t *testing.T) {
		// Provide "git" explicitly as path
		err := VerifyRevisionsUnchanged(config, rows, "git", false)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	// Case 2: Change detected
	t.Run("ChangeDetected", func(t *testing.T) {
		// Make a new commit
		fname := "newfile.txt"
		if err := os.WriteFile(contentDir+"/"+fname, []byte("change"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		exec.Command("git", "-C", contentDir, "add", ".").Run()
		exec.Command("git", "-C", contentDir, "commit", "-m", "changed").Run()

		// Verify
		err := VerifyRevisionsUnchanged(config, rows, "git", false)
		if err == nil {
			t.Error("expected error, got nil")
		} else {
			if !strings.Contains(err.Error(), "has changed since status collection") {
				t.Errorf("expected error message to contain 'has changed since status collection', got: %v", err)
			}
		}
	})
}
