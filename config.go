package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
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
		return nil, errors.New("データの形式が正しくありません")
	}

	if config.Repositories == nil {
		return nil, errors.New("データの形式が正しくありません")
	}

	for _, repo := range *config.Repositories {
		if repo.URL == nil {
			return nil, errors.New("データの形式が正しくありません")
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
				return fmt.Errorf("duplicate repository ID found: %s", *repo.ID)
			}
			seenIDs[*repo.ID] = true
		}
	}
	return nil
}

// getRepoDir determines the checkout directory name.
// If ID is present and not empty, it is used. Otherwise, it is derived from the URL.
func getRepoDir(repo Repository) string {
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
			return nil, errors.New("ファイルが見つからない")
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
