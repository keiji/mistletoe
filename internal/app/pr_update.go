package app

import (
	"fmt"
	"os"
	"flag"
	"strings"
)

// handlePrUpdate handles 'pr update'.
func handlePrUpdate(args []string, opts GlobalOptions) {
	fs := flag.NewFlagSet("pr update", flag.ExitOnError)
	var (
		fLong      string
		fShort     string
		pVal       int
		pValShort  int
		dLong      string
		dShort     string
		vLong      bool
		vShort     bool
	)

	fs.StringVar(&fLong, "file", "", "Configuration file path")
	fs.StringVar(&fShort, "f", "", "Configuration file path (shorthand)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "Number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "Number of parallel processes (shorthand)")
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

	// 5. Collect Status & PR Status
	fmt.Println("Collecting repository status and checking for existing Pull Requests...")
	spinner := NewSpinner(verbose)
	spinner.Start()

	// CollectStatus with noFetch=false (we want accurate status check, similar to pr create but strictly verifying)
	// 'pr create' uses noFetch=true for optimization, but since 'pr update' is about updating metadata,
	// checking if we are behind (and thus our snapshot is old) is valuable.
	rows := CollectStatus(config, parallel, opts.GitPath, verbose, false)

	// Collect PR Status
	prRows := CollectPrStatus(rows, config, parallel, opts.GhPath, verbose, nil)
	spinner.Stop()
	RenderPrStatusTable(prRows)

	// 6. Check for Behind/Conflict/Detached
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
		fmt.Println("Please pull changes before updating Pull Requests.")
		os.Exit(1)
	}

	// 7. Identify Active PRs to Update
	// We only update if a PR exists (Open/Draft).
	targetPrMap := make(map[string][]PrInfo)
	var activeRepos []Repository
	repoMap := make(map[string]Repository)
	for _, r := range *config.Repositories {
		repoMap[getRepoName(r)] = r
	}

	for _, prRow := range prRows {
		if len(prRow.PrItems) > 0 {
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

	// 8. Generate Snapshot
	fmt.Println("Generating configuration snapshot...")
	snapshotData, snapshotID, err := GenerateSnapshotFromStatus(config, rows)
	if err != nil {
		fmt.Printf("Error generating snapshot: %v\n", err)
		os.Exit(1)
	}

	filename := fmt.Sprintf("mistletoe-snapshot-%s.json", snapshotID)
	// We write the file because UpdatePrDescriptions needs the file name/content logic
	if err := os.WriteFile(filename, snapshotData, 0644); err != nil {
		fmt.Printf("Error writing snapshot file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Snapshot saved to %s\n", filename)

	// 9. Update Descriptions
	fmt.Println("Updating Pull Request descriptions...")

	// Convert activeRepos to list of keys for verification if needed,
	// but targetPrMap already contains the filtered list.

	if err := updatePrDescriptions(targetPrMap, parallel, opts.GhPath, verbose, string(snapshotData), filename, deps, depContent); err != nil {
		fmt.Printf("Error updating descriptions: %v\n", err)
		os.Exit(1)
	}

	// 10. Final Status
	// Re-render table to show we are done (PR status hasn't changed, but good for confirmation)
	// Since we didn't create new PRs, 'prRows' is still valid, but let's re-render it.
	// Filter for Display (Open or Draft only)
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
