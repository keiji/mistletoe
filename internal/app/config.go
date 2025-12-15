package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ErrConfigFileNotFound = errors.New("File not found")
	ErrInvalidDataFormat  = errors.New("Invalid data format")
	ErrDuplicateID        = errors.New("Duplicate repository ID")
	ErrInvalidFilePath    = errors.New("Invalid file path")
	ErrInvalidID          = errors.New("Invalid repository ID")
	ErrInvalidURL         = errors.New("Invalid repository URL")
	ErrInvalidGitRef      = errors.New("Invalid git reference")
)

var (
	// idRegex enforces safe characters for directory names.
	// Alphanumeric, underscore, hyphen, dot.
	idRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

	// safeGitRefRegex allows alphanumeric, slash, dot, underscore, hyphen.
	// It is a subset of what git allows, but safe for our usage.
	safeGitRefRegex = regexp.MustCompile(`^[a-zA-Z0-9./_-]+$`)
)

type Repository struct {
	ID       *string  `json:"id"`
	URL      *string  `json:"url"`
	Branch   *string  `json:"branch,omitempty"`
	Revision *string  `json:"revision,omitempty"`
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
// If an ID is missing, it is derived from the URL.
func validateRepositories(repos []Repository) error {
	seenIDs := make(map[string]bool)
	for i := range repos {
		repo := &repos[i]
		if repo.ID == nil || *repo.ID == "" {
			if repo.URL == nil {
				// Should have been caught by ParseConfig, but just in case
				continue
			}
			url := strings.TrimRight(*repo.URL, "/")
			base := path.Base(url)
			id := strings.TrimSuffix(base, ".git")
			repo.ID = &id
		}

		// Validate ID
		if !idRegex.MatchString(*repo.ID) {
			return fmt.Errorf("%w: %s (contains unsafe characters)", ErrInvalidID, *repo.ID)
		}
		if *repo.ID == "." || *repo.ID == ".." {
			return fmt.Errorf("%w: %s (cannot be . or ..)", ErrInvalidID, *repo.ID)
		}
		// Redundant check for abs path (idRegex excludes slash/backslash), but kept for clarity
		if filepath.IsAbs(*repo.ID) {
			return fmt.Errorf("%w: %s (must be relative)", ErrInvalidFilePath, *repo.ID)
		}

		// Validate URL
		if repo.URL != nil {
			if strings.HasPrefix(*repo.URL, "ext::") {
				return fmt.Errorf("%w: %s (ext:: protocol not allowed)", ErrInvalidURL, *repo.URL)
			}
			// Check for control characters
			if strings.ContainsAny(*repo.URL, "\n\r\t") {
				return fmt.Errorf("%w: %s (contains control characters)", ErrInvalidURL, *repo.URL)
			}
		}

		// Validate Branch
		if repo.Branch != nil && *repo.Branch != "" {
			if !isValidGitRef(*repo.Branch) {
				return fmt.Errorf("%w: %s", ErrInvalidGitRef, *repo.Branch)
			}
		}

		// Validate Revision
		if repo.Revision != nil && *repo.Revision != "" {
			if !isValidGitRef(*repo.Revision) {
				return fmt.Errorf("%w: %s", ErrInvalidGitRef, *repo.Revision)
			}
		}

		if seenIDs[*repo.ID] {
			return fmt.Errorf("%w: %s", ErrDuplicateID, *repo.ID)
		}
		seenIDs[*repo.ID] = true
	}
	return nil
}

func isValidGitRef(ref string) bool {
	// Prevent flag injection
	if strings.HasPrefix(ref, "-") {
		return false
	}
	return safeGitRefRegex.MatchString(ref)
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

func loadConfig(configFile string, configData []byte) (*Config, error) {
	var data []byte
	var err error

	if len(configData) > 0 {
		data = configData
	} else {
		if configFile == "" {
			return nil, errors.New("Error: Specify configuration file using --file or -f.")
		}

		data, err = os.ReadFile(configFile)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, ErrConfigFileNotFound
			}
			return nil, fmt.Errorf("Error reading file: %v.", err)
		}
	}

	config, err := ParseConfig(data)
	if err != nil {
		return nil, err
	}

	if err := validateRepositories(*config.Repositories); err != nil {
		return nil, fmt.Errorf("Error validating configuration: %v.", err)
	}

	return config, nil
}
