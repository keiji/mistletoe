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
	"sync"
)

func handleSnapshot(args []string, opts GlobalOptions) {
	var (
		oLong     string
		oShort    string
		fLong     string
		fShort    string
		pVal      int
		pValShort int
		vLong     bool
		vShort    bool
	)
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	fs.StringVar(&oLong, "output-file", "", "output file path")
	fs.StringVar(&oShort, "o", "", "output file path (short)")
	fs.StringVar(&fLong, "file", "", "Configuration file path")
	fs.StringVar(&fShort, "f", "", "Configuration file path (shorthand)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "number of parallel processes (short)")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

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
	configPath, parallel, configData, err := ResolveCommonValues(fLong, fShort, pVal, pValShort)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	verbose := vLong || vShort

	if configPath != "" || len(configData) > 0 {
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

	// Collect valid git directories
	var validDirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		gitDir := fmt.Sprintf("%s/.git", entry.Name())
		if _, err := os.Stat(gitDir); err == nil {
			validDirs = append(validDirs, entry.Name())
		}
	}

	var repos []Repository
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)

	for _, dirName := range validDirs {
		wg.Add(1)
		go func(dirName string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Get remote origin URL
			url, err := RunGit(dirName, opts.GitPath, verbose, "remote", "get-url", "origin")
			if err != nil {
				// Try getting it via config if get-url fails (older git versions or odd setups)
				url, err = RunGit(dirName, opts.GitPath, verbose, "config", "--get", "remote.origin.url")
				if err != nil {
					fmt.Printf("Warning: Could not get remote origin for %s. Skipping.\n", dirName)
					return
				}
			}

			// Get current branch
			branch, err := RunGit(dirName, opts.GitPath, verbose, "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				fmt.Printf("Warning: Could not get current branch for %s.\n", dirName)
				branch = ""
			}

			revision := ""
			// If branch is "HEAD", it's a detached HEAD state
			if branch == "HEAD" {
				branch = ""
				revision, err = RunGit(dirName, opts.GitPath, verbose, "rev-parse", "HEAD")
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

			mu.Lock()
			repos = append(repos, repo)
			mu.Unlock()

		}(dirName)
	}
	wg.Wait()

	if outputFile == "" {
		identifier := CalculateSnapshotIdentifier(repos)
		outputFile = fmt.Sprintf("mistletoe-snapshot-%s.json", identifier)
	}

	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("Error: Output file '%s' exists.\n", outputFile)
		os.Exit(1)
	}

	// Sort Repos to match order of CalculateSnapshotIdentifier (and general neatness)
	// Note: CalculateSnapshotIdentifier also sorts, but we sort here for the Config struct output
	sort.Slice(repos, func(i, j int) bool {
		return *repos[i].ID < *repos[j].ID
	})

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
	// GenerateSnapshot is usually called by pr create, which doesn't expose a verbose flag to GenerateSnapshot yet
	// But `pr create` has verbose. We should add verbose to GenerateSnapshot signature if we want logs inside it.
	// Currently it calls RunGit. Let's default false unless we update signature.
	// Wait, I am updating snapshot.go. I should update signature.
	// But GenerateSnapshot is exported. I need to check callers.
	// Caller: `pr create` (handlePrCreate in pr.go)

	// I'll update signature to `GenerateSnapshot(config *Config, gitPath string, verbose bool)`
	return GenerateSnapshotVerbose(config, gitPath, false)
}

// GenerateSnapshotVerbose creates a snapshot JSON with verbosity control.
func GenerateSnapshotVerbose(config *Config, gitPath string, verbose bool) ([]byte, string, error) {
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
		url, err := RunGit(dir, gitPath, verbose, "config", "--get", "remote.origin.url")
		if err != nil {
			// Fallback to config URL if git fails
			if repo.URL != nil {
				url = *repo.URL
			}
		}

		// Branch
		branch, err := RunGit(dir, gitPath, verbose, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			branch = ""
		}

		// Revision
		revision, err := RunGit(dir, gitPath, verbose, "rev-parse", "HEAD")
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
