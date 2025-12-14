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
	configPath, parallel, err := ResolveCommonValues(fLong, fShort, pVal, pValShort)
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
	config, err := loadConfig(configPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 3. Validate Integrity
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 4. Collect Status
	rows := CollectStatus(config, parallel, opts.GitPath)

	// 5. Collect PR Status
	prRows := CollectPrStatus(rows, config, parallel, opts.GhPath)

	// 6. Render
	RenderPrStatusTable(prRows)
}

type PrInfo struct {
	Number      int    `json:"number"`
	State       string `json:"state"`
	IsDraft     bool   `json:"isDraft"`
	URL         string `json:"url"`
	BaseRefName string `json:"baseRefName"`
}

type PrStatusRow struct {
	StatusRow
	PrNumber string
	PrState  string
	PrURL    string
	Base     string
}

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
				if conf.Branch != nil && *conf.Branch != "" {
					baseBranch = *conf.Branch
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
						// On error (e.g. network), we might want to show error or just N/A.
						// N/A is safer to keep table clean, or "Err".
						// Let's stick to N/A for now.
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
	table.Header("Repository", "PR", "Base", "Branch/Rev", "Status")

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

		prStr := row.PrNumber
		if row.PrState != "" {
			prStr += " - " + row.PrState
		}

		_ = table.Append(row.Repo, prStr, row.Base, row.LocalBranchRev, statusStr)
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
	)

	fs.StringVar(&fLong, "file", "", "Configuration file path")
	fs.StringVar(&fShort, "f", "", "Configuration file path (shorthand)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "Number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "Number of parallel processes (shorthand)")
	fs.StringVar(&tLong, "title", "", "Pull Request title")
	fs.StringVar(&tShort, "t", "", "Pull Request title (shorthand)")
	fs.StringVar(&bLong, "body", "", "Pull Request body")
	fs.StringVar(&bShort, "b", "", "Pull Request body (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Resolve common values
	configPath, parallel, err := ResolveCommonValues(fLong, fShort, pVal, pValShort)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Resolve title and body
	prTitle := tLong
	if prTitle == "" {
		prTitle = tShort
	}
	prBody := bLong
	if prBody == "" {
		prBody = bShort
	}

	// 1. Check gh availability
	if err := checkGhAvailability(opts.GhPath); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 2. Load Config
	config, err := loadConfig(configPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 3. Validate Integrity
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 4. Collect Status
	fmt.Println("Collecting repository status...")
	rows := CollectStatus(config, parallel, opts.GitPath)
	RenderStatusTable(rows)

	// 5. Check Pushability & Detached HEAD
	var detachedRepos []string

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
			detachedRepos = append(detachedRepos, row.Repo)
		}
	}

	// Handle Detached HEADs
	ignoredRepos := make(map[string]bool)
	if len(detachedRepos) > 0 {
		fmt.Printf("Warning: The following repositories are in a detached HEAD state and cannot participate in PR creation:\n")
		for _, r := range detachedRepos {
			fmt.Printf(" - %s\n", r)
		}

		fmt.Print("Do you want to continue processing other repositories? (yes/no): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("Aborted.")
			os.Exit(1)
		}
		for _, r := range detachedRepos {
			ignoredRepos[r] = true
		}
	}

	// Confirmation Prompt
	fmt.Print("Proceed with Push and Pull Request creation? (yes/no): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input != "y" && input != "yes" {
		fmt.Println("Aborted.")
		os.Exit(1)
	}

	// Input Message if needed
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

	// 6. Check GitHub Management & Permissions & Existing PRs
	// Filter out ignored repos
	activeRepos := filterRepositories(config, ignoredRepos)
	if len(activeRepos) == 0 {
		fmt.Println("No repositories to process.")
		return
	}

	fmt.Println("Verifying GitHub permissions and checking existing PRs...")
	existingPrURLs, err := verifyGithubRequirements(activeRepos, parallel, opts.GitPath, opts.GhPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 7. Execution: Push & Create PR
	fmt.Println("Pushing changes and creating Pull Requests...")
	prURLs, err := executePrCreation(activeRepos, parallel, opts.GitPath, opts.GhPath, existingPrURLs, prTitle, prBody)
	if err != nil {
		fmt.Printf("Error during execution: %v\n", err)
		os.Exit(1)
	}

	// Add already existing URLs to the list for description updating
	for _, url := range existingPrURLs {
		if url != "" {
			// Avoid duplicates if executePrCreation somehow added it (it shouldn't if we pass it)
			found := false
			for _, p := range prURLs {
				if p == url {
					found = true
					break
				}
			}
			if !found {
				prURLs = append(prURLs, url)
			}
		}
	}

	// 8. Post-processing: Update Descriptions
	fmt.Println("Updating Pull Request descriptions...")
	if err := updatePrDescriptions(prURLs, parallel, opts.GhPath); err != nil {
		fmt.Printf("Error updating descriptions: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Done.")
}

func filterRepositories(config *Config, ignoredRepos map[string]bool) []Repository {
	var filtered []Repository
	for _, repo := range *config.Repositories {
		name := getRepoName(repo)
		if !ignoredRepos[name] {
			filtered = append(filtered, repo)
		}
	}
	return filtered
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

// verifyGithubRequirements checks GitHub URL, permissions, and existing PRs.
// It returns a map of RepoName -> Existing PR URL.
func verifyGithubRequirements(repos []Repository, parallel int, gitPath, ghPath string) (map[string]string, error) {
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

			// 3. Check for existing PR
			// We need the branch name to check for existing PR
			repoDir := getRepoDir(r)
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

func executePrCreation(repos []Repository, parallel int, gitPath, ghPath string, existingPRs map[string]string, title, body string) ([]string, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var errs []string
	var prURLs []string

	for _, repo := range repos {
		wg.Add(1)
		go func(r Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoDir := getRepoDir(r)
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

			if r.Branch != nil && *r.Branch != "" {
				args = append(args, "--base", *r.Branch)
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
							prURLs = append(prURLs, prURL)
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
			prURLs = append(prURLs, prURL)
			mu.Unlock()

		}(repo)
	}
	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("Errors occurred:\n%s", strings.Join(errs, "\n"))
	}
	return prURLs, nil
}

func updatePrDescriptions(prURLs []string, parallel int, ghPath string) error {
	if len(prURLs) == 0 {
		return nil
	}

	footer := "\n\n----\nRelated Pull Request(s):\n"
	for _, url := range prURLs {
		footer += fmt.Sprintf("* %s\n", url)
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var errs []string

	for _, url := range prURLs {
		wg.Add(1)
		go func(targetURL string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

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

			newBody := originalBody + footer

			// Update
			editCmd := execCommand(ghPath, "pr", "edit", targetURL, "--body", newBody)
			if err := editCmd.Run(); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Failed to edit PR %s: %v", targetURL, err))
				mu.Unlock()
				return
			}
		}(url)
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
	return getRepoDir(r)
}
