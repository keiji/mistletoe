package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func branchExists(dir, branch string) bool {
	cmd := exec.Command("git", "-C", dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return cmd.Run() == nil
}

func handleSwitch(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var createShort, createLong bool

	fs := flag.NewFlagSet("switch", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")
	fs.BoolVar(&createLong, "create", false, "create branch if it does not exist")
	fs.BoolVar(&createShort, "c", false, "create branch if it does not exist (short)")

	if err := fs.Parse(args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	create := createLong || createShort

	configFile := opts.ConfigFile
	if fLong != "" {
		configFile = fLong
	} else if fShort != "" {
		configFile = fShort
	}

	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if len(fs.Args()) == 0 {
		fmt.Println("Error: branch name required")
		os.Exit(1)
	}
	branchName := fs.Args()[0]

	// Map to store existence status for each repo (keyed by local directory path)
	dirExists := make(map[string]bool)

	// Pre-check phase
	for _, repo := range config.Repositories {
		dir := getRepoDir(repo)

		// Check if directory exists
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			fmt.Printf("Error: repository directory %s does not exist\n", dir)
			os.Exit(1)
		}

		exists := branchExists(dir, branchName)
		dirExists[dir] = exists
	}

	if !create {
		// Strict mode: All must exist
		var missing []string
		for _, repo := range config.Repositories {
			dir := getRepoDir(repo)
			if !dirExists[dir] {
				missing = append(missing, repo.URL+" ("+dir+")")
			}
		}

		if len(missing) > 0 {
			fmt.Printf("Error: branch '%s' does not exist in the following repositories:\n", branchName)
			for _, item := range missing {
				fmt.Println(" - " + item)
			}
			os.Exit(1)
		}

		// Execute Checkout
		for _, repo := range config.Repositories {
			dir := getRepoDir(repo)
			fmt.Printf("Switching %s to branch %s...\n", dir, branchName)
			cmd := exec.Command("git", "-C", dir, "checkout", branchName)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Printf("Error switching branch for %s: %v\n", dir, err)
				os.Exit(1)
			}
		}
	} else {
		// Create mode
		for _, repo := range config.Repositories {
			dir := getRepoDir(repo)
			exists := dirExists[dir]

			if exists {
				fmt.Printf("Branch %s exists in %s. Switching...\n", branchName, dir)
				cmd := exec.Command("git", "-C", dir, "checkout", branchName)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					fmt.Printf("Error switching branch for %s: %v\n", dir, err)
					os.Exit(1)
				}
			} else {
				fmt.Printf("Creating and switching to branch %s in %s...\n", branchName, dir)
				cmd := exec.Command("git", "-C", dir, "checkout", "-b", branchName)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					fmt.Printf("Error creating branch for %s: %v\n", dir, err)
					os.Exit(1)
				}
			}
		}
	}
}
