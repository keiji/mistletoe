package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
	rows := CollectStatus(config, parallel, opts.GitPath, verbose)

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
					args := []string{"pr", "list", "--repo", *conf.URL, "--head", r.BranchName, "--json", "number,state,isDraft,url,baseRefName"}
					if baseBranch != "" {
						args = append(args, "--base", baseBranch)
					}

					out, err := RunGh(ghPath, verbose, args...)
					if err == nil {
						var prs []PrInfo
						if err := json.Unmarshal([]byte(out), &prs); err == nil && len(prs) > 0 {
							pr := prs[0]
							prRow.PrNumber = fmt.Sprintf("#%d", pr.Number)
							prRow.PrState = "Ready"
							if pr.IsDraft {
								prRow.PrState = "Draft"
							}
							prRow.PrURL = pr.URL

							if prRow.Base == "" {
								prRow.Base = pr.BaseRefName
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
	rows := CollectStatus(config, parallel, opts.GitPath, verbose)
	// Initial Check: No known PRs yet
	prRows := CollectPrStatus(rows, config, parallel, opts.GhPath, verbose, nil)
	spinner.Stop()
	RenderPrStatusTable(prRows)

	// 6. Check Pushability & Detached HEAD
	for _, row := range rows {
		if row.IsPullable {
			fmt.Printf("Error: Repository '%s' has unpulled commits (sync required). Cannot proceed.\n", row.Repo)
			os.Exit(1)
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

	// 6.5 Filter repos with no changes relative to base
	fmt.Println("Checking for changes relative to base branch...")
	// OPTIMIZATION: Pass rows to avoid some git calls if possible, but filterPushableRepos needs logic not fully in rows.
	// But we can optimize fetching HEAD.
	activeRepos, skippedRepos := filterPushableRepos(*config.Repositories, rows, parallel, opts.GitPath, verbose)

	if len(skippedRepos) > 0 {
		fmt.Println("The following repositories will be skipped (all local commits are already in base branch):")
		for _, r := range skippedRepos {
			fmt.Printf(" - %s\n", r)
		}
		fmt.Println()
	}

	// 7. Check for All Existing PRs (ONLY for Active Repos) & Prompt
	allExist := true
	knownPRs := make(map[string]string)

	countPRs := 0
	activeMap := make(map[string]bool)
	for _, r := range activeRepos {
		activeMap[getRepoName(r)] = true
	}

	for _, row := range prRows {
		if activeMap[row.Repo] {
			if row.PrURL != "" && row.PrURL != "-" {
				knownPRs[row.Repo] = row.PrURL
				countPRs++
			}
		}
	}

	if len(activeRepos) > 0 {
		if countPRs < len(activeRepos) {
			allExist = false
		}
	}

	var skipEditor bool

	if len(activeRepos) > 0 {
		if allExist {
			fmt.Print("No repositories to create Pull Requests for. Update existing Pull Request descriptions? (yes/no): ")
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
	} else {
		fmt.Println("No repositories with changes to process.")
		return
	}

	// Input Message if needed
	if !skipEditor {
		if prTitle == "" && prBody == "" {
			content, err := RunEditor()
			if err != nil {
				fmt.Printf("Error getting message: %v\n", err)
				os.Exit(1)
			}
			lines := strings.Split(content, "\n")
			if len(lines) > 0 {
				prTitle = lines[0]
				prBody = content
			}
		}
	}

	// 8. Check GitHub Management & Permissions & Existing PRs
	if len(activeRepos) == 0 {
		fmt.Println("No repositories to process.")
		return
	}

	fmt.Println("Verifying GitHub permissions and base branches...")
	// Pass knownPRs and rows to optimize check
	existingPrURLs, err := verifyGithubRequirements(activeRepos, rows, parallel, opts.GitPath, opts.GhPath, verbose, knownPRs)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 9. Execution: Push & Create PR (with Placeholder)
	fmt.Println("Pushing changes and creating Pull Requests...")

	// Create placeholder body
	placeholderBlock := GeneratePlaceholderMistletoeBody()
	prBodyWithPlaceholder := EmbedMistletoeBody(prBody, placeholderBlock)

	// Pass rows to avoid re-fetching branch names during push/create
	prMap, err := executePrCreation(activeRepos, rows, parallel, opts.GitPath, opts.GhPath, verbose, existingPrURLs, prTitle, prBodyWithPlaceholder)
	if err != nil {
		fmt.Printf("Error during execution: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generating configuration snapshot...")
	// OPTIMIZATION: Use GenerateSnapshotFromStatus
	// We do this AFTER creation to satisfy "Just get URL, then construct description"
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

	// 10. Post-processing: Update Descriptions
	fmt.Println("Updating Pull Request descriptions...")
	if err := updatePrDescriptions(prMap, parallel, opts.GhPath, verbose, string(snapshotData), filename, deps, depContent); err != nil {
		fmt.Printf("Error updating descriptions: %v\n", err)
		os.Exit(1)
	}

	// 11. Show Status (Final)
	fmt.Println("Collecting final status...")
	spinner = NewSpinner(verbose)
	spinner.Start()
	finalRows := CollectStatus(config, parallel, opts.GitPath, verbose)
	// OPTIMIZATION: Pass prMap (known created/updated PRs) to avoid redundant gh pr list calls
	finalPrRows := CollectPrStatus(finalRows, config, parallel, opts.GhPath, verbose, prMap)
	spinner.Stop()
	RenderPrStatusTable(finalPrRows)

	fmt.Println("Done.")
}

// filterPushableRepos filters out repositories where local branch commits are fully contained in the base branch.
func filterPushableRepos(repos []Repository, rows []StatusRow, parallel int, gitPath string, verbose bool) ([]Repository, []string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)

	shouldKeep := make([]bool, len(repos))

	// Map rows for quick lookup
	statusMap := make(map[string]StatusRow)
	for _, r := range rows {
		statusMap[r.Repo] = r
	}

	for i, repo := range repos {
		wg.Add(1)
		go func(idx int, r Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoDir := GetRepoDir(r)
			repoName := getRepoName(r)

			// Resolve Base Branch
			baseBranch := ""
			if r.BaseBranch != nil && *r.BaseBranch != "" {
				baseBranch = *r.BaseBranch
			} else if r.Branch != nil && *r.Branch != "" {
				baseBranch = *r.Branch
			}

			keep := true
			if baseBranch != "" {
				lsOut, err := RunGit(repoDir, gitPath, verbose, "ls-remote", "origin", baseBranch)

				var remoteHash string
				if err == nil && lsOut != "" {
					lines := strings.Split(lsOut, "\n")
					for _, line := range lines {
						parts := strings.Fields(line)
						if len(parts) >= 2 {
							if strings.HasSuffix(parts[1], "/"+baseBranch) {
								remoteHash = parts[0]
								break
							}
						}
					}
				}

				if remoteHash != "" {
					_, err := RunGit(repoDir, gitPath, verbose, "cat-file", "-e", remoteHash)
					if err != nil {
						keep = false
					} else {
						_, err := RunGit(repoDir, gitPath, verbose, "merge-base", "--is-ancestor", remoteHash, "HEAD")
						if err == nil {
							// Remote is ancestor.
							// Reuse HEAD from statusMap if possible
							headHash := ""
							if row, ok := statusMap[repoName]; ok && row.LocalHeadFull != "" {
								headHash = row.LocalHeadFull
							} else {
								headHash, _ = RunGit(repoDir, gitPath, verbose, "rev-parse", "HEAD")
							}

							if remoteHash == headHash {
								keep = false
							} else {
								keep = true
							}
						} else {
							keep = false
						}
					}
				} else {
					keep = true
				}
			}

			shouldKeep[idx] = keep
		}(i, repo)
	}
	wg.Wait()

	var active []Repository
	var skipped []string

	for i, keep := range shouldKeep {
		if keep {
			active = append(active, repos[i])
		} else {
			skipped = append(skipped, getRepoName(repos[i]))
		}
	}

	return active, skipped
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
				lsOut, lsErr := RunGit(repoDir, gitPath, verbose, "ls-remote", "--heads", "origin", baseBranch)
				if lsErr != nil {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("[%s] Failed to check base branch '%s': %v", repoName, baseBranch, lsErr))
					mu.Unlock()
					return
				}
				if strings.TrimSpace(lsOut) == "" {
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

// executePrCreation pushes changes and creates PRs.
// Returns a map of RepoName -> PR URL.
func executePrCreation(repos []Repository, rows []StatusRow, parallel int, gitPath, ghPath string, verbose bool, existingPRs map[string]string, title, body string) (map[string]string, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var errs []string
	prMap := make(map[string]string)

	// Pre-populate prMap with existing PRs
	for k, v := range existingPRs {
		prMap[k] = v
	}

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

			// 1. Push
			branchName := ""
			if row, ok := statusMap[repoName]; ok && row.BranchName != "" {
				branchName = row.BranchName
			} else {
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

			// 2. Create PR
			if url, ok := existingPRs[repoName]; ok {
				fmt.Printf("[%s] Pull Request already exists: %s (skipping creation)\n", repoName, url)
				// Already added to prMap
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
			prURL := lines[len(lines)-1]

			mu.Lock()
			prMap[repoName] = prURL
			mu.Unlock()

		}(repo)
	}
	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("Errors occurred:\n%s", strings.Join(errs, "\n"))
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
