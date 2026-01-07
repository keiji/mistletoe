package app

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// handlePrUpdate handles 'pr update'.
func handlePrUpdate(args []string, opts GlobalOptions) {
	fs := flag.NewFlagSet("pr update", flag.ExitOnError)
	var (
		fLong     string
		fShort    string
		pVal      int
		pValShort int
		dLong     string
		dShort    string
		vLong     bool
		vShort    bool
	)

	fs.StringVar(&fLong, "file", DefaultConfigFile, "Configuration file path")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "Configuration file path (shorthand)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "Number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "Number of parallel processes (shorthand)")
	fs.StringVar(&dLong, "dependencies", DefaultDependencies, "Dependency graph file path")
	fs.StringVar(&dShort, "d", DefaultDependencies, "Dependency graph file path (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Resolve common values
	configPath, parallel, configData, err := ResolveCommonValues(fLong, fShort, pVal, pValShort, ignoreStdin)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	verbose := vLong || vShort
	if verbose {
		parallel = 1
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
			fmt.Printf("error reading dependency file: %v\n", errRead)
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
			fmt.Printf("error loading dependencies: %v\n", errDep)
			os.Exit(1)
		}
		fmt.Println("Dependency graph loaded successfully.")
	}

	// 4. Validate Integrity
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath, verbose); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 5. Collect Status & PR Status
	fmt.Println("Collecting repository status and checking for existing Pull Requests...")
	spinner := NewSpinner(verbose)
	spinner.Start()

	rows := CollectStatus(config, parallel, opts.GitPath, verbose, false)

	// Collect PR Status
	prRows := CollectPrStatus(rows, config, parallel, opts.GhPath, verbose, nil)
	spinner.Stop()
	RenderPrStatusTable(prRows)

	// 6. Check for Behind/Conflict/Detached
	ValidateStatusForAction(rows, true)

	// 7. Identify Active PRs to Update
	targetPrMap := make(map[string][]PrInfo)
	// We also need a map of ALL PRs for Related Links generation
	allPrMap := make(map[string][]PrInfo)

	var activeRepos []Repository
	repoMap := make(map[string]Repository)
	for _, r := range *config.Repositories {
		repoMap[getRepoName(r)] = r
	}

	for _, prRow := range prRows {
		if len(prRow.PrItems) > 0 {
			allPrMap[prRow.Repo] = prRow.PrItems

			// Check if Open
			hasOpen := false
			for _, item := range prRow.PrItems {
				if strings.EqualFold(item.State, GitHubPrStateOpen) {
					hasOpen = true
					break
				}
			}

			if hasOpen {
				targetPrMap[prRow.Repo] = prRow.PrItems
				if r, ok := repoMap[prRow.Repo]; ok {
					activeRepos = append(activeRepos, r)
				}
			}
		}
	}

	if len(activeRepos) == 0 {
		fmt.Println("No active Pull Requests found to update.")
		return
	}

	// 7.5 Check for Push (Ahead)
	var pushList []Repository
	statusMap := make(map[string]StatusRow)
	for _, r := range rows {
		statusMap[r.Repo] = r
	}

	for _, repo := range activeRepos {
		repoName := getRepoName(repo)
		if status, ok := statusMap[repoName]; ok {
			if status.HasUnpushed {
				pushList = append(pushList, repo)
			}
		}
	}

	if len(pushList) > 0 {
		fmt.Println("Pushing changes for repositories with active Pull Requests...")
		if err := executePush(pushList, rows, parallel, opts.GitPath, verbose); err != nil {
			fmt.Printf("error during push: %v\n", err)
			os.Exit(1)
		}
	}

	// 8. Generate Snapshot
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

	// 9. Update Descriptions
	fmt.Println("Updating Pull Request descriptions...")

	// Pass allPrMap so that Merged/Closed PRs are included in Related Links,
	// but updatePrDescriptions will skip updating them.
	if err := updatePrDescriptions(allPrMap, parallel, opts.GhPath, verbose, string(snapshotData), filename, deps, depContent); err != nil {
		fmt.Printf("error updating descriptions: %v\n", err)
		os.Exit(1)
	}

	// 10. Final Status
	var displayRows []PrStatusRow
	for _, row := range prRows {
		if !strings.EqualFold(row.PrState, GitHubPrStateOpen) {
			row.PrDisplay = "-"
		}
		displayRows = append(displayRows, row)
	}
	RenderPrStatusTable(displayRows)

	fmt.Println("Done.")
}
