package app

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

// handlePr handles the 'pr' subcommand.
func handlePr(args []string, opts GlobalOptions) {
	if len(args) == 0 {
		fmt.Println("Usage: mstl-gh pr <subcommand> [options]")
		fmt.Println("Available subcommands: create")
		os.Exit(1)
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "create":
		handlePrCreate(subArgs, opts)
	default:
		fmt.Printf("Unknown pr subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

// handlePrCreate handles 'pr create'.
func handlePrCreate(args []string, opts GlobalOptions) {
	fs := flag.NewFlagSet("pr create", flag.ExitOnError)
	var (
		fLong      string
		fShort     string
		pVal       int
		pValShort  int
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
	if err := checkGhAvailability(); err != nil {
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
	// "If there are repositories that cannot be pushed, terminate with error."
	// Cannot push if:
	// - Behind remote (IsPullable)
	// - Conflict (HasConflict)
	// - Detached HEAD (BranchName == "HEAD") -> Prompt user
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
		// Check for detached HEAD
		// CollectStatus sets BranchName to "HEAD" if detached (based on rev-parse --abbrev-ref HEAD returning HEAD, see status_logic.go:114)
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

	// 6. Check GitHub Management & Permissions
	// Filter out ignored repos
	activeRepos := filterRepositories(config, ignoredRepos)
	if len(activeRepos) == 0 {
		fmt.Println("No repositories to process.")
		return
	}

	fmt.Println("Verifying GitHub permissions...")
	if err := verifyGithubRequirements(activeRepos, parallel); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 7. Execution: Push & Create PR
	fmt.Println("Pushing changes and creating Pull Requests...")
	prURLs, err := executePrCreation(activeRepos, parallel, opts.GitPath)
	if err != nil {
		fmt.Printf("Error during execution: %v\n", err)
		os.Exit(1)
	}

	// 8. Post-processing: Update Descriptions
	fmt.Println("Updating Pull Request descriptions...")
	if err := updatePrDescriptions(prURLs, parallel); err != nil {
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

func checkGhAvailability() error {
	// check if gh is in path
	_, err := exec.LookPath("gh")
	if err != nil {
		return errors.New("Error: 'gh' command not found. Please install GitHub CLI.")
	}
	// check auth status
	// gh auth status
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return errors.New("Error: 'gh' is not authenticated. Please run 'gh auth login'.")
	}
	return nil
}

func verifyGithubRequirements(repos []Repository, parallel int) error {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var errs []string

	for _, repo := range repos {
		wg.Add(1)
		go func(r Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 1. Check if URL is GitHub
			// Simplified check: contains "github.com"
			if r.URL == nil || !strings.Contains(*r.URL, "github.com") {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Repository %s is not a GitHub repository", getRepoName(r)))
				mu.Unlock()
				return
			}

			// 2. Check Permission
			// gh repo view <url> --json viewerPermission -q .viewerPermission
			// Expected: ADMIN, MAINTAIN, WRITE. READ is not enough.
			cmd := exec.Command("gh", "repo", "view", *r.URL, "--json", "viewerPermission", "-q", ".viewerPermission")
			out, err := cmd.Output()
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Failed to check permission for %s: %v", getRepoName(r), err))
				mu.Unlock()
				return
			}
			perm := strings.TrimSpace(string(out))
			if perm != "ADMIN" && perm != "MAINTAIN" && perm != "WRITE" {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Insufficient permission for %s: %s (need WRITE or better)", getRepoName(r), perm))
				mu.Unlock()
				return
			}
		}(repo)
	}
	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("GitHub validation failed:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

func executePrCreation(repos []Repository, parallel int, gitPath string) ([]string, error) {
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

			// 1. Push
			// git push origin <current_branch>
			// We need current branch.
			branchName, err := RunGit(repoDir, gitPath, "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("[%s] Failed to get branch: %v", getRepoName(r), err))
				mu.Unlock()
				return
			}

			fmt.Printf("[%s] Pushing to origin/%s...\n", getRepoName(r), branchName)
			if _, err := RunGit(repoDir, gitPath, "push", "origin", branchName); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("[%s] Push failed: %v", getRepoName(r), err))
				mu.Unlock()
				return
			}

			// 2. Create PR
			// Check if PR exists
			checkCmd := exec.Command("gh", "pr", "list", "--repo", *r.URL, "--head", branchName, "--json", "url", "-q", ".[0].url")
			out, err := checkCmd.Output()
			prURL := strings.TrimSpace(string(out))

			if prURL == "" {
				// Create
				fmt.Printf("[%s] Creating Pull Request...\n", getRepoName(r))

				// Arguments for gh pr create
				// gh pr create --fill --repo <url> --head <branch> --base <base_branch_if_configured>
				args := []string{"pr", "create", "--repo", *r.URL, "--head", branchName, "--fill"}

				if r.Branch != nil && *r.Branch != "" {
					args = append(args, "--base", *r.Branch)
				}

				createCmd := exec.Command("gh", args...)
				// Capture output to get URL
				createOut, err := createCmd.Output()
				if err != nil {
					// Retrieve error output if possible
					var exitErr *exec.ExitError
					if errors.As(err, &exitErr) {
						mu.Lock()
						errs = append(errs, fmt.Sprintf("[%s] PR Create failed: %s", getRepoName(r), string(exitErr.Stderr)))
						mu.Unlock()
					} else {
						mu.Lock()
						errs = append(errs, fmt.Sprintf("[%s] PR Create failed: %v", getRepoName(r), err))
						mu.Unlock()
					}
					return
				}
				// Output of gh pr create is the URL
				lines := strings.Split(strings.TrimSpace(string(createOut)), "\n")
				prURL = lines[len(lines)-1] // URL is usually last line
			} else {
				fmt.Printf("[%s] Pull Request already exists: %s\n", getRepoName(r), prURL)
			}

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

func updatePrDescriptions(prURLs []string, parallel int) error {
	if len(prURLs) == 0 {
		return nil
	}

	// Construct footer
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
			viewCmd := exec.Command("gh", "pr", "view", targetURL, "--json", "body", "-q", ".body")
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
			editCmd := exec.Command("gh", "pr", "edit", targetURL, "--body", newBody)
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
