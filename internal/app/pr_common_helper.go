package app

import "strings"

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
