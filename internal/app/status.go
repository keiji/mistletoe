package app

import (
	"flag"
	"fmt"
	"os"
)

func handleStatus(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var pVal, pValShort int
	var vLong, vShort bool

	fs := flag.NewFlagSet("status", flag.ExitOnError)
	fs.StringVar(&fLong, "file", DefaultConfigFile, "configuration file")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "configuration file (shorthand)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "number of parallel processes (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	configFile, parallel, configData, err := ResolveCommonValues(fLong, fShort, pVal, pValShort, ignoreStdin)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	verbose := vLong || vShort
	if verbose {
		parallel = 1
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

	spinner := NewSpinner(verbose)

	fail := func(format string, a ...interface{}) {
		spinner.Stop()
		fmt.Printf(format, a...)
		os.Exit(1)
	}

	spinner.Start()

	// Validation Phase
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath, verbose); err != nil {
		fail("%v\n", err)
	}

	// Output Phase
	rows := CollectStatus(config, parallel, opts.GitPath, verbose, false)

	spinner.Stop()

	RenderStatusTable(rows)
}
