package app

import (
	"flag"
	"fmt"
	"os"
	"sync"
)

func branchExists(dir, branch, gitPath string, verbose bool) bool {
	_, err := RunGit(dir, gitPath, verbose, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

func handleSwitch(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var createShort, createLong string
	var pVal, pValShort int
	var vLong, vShort bool

	fs := flag.NewFlagSet("switch", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")
	fs.StringVar(&createLong, "create", "", "create branch if it does not exist")
	fs.StringVar(&createShort, "c", "", "create branch if it does not exist (short)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "number of parallel processes (short)")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	configFile, parallel, configData, err := ResolveCommonValues(fLong, fShort, pVal, pValShort)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	verbose := vLong || vShort

	createBranchName := createLong
	if createShort != "" {
		createBranchName = createShort
	}

	var config *Config
	if configFile != "" {
		config, err = loadConfigFile(configFile)
	} else {
		config, err = loadConfigData(configData)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var branchName string
	var create bool

	if createBranchName != "" {
		if len(fs.Args()) > 0 {
			fmt.Printf("Error: Unexpected argument: %s.\n", fs.Args()[0])
			os.Exit(1)
		}
		branchName = createBranchName
		create = true
	} else {
		// If create flag not set, look for positional argument
		if len(fs.Args()) == 0 {
			fmt.Println("Error: Branch name required.")
			os.Exit(1)
		} else if len(fs.Args()) > 1 {
			fmt.Printf("Error: Too many arguments: %v.\n", fs.Args())
			os.Exit(1)
		}
		branchName = fs.Args()[0]
		create = false
	}

	// Validate Integrity (Moved after argument parsing)
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath, verbose); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Map to store existence status for each repo (keyed by local directory path)
	dirExists := make(map[string]bool)
	var mu sync.Mutex

	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)

	// Pre-check phase
	for _, repo := range *config.Repositories {
		wg.Add(1)
		go func(repo Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			dir := GetRepoDir(repo)

			// Check if directory exists
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				fmt.Printf("Error: Repository directory %s does not exist.\n", dir)
				os.Exit(1)
			}

			exists := branchExists(dir, branchName, opts.GitPath, verbose)
			mu.Lock()
			dirExists[dir] = exists
			mu.Unlock()
		}(repo)
	}
	wg.Wait()

	if !create {
		// Strict mode: All must exist
		var missing []string
		for _, repo := range *config.Repositories {
			dir := GetRepoDir(repo)
			if !dirExists[dir] {
				missing = append(missing, *repo.URL+" ("+dir+")")
			}
		}

		if len(missing) > 0 {
			fmt.Printf("Error: Branch '%s' missing in repositories:\n", branchName)
			for _, item := range missing {
				fmt.Println(" - " + item)
			}
			os.Exit(1)
		}

		// Execute Checkout
		for _, repo := range *config.Repositories {
			wg.Add(1)
			go func(repo Repository) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				dir := GetRepoDir(repo)
				fmt.Printf("Switching %s to branch %s...\n", dir, branchName)
				if err := RunGitInteractive(dir, opts.GitPath, verbose, "checkout", branchName); err != nil {
					fmt.Printf("Error switching branch for %s: %v.\n", dir, err)
					os.Exit(1)
				}
			}(repo)
		}
		wg.Wait()
	} else {
		// Create mode
		for _, repo := range *config.Repositories {
			wg.Add(1)
			go func(repo Repository) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				dir := GetRepoDir(repo)
				mu.Lock()
				exists := dirExists[dir]
				mu.Unlock()

				if exists {
					fmt.Printf("Branch %s exists in %s. Switching...\n", branchName, dir)
					if err := RunGitInteractive(dir, opts.GitPath, verbose, "checkout", branchName); err != nil {
						fmt.Printf("Error switching branch for %s: %v.\n", dir, err)
						os.Exit(1)
					}
				} else {
					fmt.Printf("Creating and switching to branch %s in %s...\n", branchName, dir)
					if err := RunGitInteractive(dir, opts.GitPath, verbose, "checkout", "-b", branchName); err != nil {
						fmt.Printf("Error creating branch for %s: %v.\n", dir, err)
						os.Exit(1)
					}
				}
			}(repo)
		}
		wg.Wait()
	}
}
