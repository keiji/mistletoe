package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// strPtr returns a pointer to the string.
func strPtr(s string) *string {
	return &s
}

func getModuleRoot(t *testing.T) string {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get module root: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// buildMstl builds the mstl binary and returns its path.
func buildMstl(t *testing.T) string {
	binPath := filepath.Join(t.TempDir(), "mstl")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	rootDir := getModuleRoot(t)
	cmdPath := filepath.Join(rootDir, "cmd", "mstl")

	buildCmd := exec.Command("go", "build", "-o", binPath, cmdPath)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build mstl: %v\nOutput: %s", err, out)
	}
	return binPath
}

// createDummyGitRepo creates a basic git repo with a remote origin (init + remote add).
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

// configureGitUser sets dummy user info for a repo to allow commits.
func configureGitUser(t *testing.T, dir string) {
	if err := exec.Command("git", "-C", dir, "config", "user.email", "test@example.com").Run(); err != nil {
		t.Fatalf("failed to config user.email in %s: %v", dir, err)
	}
	if err := exec.Command("git", "-C", dir, "config", "user.name", "Test User").Run(); err != nil {
		t.Fatalf("failed to config user.name in %s: %v", dir, err)
	}
}

// setupRemoteAndContent initializes a bare repo and a content repo, returns remote URL (file://) and content dir.
func setupRemoteAndContent(t *testing.T, commitCount int) (string, string) {
	remoteDir := t.TempDir()
	if err := exec.Command("git", "init", "--bare", remoteDir).Run(); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}

	contentDir := t.TempDir()
	if err := exec.Command("git", "init", contentDir).Run(); err != nil {
		t.Fatalf("failed to init content repo: %v", err)
	}
	configureGitUser(t, contentDir)

	// Normalize path for Windows compatibility if needed
	remoteURL := "file://" + filepath.ToSlash(remoteDir)

	if err := exec.Command("git", "-C", contentDir, "remote", "add", "origin", remoteURL).Run(); err != nil {
		t.Fatalf("failed to add remote origin: %v", err)
	}

	for i := 0; i < commitCount; i++ {
		fname := filepath.Join(contentDir, fmt.Sprintf("file-%d.txt", i))
		if err := os.WriteFile(fname, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		exec.Command("git", "-C", contentDir, "add", ".").Run()
		exec.Command("git", "-C", contentDir, "commit", "-m", fmt.Sprintf("commit %d", i)).Run()
	}

	if commitCount > 0 {
		if err := exec.Command("git", "-C", contentDir, "push", "origin", "master").Run(); err != nil {
			t.Fatalf("failed to push: %v", err)
		}
	}

	return remoteURL, contentDir
}
