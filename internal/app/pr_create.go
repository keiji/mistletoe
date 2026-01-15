package app

import (
	conf "mistletoe/internal/config"
)

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// handlePrCreate handles 'pr create'.
func handlePrCreate(args []string, opts GlobalOptions) {
	fs := flag.NewFlagSet("pr create", flag.ExitOnError)
	var (
		fLong      string
		fShort     string
		jVal       int
		jValShort  int
		tLong      string
		tShort     string
		bLong      string
		bShort     string
		dLong      string
		wLong      bool
		wShort     bool
		draft      bool
		vLong      bool
		vShort     bool
		yes        bool
		yesShort   bool
	)

	fs.StringVar(&fLong, "file", DefaultConfigFile, "Configuration file path")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "Configuration file path (shorthand)")
	fs.IntVar(&jVal, "jobs", -1, "Number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "Number of concurrent jobs (shorthand)")
	fs.StringVar(&tLong, "title", "", "Pull Request title")
	fs.StringVar(&tShort, "t", "", "Pull Request title (shorthand)")
	fs.StringVar(&bLong, "body", "", "Pull Request body")
	fs.StringVar(&bShort, "b", "", "Pull Request body (shorthand)")
	fs.StringVar(&dLong, "dependencies", DefaultDependencies, "Dependency graph file path")
	fs.BoolVar(&draft, "draft", false, "Create Pull Request as Draft if supported")
	fs.BoolVar(&wLong, "overwrite", false, "Overwrite existing Pull Request description if creator matches or forced")
	fs.BoolVar(&wShort, "w", false, "Overwrite existing Pull Request description (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")
	fs.BoolVar(&yes, "yes", false, "Automatically answer 'yes' to all prompts")
	fs.BoolVar(&yesShort, "y", false, "Automatically answer 'yes' to all prompts (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := CheckFlagDuplicates(fs, [][2]string{
		{"file", "f"},
		{"jobs", "j"},
		{"title", "t"},
		{"body", "b"},
		{"overwrite", "w"},
		{"verbose", "v"},
		{"yes", "y"},
	}); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	// Resolve common values
	configPath, jobsFlag, configData, err := ResolveCommonValues(fLong, fShort, jVal, jValShort, ignoreStdin)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	yesFlag := yes || yesShort

	configPath, err = SearchParentConfig(configPath, configData, opts.GitPath, yesFlag)
	if err != nil {
		fmt.Fprintf(Stderr, "Error searching parent config: %v\n", err)
	}

	// Resolve title, body, dependency file
	prTitle := tLong
	if prTitle == "" {
		prTitle = tShort
	}
	prBody := bLong
	if prBody == "" {
		prBody = bShort
	}
	depPath := dLong
	overwrite := wLong || wShort

	// Verbose Override (Forward declaration)
	verbose := vLong || vShort

	// 1. Check gh availability
	if err := checkGhAvailability(opts.GhPath, verbose); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 2. Load conf.Config
	var config *conf.Config
	if configPath != "" {
		config, err = conf.LoadConfigFile(configPath)
	} else {
		config, err = conf.LoadConfigData(configData)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Resolve Jobs
	jobs, err := DetermineJobs(jobsFlag, config)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Verbose Override
	if verbose && jobs > 1 {
		fmt.Println("Verbose is specified, so jobs is treated as 1.")
		jobs = 1
	}

	// 3. Load Dependencies (if specified)
	deps, depContent, err := LoadDependencyGraph(depPath, config)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	if depContent != "" {
		fmt.Println("Dependency graph loaded successfully.")
	}

	// 4. Validate Integrity
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath, verbose); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 5. Collect Status & PR Status (Moved Up)
	fmt.Println("Collecting repository status and checking for existing Pull Requests...")
	spinner := NewSpinner(verbose)
	spinner.Start()
	// Pass noFetch=true to CollectStatus. We rely on subsequent checks.
	rows := CollectStatus(config, jobs, opts.GitPath, verbose, true)
	// Initial Check: No known PRs yet
	prRows := CollectPrStatus(rows, config, jobs, opts.GhPath, verbose, nil)
	spinner.Stop()
	RenderPrStatusTable(Stdout, prRows)

	// 6. Check for Behind/Conflict/Detached
	// Abort if pull required (behind)
	ValidateStatusForAction(rows, true)

	// 6.5 Categorize Repositories
	fmt.Println("Analyzing repository states...")

	var catPushCreate []conf.Repository   // Cat 1
	var catNoPushCreate []conf.Repository // Cat 2
	var catPushUpdate []conf.Repository   // Cat 3
	var catNoPushUpdate []conf.Repository // Cat 4
	var skippedRepos []string

	// Final functional lists
	var pushList []conf.Repository
	var createList []conf.Repository
	var updateList []conf.Repository

	repoMap := make(map[string]conf.Repository)
	for _, r := range *config.Repositories {
		repoMap[getRepoName(r)] = r
	}

	// Map to track PR existence
	prExistsMap := make(map[string][]PrInfo) // RepoName -> []PrInfo
	for _, prRow := range prRows {
		if len(prRow.PrItems) > 0 {
			prExistsMap[prRow.Repo] = prRow.PrItems
		}
	}

	// Helper map for StatusRow
	statusMap := make(map[string]StatusRow)
	for _, r := range rows {
		statusMap[r.Repo] = r
	}

	// Iterate Repositories
	for _, repo := range *config.Repositories {
		repoName := getRepoName(repo)
		status, hasStatus := statusMap[repoName]
		if !hasStatus {
			continue
		}

		hasPr := false
		if items, ok := prExistsMap[repoName]; ok {
			for _, item := range items {
				if strings.EqualFold(item.State, GitHubPrStateOpen) {
					hasPr = true
					break
				}
			}
		}

		if hasPr {
			if status.HasUnpushed {
				// Cat 3: Push + Update
				catPushUpdate = append(catPushUpdate, repo)
			} else {
				// Cat 4: No Push + Update
				catNoPushUpdate = append(catNoPushUpdate, repo)
			}
		} else {
			// No PR
			if status.HasUnpushed {
				// Special Case: If remote branch doesn't exist, check if local branch is identical to base branch.
				// If so, skip (branch created but no commits).
				skipNewBranch := false
				if status.RemoteRev == "" {
					baseBranch := "main"
					if repo.BaseBranch != nil && *repo.BaseBranch != "" {
						baseBranch = *repo.BaseBranch
					} else if repo.Branch != nil && *repo.Branch != "" {
						baseBranch = *repo.Branch
					}

					// We attempt to resolve origin/BaseBranch to see if it matches local HEAD.
					// Note: status.LocalHeadFull contains the full SHA of the local branch.
					baseRev, err := RunGit(status.RepoDir, opts.GitPath, verbose, "rev-parse", "origin/"+baseBranch)
					if err == nil {
						baseRev = strings.TrimSpace(baseRev)
						if baseRev == status.LocalHeadFull {
							skipNewBranch = true
						}
					}
				}

				if skipNewBranch {
					skippedRepos = append(skippedRepos, repoName)
				} else {
					// Cat 1: Push + Create
					catPushCreate = append(catPushCreate, repo)
				}
			} else {
				// !HasUnpushed && !HasPr
				// Cat 2: No Push + Create
				// We must check if we are on the base branch (e.g. main).
				// If local branch == base branch, we skip.
				baseBranch := "main"
				if repo.BaseBranch != nil && *repo.BaseBranch != "" {
					baseBranch = *repo.BaseBranch
				} else if repo.Branch != nil && *repo.Branch != "" {
					baseBranch = *repo.Branch
				}

				// If the current branch name matches the base branch, we skip creating a PR
				// because you don't typically create a PR from main to main.
				if status.BranchName == baseBranch {
					skippedRepos = append(skippedRepos, repoName)
				} else {
					catNoPushCreate = append(catNoPushCreate, repo)
				}
			}
		}
	}

	// Reconstruct functional lists
	pushList = append(pushList, catPushCreate...)
	pushList = append(pushList, catPushUpdate...)

	createList = append(createList, catPushCreate...)
	createList = append(createList, catNoPushCreate...)

	updateList = append(updateList, catPushUpdate...)
	updateList = append(updateList, catNoPushUpdate...)

	// Display Categories
	if len(catPushCreate) > 0 {
		fmt.Println("Repositories to Push and Create Pull Request:")
		for _, r := range catPushCreate {
			fmt.Printf(" - %s\n", getRepoName(r))
		}
		fmt.Println()
	}

	if len(catNoPushCreate) > 0 {
		fmt.Println("Repositories to Create Pull Request (No Push):")
		for _, r := range catNoPushCreate {
			fmt.Printf(" - %s\n", getRepoName(r))
		}
		fmt.Println()
	}

	if len(catPushUpdate) > 0 {
		fmt.Println("Repositories to Push (Pull Request already exists):")
		for _, r := range catPushUpdate {
			fmt.Printf(" - %s\n", getRepoName(r))
		}
		fmt.Println()
	}

	if len(catNoPushUpdate) > 0 {
		fmt.Println("Repositories with no action (Pull Request already exists):")
		for _, r := range catNoPushUpdate {
			fmt.Printf(" - %s\n", getRepoName(r))
		}
		fmt.Println()
	}

	if len(skippedRepos) > 0 {
		fmt.Println("The following repositories will be skipped:")
		for _, r := range skippedRepos {
			fmt.Printf(" - %s\n", r)
		}
		fmt.Println()
	}

	// Combine createList + updateList for "Active Repos" processing
	var activeRepos []conf.Repository
	activeRepos = append(activeRepos, updateList...)
	activeRepos = append(activeRepos, createList...)

	if len(activeRepos) == 0 {
		fmt.Println("No repositories to process.")
		return
	}

	// 7. Prompt
	var skipEditor bool

	// If ALL active repos are in updateList, prompt for description update only
	allUpdates := len(createList) == 0

	reader := bufio.NewReader(os.Stdin)

	if allUpdates {
		confirmed, err := AskForConfirmation(reader, "No new Pull Requests to create. Update existing Pull Request descriptions? (yes/no): ", yesFlag)
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			os.Exit(1)
		}
		if confirmed {
			skipEditor = true
		} else {
			fmt.Println("Aborted.")
			os.Exit(1)
		}
	} else {
		confirmed, err := AskForConfirmation(reader, "Proceed with Push and Pull Request creation? (yes/no): ", yesFlag)
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			os.Exit(1)
		}
		if confirmed {
			skipEditor = false
		} else {
			fmt.Println("Aborted.")
			os.Exit(1)
		}
	}

	// Input Message if needed
	if !skipEditor {
		if prTitle == "" && prBody == "" {
			content, err := RunEditor()
			if err != nil {
				fmt.Printf("error getting message: %v\n", err)
				os.Exit(1)
			}
			prTitle, prBody = ParsePrTitleBody(content)
		}
	}

	// 8. Check GitHub Management & Permissions & Base Branches (for active repos)
	fmt.Println("Verifying GitHub permissions and base branches...")

	// Convert prExistsMap (map[string][]PrInfo) to map[string][]string for verifyGithubRequirements
	prExistsMapURLs := make(map[string][]string)
	for k, items := range prExistsMap {
		var urls []string
		for _, item := range items {
			urls = append(urls, item.URL)
		}
		prExistsMapURLs[k] = urls
	}

	_, err = verifyGithubRequirements(activeRepos, config.BaseDir, rows, jobs, opts.GitPath, opts.GhPath, verbose, prExistsMapURLs)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 9. Execution Phase 1: Push
	// Final Verification: Ensure revisions haven't changed since status collection
	fmt.Println("Verifying repository states...")
	if err := VerifyRevisionsUnchanged(config, rows, opts.GitPath, verbose); err != nil {
		fmt.Printf("error: state verification failed: %v\n", err)
		os.Exit(1)
	}

	if len(pushList) > 0 {
		fmt.Println("Pushing changes...")
		if err := executePush(pushList, config.BaseDir, rows, jobs, opts.GitPath, verbose); err != nil {
			fmt.Printf("error during push: %v\n", err)
			os.Exit(1)
		}
	}

	// 9. Execution Phase 2: Create PRs
	// We need a map of ALL PR URLs (existing + newly created) for the snapshot/related-pr logic.
	finalPrMap := make(map[string][]PrInfo)
	for k, v := range prExistsMap {
		finalPrMap[k] = v
	}

	if len(createList) > 0 {
		fmt.Println("Creating Pull Requests...")
		// Create placeholder body
		placeholderBlock := GeneratePlaceholderMistletoeBody()
		prBodyWithPlaceholder := EmbedMistletoeBody(prBody, placeholderBlock)

		createdMap, err := executePrCreationOnly(createList, rows, jobs, opts.GhPath, verbose, prTitle, prBodyWithPlaceholder, draft)
		if err != nil {
			fmt.Printf("error during PR creation: %v\n", err)
			os.Exit(1)
		}
		for k, url := range createdMap {
			// Created PR is always OPEN
			finalPrMap[k] = append(finalPrMap[k], PrInfo{URL: url, State: "OPEN"})
		}
	}

	// 9. Execution Phase 3: Update Descriptions (All Active PRs)
	fmt.Println("Generating configuration snapshot...")
	snapshotData, snapshotID, err := GenerateSnapshotFromStatus(config, rows)
	if err != nil {
		fmt.Printf("error generating snapshot: %v\n", err)
		os.Exit(1)
	}

	filename := fmt.Sprintf("mistletoe-snapshot-%s.json", snapshotID)
	if err := os.WriteFile(filename, snapshotData, 0644); err != nil {
		fmt.Printf("error writing snapshot file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Snapshot saved to %s\n", filename)

	fmt.Println("Updating Pull Request descriptions...")
	// We pass finalPrMap (containing ALL PRs, including merged/closed) to ensure Related Links are complete.
	// updatePrDescriptions will internally filter which PRs to actually update (Open/Draft only).
	if err := updatePrDescriptions(finalPrMap, jobs, opts.GhPath, verbose, string(snapshotData), filename, deps, depContent, overwrite); err != nil {
		fmt.Printf("error updating descriptions: %v\n", err)
		os.Exit(1)
	}

	// 11. Show Status (Final)
	fmt.Println("Collecting final status...")
	spinner = NewSpinner(verbose)
	spinner.Start()
	finalRows := CollectStatus(config, jobs, opts.GitPath, verbose, true)

	// Updated to pass finalPrMap directly
	finalPrRows := CollectPrStatus(finalRows, config, jobs, opts.GhPath, verbose, finalPrMap)
	spinner.Stop()

	// Filter for Display (Open or Draft only)
	var displayRows []PrStatusRow
	for _, row := range finalPrRows {
		if !strings.EqualFold(row.PrState, GitHubPrStateOpen) {
			row.PrDisplay = "-"
		}
		displayRows = append(displayRows, row)
	}
	RenderPrStatusTable(Stdout, displayRows)

	fmt.Println("Done.")
}

// executePrCreationOnly creates PRs for the given repositories.
// Returns a map of RepoName -> PR URL.
func executePrCreationOnly(repos []conf.Repository, rows []StatusRow, jobs int, ghPath string, verbose bool, title, body string, draft bool) (map[string]string, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
	var errs []string
	prMap := make(map[string]string)

	statusMap := make(map[string]StatusRow)
	for _, r := range rows {
		statusMap[r.Repo] = r
	}

	for _, repo := range repos {
		wg.Add(1)
		go func(r conf.Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoName := getRepoName(r)
			branchName := ""
			if row, ok := statusMap[repoName]; ok && row.BranchName != "" {
				branchName = row.BranchName
			} else {
				return
			}

			fmt.Printf("[%s] Creating Pull Request...\n", repoName)

			args := []string{"pr", "create", "--repo", *r.URL, "--head", branchName}

			if title != "" || body != "" {
				if title != "" {
					args = append(args, "--title", title)
				}
				if body != "" {
					args = append(args, "--body", body)
				}
			} else {
				args = append(args, "--fill")
			}

			// Resolve Base Branch
			baseBranch := ""
			if r.BaseBranch != nil && *r.BaseBranch != "" {
				baseBranch = *r.BaseBranch
			} else if r.Branch != nil && *r.Branch != "" {
				baseBranch = *r.Branch
			}

			if baseBranch != "" {
				args = append(args, "--base", baseBranch)
			}

			// Try with Draft if requested
			attemptArgs := args
			if draft {
				attemptArgs = append(attemptArgs, "--draft")
			}

			createOut, err := RunGh(ghPath, verbose, attemptArgs...)
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					stderr := string(exitErr.Stderr)

					// Fallback Logic for Draft Not Supported
					if draft && (strings.Contains(stderr, "Draft pull requests are not supported") || strings.Contains(stderr, "Draft pull requests cannot be created")) {
						if verbose {
							fmt.Printf("[%s] Draft PR not supported. Retrying as normal PR...\n", repoName)
						}
						// Retry without --draft (which is essentially original 'args')
						createOut, err = RunGh(ghPath, verbose, args...)
					}

					// Check error again after potential retry
					if err != nil {
						// Re-check exitErr for the retry attempt
						if errors.As(err, &exitErr) {
							stderr = string(exitErr.Stderr)
							// Handle cases where PR might have been created externally during race
							if strings.Contains(stderr, "already exists") {
								out, _ := RunGh(ghPath, verbose, "pr", "list", "--repo", *r.URL, "--head", branchName, "--json", "url", "-q", ".[0].url")
								prURL := strings.TrimSpace(out)
								if prURL != "" {
									fmt.Printf("[%s] Pull Request already exists: %s\n", repoName, prURL)
									mu.Lock()
									prMap[repoName] = prURL
									mu.Unlock()
									return
								}
							}
							// No commits between?
							if strings.Contains(stderr, "No commits between") {
								fmt.Printf("[%s] No commits between %s and %s. Skipping PR creation.\n", repoName, baseBranch, branchName)
								return
							}

							mu.Lock()
							errs = append(errs, fmt.Sprintf("[%s] PR Create failed: %s", repoName, stderr))
							mu.Unlock()
							return
						}
						mu.Lock()
						errs = append(errs, fmt.Sprintf("[%s] PR Create failed: %v", repoName, err))
						mu.Unlock()
						return
					}
				} else {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("[%s] PR Create failed: %v", repoName, err))
					mu.Unlock()
					return
				}
			}
			lines := strings.Split(strings.TrimSpace(string(createOut)), "\n")
			// The last line typically contains the URL
			prURL := lines[len(lines)-1]

			mu.Lock()
			prMap[repoName] = prURL
			mu.Unlock()

		}(repo)
	}
	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("errors occurred during PR creation:\n%s", strings.Join(errs, "\n"))
	}
	return prMap, nil
}
