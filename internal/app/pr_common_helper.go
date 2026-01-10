package app


import (
	"strings"
	"fmt"
)

// GetGhUser returns the current authenticated GitHub user's login.
func GetGhUser(ghPath string, verbose bool) (string, error) {
	out, err := RunGh(ghPath, verbose, "api", "user", "--jq", ".login")
	if err != nil {
		return "", fmt.Errorf("failed to get GitHub user: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// ValidatePrPermissionAndOverwrite checks if we can overwrite an existing PR.
// It returns nil if allowed, or an error if permission denied or overwrite flag required.
func ValidatePrPermissionAndOverwrite(repoID string, pr PrInfo, currentUser string, overwrite bool) error {
	// 1. Permission Check
	if !pr.ViewerCanEditFiles {
		return fmt.Errorf("permission denied: you do not have edit permission for PR %s (Repo: %s)", pr.URL, repoID)
	}

	// 2. Overwrite Logic
	// Check for Mistletoe block
	_, _, _, found := ParseMistletoeBlock(pr.Body)
	if found {
		// Existing block found -> Safe to overwrite
		return nil
	}

	// No Mistletoe block
	if strings.EqualFold(pr.Author.Login, currentUser) {
		// Creator is me -> Safe to overwrite (append)
		return nil
	}

	// Creator is NOT me
	if overwrite {
		// Overwrite flag set -> Safe
		return nil
	}

	return fmt.Errorf("PR %s (Repo: %s) was created by %s and does not have a Mistletoe block. Use --overwrite (-w) to force update", pr.URL, repoID, pr.Author.Login)
}

// isPrFromConfiguredRepo checks if the PR's head repository matches the configured repository URL.
// It handles potential .git suffix differences and URL protocol variations by relying on canonical URL comparison if available.
func isPrFromConfiguredRepo(pr PrInfo, configCanonicalURL string) bool {
	if pr.HeadRepository.URL != nil && *pr.HeadRepository.URL != "" {
		prHead := strings.TrimSuffix(*pr.HeadRepository.URL, ".git")
		confURL := strings.TrimSuffix(configCanonicalURL, ".git")
		return strings.EqualFold(prHead, confURL)
	}
	// Fallback: If HeadRepository is missing from response, assume it's a match to be safe.
	return true
}
