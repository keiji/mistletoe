package app

import (
	conf "mistletoe/internal/config"
	"mistletoe/internal/sys"
	"mistletoe/internal/ui"
)

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func handlePrCheckout(args []string, opts GlobalOptions) {
	if err := prCheckoutCommand(args, opts); err != nil {
		os.Exit(1)
	}
}

func prCheckoutCommand(args []string, opts GlobalOptions) error {
	fs := flag.NewFlagSet("pr checkout", flag.ContinueOnError)
	var (
		uLong     string
		uShort    string
		dLong     string
		depth     int
		jVal      int
		jValShort int
		vLong     bool
		vShort    bool
		yes       bool
		yesShort  bool
	)

	fs.StringVar(&uLong, "url", "", "Pull Request URL")
	fs.StringVar(&uShort, "u", "", "Pull Request URL (shorthand)")
	fs.StringVar(&dLong, "dest", "", "Destination directory")
	fs.IntVar(&depth, "depth", 0, "Create a shallow clone with a history truncated to the specified number of commits")
	fs.IntVar(&jVal, "jobs", -1, "Number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "Number of concurrent jobs (shorthand)")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")
	fs.BoolVar(&yes, "yes", false, "Automatically answer 'yes' to all prompts")
	fs.BoolVar(&yesShort, "y", false, "Automatically answer 'yes' to all prompts (shorthand)")

	fs.SetOutput(sys.Stderr)

	if err := ParseFlagsFlexible(fs, args); err != nil {
		// fmt.Println(err) // flag package handles printing
		return err
	}

	if err := CheckFlagDuplicates(fs, [][2]string{
		{"url", "u"},
		{"jobs", "j"},
		{"verbose", "v"},
		{"yes", "y"},
	}); err != nil {
		fmt.Println("Error:", err)
		return err
	}

	dest := dLong
	if dest == "" {
		dest = "."
	}

	prURL := uLong
	if prURL == "" {
		prURL = uShort
	}

	if prURL == "" {
		fmt.Println("Error: Pull Request URL is required (-u or --url)")
		return fmt.Errorf("URL required")
	}

	// Resolve Jobs - pr_checkout is unique, it doesn't utilize ResolveCommonValues for config loading initially
	// because it doesn't take a config file argument (config comes from snapshot inside PR).
	// However, it does take jobs flags.
	// Since we don't have a config file to fallback to for 'jobs' initially, we just resolve flag.

	jobsFlag := -1
	if jVal != -1 {
		jobsFlag = jVal
	} else if jValShort != -1 {
		jobsFlag = jValShort
	}

	verbose := vLong || vShort

	// 1. Check gh availability
	if err := checkGhAvailability(opts.GhPath, verbose); err != nil {
		fmt.Println(err)
		return err
	}

	// 2. Fetch PR Body
	fmt.Printf("Fetching Pull Request information from %s...\n", prURL)
	out, err := RunGh(opts.GhPath, verbose, "pr", "view", prURL, "--json", "body", "-q", ".body")
	if err != nil {
		fmt.Printf("Error fetching PR body: %v\n", err)
		return err
	}
	prBody := string(out)

	// 3. Parse Mistletoe Block
	fmt.Println("Parsing Mistletoe block...")
	config, relatedJSON, dependencyContent, found := ParseMistletoeBlock(prBody)
	if !found {
		fmt.Println("Error: Mistletoe block not found in PR body")
		return fmt.Errorf("Mistletoe block not found")
	}
	if config == nil {
		fmt.Println("Error: Snapshot data missing in Mistletoe block")
		return fmt.Errorf("snapshot data missing")
	}

	// Resolve Jobs (Config fallback from Snapshot)
	jobs, err := DetermineJobs(jobsFlag, config)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return err
	}

	// Verbose Override
	if verbose && jobs > 1 {
		fmt.Println("Verbose is specified, so jobs is treated as 1.")
		jobs = 1
	}

	// Filter repositories based on Related PR status (Open/Draft only)
	if len(relatedJSON) > 0 {
		var rel RelatedPRsJSON
		if err := json.Unmarshal(relatedJSON, &rel); err == nil {
			// Gather all URLs
			var allURLs []string
			allURLs = append(allURLs, rel.Dependencies...)
			allURLs = append(allURLs, rel.Dependents...)
			allURLs = append(allURLs, rel.Others...)

			if len(allURLs) > 0 {
				fmt.Println("Verifying related Pull Requests status...")

				// Map URL -> State
				// We need to check state for these PRs.
				// Optimization: Check in parallel or sequentially.
				// Since we might have many, let's use a simple parallel loop.

				type prCheckResult struct {
					url   string
					state string
					err   error
				}

				ch := make(chan prCheckResult, len(allURLs))
				for _, u := range allURLs {
					go func(url string) {
						out, err := RunGh(opts.GhPath, verbose, "pr", "view", url, "--json", "state", "-q", ".state")
						ch <- prCheckResult{url: url, state: string(out), err: err}
					}(u)
				}

				// Collect
				closedPRs := make(map[string]bool)
				for i := 0; i < len(allURLs); i++ {
					res := <-ch
					if res.err != nil {
						fmt.Printf("Warning: Failed to check status for %s: %v\n", res.url, res.err)
						continue
					}
					// Parse state (trim whitespace)
					st := strings.TrimSpace(res.state)

					// Check if Open
					if st != "OPEN" {
						// Note: Draft is also OPEN in GitHub API json state, usually.
						// If specific field isDraft is needed, we should query it.
						// But usually "state": "OPEN" covers draft.
						// If state is MERGED or CLOSED, we filter out.
						closedPRs[res.url] = true
					}
				}

				if len(closedPRs) > 0 {
					// Filter snapshot repositories
					var newRepos []conf.Repository
					for _, r := range *config.Repositories {
						// We need to match Repo to PR URL.
						// Heuristic: PR URL starts with Repo URL (minus .git)
						keep := true
						repoURL := ""
						if r.URL != nil {
							repoURL = *r.URL
							if len(repoURL) > 4 && repoURL[len(repoURL)-4:] == ".git" {
								repoURL = repoURL[:len(repoURL)-4]
							}
						}

						for closedURL := range closedPRs {
							// If PR URL belongs to this Repo
							// e.g. https://github.com/org/repo/pull/1 belongs to https://github.com/org/repo
							// Check prefix
							if repoURL != "" && len(closedURL) > len(repoURL) && closedURL[:len(repoURL)] == repoURL {
								// Confirm separator
								if closedURL[len(repoURL)] == '/' {
									fmt.Printf("Skipping repository '%s' because linked PR %s is not Open/Draft.\n", getRepoName(r), closedURL)
									keep = false
									break
								}
							}
						}
						if keep {
							newRepos = append(newRepos, r)
						}
					}
					config.Repositories = &newRepos
				}
			}
		} else {
			fmt.Printf("Warning: related PR JSON is invalid: %v\n", err)
		}
	}

	// 4. Init / Checkout
	if len(*config.Repositories) == 0 {
		fmt.Println("No repositories to initialize (all filtered or empty snapshot).")
		return nil
	}

	// We capture absDest but we don't strictly need to pass it to safety check here
	// because pr checkout implicitly trusts the PR content via user action?
	// Actually, pr checkout works similarly to init.
	// However, pr checkout relies on `PerformInit` which does `validateEnvironment` (repo based).
	// The prompt didn't ask to add safety check to `pr checkout`, only `init`.
	// But `pr checkout` calls `validateAndPrepareInitDest`.
	// I will just ignore the return string for now to fix compilation.
	if _, err := validateAndPrepareInitDest(dest); err != nil {
		fmt.Printf("Error preparing destination: %v\n", err)
		return err
	}

	fmt.Println("Initializing repositories based on snapshot...")
	// The snapshot contains the target state. We treat it as the config.
	// PerformInit handles validation, cloning, and checking out.
	if err := PerformInit(*config.Repositories, "", opts.GitPath, jobs, depth, verbose); err != nil {
		fmt.Printf("Error during initialization: %v\n", err)
		// We continue to status even if some failed? Or exit?
		// Usually Init failure is critical.
		return err
	}

	// Save config and dependency
	mstlDir := ".mstl"
	if err := os.MkdirAll(mstlDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create .mstl directory: %v\n", err)
	} else {
		// Filter config (exclude private repos)
		var filteredRepos []conf.Repository
		var validIDs []string
		for _, repo := range *config.Repositories {
			if repo.Private != nil && *repo.Private {
				continue
			}
			filteredRepos = append(filteredRepos, repo)
			// Collect valid IDs for dependency filtering
			if repo.ID != nil && *repo.ID != "" {
				validIDs = append(validIDs, *repo.ID)
			} else {
				// Should have been set by parsing or validating
				// ParseMistletoeBlock -> Unmarshal -> Doesn't call validateRepositories immediately?
				// We should derive ID if missing, similar to validateRepositories.
				// Use GetRepoDirName logic from config package.
				dirName := conf.GetRepoDirName(repo)
				if dirName != "" {
					validIDs = append(validIDs, dirName)
				}
			}
		}
		filteredConfig := *config
		filteredConfig.Repositories = &filteredRepos

		// Save config.json
		configFile := filepath.Join(mstlDir, "config.json")
		configBytes, err := json.MarshalIndent(filteredConfig, "", "  ")
		if err == nil {
			if err := os.WriteFile(configFile, configBytes, 0644); err != nil {
				fmt.Printf("Warning: Failed to write %s: %v\n", configFile, err)
			} else {
				fmt.Printf("Saved configuration to %s\n", configFile)
			}
		}

		// Save dependency-graph.md if exists
		if dependencyContent != "" {
			// We no longer filter dependencies as per design change in init (copying raw behavior).
			// If validation or filtering is strictly required for checkout, it should be re-evaluated.
			// For now, we mirror the 'cp' behavior of init for consistency, assuming the embedded graph is correct.
			if err := writeDependencyFile(mstlDir, dependencyContent); err != nil {
				fmt.Printf("Warning: Failed to write dependency-graph.md: %v\n", err)
			} else {
				fmt.Printf("Saved dependency graph to %s\n", filepath.Join(mstlDir, "dependency-graph.md"))
			}
		}
	}

	// 5. Status
	fmt.Println("Verifying status...")
	spinner := ui.NewSpinner(verbose)
	spinner.Start()
	rows := CollectStatus(config, jobs, opts.GitPath, verbose, false)
	prRows := CollectPrStatus(rows, config, jobs, opts.GhPath, verbose, nil)
	spinner.Stop()

	RenderPrStatusTable(sys.Stdout, prRows)

	return nil
}

func writeDependencyFile(dir, content string) error {
	trimmed := strings.TrimSpace(content)
	finalContent := content
	if !strings.HasPrefix(trimmed, "```mermaid") {
		var sb strings.Builder
		sb.WriteString("```mermaid\n")
		sb.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("```\n")
		finalContent = sb.String()
	}

	depFile := filepath.Join(dir, "dependency-graph.md")
	return os.WriteFile(depFile, []byte(finalContent), 0644)
}
