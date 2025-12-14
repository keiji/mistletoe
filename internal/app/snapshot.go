package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func handleSnapshot(args []string, opts GlobalOptions) {
	var oShort, oLong string
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	fs.StringVar(&oLong, "output-file", "", "output file path")
	fs.StringVar(&oShort, "o", "", "output file path (short)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	outputFile := oLong
	if outputFile == "" {
		outputFile = oShort
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		fmt.Printf("Error reading current directory: %v.\n", err)
		os.Exit(1)
	}

	var repos []Repository

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		gitDir := fmt.Sprintf("%s/.git", dirName)

		if _, err := os.Stat(gitDir); err != nil {
			// Not a git repository
			continue
		}

		// Get remote origin URL
		url, err := RunGit(dirName, opts.GitPath, "remote", "get-url", "origin")
		if err != nil {
			// Try getting it via config if get-url fails (older git versions or odd setups)
			url, err = RunGit(dirName, opts.GitPath, "config", "--get", "remote.origin.url")
			if err != nil {
				fmt.Printf("Warning: Could not get remote origin for %s. Skipping.\n", dirName)
				continue
			}
		}
		// RunGit already trims

		// Get current branch
		branch, err := RunGit(dirName, opts.GitPath, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			fmt.Printf("Warning: Could not get current branch for %s.\n", dirName)
			branch = ""
		}

		revision := ""
		// If branch is "HEAD", it's a detached HEAD state
		if branch == "HEAD" {
			branch = ""
			revision, err = RunGit(dirName, opts.GitPath, "rev-parse", "HEAD")
			if err != nil {
				fmt.Printf("Warning: Could not get revision for %s.\n", dirName)
				revision = ""
			}
		}

		id := dirName
		// Construct repository
		var branchPtr *string
		if branch != "" {
			branchPtr = &branch
		}
		var revisionPtr *string
		if revision != "" {
			revisionPtr = &revision
		}
		urlPtr := &url

		repo := Repository{
			ID:       &id,
			URL:      urlPtr,
			Branch:   branchPtr,
			Revision: revisionPtr,
		}
		repos = append(repos, repo)
	}

	if outputFile == "" {
		identifier := CalculateSnapshotIdentifier(repos)
		outputFile = fmt.Sprintf("mistletoe-snapshot-%s.json", identifier)
	}

	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("Error: Output file '%s' exists.\n", outputFile)
		os.Exit(1)
	}

	config := Config{
		Repositories: &repos,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Printf("Error generating JSON: %v.\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		fmt.Printf("Error writing to file '%s': %v.\n", outputFile, err)
		os.Exit(1)
	}

	fmt.Printf("Snapshot saved to %s\n", outputFile)
}
