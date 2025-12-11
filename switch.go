package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func branchExists(dir, branch, gitPath string) bool {
	cmd := exec.Command(gitPath, "-C", dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return cmd.Run() == nil
}

func handleSwitch(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var createShort, createLong string

	fs := flag.NewFlagSet("switch", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")
	fs.StringVar(&createLong, "create", "", "create branch if it does not exist")
	fs.StringVar(&createShort, "c", "", "create branch if it does not exist (short)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	createBranchName := createLong
	if createShort != "" {
		createBranchName = createShort
	}

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

	var branchName string
	var create bool

	if createBranchName != "" {
		if len(fs.Args()) > 0 {
			fmt.Printf("Error: Unexpected argument: %s\n", fs.Args()[0])
			os.Exit(1)
		}
		branchName = createBranchName
		create = true
	} else {
		// If create flag not set, look for positional argument
		if len(fs.Args()) == 0 {
			fmt.Println("Error: branch name required")
			os.Exit(1)
		} else if len(fs.Args()) > 1 {
			fmt.Printf("Error: Too many arguments: %v\n", fs.Args())
			os.Exit(1)
		}
		branchName = fs.Args()[0]
		create = false
	}

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

		exists := branchExists(dir, branchName, opts.GitPath)
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
			cmd := exec.Command(opts.GitPath, "-C", dir, "checkout", branchName)
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
				cmd := exec.Command(opts.GitPath, "-C", dir, "checkout", branchName)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					fmt.Printf("Error switching branch for %s: %v\n", dir, err)
					os.Exit(1)
				}
			} else {
				fmt.Printf("Creating and switching to branch %s in %s...\n", branchName, dir)
				cmd := exec.Command(opts.GitPath, "-C", dir, "checkout", "-b", branchName)
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
