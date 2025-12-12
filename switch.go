package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

func branchExists(dir, branch, gitPath string) bool {
	cmd := exec.Command(gitPath, "-C", dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return cmd.Run() == nil
}

func handleSwitch(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var createShort, createLong string
	var pVal, pValShort int
	var lLong, lShort string

	fs := flag.NewFlagSet("switch", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")
	fs.StringVar(&createLong, "create", "", "create branch if it does not exist")
	fs.StringVar(&createShort, "c", "", "create branch if it does not exist (short)")
	fs.StringVar(&lLong, "labels", "", "comma-separated list of labels to filter repositories")
	fs.StringVar(&lShort, "l", "", "labels (short)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "number of parallel processes (short)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	parallel := DefaultParallel
	if pVal != DefaultParallel {
		parallel = pVal
	} else if pValShort != DefaultParallel {
		parallel = pValShort
	}

	if parallel < MinParallel {
		fmt.Printf("Error: parallel must be at least %d\n", MinParallel)
		os.Exit(1)
	}
	if parallel > MaxParallel {
		fmt.Printf("Error: parallel must be at most %d\n", MaxParallel)
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

	labels := lLong
	if lShort != "" {
		labels = lShort
	}

	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Validate integrity of all repositories first
	if err := ValidateRepositories(*config.Repositories, opts.GitPath); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Filter repositories
	repos := FilterRepositories(*config.Repositories, labels)

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
	var mu sync.Mutex

	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)

	// Pre-check phase
	for _, repo := range repos {
		wg.Add(1)
		go func(repo Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			dir := GetRepoDir(repo)

			// Check if directory exists
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				fmt.Printf("Error: repository directory %s does not exist\n", dir)
				os.Exit(1)
			}

			exists := branchExists(dir, branchName, opts.GitPath)
			mu.Lock()
			dirExists[dir] = exists
			mu.Unlock()
		}(repo)
	}
	wg.Wait()

	if !create {
		// Strict mode: All must exist
		var missing []string
		for _, repo := range repos {
			dir := GetRepoDir(repo)
			if !dirExists[dir] {
				missing = append(missing, *repo.URL+" ("+dir+")")
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
		for _, repo := range repos {
			wg.Add(1)
			go func(repo Repository) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				dir := GetRepoDir(repo)
				fmt.Printf("Switching %s to branch %s...\n", dir, branchName)
				cmd := exec.Command(opts.GitPath, "-C", dir, "checkout", branchName)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					fmt.Printf("Error switching branch for %s: %v\n", dir, err)
					os.Exit(1)
				}
			}(repo)
		}
		wg.Wait()
	} else {
		// Create mode
		for _, repo := range repos {
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
			}(repo)
		}
		wg.Wait()
	}
}
