package app

import (
	conf "mistletoe/internal/config"
)

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
		jVal      int
		jValShort int
		vLong     bool
		vShort    bool
	)
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	fs.SetOutput(Stderr)
	fs.StringVar(&oLong, "output-file", "", "output file path")
	fs.StringVar(&oShort, "o", "", "output file path (shorthand)")
	fs.StringVar(&fLong, "file", DefaultConfigFile, "configuration file")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "configuration file (shorthand)")
	fs.IntVar(&jVal, "jobs", -1, "number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "number of concurrent jobs (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Fprintln(Stderr, "Error parsing flags:", err)
		osExit(1)
		return
	}

	if err := CheckFlagDuplicates(fs, [][2]string{
		{"output-file", "o"},
		{"file", "f"},
		{"jobs", "j"},
		{"verbose", "v"},
	}); err != nil {
		fmt.Fprintln(Stderr, "Error:", err)
		osExit(1)
		return
	}

	outputFile := oLong
	if outputFile == "" {
		outputFile = oShort
	}

	// Load conf.Config (Optional) to resolve base branches
	var config *conf.Config
	configPath, jobsFlag, configData, err := ResolveCommonValues(fLong, fShort, jVal, jValShort, ignoreStdin)
	if err != nil {
		fmt.Fprintln(Stderr, err)
		osExit(1)
		return
	}

	if configPath != "" || len(configData) > 0 {
		if configPath != "" {
			config, err = conf.LoadConfigFile(configPath)
		} else {
			config, err = conf.LoadConfigData(configData)
		}
		if err != nil {
			fmt.Fprintln(Stderr, err)
			osExit(1)
			return
		}
	}

	// Resolve Jobs
	jobs, err := DetermineJobs(jobsFlag, config)
	if err != nil {
		fmt.Fprintf(Stderr, "Error: %v\n", err)
		osExit(1)
		return
	}

	// Verbose Override
	verbose := vLong || vShort
	if verbose && jobs > 1 {
		fmt.Fprintln(Stdout, "Verbose is specified, so jobs is treated as 1.")
		jobs = 1
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		fmt.Fprintf(Stderr, "Error reading current directory: %v.\n", err)
		osExit(1)
		return
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

	var repos []conf.Repository
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)

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
					fmt.Fprintf(Stdout, "Warning: Could not get remote origin for %s. Skipping.\n", dirName)
					return
				}
			}

			// Get current branch
			branch, err := RunGit(dirName, opts.GitPath, verbose, "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				fmt.Fprintf(Stdout, "Warning: Could not get current branch for %s.\n", dirName)
				branch = ""
			}

			revision := ""
			// If branch is "HEAD", it's a detached HEAD state
			if branch == "HEAD" {
				branch = ""
				revision, err = RunGit(dirName, opts.GitPath, verbose, "rev-parse", "HEAD")
				if err != nil {
					fmt.Fprintf(Stdout, "Warning: Could not get revision for %s.\n", dirName)
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

			// Resolve BaseBranch from conf.Config
			var baseBranchPtr *string
			if config != nil && config.Repositories != nil {
				for _, confRepo := range *config.Repositories {
					confID := conf.GetRepoDirName(confRepo)
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

			repo := conf.Repository{
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

	fmt.Fprintf(Stderr, "DEBUG: Checking existence of %s\n", outputFile)
	if _, err := os.Stat(outputFile); err == nil {
		fmt.Fprintf(Stderr, "Error: Output file '%s' exists.\n", outputFile)
		osExit(1)
		return
	}

	// Sort Repos to match order of CalculateSnapshotIdentifier (and general neatness)
	// Note: CalculateSnapshotIdentifier also sorts, but we sort here for the conf.Config struct output
	sort.Slice(repos, func(i, j int) bool {
		return *repos[i].ID < *repos[j].ID
	})

	outputConfig := conf.Config{
		Repositories: &repos,
	}

	data, err := json.MarshalIndent(outputConfig, "", "  ")
	if err != nil {
		fmt.Fprintf(Stderr, "Error generating JSON: %v.\n", err)
		osExit(1)
		return
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		fmt.Fprintf(Stderr, "Error writing to file '%s': %v.\n", outputFile, err)
		osExit(1)
		return
	}

	fmt.Fprintf(Stdout, "Snapshot saved to %s\n", outputFile)
}

// GenerateSnapshot creates a snapshot JSON of the current state of repositories defined in config
// that also exist on disk.
// It returns the JSON content and a unique identifier based on the revisions.
func GenerateSnapshot(config *conf.Config, gitPath string) ([]byte, string, error) {
	// GenerateSnapshot is usually called by pr create, which doesn't expose a verbose flag to GenerateSnapshot yet
	// But `pr create` has verbose. We should add verbose to GenerateSnapshot signature if we want logs inside it.
	// Currently it calls RunGit. Let's default false unless we update signature.
	// Wait, I am updating snapshot.go. I should update signature.
	// But GenerateSnapshot is exported. I need to check callers.
	// Caller: `pr create` (handlePrCreate in pr.go)

	// I'll update signature to `GenerateSnapshot(config *conf.Config, gitPath string, verbose bool)`
	return GenerateSnapshotVerbose(config, gitPath, false)
}

// GenerateSnapshotVerbose creates a snapshot JSON with verbosity control.
func GenerateSnapshotVerbose(config *conf.Config, gitPath string, verbose bool) ([]byte, string, error) {
	var currentRepos []conf.Repository

	// Iterate config repos and check if they exist on disk.
	for _, repo := range *config.Repositories {
		dir := config.GetRepoPath(repo)
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

		currentRepos = append(currentRepos, conf.Repository{
			ID:         &id,
			URL:        urlPtr,
			Branch:     branchPtr,
			Revision:   revisionPtr,
			BaseBranch: baseBranchPtr,
		})
	}

	identifier := CalculateSnapshotIdentifier(currentRepos)

	// Create JSON
	snapshotConfig := conf.Config{
		Repositories: &currentRepos,
	}
	data, err := json.MarshalIndent(snapshotConfig, "", "  ")
	if err != nil {
		return nil, "", err
	}

	return data, identifier, nil
}

// GenerateSnapshotFromStatus creates a snapshot JSON using cached status rows.
func GenerateSnapshotFromStatus(config *conf.Config, statusRows []StatusRow) ([]byte, string, error) {
	var currentRepos []conf.Repository
	statusMap := make(map[string]StatusRow)
	for _, row := range statusRows {
		statusMap[row.Repo] = row
	}

	for _, repo := range *config.Repositories {
		repoName := conf.GetRepoDirName(repo)
		if repo.ID != nil && *repo.ID != "" {
			repoName = *repo.ID
		}

		row, ok := statusMap[repoName]
		if !ok {
			// Repo not found in status (maybe directory missing), skip
			continue
		}

		// Use cached data
		// URL: we don't store URL in StatusRow, but we have it in conf.Config.
		// However, GenerateSnapshot checked `git config --get remote.origin.url`.
		// StatusRow validation ensures URL matches config. So we can use config URL safely.
		url := ""
		if repo.URL != nil {
			url = *repo.URL
		}
		urlPtr := &url

		// Branch
		branch := row.BranchName
		if branch == "HEAD" {
			branch = ""
		}

		// Revision
		// LocalHeadFull was added to StatusRow for this purpose
		revision := row.LocalHeadFull

		// Use ID from config if present
		id := repoName

		var branchPtr *string
		if branch != "" {
			branchPtr = &branch
		}
		var revisionPtr *string
		if revision != "" {
			revisionPtr = &revision
		}

		// Resolve BaseBranch
		var baseBranchPtr *string
		if repo.BaseBranch != nil && *repo.BaseBranch != "" {
			baseBranchPtr = repo.BaseBranch
		} else if repo.Branch != nil && *repo.Branch != "" {
			baseBranchPtr = repo.Branch
		}

		currentRepos = append(currentRepos, conf.Repository{
			ID:         &id,
			URL:        urlPtr,
			Branch:     branchPtr,
			Revision:   revisionPtr,
			BaseBranch: baseBranchPtr,
		})
	}

	identifier := CalculateSnapshotIdentifier(currentRepos)

	snapshotConfig := conf.Config{
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
func CalculateSnapshotIdentifier(repos []conf.Repository) string {
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
