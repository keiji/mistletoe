package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"strings"
)

// GenerateSnapshot creates a snapshot JSON of the current state of repositories defined in config
// that also exist on disk.
// It returns the JSON content and a unique identifier based on the revisions.
func GenerateSnapshot(config *Config, gitPath string) ([]byte, string, error) {
	var currentRepos []Repository

	// Iterate config repos and check if they exist on disk.
	for _, repo := range *config.Repositories {
		dir := GetRepoDir(repo)
		if _, err := os.Stat(dir); err != nil {
			// Skip missing repos
			continue
		}

		// Get current state
		// URL
		url, err := RunGit(dir, gitPath, "config", "--get", "remote.origin.url")
		if err != nil {
			// Fallback to config URL if git fails
			if repo.URL != nil {
				url = *repo.URL
			}
		}

		// Branch
		branch, err := RunGit(dir, gitPath, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			branch = ""
		}

		// Revision
		revision, err := RunGit(dir, gitPath, "rev-parse", "HEAD")
		if err != nil {
			revision = ""
		}

		// Detached HEAD handling
		if branch == "HEAD" {
			branch = ""
		}

		// Use ID from config if present
		id := dir
		if repo.ID != nil && *repo.ID != "" {
			id = *repo.ID
		}

		var branchPtr *string
		if branch != "" {
			branchPtr = &branch
		}
		var revisionPtr *string
		if revision != "" {
			revisionPtr = &revision
		}
		urlPtr := &url

		currentRepos = append(currentRepos, Repository{
			ID:       &id,
			URL:      urlPtr,
			Branch:   branchPtr,
			Revision: revisionPtr,
		})
	}

	// Sort by ID
	sort.Slice(currentRepos, func(i, j int) bool {
		return *currentRepos[i].ID < *currentRepos[j].ID
	})

	// Calculate Identifier
	var revisions []string
	for _, r := range currentRepos {
		rev := ""
		if r.Revision != nil {
			rev = *r.Revision
		}
		revisions = append(revisions, rev)
	}
	concat := strings.Join(revisions, ",")
	hash := sha256.Sum256([]byte(concat))
	identifier := hex.EncodeToString(hash[:])

	// Create JSON
	snapshotConfig := Config{
		Repositories: &currentRepos,
	}
	data, err := json.MarshalIndent(snapshotConfig, "", "  ")
	if err != nil {
		return nil, "", err
	}

	return data, identifier, nil
}
