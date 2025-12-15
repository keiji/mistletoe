package app

import (
	"encoding/json"
	"flag"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
)

func handleSnapshot(args []string, opts GlobalOptions) {
	var (
		oLong  string
		oShort string
		fLong  string
		fShort string
	)
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	fs.StringVar(&oLong, "output-file", "", "output file path")
	fs.StringVar(&oShort, "o", "", "output file path (short)")
	fs.StringVar(&fLong, "file", "", "Configuration file path")
	fs.StringVar(&fShort, "f", "", "Configuration file path (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	outputFile := oLong
	if outputFile == "" {
		outputFile = oShort
	}

	// Load Config (Optional) to resolve base branches
	var config *Config
	if fLong != "" || fShort != "" {
		configPath, _, configData, err := ResolveCommonValues(fLong, fShort, 0, 0)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if configPath != "" {
			config, err = loadConfigFile(configPath)
		} else {
			config, err = loadConfigData(configData)
		}
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		fmt.Printf("Error reading current directory: %v.\n", err)
		os.Exit(1)
	}

	var repos []Repository

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		gitDir := fmt.Sprintf("%s/.git", dirName)

		if _, err := os.Stat(gitDir); err != nil {
			// Not a git repository
			continue
		}

		// Get remote origin URL
		url, err := RunGit(dirName, opts.GitPath, "remote", "get-url", "origin")
		if err != nil {
			// Try getting it via config if get-url fails (older git versions or odd setups)
			url, err = RunGit(dirName, opts.GitPath, "config", "--get", "remote.origin.url")
			if err != nil {
				fmt.Printf("Warning: Could not get remote origin for %s. Skipping.\n", dirName)
				continue
			}
		}
		// RunGit already trims

		// Get current branch
		branch, err := RunGit(dirName, opts.GitPath, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			fmt.Printf("Warning: Could not get current branch for %s.\n", dirName)
			branch = ""
		}

		revision := ""
		// If branch is "HEAD", it's a detached HEAD state
		if branch == "HEAD" {
			branch = ""
			revision, err = RunGit(dirName, opts.GitPath, "rev-parse", "HEAD")
			if err != nil {
				fmt.Printf("Warning: Could not get revision for %s.\n", dirName)
				revision = ""
			}
		}

		id := dirName
		// Construct repository
		var branchPtr *string
		if branch != "" {
			branchPtr = &branch
		}
		var revisionPtr *string
		if revision != "" {
			revisionPtr = &revision
		}
		urlPtr := &url

		// Resolve BaseBranch from Config
		var baseBranchPtr *string
		if config != nil && config.Repositories != nil {
			for _, confRepo := range *config.Repositories {
				confID := GetRepoDir(confRepo)
				if confID == dirName {
					// Found matching repo in config
					// "If base-branch is specified... uses loaded config's branch as base-branch"
					// "If base-branch not specified... treat base-branch and branch as same."
					// Implementation:
					// If confRepo.BaseBranch exists, use it.
					// If confRepo.BaseBranch missing, use confRepo.Branch.
					if confRepo.BaseBranch != nil && *confRepo.BaseBranch != "" {
						baseBranchPtr = confRepo.BaseBranch
					} else if confRepo.Branch != nil && *confRepo.Branch != "" {
						baseBranchPtr = confRepo.Branch
					}
					break
				}
			}
		}

		repo := Repository{
			ID:         &id,
			URL:        urlPtr,
			Branch:     branchPtr,
			Revision:   revisionPtr,
			BaseBranch: baseBranchPtr,
		}
		repos = append(repos, repo)
	}

	if outputFile == "" {
		identifier := CalculateSnapshotIdentifier(repos)
		outputFile = fmt.Sprintf("mistletoe-snapshot-%s.json", identifier)
	}

	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("Error: Output file '%s' exists.\n", outputFile)
		os.Exit(1)
	}

	outputConfig := Config{
		Repositories: &repos,
	}

	data, err := json.MarshalIndent(outputConfig, "", "  ")
	if err != nil {
		fmt.Printf("Error generating JSON: %v.\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		fmt.Printf("Error writing to file '%s': %v.\n", outputFile, err)
		os.Exit(1)
	}

	fmt.Printf("Snapshot saved to %s\n", outputFile)
}

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

		// Resolve BaseBranch
		// "pr create: generated snapshot's base-branch uses config's base-branch. If no base-branch, use branch."
		var baseBranchPtr *string
		if repo.BaseBranch != nil && *repo.BaseBranch != "" {
			baseBranchPtr = repo.BaseBranch
		} else if repo.Branch != nil && *repo.Branch != "" {
			baseBranchPtr = repo.Branch
		}

		currentRepos = append(currentRepos, Repository{
			ID:         &id,
			URL:        urlPtr,
			Branch:     branchPtr,
			Revision:   revisionPtr,
			BaseBranch: baseBranchPtr,
		})
	}

	identifier := CalculateSnapshotIdentifier(currentRepos)

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

// CalculateSnapshotIdentifier calculates the unique identifier for a list of repositories.
// It sorts the repositories by ID to ensure a deterministic hash.
func CalculateSnapshotIdentifier(repos []Repository) string {
	// Sort by ID
	sort.Slice(repos, func(i, j int) bool {
		return *repos[i].ID < *repos[j].ID
	})

	var parts []string
	for _, r := range repos {
		val := ""
		if r.Branch != nil && *r.Branch != "" {
			val = *r.Branch
		} else if r.Revision != nil {
			val = *r.Revision
		}
		parts = append(parts, val)
	}
	concat := strings.Join(parts, ",")
	hash := sha256.Sum256([]byte(concat))
	return hex.EncodeToString(hash[:])
}
