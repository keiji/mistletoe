package app

import (
	conf "mistletoe/internal/config"
)

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
	var jVal, jValShort int
	var vLong, vShort bool

	fs := flag.NewFlagSet("switch", flag.ExitOnError)
	fs.StringVar(&fLong, "file", DefaultConfigFile, "configuration file")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "configuration file (shorthand)")
	fs.StringVar(&createLong, "create", "", "create branch if it does not exist")
	fs.StringVar(&createShort, "c", "", "create branch if it does not exist (shorthand)")
	fs.IntVar(&jVal, "jobs", -1, "number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "number of concurrent jobs (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	configFile, jobs, configData, err := ResolveCommonValues(fLong, fShort, jVal, jValShort, ignoreStdin)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	createBranchName := createLong
	if createShort != "" {
		createBranchName = createShort
	}

	var config *conf.Config
	if configFile != "" {
		config, err = conf.LoadConfigFile(configFile)
	} else {
		config, err = conf.LoadConfigData(configData)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Resolve Jobs (Config fallback)
	if jobs == -1 {
		if config.Jobs != nil {
			jobs = *config.Jobs
		} else {
			jobs = DefaultJobs
		}
	}

	// Verbose Override
	verbose := vLong || vShort
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
	sem := make(chan struct{}, jobs)

	// Pre-check phase
	for _, repo := range *config.Repositories {
		wg.Add(1)
		go func(repo conf.Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			dir := config.GetRepoPath(repo)

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
			dir := config.GetRepoPath(repo)
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
			go func(repo conf.Repository) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				dir := config.GetRepoPath(repo)
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
			go func(repo conf.Repository) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				dir := config.GetRepoPath(repo)
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
