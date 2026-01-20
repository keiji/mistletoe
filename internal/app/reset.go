package app

import (
	conf "mistletoe/internal/config"
	"mistletoe/internal/sys"
)

import (
	"flag"
	"fmt"
	"sync"
)

// resolveResetTarget determines the target for reset based on priority:
// 1. Revision
// 2. BaseBranch
// 3. Branch
func resolveResetTarget(repo conf.Repository) (string, error) {
	if repo.Revision != nil && *repo.Revision != "" {
		return *repo.Revision, nil
	}
	if repo.BaseBranch != nil && *repo.BaseBranch != "" {
		return *repo.BaseBranch, nil
	}
	if repo.Branch != nil && *repo.Branch != "" {
		return *repo.Branch, nil
	}
	return "", fmt.Errorf("No target (revision, base-branch, or branch) specified for repository %s", *repo.ID)
}


// verifyResetTargetWithResolution checks and resolves target.
func verifyResetTargetWithResolution(dir string, target string, gitPath string, verbose bool) (string, error) {
	// 1. Check direct resolution (local branch, tag, SHA)
	_, err := RunGit(dir, gitPath, verbose, "rev-parse", "--verify", target)
	if err == nil {
		return target, nil
	}

	// 2. Fetch
	_, errFetch := RunGit(dir, gitPath, verbose, "fetch", "origin", target)
	if errFetch != nil {
		// Fallback to general fetch
		_, _ = RunGit(dir, gitPath, verbose, "fetch", "origin")
	}

	// 3. Check direct resolution again
	_, err = RunGit(dir, gitPath, verbose, "rev-parse", "--verify", target)
	if err == nil {
		return target, nil
	}

	// 4. Check remote branch resolution (origin/target)
	originTarget := "origin/" + target
	_, err = RunGit(dir, gitPath, verbose, "rev-parse", "--verify", originTarget)
	if err == nil {
		return originTarget, nil
	}

	return "", fmt.Errorf("Target '%s' (or '%s') not found.", target, originTarget)
}

func handleReset(args []string, opts GlobalOptions) error {
	var (
		fShort, fLong string
		jVal, jValShort int
		vLong, vShort bool
	)

	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	fs.SetOutput(sys.Stderr)
	fs.StringVar(&fLong, "file", DefaultConfigFile, "Configuration file path")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "Configuration file path (shorthand)")
	fs.IntVar(&jVal, "jobs", -1, "number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "number of concurrent jobs (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		return fmt.Errorf("Error parsing flags: %w", err)
	}

	if len(fs.Args()) > 0 {
		return fmt.Errorf("Error: reset command does not accept positional arguments")
	}

	if err := CheckFlagDuplicates(fs, [][2]string{
		{"file", "f"},
		{"jobs", "j"},
		{"verbose", "v"},
	}); err != nil {
		return fmt.Errorf("Error: %w", err)
	}

	configFile, jobsFlag, configData, err := ResolveCommonValues(fLong, fShort, jVal, jValShort, ignoreStdin)
	if err != nil {
		return fmt.Errorf("Error: %w", err)
	}

	configFile, err = SearchParentConfig(configFile, configData, opts.GitPath)
	if err != nil {
		fmt.Fprintf(sys.Stderr, "Error searching parent config: %v\n", err)
	}

	var config *conf.Config
	if configFile != "" {
		config, err = conf.LoadConfigFile(configFile)
	} else {
		config, err = conf.LoadConfigData(configData)
	}

	if err != nil {
		return err
	}

	// Resolve Jobs
	jobs, err := DetermineJobs(jobsFlag, config)
	if err != nil {
		return fmt.Errorf("Error: %w", err)
	}

	// Verbose Override
	verbose := vLong || vShort
	if verbose && jobs > 1 {
		fmt.Fprintln(sys.Stdout, "Verbose is specified, so jobs is treated as 1.")
		jobs = 1
	}

	// Validate Integrity
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath, verbose); err != nil {
		return err
	}

	// Map to store resolved targets
	resolvedTargets := make(map[string]string) // Key: Repo ID, Value: Resolved Target
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
	var firstErr error
	var errMu sync.Mutex

	// Phase 1: Verification
	for _, repo := range *config.Repositories {
		wg.Add(1)
		go func(repo conf.Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			dir := config.GetRepoPath(repo)
			repoID := *repo.ID

			target, err := resolveResetTarget(repo)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("[%s] %w", repoID, err)
				}
				errMu.Unlock()
				return
			}

			// Verify and Resolve (fetch if needed)
			finalTarget, err := verifyResetTargetWithResolution(dir, target, opts.GitPath, verbose)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("[%s] %w", repoID, err)
				}
				errMu.Unlock()
				return
			}

			// Check unrelated histories / consistency?
			// git merge-base HEAD target
			// If it fails, they might be unrelated.
			// But wait, reset --hard simply moves HEAD. It doesn't care about ancestry strictly speaking,
			// unless we want to prevent switching to a completely different project.
			// The prompt says: "If history tree is completely different... error".
			// So we should check common ancestor.
			// Note: if current HEAD is detached or initial, merge-base might behave specific ways.

			// Get current HEAD
			currentHead, errHead := RunGit(dir, opts.GitPath, verbose, "rev-parse", "HEAD")
			if errHead == nil {
				// Only check if we have a current HEAD (empty repo might not)
				_, errBase := RunGit(dir, opts.GitPath, verbose, "merge-base", currentHead, finalTarget)
				if errBase != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("[%s] Incompatible history between HEAD and '%s'.", repoID, finalTarget)
					}
					errMu.Unlock()
					return
				}
			}

			mu.Lock()
			resolvedTargets[repoID] = finalTarget
			mu.Unlock()
		}(repo)
	}
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}

	// Phase 2: Execution
	for _, repo := range *config.Repositories {
		repoID := *repo.ID
		dir := config.GetRepoPath(repo)
		target := resolvedTargets[repoID]

		fmt.Fprintf(sys.Stdout, "[%s] Resetting to %s...\n", repoID, target)

		// Use mixed reset (default) to keep changes in working directory
		if err := RunGitInteractive(dir, opts.GitPath, verbose, "reset", target); err != nil {
			return fmt.Errorf("Error resetting %s: %w", repoID, err)
		}
	}

	return nil
}
