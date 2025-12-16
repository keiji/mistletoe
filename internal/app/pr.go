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

var execCommand = exec.Command

// handlePr handles the 'pr' subcommand.
func handlePr(args []string, opts GlobalOptions) {
	if len(args) == 0 {
		fmt.Println("Usage: mstl-gh pr <subcommand> [options]")
		fmt.Println("Available subcommands: create, status")
		os.Exit(1)
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "create":
		handlePrCreate(subArgs, opts)
	case "status":
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
	)

	fs.StringVar(&fLong, "file", "", "Configuration file path")
	fs.StringVar(&fShort, "f", "", "Configuration file path (shorthand)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "Number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "Number of parallel processes (shorthand)")

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

	// 1. Check gh availability
	if err := checkGhAvailability(opts.GhPath); err != nil {
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
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Initialize Spinner
	spinner := NewSpinner()
	spinner.Start()

	// 4. Collect Status
	rows := CollectStatus(config, parallel, opts.GitPath)

	// 5. Collect PR Status
	prRows := CollectPrStatus(rows, config, parallel, opts.GhPath)

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
func CollectPrStatus(statusRows []StatusRow, config *Config, parallel int, ghPath string) []PrStatusRow {
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

				if r.RepoDir != "" && r.BranchName != "HEAD" && r.BranchName != "" {
					args := []string{"pr", "list", "--repo", *conf.URL, "--head", r.BranchName, "--json", "number,state,isDraft,url,baseRefName"}
					if baseBranch != "" {
						args = append(args, "--base", baseBranch)
					}

					cmd := execCommand(ghPath, args...)
					out, err := cmd.Output()
					if err == nil {
						var prs []PrInfo
						if err := json.Unmarshal(out, &prs); err == nil && len(prs) > 0 {
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
				} else {
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
			statusStr += FgGreen + ">" + Reset
		}

		if row.HasConflict {
			statusStr += FgYellow + "!" + Reset
		} else if row.IsPullable {
			statusStr += FgYellow + "<" + Reset
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
	fmt.Println("Status Legend: < Pullable, > Unpushed, ! Conflict")
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
	if err := checkGhAvailability(opts.GhPath); err != nil {
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
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 5. Collect Status & PR Status (Moved Up)
	fmt.Println("Collecting repository status and checking for existing Pull Requests...")
	spinner := NewSpinner()
	spinner.Start()
	rows := CollectStatus(config, parallel, opts.GitPath)
	prRows := CollectPrStatus(rows, config, parallel, opts.GhPath)
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
	activeRepos, skippedRepos := filterPushableRepos(*config.Repositories, parallel, opts.GitPath)

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
	// We need to check existence ONLY for activeRepos.
	// But prRows contains info for ALL repos.
	activeMap := make(map[string]bool)
	for _, r := range activeRepos {
		activeMap[getRepoName(r)] = true
	}

	for _, row := range prRows {
		// Only consider active repos for "allExist" logic
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
			fmt.Print("All active repositories have existing Pull Requests. Do you want to update the description? (yes/no): ")
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
	// Pass knownPRs to optimize check
	existingPrURLs, err := verifyGithubRequirements(activeRepos, parallel, opts.GitPath, opts.GhPath, knownPRs)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Generating configuration snapshot...")
	snapshotData, snapshotID, err := GenerateSnapshot(config, opts.GitPath)
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

	// Generate initial Mistletoe block without related PRs (pass empty map)
	initialMistletoeBlock := GenerateMistletoeBody(string(snapshotData), filename, "", make(map[string]string), deps, depContent)
	prBodyWithSnapshot := EmbedMistletoeBody(prBody, initialMistletoeBlock)

	// 9. Execution: Push & Create PR
	fmt.Println("Pushing changes and creating Pull Requests...")
	// executePrCreation returns a map of [RepoID] -> URL
	prMap, err := executePrCreation(activeRepos, parallel, opts.GitPath, opts.GhPath, existingPrURLs, prTitle, prBodyWithSnapshot)
	if err != nil {
		fmt.Printf("Error during execution: %v\n", err)
		os.Exit(1)
	}

	// 10. Post-processing: Update Descriptions
	fmt.Println("Updating Pull Request descriptions...")
	if err := updatePrDescriptions(prMap, parallel, opts.GhPath, string(snapshotData), filename, deps, depContent); err != nil {
		fmt.Printf("Error updating descriptions: %v\n", err)
		os.Exit(1)
	}

	// 11. Show Status (Final)
	fmt.Println("Collecting final status...")
	spinner = NewSpinner()
	spinner.Start()
	// We re-collect status to show the most up-to-date information including new PRs
	finalRows := CollectStatus(config, parallel, opts.GitPath)
	finalPrRows := CollectPrStatus(finalRows, config, parallel, opts.GhPath)
	spinner.Stop()
	RenderPrStatusTable(finalPrRows)

	fmt.Println("Done.")
}

// filterPushableRepos filters out repositories where local branch commits are fully contained in the base branch.
func filterPushableRepos(repos []Repository, parallel int, gitPath string) ([]Repository, []string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)

	shouldKeep := make([]bool, len(repos))

	for i, repo := range repos {
		wg.Add(1)
		go func(idx int, r Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoDir := GetRepoDir(r)

			// Resolve Base Branch
			baseBranch := ""
			if r.BaseBranch != nil && *r.BaseBranch != "" {
				baseBranch = *r.BaseBranch
			} else if r.Branch != nil && *r.Branch != "" {
				baseBranch = *r.Branch
			}

			keep := true
			if baseBranch != "" {
				// We want to check if the local branch has any commits not present in the remote base branch.
				// To do this without fetching (to avoid changing local state), we use git ls-remote to get the hash of the remote base branch.

				// 1. Get Remote Hash for baseBranch
				lsOut, err := RunGit(repoDir, gitPath, "ls-remote", "origin", baseBranch)
				// ls-remote output format: <hash>\trefs/heads/<branch>\n
				// We expect one line if it matches exactly, or we need to parse carefully.
				// Since baseBranch is usually simple name (main), ls-remote origin main matches refs/heads/main usually.

				var remoteHash string
				if err == nil && lsOut != "" {
					lines := strings.Split(lsOut, "\n")
					for _, line := range lines {
						parts := strings.Fields(line)
						if len(parts) >= 2 {
							// Check if ref matches exact branch (refs/heads/baseBranch)
							// Or if we just trust the first one if exact match wasn't enforced by ls-remote arg?
							// "git ls-remote origin baseBranch" usually returns refs/heads/baseBranch if ambiguous,
							// or just that ref.
							if strings.HasSuffix(parts[1], "/"+baseBranch) {
								remoteHash = parts[0]
								break
							}
						}
					}
				}

				if remoteHash != "" {
					// 2. Check if we have this object locally
					err := execCommand(gitPath, "-C", repoDir, "cat-file", "-e", remoteHash).Run()
					if err != nil {
						// Remote object missing locally. This means remote has advanced (we are behind or diverged)
						// and we haven't fetched it.
						// In this case, we definitely cannot do a clean push or PR creation without pulling/fetching.
						// So we consider this "not pushable/PR-ready" in the context of "just creating PR for local changes".
						// We skip it.
						keep = false
					} else {
						// 3. Object exists. Check ancestry.
						// Is remoteHash an ancestor of HEAD?
						// git merge-base --is-ancestor <remote> <local>
						// If exit 0: Yes, remote is ancestor. (Local is ahead or same)
						// If exit 1: No. (Diverged or Local is behind)

						err := execCommand(gitPath, "-C", repoDir, "merge-base", "--is-ancestor", remoteHash, "HEAD").Run()
						if err == nil {
							// Remote is ancestor.
							// Check if they are the same commit.
							headHash, _ := RunGit(repoDir, gitPath, "rev-parse", "HEAD")
							if remoteHash == headHash {
								// Identical. No new commits.
								keep = false
							} else {
								// Ahead. Keep.
								keep = true
							}
						} else {
							// Diverged or Behind. Skip.
							keep = false
						}
					}
				} else {
					// Remote branch not found?
					// If verifyGithubRequirements doesn't catch it, we default to keep?
					// Or if remote branch missing, then everything is new?
					// Let's Keep.
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

func checkGhAvailability(ghPath string) error {
	_, err := lookPath(ghPath)
	if err != nil {
		return errors.New("Error: 'gh' command not found. Please install GitHub CLI.")
	}
	cmd := execCommand(ghPath, "auth", "status")
	if err := cmd.Run(); err != nil {
		return errors.New("Error: 'gh' is not authenticated. Please run 'gh auth login'.")
	}
	return nil
}

// verifyGithubRequirements checks GitHub URL, permissions, base branch existence, and existing PRs.
// It returns a map of RepoName -> Existing PR URL.
// Accepts knownPRs map[string]string (ID -> URL) to optimize existing PR check.
func verifyGithubRequirements(repos []Repository, parallel int, gitPath, ghPath string, knownPRs map[string]string) (map[string]string, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var errs []string
	existingPRs := make(map[string]string)

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
			cmd := execCommand(ghPath, "repo", "view", *r.URL, "--json", "viewerPermission", "-q", ".viewerPermission")
			out, err := cmd.Output()
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Failed to check permission for %s: %v", repoName, err))
				mu.Unlock()
				return
			}
			perm := strings.TrimSpace(string(out))
			if perm != "ADMIN" && perm != "MAINTAIN" && perm != "WRITE" {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Insufficient permission for %s: %s (need WRITE or better)", repoName, perm))
				mu.Unlock()
				return
			}

			// 3. Check Base Branch Existence
			// Resolve Base Branch: base-branch >> branch
			baseBranch := ""
			if r.BaseBranch != nil && *r.BaseBranch != "" {
				baseBranch = *r.BaseBranch
			} else if r.Branch != nil && *r.Branch != "" {
				baseBranch = *r.Branch
			}

			if baseBranch != "" {
				repoDir := GetRepoDir(r)
				// We check if the branch exists on remote using git ls-remote.
				// We need to use origin as the remote name (standard in this tool).
				// We run this command inside the repo directory.
				lsCmd := execCommand(gitPath, "-C", repoDir, "ls-remote", "--heads", "origin", baseBranch)
				lsOut, lsErr := lsCmd.Output()
				if lsErr != nil {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("[%s] Failed to check base branch '%s': %v", repoName, baseBranch, lsErr))
					mu.Unlock()
					return
				}
				if strings.TrimSpace(string(lsOut)) == "" {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("[%s] Base branch '%s' does not exist on remote", repoName, baseBranch))
					mu.Unlock()
					return
				}
			}

			// 4. Check for existing PR
			// Use knownPRs if available
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
			branchName, err := RunGit(repoDir, gitPath, "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("[%s] Failed to get branch for PR check: %v", repoName, err))
				mu.Unlock()
				return
			}

			checkCmd := execCommand(ghPath, "pr", "list", "--repo", *r.URL, "--head", branchName, "--json", "url", "-q", ".[0].url")
			out, errCheck := checkCmd.Output()
			if errCheck != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("[%s] Failed to check for existing PR: %v", repoName, errCheck))
				mu.Unlock()
				return
			}
			prURL := strings.TrimSpace(string(out))

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
func executePrCreation(repos []Repository, parallel int, gitPath, ghPath string, existingPRs map[string]string, title, body string) (map[string]string, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var errs []string
	prMap := make(map[string]string)

	// Pre-populate prMap with existing PRs
	for k, v := range existingPRs {
		prMap[k] = v
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
			branchName, err := RunGit(repoDir, gitPath, "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("[%s] Failed to get branch: %v", repoName, err))
				mu.Unlock()
				return
			}

			fmt.Printf("[%s] Pushing to origin/%s...\n", repoName, branchName)
			if _, err := RunGit(repoDir, gitPath, "push", "origin", branchName); err != nil {
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

			// Resolve Base Branch: base-branch >> branch
			baseBranch := ""
			if r.BaseBranch != nil && *r.BaseBranch != "" {
				baseBranch = *r.BaseBranch
			} else if r.Branch != nil && *r.Branch != "" {
				baseBranch = *r.Branch
			}

			if baseBranch != "" {
				args = append(args, "--base", baseBranch)
			}

			createCmd := execCommand(ghPath, args...)
			createOut, err := createCmd.Output()
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					stderr := string(exitErr.Stderr)
					if strings.Contains(stderr, "already exists") {
						checkCmd := execCommand(ghPath, "pr", "list", "--repo", *r.URL, "--head", branchName, "--json", "url", "-q", ".[0].url")
						out, _ := checkCmd.Output()
						prURL := strings.TrimSpace(string(out))
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

func updatePrDescriptions(prMap map[string]string, parallel int, ghPath string, snapshotData, snapshotFilename string, deps *DependencyGraph, depContent string) error {
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
			stateCmd := execCommand(ghPath, "pr", "view", targetURL, "--json", "state", "-q", ".state")
			stateOut, err := stateCmd.Output()
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
			viewCmd := execCommand(ghPath, "pr", "view", targetURL, "--json", "body", "-q", ".body")
			bodyOut, err := viewCmd.Output()
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
			editCmd := execCommand(ghPath, "pr", "edit", targetURL, "--body", newBody)
			if err := editCmd.Run(); err != nil {
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
