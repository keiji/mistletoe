package app

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// branchExistsLocallyOrRemotely checks if a branch exists locally or remotely.
func branchExistsLocallyOrRemotely(gitPath, dir, branch string) (bool, error) {
	// Check local
	// show-ref returns exit code 1 if not found, which RunGit returns as error.
	_, err := RunGit(dir, gitPath, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}

	// Check remote
	out, err := RunGit(dir, gitPath, "ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, err
	}
	if len(out) > 0 {
		return true, nil
	}
	return false, nil
}

// validateEnvironment checks if the current directory state is consistent with the configuration.
func validateEnvironment(repos []Repository, gitPath string) error {
	for _, repo := range repos {
		targetDir := GetRepoDir(repo)
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

		// Check if it is a git repo
		gitDir := filepath.Join(targetDir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			// It's a git repo. Check remote.
			currentURL, err := RunGit(targetDir, gitPath, "config", "--get", "remote.origin.url")
			if err != nil {
				// Failed to get remote origin (maybe none configured).
				return fmt.Errorf("directory %s is a git repo but failed to get remote origin: %v", targetDir, err)
			}

			if currentURL != *repo.URL {
				return fmt.Errorf("directory %s exists with different remote origin: %s (expected %s)", targetDir, currentURL, *repo.URL)
			}

			// If Revision is specified and Branch is specified, check if branch already exists.
			if repo.Revision != nil && *repo.Revision != "" && repo.Branch != nil && *repo.Branch != "" {
				exists, err := branchExistsLocallyOrRemotely(gitPath, targetDir, *repo.Branch)
				if err != nil {
					return fmt.Errorf("failed to check branch existence for %s: %v", targetDir, err)
				}
				if exists {
					return fmt.Errorf("branch %s already exists in %s (locally or remotely), skipping init", *repo.Branch, targetDir)
				}
			}
			// Match -> OK.
		} else {
			// Not a git repo. Check if empty.
			err := func() error {
				f, err := os.Open(targetDir)
				if err != nil {
					return fmt.Errorf("failed to open directory %s: %v", targetDir, err)
				}
				defer f.Close()

				_, err = f.Readdirnames(1)
				if err == nil {
					// No error means we found at least one file/dir, so it's not empty.
					return fmt.Errorf("directory %s exists, is not a git repo, and is not empty", targetDir)
				}
				// io.EOF is expected if empty.
				return nil
			}()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// PerformInit executes the initialization (clone/checkout) logic for the given repositories.
func PerformInit(repos []Repository, gitPath string, parallel, depth int) error {
	if err := validateEnvironment(repos, gitPath); err != nil {
		return fmt.Errorf("error validating environment: %w", err)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)

	for _, repo := range repos {
		wg.Add(1)
		go func(repo Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 1. Git Clone
			gitArgs := []string{"clone"}
			if depth > 0 {
				gitArgs = append(gitArgs, "--depth", fmt.Sprintf("%d", depth))
			}
			gitArgs = append(gitArgs, *repo.URL)
			targetDir := GetRepoDir(repo)

			// Explicitly pass target directory to avoid ambiguity and to know where to checkout later.
			gitArgs = append(gitArgs, targetDir)

			// Check if directory already exists and is a git repo.
			shouldClone := true
			if info, err := os.Stat(targetDir); err == nil && info.IsDir() {
				gitDir := filepath.Join(targetDir, ".git")
				if _, err := os.Stat(gitDir); err == nil {
					fmt.Printf("Repository %s exists. Skipping clone.\n", targetDir)
					shouldClone = false
				}
			}

			if shouldClone {
				fmt.Printf("Cloning %s into %s...\n", *repo.URL, targetDir)
				if err := RunGitInteractive("", gitPath, gitArgs...); err != nil {
					fmt.Printf("Error cloning %s: %v\n", *repo.URL, err)
					// Skip checkout if clone failed
					return
				}
			}

			// 2. Switch Branch / Checkout Revision
			if repo.Revision != nil && *repo.Revision != "" {
				// Checkout revision
				fmt.Printf("Checking out revision %s in %s...\n", *repo.Revision, targetDir)
				if err := RunGitInteractive(targetDir, gitPath, "checkout", *repo.Revision); err != nil {
					fmt.Printf("Error checking out revision %s in %s: %v\n", *repo.Revision, targetDir, err)
					return
				}

				if repo.Branch != nil && *repo.Branch != "" {
					// Create branch
					fmt.Printf("Creating branch %s at revision %s in %s...\n", *repo.Branch, *repo.Revision, targetDir)
					if err := RunGitInteractive(targetDir, gitPath, "checkout", "-b", *repo.Branch); err != nil {
						fmt.Printf("Error creating branch %s in %s: %v\n", *repo.Branch, targetDir, err)
					}
				}
			} else if repo.Branch != nil && *repo.Branch != "" {
				// "チェックアウト後、各要素についてbranchで示されたブランチに切り替える。"
				fmt.Printf("Switching %s to branch %s...\n", targetDir, *repo.Branch)
				if err := RunGitInteractive(targetDir, gitPath, "checkout", *repo.Branch); err != nil {
					fmt.Printf("Error switching branch for %s: %v.\n", targetDir, err)
				}
			}
		}(repo)
	}
	wg.Wait()
	return nil
}

func handleInit(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var depth int
	var pVal, pValShort int

	fs := flag.NewFlagSet("init", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")
	fs.IntVar(&depth, "depth", 0, "Create a shallow clone with a history truncated to the specified number of commits")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "number of parallel processes (short)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	configFile, parallel, configData, err := ResolveCommonValues(fLong, fShort, pVal, pValShort)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	var config *Config
	if configFile != "" {
		config, err = loadConfigFile(configFile)
	} else {
		config, err = loadConfigData(configData)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := PerformInit(*config.Repositories, opts.GitPath, parallel, depth); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
