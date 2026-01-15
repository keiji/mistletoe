package app

import (
	conf "mistletoe/internal/config"
)

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// branchExistsLocallyOrRemotely checks if a branch exists locally or remotely.
func branchExistsLocallyOrRemotely(gitPath, dir, branch string, verbose bool) (bool, error) {
	// Check local
	// show-ref returns exit code 1 if not found, which RunGit returns as error.
	_, err := RunGit(dir, gitPath, verbose, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}

	// Check remote
	out, err := RunGit(dir, gitPath, verbose, "ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, err
	}
	if len(out) > 0 {
		return true, nil
	}
	return false, nil
}

// isDirEmpty checks if a directory is empty.
func isDirEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == nil {
		return false, nil // Not empty
	}
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// validateEnvironment checks if the current directory state is consistent with the configuration.
func validateEnvironment(repos []conf.Repository, baseDir, gitPath string, verbose bool) error {
	for _, repo := range repos {
		targetDir := filepath.Join(baseDir, conf.GetRepoDirName(repo))
		info, err := os.Stat(targetDir)
		if os.IsNotExist(err) {
			continue // Directory doesn't exist, safe to clone
		}
		if err != nil {
			return fmt.Errorf("error checking directory %s: %v", targetDir, err)
		}

		if !info.IsDir() {
			return fmt.Errorf("target %s exists and is not a directory", targetDir)
		}

		// Check eligibility
		isEligible := false
		var eligibilityErr error

		gitDir := filepath.Join(targetDir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			// Is Git Repo. Check details.
			// 1. URL
			currentURL, err := RunGit(targetDir, gitPath, verbose, "config", "--get", "remote.origin.url")
			if err != nil {
				eligibilityErr = fmt.Errorf("failed to get remote origin for %s: %v", targetDir, err)
			} else if currentURL != *repo.URL {
				eligibilityErr = fmt.Errorf("directory %s remote origin mismatch: expected %s, got %s", targetDir, *repo.URL, currentURL)
			} else {
				// URL Matches. Check Branches/Revision.
				checksPassed := true

				// Branch
				if repo.Branch != nil && *repo.Branch != "" {
					exists, err := branchExistsLocallyOrRemotely(gitPath, targetDir, *repo.Branch, verbose)
					if err != nil {
						eligibilityErr = fmt.Errorf("failed to check branch %s in %s: %v", *repo.Branch, targetDir, err)
						checksPassed = false
					} else if !exists {
						eligibilityErr = fmt.Errorf("directory %s missing required branch: %s", targetDir, *repo.Branch)
						checksPassed = false
					}
				}

				// BaseBranch
				if checksPassed && repo.BaseBranch != nil && *repo.BaseBranch != "" {
					exists, err := branchExistsLocallyOrRemotely(gitPath, targetDir, *repo.BaseBranch, verbose)
					if err != nil {
						eligibilityErr = fmt.Errorf("failed to check base-branch %s in %s: %v", *repo.BaseBranch, targetDir, err)
						checksPassed = false
					} else if !exists {
						eligibilityErr = fmt.Errorf("directory %s missing required base-branch: %s", targetDir, *repo.BaseBranch)
						checksPassed = false
					}
				}

				// Revision
				if checksPassed && repo.Revision != nil && *repo.Revision != "" {
					// Check local existence of revision
					_, err := RunGit(targetDir, gitPath, verbose, "cat-file", "-e", *repo.Revision+"^{commit}")
					if err != nil {
						eligibilityErr = fmt.Errorf("directory %s missing required revision: %s", targetDir, *repo.Revision)
						checksPassed = false
					}
				}

				if checksPassed {
					isEligible = true
				}
			}
		} else {
			eligibilityErr = fmt.Errorf("directory %s exists and is not a git repository", targetDir)
		}

		if isEligible {
			continue // Eligible -> Proceed to Normal Init
		}

		// Ineligible. Check if empty.
		empty, err := isDirEmpty(targetDir)
		if err != nil {
			return fmt.Errorf("failed to check emptiness of %s: %v", targetDir, err)
		}

		if empty {
			continue // Empty -> Proceed to Init (Clone)
		}

		// Not empty and Ineligible -> Error
		return fmt.Errorf("directory %s exists, is not empty, and is ineligible for init: %v", targetDir, eligibilityErr)
	}
	return nil
}

// PerformInit executes the initialization (clone/checkout) logic for the given repositories.
func PerformInit(repos []conf.Repository, baseDir, gitPath string, jobs, depth int, verbose bool) error {
	if err := validateEnvironment(repos, baseDir, gitPath, verbose); err != nil {
		return fmt.Errorf("error validating environment: %w", err)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)

	for _, repo := range repos {
		wg.Add(1)
		go func(repo conf.Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 1. Git Clone
			gitArgs := []string{"clone"}
			if depth > 0 {
				gitArgs = append(gitArgs, "--depth", fmt.Sprintf("%d", depth))
			}
			gitArgs = append(gitArgs, *repo.URL)
			targetDir := filepath.Join(baseDir, conf.GetRepoDirName(repo))

			// Explicitly pass target directory to avoid ambiguity and to know where to checkout later.
			gitArgs = append(gitArgs, targetDir)

			// Check if directory already exists and is a git repo.
			shouldClone := true
			if info, err := os.Stat(targetDir); err == nil && info.IsDir() {
				gitDir := filepath.Join(targetDir, ".git")
				if _, err := os.Stat(gitDir); err == nil {
					fmt.Fprintf(Stdout, "Repository %s exists. Skipping clone.\n", targetDir)
					shouldClone = false
				}
			}

			if shouldClone {
				fmt.Fprintf(Stdout, "Cloning %s into %s...\n", *repo.URL, targetDir)
				if err := RunGitInteractive("", gitPath, verbose, gitArgs...); err != nil {
					fmt.Fprintf(Stderr, "Error cloning %s: %v\n", *repo.URL, err)
					// Skip checkout if clone failed
					return
				}
			}

			// 2. Switch Branch / Checkout Revision
			if repo.Revision != nil && *repo.Revision != "" {
				// Checkout revision
				fmt.Fprintf(Stdout, "Checking out revision %s in %s...\n", *repo.Revision, targetDir)
				if err := RunGitInteractive(targetDir, gitPath, verbose, "checkout", *repo.Revision); err != nil {
					fmt.Fprintf(Stderr, "Error checking out revision %s in %s: %v\n", *repo.Revision, targetDir, err)
					return
				}

				if repo.Branch != nil && *repo.Branch != "" {
					// Create branch (or reset if exists)
					fmt.Fprintf(Stdout, "Creating branch %s at revision %s in %s...\n", *repo.Branch, *repo.Revision, targetDir)
					// Use -B to force create/reset branch to the revision point.
					// This matches the intent of initializing the workspace to the specified state.
					if err := RunGitInteractive(targetDir, gitPath, verbose, "checkout", "-B", *repo.Branch); err != nil {
						fmt.Fprintf(Stderr, "Error creating/resetting branch %s in %s: %v\n", *repo.Branch, targetDir, err)
					}
				}
			} else if repo.Branch != nil && *repo.Branch != "" {
				// "チェックアウト後、各要素についてbranchで示されたブランチに切り替える。"
				fmt.Fprintf(Stdout, "Switching %s to branch %s...\n", targetDir, *repo.Branch)
				if err := RunGitInteractive(targetDir, gitPath, verbose, "checkout", *repo.Branch); err != nil {
					fmt.Fprintf(Stderr, "Error switching branch for %s: %v.\n", targetDir, err)
				}
			}
		}(repo)
	}
	wg.Wait()
	return nil
}

func validateAndPrepareInitDest(dest string) error {
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for %s: %w", dest, err)
	}

	info, err := os.Stat(absDest)
	if err == nil {
		// Exists
		if !info.IsDir() {
			return fmt.Errorf("specified path is a file: %s", dest)
		}
	} else if os.IsNotExist(err) {
		// Does not exist
		parent := filepath.Dir(absDest)
		pInfo, pErr := os.Stat(parent)
		if pErr != nil {
			if os.IsNotExist(pErr) {
				return fmt.Errorf("invalid path: parent directory %s does not exist", parent)
			}
			return fmt.Errorf("error checking parent directory %s: %w", parent, pErr)
		}
		if !pInfo.IsDir() {
			return fmt.Errorf("parent path %s is not a directory", parent)
		}

		// Create directory
		if err := os.Mkdir(absDest, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", absDest, err)
		}
	} else {
		// Other error
		return fmt.Errorf("error checking path %s: %w", dest, err)
	}

	// Change directory
	if err := os.Chdir(absDest); err != nil {
		return fmt.Errorf("failed to change directory to %s: %w", absDest, err)
	}
	return nil
}

func handleInit(args []string, opts GlobalOptions) {
	var (
		fShort, fLong    string
		destLong         string
		dependenciesLong string
		depth            int
		jVal, jValShort  int
		vLong, vShort    bool
		yes, yesShort    bool
	)

	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(Stderr)
	fs.StringVar(&fLong, "file", DefaultConfigFile, "configuration file")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "configuration file (shorthand)")
	fs.StringVar(&destLong, "dest", "", "destination directory")
	fs.StringVar(&dependenciesLong, "dependencies", "", "Path to dependency graph file")
	fs.IntVar(&depth, "depth", 0, "Create a shallow clone with a history truncated to the specified number of commits")
	fs.IntVar(&jVal, "jobs", -1, "number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "number of concurrent jobs (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")
	fs.BoolVar(&yes, "yes", false, "Automatically answer 'yes' to all prompts")
	fs.BoolVar(&yesShort, "y", false, "Automatically answer 'yes' to all prompts (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Fprintln(Stderr, "Error parsing flags:", err)
		osExit(1)
		return
	}

	if err := CheckFlagDuplicates(fs, [][2]string{
		{"file", "f"},
		{"jobs", "j"},
		{"verbose", "v"},
		{"yes", "y"},
	}); err != nil {
		fmt.Fprintln(Stderr, "Error:", err)
		osExit(1)
		return
	}

	configFile, jobsFlag, configData, err := ResolveCommonValues(fLong, fShort, jVal, jValShort, ignoreStdin)
	if err != nil {
		fmt.Fprintf(Stderr, "Error: %v\n", err)
		osExit(1)
		return
	}

	configFile, err = SearchParentConfig(configFile, configData, opts.GitPath)
	if err != nil {
		fmt.Fprintf(Stderr, "Error searching parent config: %v\n", err)
	}

	// Resolve absolute path for config file before any directory change
	if configFile != "" {
		absPath, err := filepath.Abs(configFile)
		if err == nil {
			configFile = absPath
		}
	}

	var config *conf.Config
	if configFile != "" {
		config, err = conf.LoadConfigFile(configFile)
	} else {
		config, err = conf.LoadConfigData(configData)
	}

	if err != nil {
		fmt.Fprintln(Stderr, err)
		osExit(1)
		return
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

	dest := "."
	if destLong != "" {
		dest = destLong
	}

	if err := validateAndPrepareInitDest(dest); err != nil {
		fmt.Fprintln(Stderr, "Error:", err)
		osExit(1)
		return
	}

	// Validate dependency file if provided
	var depContent []byte
	if dependenciesLong != "" {
		// Collect all valid IDs (including private ones) for initial validation
		var allIDs []string
		for _, repo := range *config.Repositories {
			// ID is guaranteed to be set by validateRepositories in LoadConfig*
			if repo.ID != nil {
				allIDs = append(allIDs, *repo.ID)
			}
		}

		// Read and validate
		// Since ParseDependencies takes string, we read file first.
		rawContent, err := os.ReadFile(dependenciesLong)
		if err != nil {
			fmt.Fprintf(Stderr, "Error reading dependency file: %v\n", err)
			osExit(1)
			return
		}
		depContent = rawContent

		if _, err := ParseDependencies(string(depContent), allIDs); err != nil {
			fmt.Fprintf(Stderr, "Error validating dependency graph: %v\n", err)
			osExit(1)
			return
		}
	}

	if err := PerformInit(*config.Repositories, "", opts.GitPath, jobs, depth, verbose); err != nil {
		fmt.Fprintln(Stderr, err)
		osExit(1)
		return
	}

	// Post-init: Create .mstl directory and save config/dependencies
	// "Reading source file is not performed. We write what is loaded in memory."
	// Also filter out private repositories.

	// Check if we should skip overwriting config/dependencies
	// "If .mstl/config.json was read (explicitly specified or used as default), do not overwrite"
	shouldSkipWrite := false
	if configFile != "" {
		suffix := filepath.FromSlash(DefaultConfigFile)
		if strings.HasSuffix(configFile, suffix) {
			shouldSkipWrite = true
		}
	}

	if !shouldSkipWrite {
		mstlDir := ".mstl"
		if err := os.MkdirAll(mstlDir, 0755); err != nil {
			fmt.Fprintf(Stderr, "Warning: Failed to create .mstl directory: %v\n", err)
			return
		}

		// Filter config
		var filteredRepos []conf.Repository
		for _, repo := range *config.Repositories {
			if repo.Private != nil && *repo.Private {
				continue
			}
			filteredRepos = append(filteredRepos, repo)
		}
		filteredConfig := *config
		filteredConfig.Repositories = &filteredRepos

		// Marshal filtered config
		configBytes, err := json.MarshalIndent(filteredConfig, "", "  ")
		if err != nil {
			fmt.Fprintf(Stderr, "Warning: Failed to marshal configuration: %v\n", err)
		} else {
			configPath := filepath.Join(mstlDir, "config.json")
			if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
				fmt.Fprintf(Stderr, "Warning: Failed to write %s: %v\n", configPath, err)
			} else {
				fmt.Fprintf(Stdout, "Configuration saved to %s\n", configPath)
			}
		}

		// Save dependency-graph.md
		depPath := filepath.Join(mstlDir, "dependency-graph.md")
		var graphContent string

		if dependenciesLong != "" {
			// Use the provided content as is (already validated)
			graphContent = string(depContent)
		} else {
			// Generate default graph (Mermaid graph with nodes only)
			graphContent = "```mermaid\ngraph TD\n"
			for _, repo := range filteredRepos {
				name := getRepoName(repo)
				graphContent += fmt.Sprintf("    %s\n", name)
			}
			graphContent += "```\n"
		}

		if err := os.WriteFile(depPath, []byte(graphContent), 0644); err != nil {
			fmt.Fprintf(Stderr, "Warning: Failed to write %s: %v\n", depPath, err)
		} else {
			fmt.Fprintf(Stdout, "Dependencies graph saved to %s\n", depPath)
		}
	}
}
