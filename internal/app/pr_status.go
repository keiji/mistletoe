package app

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
	fs.IntVar(&pVal, "parallel", DefaultParallel, "Number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "Number of parallel processes (shorthand)")
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

	if verbose {
		parallel = 1
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
	RenderPrStatusTable(prRows)
}
