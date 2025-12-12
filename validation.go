package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ValidateRepositories checks if repositories exist and are valid git repositories with correct remote.
// It iterates all repositories in the list.
func ValidateRepositories(repos []Repository, gitPath string) error {
	for _, repo := range repos {
		targetDir := GetRepoDir(repo)
		info, err := os.Stat(targetDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("Error checking directory %s: %v", targetDir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("Error: target %s exists and is not a directory", targetDir)
		}

		// Check if Git repository
		gitDir := filepath.Join(targetDir, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			return fmt.Errorf("Error: directory %s exists but is not a git repository", targetDir)
		}

		// Check remote origin
		cmd := exec.Command(gitPath, "-C", targetDir, "config", "--get", "remote.origin.url")
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("Error: directory %s is a git repo but failed to get remote origin: %v", targetDir, err)
		}
		currentURL := strings.TrimSpace(string(out))
		if currentURL != *repo.URL {
			return fmt.Errorf("Error: directory %s exists with different remote origin: %s (expected %s)", targetDir, currentURL, *repo.URL)
		}
	}
	return nil
}
