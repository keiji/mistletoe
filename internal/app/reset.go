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
	"sort"
	"strings"
	"sync"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
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

// ResetInfo holds information for display in the summary table
type ResetInfo struct {
	RepoName      string
	LocalBranch   string
	ResolvedTarget string
}

func handleReset(args []string, opts GlobalOptions) error {
	var (
		fShort, fLong string
		jVal, jValShort int
		vLong, vShort bool
		yes, yesShort bool
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
	fs.BoolVar(&yes, "yes", false, "Automatically answer 'yes' to all prompts")
	fs.BoolVar(&yesShort, "y", false, "Automatically answer 'yes' to all prompts (shorthand)")

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
		{"yes", "y"},
	}); err != nil {
		return fmt.Errorf("Error: %w", err)
	}

	configFile, jobsFlag, configData, err := ResolveCommonValues(fLong, fShort, jVal, jValShort, ignoreStdin)
	if err != nil {
		return fmt.Errorf("Error: %w", err)
	}

	yesFlag := yes || yesShort

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

	// Map to store resolved targets and info
	var resetInfos []ResetInfo
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

			// Get current HEAD
			currentHead, errHead := RunGit(dir, opts.GitPath, verbose, "rev-parse", "HEAD")
			localBranch := "HEAD (detached)"

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

				// Try to get branch name
				branchName, errBranch := RunGit(dir, opts.GitPath, verbose, "rev-parse", "--abbrev-ref", "HEAD")
				if errBranch == nil && branchName != "HEAD" {
					localBranch = strings.TrimSpace(branchName)
				}
			}

			mu.Lock()
			resetInfos = append(resetInfos, ResetInfo{
				RepoName:       repoID,
				LocalBranch:    localBranch,
				ResolvedTarget: finalTarget,
			})
			mu.Unlock()
		}(repo)
	}
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}

	// Sort infos for stable display
	sort.Slice(resetInfos, func(i, j int) bool {
		return resetInfos[i].RepoName < resetInfos[j].RepoName
	})

	// Phase 2: Confirmation
	if !yesFlag {
		// Render Table
		table := tablewriter.NewTable(sys.Stdout,
			tablewriter.WithHeaderAutoFormat(tw.Off),
			tablewriter.WithRowAutoWrap(tw.WrapNone),
			tablewriter.WithAlignment(tw.MakeAlign(5, tw.AlignLeft)),
			tablewriter.WithRendition(tw.Rendition{
				Borders: tw.Border{Left: tw.On, Top: tw.Off, Right: tw.On, Bottom: tw.Off},
				Settings: tw.Settings{
					Separators: tw.Separators{BetweenColumns: tw.On, BetweenRows: tw.Off},
				},
				Symbols: tw.NewSymbolCustom("v0.0.5-like").
					WithColumn("|").
					WithRow("-").
					WithCenter("|").
					WithHeaderMid("-").
					WithTopMid("-").
					WithBottomMid("-"),
			}),
		)
		table.Header("Repository", "Local Branch", "Target Branch/Revision")
		for _, info := range resetInfos {
			table.Append(info.RepoName, info.LocalBranch, info.ResolvedTarget)
		}
		table.Render()

		// Prompt
		promptMsg := "Reset these repositories? The working directory changes will NOT be lost. (mixed reset) [yes/no]: "
		reader := bufio.NewReader(sys.Stdin)
		confirmed, err := ui.AskForConfirmationRequired(reader, promptMsg, false)
		if err != nil {
			return fmt.Errorf("Error reading input: %w", err)
		}
		if !confirmed {
			fmt.Fprintln(sys.Stdout, "Aborted.")
			return nil
		}
	} else {
		fmt.Fprintln(sys.Stdout, "Skipping confirmation due to --yes flag.")
	}

	// Phase 3: Execution
	// We use resetInfos which contains resolved targets
	// We need to re-fetch dir path. Ideally we should have stored it in ResetInfo or use a map.
	// But resetting order should probably match table (sorted).
	// Let's create a map for easy lookup or iterate config again (O(N) is cheap).
	// Actually, just iterating config is easier if we made a map of resolved targets.
	// Let's rebuild the map from resetInfos.
	targetMap := make(map[string]string)
	for _, info := range resetInfos {
		targetMap[info.RepoName] = info.ResolvedTarget
	}

	// Note: We iterate over config.Repositories to ensure we cover everything,
	// though resetInfos should have same count if no error.
	// But we also sorted resetInfos. Execution order doesn't strictly matter but sequential is safer.
	// Let's use the sorted order from resetInfos for execution log consistency.

	for _, info := range resetInfos {
		repoID := info.RepoName
		target := info.ResolvedTarget

		// Find repo in config to get path (a bit inefficient but N is small)
		var dir string
		for _, r := range *config.Repositories {
			if *r.ID == repoID {
				dir = config.GetRepoPath(r)
				break
			}
		}

		fmt.Fprintf(sys.Stdout, "[%s] Resetting to %s...\n", repoID, target)

		// Use mixed reset (default) to keep changes in working directory
		if err := RunGitInteractive(dir, opts.GitPath, verbose, "reset", target); err != nil {
			return fmt.Errorf("Error resetting %s: %w", repoID, err)
		}
	}

	return nil
}
