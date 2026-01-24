package app

import (
	"bufio"
	conf "mistletoe/internal/config"
	"mistletoe/internal/sys"
	"mistletoe/internal/ui"
)

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
)

func branchExists(dir, branch, gitPath string, verbose bool) bool {
	_, err := RunGit(dir, gitPath, verbose, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

func configureUpstreamIfSafe(dir, branch, gitPath string, verbose bool) {
	// 1. Fetch
	_, err := RunGit(dir, gitPath, verbose, "fetch", "origin", branch)
	if err != nil {
		return // Remote likely doesn't exist or fetch failed
	}

	// 2. Check remote existence
	_, err = RunGit(dir, gitPath, verbose, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	if err != nil {
		return // Remote branch doesn't exist
	}

	// 3. Check for conflict
	// Get SHAs
	localHead, err := RunGit(dir, gitPath, verbose, "rev-parse", "HEAD")
	if err != nil {
		return
	}
	remoteHead, err := RunGit(dir, gitPath, verbose, "rev-parse", "refs/remotes/origin/"+branch)
	if err != nil {
		return
	}

	// If same, safe.
	if localHead == remoteHead {
		_ = RunGitInteractive(dir, gitPath, verbose, "branch", "--set-upstream-to=origin/"+branch, branch)
		return
	}

	// Merge base
	base, err := RunGit(dir, gitPath, verbose, "merge-base", localHead, remoteHead)
	if err != nil || base == "" {
		return // Unrelated histories?
	}

	// Merge tree to check conflict
	out, err := RunGit(dir, gitPath, verbose, "merge-tree", base, localHead, remoteHead)
	if err == nil && !strings.Contains(out, "<<<<<<<") {
		// Safe!
		_ = RunGitInteractive(dir, gitPath, verbose, "branch", "--set-upstream-to=origin/"+branch, branch)
	}
}

func handleSwitch(args []string, opts GlobalOptions) error {
	var (
		fShort, fLong           string
		createShort, createLong string
		jVal, jValShort         int
		vLong, vShort           bool
		yes, yesShort           bool
	)

	fs := flag.NewFlagSet("switch", flag.ContinueOnError)
	fs.SetOutput(sys.Stderr)
	fs.StringVar(&fLong, "file", DefaultConfigFile, "configuration file")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "configuration file (shorthand)")
	fs.StringVar(&createLong, "create", "", "create branch if it does not exist")
	fs.StringVar(&createShort, "c", "", "create branch if it does not exist (shorthand)")
	fs.IntVar(&jVal, "jobs", -1, "number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "number of concurrent jobs (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")
	fs.BoolVar(&yes, "yes", false, "Automatically answer 'yes' to all prompts")
	fs.BoolVar(&yesShort, "y", false, "Automatically answer 'yes' to all prompts (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		return fmt.Errorf("Error parsing flags: %w", err)
	}

	if err := CheckFlagDuplicates(fs, [][2]string{
		{"file", "f"},
		{"create", "c"},
		{"jobs", "j"},
		{"verbose", "v"},
		{"yes", "y"},
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

	createBranchName := createLong
	if createShort != "" {
		createBranchName = createShort
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

	var branchName string
	var create bool

	if createBranchName != "" {
		if len(fs.Args()) > 0 {
			return fmt.Errorf("Error: Unexpected argument: %s.", fs.Args()[0])
		}
		branchName = createBranchName
		create = true
	} else {
		// If create flag not set, look for positional argument
		if len(fs.Args()) == 0 {
			return fmt.Errorf("Error: Branch name required.")
		} else if len(fs.Args()) > 1 {
			return fmt.Errorf("Error: Too many arguments: %v.", fs.Args())
		}
		branchName = fs.Args()[0]
		create = false
	}

	// Validate Integrity (Moved after argument parsing)
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath, verbose); err != nil {
		return err
	}

	// Map to store existence status for each repo (keyed by local directory path)
	dirExists := make(map[string]bool)
	// Map to store current branch for each repo (keyed by repo ID)
	currentBranches := make(map[string]string)
	var mu sync.Mutex

	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
	var threadErr error
	var threadErrMu sync.Mutex

	// Pre-check phase
	for _, repo := range *config.Repositories {
		wg.Add(1)
		go func(repo conf.Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			dir := config.GetRepoPath(repo)

			// Check if directory exists
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				threadErrMu.Lock()
				if threadErr == nil {
					threadErr = fmt.Errorf("Error: Repository directory %s does not exist.", dir)
				}
				threadErrMu.Unlock()
				return
			}

			// Get current branch
			curr, err := RunGit(dir, opts.GitPath, verbose, "rev-parse", "--abbrev-ref", "HEAD")
			if err == nil {
				mu.Lock()
				currentBranches[*repo.ID] = curr
				mu.Unlock()
			}

			exists := branchExists(dir, branchName, opts.GitPath, verbose)
			if !exists {
				// If not found locally, check if it exists on remote (fetch first)
				// This allows switching to a branch that only exists on remote (git checkout <branch> will track it)
				_, err := RunGit(dir, opts.GitPath, verbose, "fetch", "origin", branchName)
				if err == nil {
					// Check if remote branch exists after fetch
					_, err = RunGit(dir, opts.GitPath, verbose, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branchName)
					if err == nil {
						exists = true
					}
				}
			}
			mu.Lock()
			dirExists[dir] = exists
			mu.Unlock()
		}(repo)
	}
	wg.Wait()

	if threadErr != nil {
		return threadErr
	}

	if create {
		// Consistency Check
		var firstBranch string
		allMatch := true
		first := true

		for _, repo := range *config.Repositories {
			mu.Lock()
			branch := currentBranches[*repo.ID]
			mu.Unlock()
			if first {
				firstBranch = branch
				first = false
			} else {
				if branch != firstBranch {
					allMatch = false
					break
				}
			}
		}

		if !allMatch {
			fmt.Fprintln(sys.Stderr, "Branch names do not match. Current status:")
			// Output current status table
			rows := CollectStatus(config, jobs, opts.GitPath, verbose, false)
			RenderStatusTable(sys.Stdout, rows)

			msg := "Do you want to continue? (yes/no): "
			reader := bufio.NewReader(sys.Stdin)
			proceed, err := ui.AskForConfirmationRequired(reader, msg, yes)
			if err != nil {
				return fmt.Errorf("Error reading input: %w", err)
			}
			if !proceed {
				return fmt.Errorf("Aborted by user.")
			}
		}
	}

	if !create {
		// Strict mode: All must exist
		var missing []string
		for _, repo := range *config.Repositories {
			dir := config.GetRepoPath(repo)
			if !dirExists[dir] {
				missing = append(missing, *repo.URL+" ("+dir+")")
			}
		}

		if len(missing) > 0 {
			msg := fmt.Sprintf("Error: Branch '%s' missing in repositories:\n", branchName)
			for _, item := range missing {
				msg += " - " + item + "\n"
			}
			return fmt.Errorf("%s", msg)
		}

		// Execute Checkout
		for _, repo := range *config.Repositories {
			wg.Add(1)
			go func(repo conf.Repository) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				dir := config.GetRepoPath(repo)
				repoID := *repo.ID
				fmt.Fprintf(sys.Stdout, "[%s] Switching to branch %s...\n", repoID, branchName)
				if err := RunGitInteractive(dir, opts.GitPath, verbose, "checkout", branchName); err != nil {
					threadErrMu.Lock()
					if threadErr == nil {
						threadErr = fmt.Errorf("Error switching branch for %s: %w.", dir, err)
					}
					threadErrMu.Unlock()
					return
				}
				configureUpstreamIfSafe(dir, branchName, opts.GitPath, verbose)
			}(repo)
		}
		wg.Wait()
	} else {
		// Create mode
		for _, repo := range *config.Repositories {
			wg.Add(1)
			go func(repo conf.Repository) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				dir := config.GetRepoPath(repo)
				repoID := *repo.ID
				mu.Lock()
				exists := dirExists[dir]
				mu.Unlock()

				if exists {
					fmt.Fprintf(sys.Stdout, "[%s] Branch %s exists. Switching...\n", repoID, branchName)
					if err := RunGitInteractive(dir, opts.GitPath, verbose, "checkout", branchName); err != nil {
						threadErrMu.Lock()
						if threadErr == nil {
							threadErr = fmt.Errorf("Error switching branch for %s: %w.", dir, err)
						}
						threadErrMu.Unlock()
						return
					}
				} else {
					fmt.Fprintf(sys.Stdout, "[%s] Creating and switching to branch %s...\n", repoID, branchName)
					if err := RunGitInteractive(dir, opts.GitPath, verbose, "checkout", "-b", branchName); err != nil {
						threadErrMu.Lock()
						if threadErr == nil {
							threadErr = fmt.Errorf("Error creating branch for %s: %w.", dir, err)
						}
						threadErrMu.Unlock()
						return
					}
				}
				configureUpstreamIfSafe(dir, branchName, opts.GitPath, verbose)
			}(repo)
		}
		wg.Wait()
	}
	if threadErr != nil {
		return threadErr
	}
	return nil
}
