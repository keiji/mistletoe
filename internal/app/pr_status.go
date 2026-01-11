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
		pVal      int
		pValShort int
		vLong     bool
		vShort    bool
	)

	fs.StringVar(&fLong, "file", DefaultConfigFile, "Configuration file path")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "Configuration file path (shorthand)")
	fs.IntVar(&pVal, "parallel", -1, "Number of parallel processes")
	fs.IntVar(&pValShort, "p", -1, "Number of parallel processes (shorthand)")
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

	// Resolve Parallel (Config fallback)
	if parallel == -1 {
		if config.Parallel != nil {
			parallel = *config.Parallel
		} else {
			parallel = DefaultParallel
		}
	}

	// Verbose Override
	if verbose && parallel > 1 {
		fmt.Println("Verbose is specified, so parallel is treated as 1.")
		parallel = 1
	}

	// Final Validation
	if parallel < MinParallel {
		fmt.Printf("Error: Parallel must be at least %d.\n", MinParallel)
		os.Exit(1)
	}
	if parallel > MaxParallel {
		fmt.Printf("Error: Parallel must be at most %d.\n", MaxParallel)
		os.Exit(1)
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
	rows := CollectStatus(config, parallel, opts.GitPath, verbose, false)

	// 5. Collect PR Status
	prRows := CollectPrStatus(rows, config, parallel, opts.GhPath, verbose, nil)

	spinner.Stop()

	// 6. Render
	RenderPrStatusTable(Stdout, prRows)
}
