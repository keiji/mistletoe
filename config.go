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

// FilterRepositories filters repositories based on provided labels.
// If labelsStr is empty, returns all repositories.
// If labelsStr contains comma-separated labels, returns repositories that match at least one label.
func FilterRepositories(repos []Repository, labelsStr string) []Repository {
	if labelsStr == "" {
		return repos
	}

	targets := strings.Split(labelsStr, ",")
	targetMap := make(map[string]bool)
	for _, t := range targets {
		trimmed := strings.TrimSpace(t)
		if trimmed != "" {
			targetMap[trimmed] = true
		}
	}

	if len(targetMap) == 0 {
		return repos
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
