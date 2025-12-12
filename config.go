package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
)

var (
	ErrConfigFileNotFound = errors.New("File not found")
	ErrInvalidDataFormat  = errors.New("Invalid data format")
	ErrDuplicateID        = errors.New("Duplicate repository ID")
)

type Repository struct {
	ID       *string  `json:"id"`
	URL      *string  `json:"url"`
	Branch   *string  `json:"branch,omitempty"`
	Revision *string  `json:"revision,omitempty"`
	Labels   []string `json:"labels"`
}

type Config struct {
	Repositories *[]Repository `json:"repositories"`
}

func ParseConfig(data []byte) (*Config, error) {
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, ErrInvalidDataFormat
	}

	if config.Repositories == nil {
		return nil, ErrInvalidDataFormat
	}

	for _, repo := range *config.Repositories {
		if repo.URL == nil {
			return nil, ErrInvalidDataFormat
		}
	}

	return &config, nil
}

// validateRepositories checks for duplicate IDs in the repository list.
// IDs that are nil are ignored.
func validateRepositories(repos []Repository) error {
	seenIDs := make(map[string]bool)
	for _, repo := range repos {
		if repo.ID != nil && *repo.ID != "" {
			if seenIDs[*repo.ID] {
				return fmt.Errorf("%w: %s", ErrDuplicateID, *repo.ID)
			}
			seenIDs[*repo.ID] = true
		}
	}
	return nil
}

// GetRepoDir determines the checkout directory name.
// If ID is present and not empty, it is used. Otherwise, it is derived from the URL.
func GetRepoDir(repo Repository) string {
	if repo.ID != nil && *repo.ID != "" {
		return *repo.ID
	}
	// Derive from URL using path.Base because URLs use forward slashes
	if repo.URL == nil {
		return ""
	}
	url := strings.TrimRight(*repo.URL, "/")
	base := path.Base(url)
	return strings.TrimSuffix(base, ".git")
}

func loadConfig(configFile string) (*Config, error) {
	if configFile == "" {
		return nil, errors.New("Error: Please specify a configuration file using --file or -f")
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConfigFileNotFound
		}
		return nil, fmt.Errorf("Error reading file: %v", err)
	}

	config, err := ParseConfig(data)
	if err != nil {
		return nil, err
	}

	if err := validateRepositories(*config.Repositories); err != nil {
		return nil, fmt.Errorf("Error validating configuration: %v", err)
	}

	return config, nil
}

// ParseLabels parses a comma-separated string into a slice of labels.
func ParseLabels(labelsStr string) []string {
	var labels []string
	if labelsStr == "" {
		return labels
	}
	parts := strings.Split(labelsStr, ",")
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			labels = append(labels, trimmed)
		}
	}
	return labels
}

// FilterRepositories filters repositories based on provided labels.
// If labels is empty, returns all repositories.
// If labels is not empty, returns repositories that match at least one label.
func FilterRepositories(repos []Repository, labels []string) []Repository {
	if len(labels) == 0 {
		return repos
	}

	targetMap := make(map[string]bool)
	for _, l := range labels {
		targetMap[l] = true
	}

	var filtered []Repository
	for _, repo := range repos {
		matched := false
		for _, l := range repo.Labels {
			if targetMap[l] {
				matched = true
				break
			}
		}
		if matched {
			filtered = append(filtered, repo)
		}
	}
	return filtered
}
