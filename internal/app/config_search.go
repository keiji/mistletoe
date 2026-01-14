package app

import (
	"bufio"
	"fmt"
	conf "mistletoe/internal/config"
	"os"
	"path/filepath"
	"strings"
)

// SearchParentConfig attempts to find a configuration file in the parent directory
// of the current git repository if one is not found in the current directory.
// It performs validation and prompts the user for confirmation.
func SearchParentConfig(candidatePath string, configData []byte, gitPath string) (string, error) {
	// If configData is provided (stdin), or if candidatePath is NOT the default,
	// we rely on existing logic (caller will attempt to load it and fail if missing).
	// We only search if we are looking for the default config file.
	if len(configData) > 0 || candidatePath != DefaultConfigFile {
		return candidatePath, nil
	}

	// Check if candidatePath exists in current directory
	if _, err := os.Stat(candidatePath); err == nil {
		return candidatePath, nil
	} else if !os.IsNotExist(err) {
		// Error other than NotExist (e.g. permission)
		return candidatePath, err
	}

	// 1. Check if we are in a Git repository
	isInside, err := RunGit("", gitPath, false, "rev-parse", "--is-inside-work-tree")
	if err != nil || isInside != "true" {
		// Not in a git repo, or error checking. Return original to let it fail normally.
		return candidatePath, nil
	}

	// 2. Find Git root
	gitRoot, err := RunGit("", gitPath, false, "rev-parse", "--show-toplevel")
	if err != nil {
		return candidatePath, nil
	}

	// 3. Check parent directory
	parentDir := filepath.Dir(gitRoot)
	parentConfigPath := filepath.Join(parentDir, DefaultConfigFile) // .mstl/config.json

	if _, err := os.Stat(parentConfigPath); os.IsNotExist(err) {
		return candidatePath, nil
	}

	// 4. Validate parent config
	if err := validateParentConfig(parentConfigPath, parentDir, gitPath); err != nil {
		// Validation failed, do not prompt.
		// We could log this if verbose, but we don't have verbose flag here easily.
		// Just fall back.
		return candidatePath, nil
	}

	// 5. Prompt user
	fmt.Printf("Current directory does not have .mstl, but found one in %s/. Use this configuration? (yes/no): ", parentDir)

	// Read user input
	scanner := bufio.NewScanner(Stdin)
	if scanner.Scan() {
		input := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if input == "yes" || input == "y" {
			return parentConfigPath, nil
		}
	}

	// Default: return original (which will fail)
	return candidatePath, nil
}

func validateParentConfig(configPath, parentDir, gitPath string) error {
	// Load config without validation first? Or use standard loader?
	// Use standard loader.
	config, err := conf.LoadConfigFile(configPath)
	if err != nil {
		return err
	}

	if config.Repositories == nil {
		return fmt.Errorf("no repositories in config")
	}

	for _, repo := range *config.Repositories {
		repoDirName := conf.GetRepoDirName(repo)
		repoPath := filepath.Join(parentDir, repoDirName)

		// 1. Check if directory exists
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			return fmt.Errorf("directory %s not found", repoDirName)
		}

		// 2. Check if it is a Git repository
		// We execute git rev-parse inside the repoPath
		_, err := RunGit(repoPath, gitPath, false, "rev-parse", "--is-inside-work-tree")
		if err != nil {
			return fmt.Errorf("%s is not a git repository", repoDirName)
		}

		// 3. Check origin URL
		if repo.URL != nil {
			out, err := RunGit(repoPath, gitPath, false, "remote", "get-url", "origin")
			if err != nil {
				return fmt.Errorf("failed to get remote url for %s", repoDirName)
			}

			// Normalize check? Simple string equality for now.
			// Git might return url with .git or without.
			// Config might have .git or without.
			// Let's relax: check if one contains the other or identical.
			// Or strictly follow existing URL.
			configURL := strings.TrimSpace(*repo.URL)
			remoteURL := strings.TrimSpace(out)

			if configURL != remoteURL {
				// Try ignoring .git suffix
				cNorm := strings.TrimSuffix(configURL, ".git")
				rNorm := strings.TrimSuffix(remoteURL, ".git")
				if cNorm != rNorm {
					return fmt.Errorf("URL mismatch for %s: expected %s, got %s", repoDirName, configURL, remoteURL)
				}
			}
		}
	}

	return nil
}
