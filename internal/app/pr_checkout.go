package app

import (
	conf "mistletoe/internal/config"
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
	fs := flag.NewFlagSet("pr checkout", flag.ExitOnError)
	var (
		uLong     string
		uShort    string
		dLong     string
		jVal      int
		jValShort int
		vLong     bool
		vShort    bool
	)

	fs.StringVar(&uLong, "url", "", "Pull Request URL")
	fs.StringVar(&uShort, "u", "", "Pull Request URL (shorthand)")
	fs.StringVar(&dLong, "dest", "", "Destination directory")
	fs.IntVar(&jVal, "jobs", -1, "Number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "Number of concurrent jobs (shorthand)")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println(err)
		os.Exit(1)
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
		os.Exit(1)
	}

	// Resolve Jobs - pr_checkout is unique, it doesn't utilize ResolveCommonValues for config loading initially
	// because it doesn't take a config file argument (config comes from snapshot inside PR).
	// However, it does take jobs flags.
	// Since we don't have a config file to fallback to for 'jobs', we just use the flag or default.
	// Wait, if config is inside snapshot, can we use jobs from snapshot config?
	// The snapshot config struct has "Repositories". It's the same Config struct.
	// So yes, if the snapshot JSON has "jobs", we could respect it.

	jobs := -1
	if jVal != -1 {
		jobs = jVal
	} else if jValShort != -1 {
		jobs = jValShort
	}

	verbose := vLong || vShort

	// 1. Check gh availability
	if err := checkGhAvailability(opts.GhPath, verbose); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 2. Fetch PR Body
	fmt.Printf("Fetching Pull Request information from %s...\n", prURL)
	out, err := RunGh(opts.GhPath, verbose, "pr", "view", prURL, "--json", "body", "-q", ".body")
	if err != nil {
		fmt.Printf("Error fetching PR body: %v\n", err)
		os.Exit(1)
	}
	prBody := string(out)

	// 3. Parse Mistletoe Block
	fmt.Println("Parsing Mistletoe block...")
	config, relatedJSON, dependencyContent, found := ParseMistletoeBlock(prBody)
	if !found {
		fmt.Println("Error: Mistletoe block not found in PR body")
		os.Exit(1)
	}
	if config == nil {
		fmt.Println("Error: Snapshot data missing in Mistletoe block")
		os.Exit(1)
	}

	// Resolve Jobs (Config fallback from Snapshot)
	if jobs == -1 {
		if config.Jobs != nil {
			jobs = *config.Jobs
		} else {
			jobs = DefaultJobs
		}
	}

	// Verbose Override
	if verbose && jobs > 1 {
		fmt.Println("Verbose is specified, so jobs is treated as 1.")
		jobs = 1
	}

	// Final Validation
	if jobs < MinJobs {
		fmt.Printf("Error: Jobs must be at least %d.\n", MinJobs)
		os.Exit(1)
	}
	if jobs > MaxJobs {
		fmt.Printf("Error: Jobs must be at most %d.\n", MaxJobs)
		os.Exit(1)
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
			fmt.Printf("Warning: related PR JSON is invalid: %v\n", err) // err is from Unmarshal now? No, err variable scope issue.
			// Actually err was defined in if err := json.Unmarshal...
			// So it's fine.
		}
	}

	// 4. Init / Checkout
	if len(*config.Repositories) == 0 {
		fmt.Println("No repositories to initialize (all filtered or empty snapshot).")
		return
	}

	if err := validateAndPrepareInitDest(dest); err != nil {
		fmt.Printf("Error preparing destination: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Initializing repositories based on snapshot...")
	// The snapshot contains the target state. We treat it as the config.
	// PerformInit handles validation, cloning, and checking out.
	if err := PerformInit(*config.Repositories, "", opts.GitPath, jobs, 0, verbose); err != nil {
		fmt.Printf("Error during initialization: %v\n", err)
		// We continue to status even if some failed? Or exit?
		// Usually Init failure is critical.
		os.Exit(1)
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

		// Save dependencies.md if exists (filtered)
		if dependencyContent != "" {
			filteredDepContent := FilterDependencyContent(dependencyContent, validIDs)
			if err := writeDependencyFile(mstlDir, filteredDepContent); err != nil {
				fmt.Printf("Warning: Failed to write dependencies.md: %v\n", err)
			} else {
				fmt.Printf("Saved dependency graph to %s\n", filepath.Join(mstlDir, "dependencies.md"))
			}
		}
	}

	// 5. Status
	fmt.Println("Verifying status...")
	spinner := NewSpinner(verbose)
	spinner.Start()
	rows := CollectStatus(config, jobs, opts.GitPath, verbose, false)
	prRows := CollectPrStatus(rows, config, jobs, opts.GhPath, verbose, nil)
	spinner.Stop()

	RenderPrStatusTable(Stdout, prRows)
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

	depFile := filepath.Join(dir, "dependencies.md")
	return os.WriteFile(depFile, []byte(finalContent), 0644)
}
