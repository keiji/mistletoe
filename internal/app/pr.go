package app

import (
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

	// 5. Check Pushability
	// "If there are repositories that cannot be pushed, terminate with error."
	// Cannot push if:
	// - Behind remote (IsPullable)
	// - Conflict (HasConflict)
	// - (Ideally we also check if we are ahead, but prompting says "check if pushable (no unpulled commits)")
	for _, row := range rows {
		if row.IsPullable {
			fmt.Printf("Error: Repository '%s' has unpulled commits (sync required). Cannot proceed.\n", row.Repo)
			os.Exit(1)
		}
		if row.HasConflict {
			fmt.Printf("Error: Repository '%s' has conflicts. Cannot proceed.\n", row.Repo)
			os.Exit(1)
		}
	}

	// 6. Check GitHub Management & Permissions
	fmt.Println("Verifying GitHub permissions...")
	if err := verifyGithubRequirements(config, parallel); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 7. Execution: Push & Create PR
	fmt.Println("Pushing changes and creating Pull Requests...")
	prURLs, err := executePrCreation(config, parallel, opts.GitPath)
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

func verifyGithubRequirements(config *Config, parallel int) error {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var errs []string

	for _, repo := range *config.Repositories {
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

func executePrCreation(config *Config, parallel int, gitPath string) ([]string, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var errs []string
	var prURLs []string

	for _, repo := range *config.Repositories {
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

			// Check if we actually need to push?
			// The prompt says "Push repositories". We can force push or just push.
			// Ideally we assume standard push.
			fmt.Printf("[%s] Pushing to origin/%s...\n", getRepoName(r), branchName)
			if _, err := RunGit(repoDir, gitPath, "push", "origin", branchName); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("[%s] Push failed: %v", getRepoName(r), err))
				mu.Unlock()
				return
			}

			// 2. Create PR
			// gh pr create --fill --repo <url> --head <branch>
			// We should run this command inside the repo dir or specify repo?
			// Specifying --repo is safer.
			// "gh" command is not handled by RunGit, so we use exec.Command.
			// NOTE: gh pr create might fail if PR already exists.
			// If it exists, we should probably get the URL.
			// "gh pr list --head <branch> --json url -q .[0].url"

			// First, check if PR exists
			checkCmd := exec.Command("gh", "pr", "list", "--repo", *r.URL, "--head", branchName, "--json", "url", "-q", ".[0].url")
			out, err := checkCmd.Output()
			prURL := strings.TrimSpace(string(out))

			if prURL == "" {
				// Create
				fmt.Printf("[%s] Creating Pull Request...\n", getRepoName(r))
				createCmd := exec.Command("gh", "pr", "create", "--repo", *r.URL, "--head", branchName, "--fill")
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
	footer := "\n----\nRelated Pull Request(s):\n"
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
			// gh pr view <url> --json body -q .body
			viewCmd := exec.Command("gh", "pr", "view", targetURL, "--json", "body", "-q", ".body")
			bodyOut, err := viewCmd.Output()
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("Failed to view PR %s: %v", targetURL, err))
				mu.Unlock()
				return
			}
			originalBody := strings.TrimSpace(string(bodyOut))

			// Check if already updated to avoid duplication if run multiple times
			// Simplified check
			if strings.Contains(originalBody, "Related Pull Request(s):") {
				// Maybe strip old footer and replace?
				// For now, assume we append. But if we append every time, it gets messy.
				// Let's split by "----\nRelated Pull Request(s):"
				parts := strings.Split(originalBody, "----\nRelated Pull Request(s):")
				originalBody = strings.TrimSpace(parts[0])
			}

			newBody := originalBody + "\n" + footer

			// Update
			// gh pr edit <url> --body <newBody>
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
