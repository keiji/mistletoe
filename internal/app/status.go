package app

import (
	conf "mistletoe/internal/config"
)

import (
	"flag"
	"fmt"
)

func handleStatus(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var jVal, jValShort int
	var vLong, vShort bool

	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(Stderr)
	fs.StringVar(&fLong, "file", DefaultConfigFile, "configuration file")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "configuration file (shorthand)")
	fs.IntVar(&jVal, "jobs", -1, "number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "number of concurrent jobs (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Fprintln(Stderr, "Error parsing flags:", err)
		osExit(1)
		return
	}

	if err := CheckFlagDuplicates(fs, [][2]string{
		{"file", "f"},
		{"jobs", "j"},
		{"verbose", "v"},
	}); err != nil {
		fmt.Fprintln(Stderr, "Error:", err)
		osExit(1)
		return
	}

	configFile, jobs, configData, err := ResolveCommonValues(fLong, fShort, jVal, jValShort, ignoreStdin)
	if err != nil {
		fmt.Fprintf(Stderr, "Error: %v\n", err)
		osExit(1)
		return
	}

	var config *conf.Config
	if configFile != "" {
		config, err = conf.LoadConfigFile(configFile)
	} else {
		config, err = conf.LoadConfigData(configData)
	}

	if err != nil {
		fmt.Fprintln(Stderr, err)
		osExit(1)
		return
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
		fmt.Fprintln(Stdout, "Verbose is specified, so jobs is treated as 1.")
		jobs = 1
	}

	// Final Validation
	if jobs < MinJobs {
		fmt.Fprintf(Stderr, "Error: Jobs must be at least %d.\n", MinJobs)
		osExit(1)
		return
	}
	if jobs > MaxJobs {
		fmt.Fprintf(Stderr, "Error: Jobs must be at most %d.\n", MaxJobs)
		osExit(1)
		return
	}

	spinner := NewSpinner(verbose)

	fail := func(format string, a ...interface{}) {
		spinner.Stop()
		fmt.Fprintf(Stderr, format, a...)
		osExit(1)
	}

	spinner.Start()

	// Validation Phase
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath, verbose); err != nil {
		fail("%v\n", err)
		return
	}

	// Output Phase
	rows := CollectStatus(config, jobs, opts.GitPath, verbose, false)

	spinner.Stop()

	RenderStatusTable(Stdout, rows)
}
