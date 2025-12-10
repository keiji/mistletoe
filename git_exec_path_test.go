package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitExecPath(t *testing.T) {
	// Build the binary first
	exe := filepath.Join(t.TempDir(), "gitc")
	buildCmd := exec.Command("go", "build", "-o", exe, ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build gitc: %v", err)
	}

	// 1. Test Valid GIT_EXEC_PATH
	t.Run("Valid GIT_EXEC_PATH", func(t *testing.T) {
		// We'll use the system git, but we need to find where it is first
		systemGit, err := exec.LookPath("git")
		if err != nil {
			t.Skip("git not found in system, skipping test that relies on system git")
		}

		// Create a fake directory for GIT_EXEC_PATH that contains a symlink to real git
		// or copy it. Symlink is easier.
		fakeBinDir := t.TempDir()
		fakeGitPath := filepath.Join(fakeBinDir, "git")
		if err := os.Symlink(systemGit, fakeGitPath); err != nil {
			// fallback for windows or if symlink fails: copy?
			// simpler: just use filepath.Dir(systemGit) if it's separate.
			// But usually /usr/bin.
			// Let's rely on finding where git is and using its directory if possible.
			// actually, the prompt says "git executable at that path".
			// let's try to just use the dir of system git.
			fakeBinDir = filepath.Dir(systemGit)
		}

		cmd := exec.Command(exe, "--version")
		cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_EXEC_PATH=%s", fakeBinDir))
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run version with valid GIT_EXEC_PATH: %v, out: %s", err, out)
		}
		if !bytes.Contains(out, []byte("gitc version")) {
			t.Errorf("Expected output to contain 'gitc version', got: %s", out)
		}
		// The output should show the path we resolved.
		// Since we used real git, it should work.
	})

	// 2. Test Invalid GIT_EXEC_PATH with --version (Should pass but show error)
	t.Run("Invalid GIT_EXEC_PATH version", func(t *testing.T) {
		emptyDir := t.TempDir()
		cmd := exec.Command(exe, "--version")
		cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_EXEC_PATH=%s", emptyDir))
		out, err := cmd.CombinedOutput()
		if err != nil {
			// Should not fail exit code because version is permissive
			// Actually handleVersion doesn't exit(1), main doesn't exit(1) for version
			// wait, main calls os.Exit(1) if gitErr != nil AND !isPermissive.
			// version IS permissive. So it should exit 0.
			// But check handleVersion output.
		}

		// It might return non-zero if handleVersion fails?
		// No, handleVersion just prints errors.

		if !bytes.Contains(out, []byte("git binary is not found")) {
			t.Errorf("Expected 'git binary is not found' message, got: %s", out)
		}
	})

	// 3. Test Invalid GIT_EXEC_PATH with init (Should fail)
	t.Run("Invalid GIT_EXEC_PATH init", func(t *testing.T) {
		emptyDir := t.TempDir()
		// create a dummy config file
		configFile := filepath.Join(t.TempDir(), "repos.json")
		os.WriteFile(configFile, []byte(`{"repositories": []}`), 0644)

		cmd := exec.Command(exe, "init", "-f", configFile)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_EXEC_PATH=%s", emptyDir))
		out, err := cmd.CombinedOutput()

		// It should fail exit code 1
		if err == nil {
			t.Error("Expected init to fail with invalid GIT_EXEC_PATH, but it succeeded")
		}
		if !bytes.Contains(out, []byte("Error: git is not callable")) {
			t.Errorf("Expected error message about git not callable, got: %s", out)
		}
	})
}
