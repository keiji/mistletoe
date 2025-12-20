package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// HandlePr handles the 'pr' subcommand.
func HandlePr(args []string, opts GlobalOptions) {
	if len(args) == 0 {
		fmt.Println("Usage: mstl-gh pr <subcommand> [options]")
		fmt.Println("Available subcommands: create, checkout, status")
		os.Exit(1)
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case CmdCreate:
		handlePrCreate(subArgs, opts)
	case CmdCheckout:
		handlePrCheckout(subArgs, opts)
	case CmdStatus:
		handlePrStatus(subArgs, opts)
	default:
		fmt.Printf("Unknown pr subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

// handlePrStatus handles 'pr status'.
func handlePrStatus(args []string, opts GlobalOptions) {
	fs := flag.NewFlagSet("pr status", flag.ExitOnError)
	var (
		fLong     string
		fShort    string
		pVal      int
		pValShort int
		vLong     bool
		vShort    bool
	)

	fs.StringVar(&fLong, "file", "", "Configuration file path")
	fs.StringVar(&fShort, "f", "", "Configuration file path (shorthand)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "Number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "Number of parallel processes (shorthand)")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Resolve common values
	configPath, parallel, configData, err := ResolveCommonValues(fLong, fShort, pVal, pValShort)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	verbose := vLong || vShort

	// 1. Check gh availability
	if err := checkGhAvailability(opts.GhPath, verbose); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 2. Load Config
	var config *Config
	if configPath != "" {
		config, err = loadConfigFile(configPath)
	} else {
		config, err = loadConfigData(configData)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 3. Validate Integrity
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath, verbose); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Initialize Spinner
	spinner := NewSpinner(verbose)
	spinner.Start()

	// 4. Collect Status
	rows := CollectStatus(config, parallel, opts.GitPath, verbose, false)

	// 5. Collect PR Status
	prRows := CollectPrStatus(rows, config, parallel, opts.GhPath, verbose, nil)

	spinner.Stop()

	// 6. Render
	RenderPrStatusTable(prRows)
}

// PrInfo holds information about a Pull Request.
type PrInfo struct {
	Number      int    `json:"number"`
	State       string `json:"state"`
	IsDraft     bool   `json:"isDraft"`
	URL         string `json:"url"`
	BaseRefName string `json:"baseRefName"`
}

// PrStatusRow represents a row in the PR status table.
type PrStatusRow struct {
	StatusRow
	PrNumber string
	PrState  string
	PrURL    string
	Base     string
}

// CollectPrStatus collects Pull Request status for the given repositories.
// knownPRs is an optional map of [RepoID] -> URL to skip querying existing PRs.
func CollectPrStatus(statusRows []StatusRow, config *Config, parallel int, ghPath string, verbose bool, knownPRs map[string]string) []PrStatusRow {
	repoMap := make(map[string]Repository)
	for _, r := range *config.Repositories {
		repoMap[getRepoName(r)] = r
	}

	prRows := make([]PrStatusRow, len(statusRows))
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var mu sync.Mutex

	for i, row := range statusRows {
		wg.Add(1)
		go func(idx int, r StatusRow) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			prRow := PrStatusRow{StatusRow: r}

			isKnown := false
			if knownPRs != nil {
				if url, ok := knownPRs[r.Repo]; ok && url != "" {
					isKnown = true
					prRow.PrURL = url
					parts := strings.Split(url, "/")
					if len(parts) > 0 {
						prRow.PrNumber = "#" + parts[len(parts)-1]
					} else {
						prRow.PrNumber = "?"
					}
					// Approximation since we don't store state in map
					prRow.PrState = "Ready"
				}
			}

			conf, ok := repoMap[r.Repo]
			if ok && conf.URL != nil {
				baseBranch := ""
				if conf.BaseBranch != nil && *conf.BaseBranch != "" {
					baseBranch = *conf.BaseBranch
				} else if conf.Branch != nil && *conf.Branch != "" {
					baseBranch = *conf.Branch
				}
				if baseBranch != "" {
					prRow.Base = baseBranch
				}

				if !isKnown && r.RepoDir != "" && r.BranchName != "HEAD" && r.BranchName != "" {
					args := []string{"pr", "list", "--repo", *conf.URL, "--head", r.BranchName, "--state", "all", "--json", "number,state,isDraft,url,baseRefName"}
					if baseBranch != "" {
						args = append(args, "--base", baseBranch)
					}

					out, err := RunGh(ghPath, verbose, args...)
					if err == nil {
						var prs []PrInfo
						if err := json.Unmarshal([]byte(out), &prs); err == nil && len(prs) > 0 {
							// Sort PRs
							sortPrs(prs)

							// Format PR column
							var prLines []string
							for _, pr := range prs {
								displayState := ""
								// Determine display state
								if pr.IsDraft && pr.State == GitHubPrStateOpen {
									displayState = DisplayPrStateDraft
								} else {
									switch pr.State {
									case GitHubPrStateOpen:
										displayState = DisplayPrStateOpen
									case GitHubPrStateMerged:
										displayState = DisplayPrStateMerged
									case GitHubPrStateClosed:
										displayState = DisplayPrStateClosed
									default:
										displayState = pr.State // Fallback
									}
								}

								prLines = append(prLines, fmt.Sprintf("%s [%s]", pr.URL, displayState))
							}
							prRow.PrURL = strings.Join(prLines, "\n")

							// Set other fields based on the first (most relevant) PR
							topPr := prs[0]
							prRow.PrNumber = fmt.Sprintf("#%d", topPr.Number)
							prRow.PrState = topPr.State // Raw state

							if prRow.Base == "" {
								prRow.Base = topPr.BaseRefName
							}
						} else {
							prRow.PrNumber = "N/A"
						}
					} else {
						prRow.PrNumber = "N/A"
					}
				} else if !isKnown {
					prRow.PrNumber = "N/A"
				}
			}

			mu.Lock()
			prRows[idx] = prRow
			mu.Unlock()

		}(i, row)
	}
	wg.Wait()

	return prRows
}

func sortPrs(prs []PrInfo) {
	stateRank := func(pr PrInfo) int {
		// Handle Draft explicitly
		if pr.IsDraft && strings.ToUpper(pr.State) == GitHubPrStateOpen {
			return 1
		}

		switch strings.ToUpper(pr.State) {
		case GitHubPrStateOpen:
			return 0
		case GitHubPrStateMerged:
			return 2
		case GitHubPrStateClosed:
			return 3
		default:
			return 4
		}
	}

	sort.Slice(prs, func(i, j int) bool {
		rankI := stateRank(prs[i])
		rankJ := stateRank(prs[j])

		if rankI != rankJ {
			return rankI < rankJ
		}
		// Same state, sort by number descending
		return prs[i].Number > prs[j].Number
	})
}

// RenderPrStatusTable renders the PR status table.
func RenderPrStatusTable(rows []PrStatusRow) {
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeaderAutoFormat(tw.Off),
		tablewriter.WithRowAutoWrap(tw.WrapNone),
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
	// Change Header Order: Repository, Base, Branch/Rev, Status, PR
	table.Header("Repository", "Base", "Branch/Rev", "Status", "PR")

	const (
		Reset    = "\033[0m"
		FgRed    = "\033[31m"
		FgGreen  = "\033[32m"
		FgYellow = "\033[33m"
	)

	for _, row := range rows {
		statusStr := ""
		if row.HasUnpushed {
			statusStr += FgGreen + StatusSymbolUnpushed + Reset
		}

		if row.HasConflict {
			statusStr += FgYellow + StatusSymbolConflict + Reset
		} else if row.IsPullable {
			statusStr += FgYellow + StatusSymbolPullable + Reset
		}

		if statusStr == "" {
			statusStr = "-"
		}

		prContent := row.PrURL
		if prContent == "" {
			prContent = "-"
		}

		_ = table.Append(row.Repo, row.Base, row.LocalBranchRev, statusStr, prContent)
	}
	if err := table.Render(); err != nil {
		fmt.Printf("Error rendering table: %v\n", err)
	}
	fmt.Printf("Status Legend: %s Pullable, %s Unpushed, %s Conflict\n", StatusSymbolPullable, StatusSymbolUnpushed, StatusSymbolConflict)
}

// handlePrCreate handles 'pr create'.
func handlePrCreate(args []string, opts GlobalOptions) {
	fs := flag.NewFlagSet("pr create", flag.ExitOnError)
	var (
		fLong      string
		fShort     string
		pVal       int
		pValShort  int
		tLong      string
		tShort     string
		bLong      string
		bShort     string
		dLong      string
		dShort     string
		vLong      bool
		vShort     bool
	)

	fs.StringVar(&fLong, "file", "", "Configuration file path")
	fs.StringVar(&fShort, "f", "", "Configuration file path (shorthand)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "Number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "Number of parallel processes (shorthand)")
	fs.StringVar(&tLong, "title", "", "Pull Request title")
	fs.StringVar(&tShort, "t", "", "Pull Request title (shorthand)")
	fs.StringVar(&bLong, "body", "", "Pull Request body")
	fs.StringVar(&bShort, "b", "", "Pull Request body (shorthand)")
	fs.StringVar(&dLong, "dependencies", "", "Dependency graph file path")
	fs.StringVar(&dShort, "d", "", "Dependency graph file path (shorthand)")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Resolve common values
	configPath, parallel, configData, err := ResolveCommonValues(fLong, fShort, pVal, pValShort)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	verbose := vLong || vShort

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
	if depPath == "" {
		depPath = dShort
	}

	// 1. Check gh availability
	if err := checkGhAvailability(opts.GhPath, verbose); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 2. Load Config
	var config *Config
	if configPath != "" {
		config, err = loadConfigFile(configPath)
	} else {
		config, err = loadConfigData(configData)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 3. Load Dependencies (if specified)
	var deps *DependencyGraph
	var depContent string
	if depPath != "" {
		contentBytes, errRead := os.ReadFile(depPath)
		if errRead != nil {
			fmt.Printf("Error reading dependency file: %v\n", errRead)
			os.Exit(1)
		}
		depContent = string(contentBytes)

		var validIDs []string
		for _, r := range *config.Repositories {
			validIDs = append(validIDs, getRepoName(r))
		}
		var errDep error
		deps, errDep = ParseDependencies(depContent, validIDs)
		if errDep != nil {
			fmt.Printf("Error loading dependencies: %v\n", errDep)
			os.Exit(1)
		}
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
	rows := CollectStatus(config, parallel, opts.GitPath, verbose, true)
	// Initial Check: No known PRs yet
	prRows := CollectPrStatus(rows, config, parallel, opts.GhPath, verbose, nil)
	spinner.Stop()
	RenderPrStatusTable(prRows)

	// 6. Check for Behind/Conflict/Detached
	// New Requirement: Abort if pull required (behind)
	var behindRepos []string
	for _, row := range rows {
		if row.IsPullable {
			behindRepos = append(behindRepos, row.Repo)
		}
		if row.HasConflict {
			fmt.Printf("Error: Repository '%s' has conflicts. Cannot proceed.\n", row.Repo)
			os.Exit(1)
		}
		if row.BranchName == "HEAD" {
			fmt.Printf("Error: Repository '%s' is in a detached HEAD state. Cannot proceed.\n", row.Repo)
			os.Exit(1)
		}
	}

	if len(behindRepos) > 0 {
		fmt.Printf("Error: The following repositories are behind remote and require a pull:\n")
		for _, r := range behindRepos {
			fmt.Printf(" - %s\n", r)
		}
		fmt.Println("Please pull changes before creating Pull Requests.")
		os.Exit(1)
	}

	// 6.5 Categorize Repositories
	fmt.Println("Analyzing repository states...")

	var pushList []Repository
	var createList []Repository
	var updateList []Repository
	var skippedRepos []string

	repoMap := make(map[string]Repository)
	for _, r := range *config.Repositories {
		repoMap[getRepoName(r)] = r
	}

	// Map to track PR existence
	prExistsMap := make(map[string]string) // RepoName -> URL
	for _, prRow := range prRows {
		if prRow.PrURL != "" && prRow.PrURL != "-" && prRow.PrURL != "N/A" {
			prExistsMap[prRow.Repo] = prRow.PrURL
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
			// Should not happen if integrity passed
			continue
		}

		_, hasPR := prExistsMap[repoName]

		// Condition 1: PR Exists (Open/Draft)
		if hasPR {
			// User: "If PR exists ... need to push ... keep as Update PR"
			// Push is needed if local ahead. If local == remote, push is no-op, but we include in pushList for simplicity/consistency.
			// Effectively, if PR exists, we treat it as "Active".
			pushList = append(pushList, repo)
			updateList = append(updateList, repo)
		} else {
			// Condition 2: No PR Exists
			// User: "If local has NO new commits (not ahead) -> No Push, No Create (Skip)"
			// User: "If local has new commits (ahead) -> Push, Create"

			// Check if Ahead
			// Note: status.HasUnpushed is true if Local > Remote.
			// Note: CollectStatus sets HasUnpushed=true if Remote is missing (Unborn remote branch) or Local ahead.
			if status.HasUnpushed {
				pushList = append(pushList, repo)
				createList = append(createList, repo)
			} else {
				// Local == Remote (or verify fail)
				skippedRepos = append(skippedRepos, repoName)
			}
		}
	}

	if len(skippedRepos) > 0 {
		fmt.Println("The following repositories will be skipped (no changes and no existing PR):")
		for _, r := range skippedRepos {
			fmt.Printf(" - %s\n", r)
		}
		fmt.Println()
	}

	// Combine createList + updateList for "Active Repos" processing
	var activeRepos []Repository
	// We need unique list, but lists are mutually exclusive in terms of (Repo in createList) vs (Repo in updateList)
	// Because `if hasPR { update } else { create/skip }`
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

	if allUpdates {
		fmt.Print("No new Pull Requests to create. Update existing Pull Request descriptions? (yes/no): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "y" || input == "yes" {
			skipEditor = true
		} else {
			fmt.Println("Aborted.")
			os.Exit(1)
		}
	} else {
		fmt.Print("Proceed with Push and Pull Request creation? (yes/no): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "y" || input == "yes" {
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
				fmt.Printf("Error getting message: %v\n", err)
				os.Exit(1)
			}
			prTitle, prBody = ParsePrTitleBody(content)
		}
	}

	// 8. Check GitHub Management & Permissions & Base Branches (for active repos)
	fmt.Println("Verifying GitHub permissions and base branches...")
	// We pass existing PR map to optimize check
	// verifyGithubRequirements returns valid existing PRs, but we already have `prExistsMap`.
	// However, it also checks PERMISSIONS and BASE BRANCH existence.
	_, err = verifyGithubRequirements(activeRepos, rows, parallel, opts.GitPath, opts.GhPath, verbose, prExistsMap)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 9. Execution Phase 1: Push
	if len(pushList) > 0 {
		fmt.Println("Pushing changes...")
		if err := executePush(pushList, rows, parallel, opts.GitPath, verbose); err != nil {
			fmt.Printf("Error during push: %v\n", err)
			os.Exit(1)
		}
	}

	// 9. Execution Phase 2: Create PRs
	// We need a map of ALL PR URLs (existing + newly created) for the snapshot/related-pr logic.
	finalPrMap := make(map[string]string)
	for k, v := range prExistsMap {
		finalPrMap[k] = v
	}

	if len(createList) > 0 {
		fmt.Println("Creating Pull Requests...")
		// Create placeholder body
		placeholderBlock := GeneratePlaceholderMistletoeBody()
		prBodyWithPlaceholder := EmbedMistletoeBody(prBody, placeholderBlock)

		createdMap, err := executePrCreationOnly(createList, rows, parallel, opts.GhPath, verbose, prTitle, prBodyWithPlaceholder)
		if err != nil {
			fmt.Printf("Error during PR creation: %v\n", err)
			os.Exit(1)
		}
		for k, v := range createdMap {
			finalPrMap[k] = v
		}
	}

	// 9. Execution Phase 3: Update Descriptions (All Active PRs)
	// We need to update descriptions for BOTH newly created PRs (to fill real snapshot) AND existing PRs (to update snapshot).
	fmt.Println("Generating configuration snapshot...")
	snapshotData, snapshotID, err := GenerateSnapshotFromStatus(config, rows)
	if err != nil {
		fmt.Printf("Error generating snapshot: %v\n", err)
		os.Exit(1)
	}

	filename := fmt.Sprintf("mistletoe-snapshot-%s.json", snapshotID)
	if err := os.WriteFile(filename, snapshotData, 0644); err != nil {
		fmt.Printf("Error writing snapshot file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Snapshot saved to %s\n", filename)

	fmt.Println("Updating Pull Request descriptions...")
	// Determine targets for update: All activeRepos should have a PR in finalPrMap now.
	// But we only update if they are in activeRepos (Create List + Update List).
	targetPrMap := make(map[string]string)
	for _, r := range activeRepos {
		rID := getRepoName(r)
		if url, ok := finalPrMap[rID]; ok {
			targetPrMap[rID] = url
		}
	}

	if err := updatePrDescriptions(targetPrMap, parallel, opts.GhPath, verbose, string(snapshotData), filename, deps, depContent); err != nil {
		fmt.Printf("Error updating descriptions: %v\n", err)
		os.Exit(1)
	}

	// 11. Show Status (Final)
	fmt.Println("Collecting final status...")
	spinner = NewSpinner(verbose)
	spinner.Start()
	finalRows := CollectStatus(config, parallel, opts.GitPath, verbose, true)
	finalPrRows := CollectPrStatus(finalRows, config, parallel, opts.GhPath, verbose, finalPrMap)
	spinner.Stop()
	RenderPrStatusTable(finalPrRows)

	fmt.Println("Done.")
}

// executePush pushes changes for the given repositories.
func executePush(repos []Repository, rows []StatusRow, parallel int, gitPath string, verbose bool) error {
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var mu sync.Mutex
	var errs []string

	statusMap := make(map[string]StatusRow)
	for _, r := range rows {
		statusMap[r.Repo] = r
	}

	for _, repo := range repos {
		wg.Add(1)
		go func(r Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoDir := GetRepoDir(r)
			repoName := getRepoName(r)

			branchName := ""
			if row, ok := statusMap[repoName]; ok && row.BranchName != "" {
				branchName = row.BranchName
			} else {
				// Fallback
				b, err := RunGit(repoDir, gitPath, verbose, "rev-parse", "--abbrev-ref", "HEAD")
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("[%s] Failed to get branch: %v", repoName, err))
					mu.Unlock()
					return
				}
				branchName = b
			}

			fmt.Printf("[%s] Pushing to origin/%s...\n", repoName, branchName)
			if _, err := RunGit(repoDir, gitPath, verbose, "push", "origin", branchName); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("[%s] Push failed: %v", repoName, err))
				mu.Unlock()
				return
			}
		}(repo)
	}
	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("Errors occurred during push:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

// executePrCreationOnly creates PRs for the given repositories.
// Returns a map of RepoName -> PR URL.
func executePrCreationOnly(repos []Repository, rows []StatusRow, parallel int, ghPath string, verbose bool, title, body string) (map[string]string, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var errs []string
	prMap := make(map[string]string)

	statusMap := make(map[string]StatusRow)
	for _, r := range rows {
		statusMap[r.Repo] = r
	}

	for _, repo := range repos {
		wg.Add(1)
		go func(r Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoName := getRepoName(r)
			branchName := ""
			if row, ok := statusMap[repoName]; ok && row.BranchName != "" {
				branchName = row.BranchName
			} else {
				// Should have been resolved by now, but strictly speaking:
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

			createOut, err := RunGh(ghPath, verbose, args...)
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					stderr := string(exitErr.Stderr)
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
				} else {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("[%s] PR Create failed: %v", repoName, err))
					mu.Unlock()
				}
				return
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
		return nil, fmt.Errorf("Errors occurred during PR creation:\n%s", strings.Join(errs, "\n"))
	}
	return prMap, nil
}

func updatePrDescriptions(prMap map[string]string, parallel int, ghPath string, verbose bool, snapshotData, snapshotFilename string, deps *DependencyGraph, depContent string) error {
	if len(prMap) == 0 {
		return nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var errs []string

	for id, url := range prMap {
		wg.Add(1)
		go func(repoID, targetURL string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check PR State
			stateOut, err := RunGh(ghPath, verbose, "pr", "view", targetURL, "--json", "state", "-q", ".state")
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Failed to check state for PR %s: %v", targetURL, err))
				mu.Unlock()
				return
			}
			state := strings.TrimSpace(string(stateOut))
			if state == "MERGED" || state == "CLOSED" {
				return
			}

			// Get current body
			bodyOut, err := RunGh(ghPath, verbose, "pr", "view", targetURL, "--json", "body", "-q", ".body")
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Failed to view PR %s: %v", targetURL, err))
				mu.Unlock()
				return
			}
			originalBody := strings.TrimSpace(string(bodyOut))

			// Generate new Mistletoe block
			newBlock := GenerateMistletoeBody(snapshotData, snapshotFilename, repoID, prMap, deps, depContent)

			// Update body
			newBody := EmbedMistletoeBody(originalBody, newBlock)

			// Update
			_, err = RunGh(ghPath, verbose, "pr", "edit", targetURL, "--body", newBody)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Failed to edit PR %s: %v", targetURL, err))
				mu.Unlock()
				return
			}
		}(id, url)
	}
	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("Errors updating descriptions:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

func getRepoName(r Repository) string {
	if r.ID != nil && *r.ID != "" {
		return *r.ID
	}
	// Fallback to dir name
	return GetRepoDir(r)
}

// resolveRemoteBranchHash tries to resolve the remote branch hash locally first,
// and falls back to ls-remote if necessary.
func resolveRemoteBranchHash(repoDir, gitPath, branchName string, verbose bool) (string, error) {
	// 1. Try local ref (fast)
	// checks refs/remotes/origin/<branchName>
	out, err := RunGit(repoDir, gitPath, verbose, "rev-parse", "--verify", "refs/remotes/origin/"+branchName)
	if err == nil && out != "" {
		return strings.TrimSpace(out), nil
	}

	// 2. Fallback to ls-remote (network, slow)
	lsOut, err := RunGit(repoDir, gitPath, verbose, "ls-remote", "--heads", "origin", branchName)
	if err != nil {
		return "", err
	}

	lines := strings.Split(lsOut, "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			// exact match for branch
			if parts[1] == "refs/heads/"+branchName {
				return parts[0], nil
			}
		}
	}

	return "", nil
}

// Mockable lookPath for testing
var lookPath = exec.LookPath

func checkGhAvailability(ghPath string, verbose bool) error {
	_, err := lookPath(ghPath)
	if err != nil {
		return errors.New("Error: 'gh' command not found. Please install GitHub CLI.")
	}
	_, err = RunGh(ghPath, verbose, "auth", "status")
	if err != nil {
		return errors.New("Error: 'gh' is not authenticated. Please run 'gh auth login'.")
	}
	return nil
}

// verifyGithubRequirements checks GitHub URL, permissions, base branch existence, and existing PRs.
// It returns a map of RepoName -> Existing PR URL.
// Accepts knownPRs map[string]string (ID -> URL) to optimize existing PR check.
func verifyGithubRequirements(repos []Repository, rows []StatusRow, parallel int, gitPath, ghPath string, verbose bool, knownPRs map[string]string) (map[string]string, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var errs []string
	existingPRs := make(map[string]string)

	statusMap := make(map[string]StatusRow)
	for _, r := range rows {
		statusMap[r.Repo] = r
	}

	for _, repo := range repos {
		wg.Add(1)
		go func(r Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoName := getRepoName(r)

			// 1. Check if URL is GitHub
			if r.URL == nil || !strings.Contains(*r.URL, "github.com") {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Repository %s is not a GitHub repository", repoName))
				mu.Unlock()
				return
			}

			// 2. Check Permission
			out, err := RunGh(ghPath, verbose, "repo", "view", *r.URL, "--json", "viewerPermission", "-q", ".viewerPermission")
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Failed to check permission for %s: %v", repoName, err))
				mu.Unlock()
				return
			}
			perm := strings.TrimSpace(out)
			if perm != "ADMIN" && perm != "MAINTAIN" && perm != "WRITE" {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Insufficient permission for %s: %s (need WRITE or better)", repoName, perm))
				mu.Unlock()
				return
			}

			// 3. Check Base Branch Existence
			baseBranch := ""
			if r.BaseBranch != nil && *r.BaseBranch != "" {
				baseBranch = *r.BaseBranch
			} else if r.Branch != nil && *r.Branch != "" {
				baseBranch = *r.Branch
			}

			if baseBranch != "" {
				repoDir := GetRepoDir(r)
				remoteHash, err := resolveRemoteBranchHash(repoDir, gitPath, baseBranch, verbose)
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("[%s] Failed to check base branch '%s': %v", repoName, baseBranch, err))
					mu.Unlock()
					return
				}
				if remoteHash == "" {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("[%s] Base branch '%s' does not exist on remote", repoName, baseBranch))
					mu.Unlock()
					return
				}
			}

			// 4. Check for existing PR
			if knownPRs != nil {
				if url, ok := knownPRs[repoName]; ok && url != "" {
					mu.Lock()
					existingPRs[repoName] = url
					mu.Unlock()
					return
				}
			}

			// Fallback to query
			repoDir := GetRepoDir(r)
			branchName := ""

			if row, ok := statusMap[repoName]; ok && row.BranchName != "" {
				branchName = row.BranchName
			} else {
				// Redundant fallback
				b, err := RunGit(repoDir, gitPath, verbose, "rev-parse", "--abbrev-ref", "HEAD")
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("[%s] Failed to get branch for PR check: %v", repoName, err))
					mu.Unlock()
					return
				}
				branchName = b
			}

			out, errCheck := RunGh(ghPath, verbose, "pr", "list", "--repo", *r.URL, "--head", branchName, "--json", "url", "-q", ".[0].url")
			if errCheck != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("[%s] Failed to check for existing PR: %v", repoName, errCheck))
				mu.Unlock()
				return
			}
			prURL := strings.TrimSpace(out)

			if prURL != "" {
				mu.Lock()
				existingPRs[repoName] = prURL
				mu.Unlock()
			}

		}(repo)
	}
	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("GitHub validation failed:\n%s", strings.Join(errs, "\n"))
	}
	return existingPRs, nil
}
