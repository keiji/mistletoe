package app

import (
	conf "mistletoe/internal/config"
)

import (
	"flag"
	"fmt"
	"os"
)

// handlePrStatus handles 'pr status'.
func handlePrStatus(args []string, opts GlobalOptions) {
	fs := flag.NewFlagSet("pr status", flag.ExitOnError)
	var (
		fLong     string
		fShort    string
		jVal      int
		jValShort int
		vLong     bool
		vShort    bool
		yes       bool
		yesShort  bool
	)

	fs.StringVar(&fLong, "file", DefaultConfigFile, "Configuration file path")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "Configuration file path (shorthand)")
	fs.IntVar(&jVal, "jobs", -1, "Number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "Number of concurrent jobs (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")
	fs.BoolVar(&yes, "yes", false, "Automatically answer 'yes' to all prompts")
	fs.BoolVar(&yesShort, "y", false, "Automatically answer 'yes' to all prompts (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := CheckFlagDuplicates(fs, [][2]string{
		{"file", "f"},
		{"jobs", "j"},
		{"verbose", "v"},
		{"yes", "y"},
	}); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	// Resolve common values
	configPath, jobsFlag, configData, err := ResolveCommonValues(fLong, fShort, jVal, jValShort, ignoreStdin)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	yesFlag := yes || yesShort
	configPath, err = SearchParentConfig(configPath, configData, opts.GitPath, yesFlag)
	if err != nil {
		fmt.Fprintf(Stderr, "Error searching parent config: %v\n", err)
	}

	// Verbose Override (Forward declaration needed)
	verbose := vLong || vShort

	// 1. Check gh availability
	if err := checkGhAvailability(opts.GhPath, verbose); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 2. Load conf.Config
	var config *conf.Config
	if configPath != "" {
		config, err = conf.LoadConfigFile(configPath)
	} else {
		config, err = conf.LoadConfigData(configData)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Resolve Jobs
	jobs, err := DetermineJobs(jobsFlag, config)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Verbose Override
	if verbose && jobs > 1 {
		fmt.Println("Verbose is specified, so jobs is treated as 1.")
		jobs = 1
	}

	// 3. Validate Integrity
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath, verbose); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Initialize Spinner
	spinner := NewSpinner(verbose)
	spinner.Start()

	// 4. Collect Status
	rows := CollectStatus(config, jobs, opts.GitPath, verbose, false)

	// 5. Collect PR Status
	prRows := CollectPrStatus(rows, config, jobs, opts.GhPath, verbose, nil)

	spinner.Stop()

	// 6. Render
	RenderPrStatusTable(Stdout, prRows)
}
