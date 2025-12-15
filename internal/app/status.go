package app

import (
	"flag"
	"fmt"
	"os"
)

func handleStatus(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var pVal, pValShort int

	fs := flag.NewFlagSet("status", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "number of parallel processes (short)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	configFile, parallel, configData, err := ResolveCommonValues(fLong, fShort, pVal, pValShort)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	config, err := loadConfig(configFile, configData)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	spinner := NewSpinner()

	fail := func(format string, a ...interface{}) {
		spinner.Stop()
		fmt.Printf(format, a...)
		os.Exit(1)
	}

	spinner.Start()

	// Validation Phase
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath); err != nil {
		fail("%v\n", err)
	}

	// Output Phase
	rows := CollectStatus(config, parallel, opts.GitPath)

	spinner.Stop()

	RenderStatusTable(rows)
}
