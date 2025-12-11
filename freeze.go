package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func handleFreeze(args []string, opts GlobalOptions) {
	var fShort, fLong string
	fs := flag.NewFlagSet("freeze", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	outputFile := opts.ConfigFile
	if fLong != "" {
		outputFile = fLong
	} else if fShort != "" {
		outputFile = fShort
	}

	if outputFile == "" {
		fmt.Println("Error: Please specify an output file using --file or -f")
		os.Exit(1)
	}

	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("Error: Output file '%s' already exists.\n", outputFile)
		os.Exit(1)
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		fmt.Printf("Error reading current directory: %v\n", err)
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
		cmdURL := exec.Command(opts.GitPath, "-C", dirName, "remote", "get-url", "origin")
		outURL, err := cmdURL.Output()
		if err != nil {
			// Try getting it via config if get-url fails (older git versions or odd setups)
			cmdURL = exec.Command(opts.GitPath, "-C", dirName, "config", "--get", "remote.origin.url")
			outURL, err = cmdURL.Output()
			if err != nil {
				fmt.Printf("Warning: Could not get remote origin for %s, skipping.\n", dirName)
				continue
			}
		}
		url := strings.TrimSpace(string(outURL))

		// Get current branch
		cmdBranch := exec.Command(opts.GitPath, "-C", dirName, "rev-parse", "--abbrev-ref", "HEAD")
		outBranch, err := cmdBranch.Output()
		branch := ""
		revision := ""
		if err != nil {
			fmt.Printf("Warning: Could not get current branch for %s.\n", dirName)
		} else {
			branch = strings.TrimSpace(string(outBranch))
		}

		// If branch is "HEAD", it's a detached HEAD state
		if branch == "HEAD" {
			branch = ""
			cmdRev := exec.Command(opts.GitPath, "-C", dirName, "rev-parse", "HEAD")
			outRev, err := cmdRev.Output()
			if err != nil {
				fmt.Printf("Warning: Could not get revision for %s.\n", dirName)
			} else {
				revision = strings.TrimSpace(string(outRev))
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

		repo := Repository{
			ID:       &id,
			URL:      url,
			Branch:   branchPtr,
			Revision: revisionPtr,
			Labels:   []string{},
		}
		repos = append(repos, repo)
	}

	config := Config{
		Repositories: repos,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Printf("Error generating JSON: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		fmt.Printf("Error writing to file '%s': %v\n", outputFile, err)
		os.Exit(1)
	}
}
