package app

import (
	conf "mistletoe/internal/config"
	"mistletoe/internal/sys"
	"mistletoe/internal/ui"
)

import (
	"flag"
	"fmt"
)

func handleStatus(args []string, opts GlobalOptions) error {
	var (
		fShort, fLong   string
		jVal, jValShort int
		vLong, vShort   bool
		yes, yesShort   bool
	)

	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(sys.Stderr)
	fs.StringVar(&fLong, "file", DefaultConfigFile, "configuration file")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "configuration file (shorthand)")
	fs.IntVar(&jVal, "jobs", -1, "number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "number of concurrent jobs (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")
	fs.BoolVar(&yes, "yes", false, "Automatically answer 'yes' to all prompts")
	fs.BoolVar(&yesShort, "y", false, "Automatically answer 'yes' to all prompts (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		return fmt.Errorf("Error parsing flags: %w", err)
	}

	if len(fs.Args()) > 0 {
		return fmt.Errorf("Error: status command does not accept positional arguments")
	}

	if err := CheckFlagDuplicates(fs, [][2]string{
		{"file", "f"},
		{"jobs", "j"},
		{"verbose", "v"},
		{"yes", "y"},
	}); err != nil {
		return fmt.Errorf("Error: %w", err)
	}

	configFile, jobsFlag, configData, err := ResolveCommonValues(fLong, fShort, jVal, jValShort, ignoreStdin)
	if err != nil {
		return fmt.Errorf("Error: %w", err)
	}

	configFile, err = SearchParentConfig(configFile, configData, opts.GitPath)
	if err != nil {
		fmt.Fprintf(sys.Stderr, "Error searching parent config: %v\n", err)
	}

	var config *conf.Config
	if configFile != "" {
		config, err = conf.LoadConfigFile(configFile)
	} else {
		config, err = conf.LoadConfigData(configData)
	}

	if err != nil {
		return err
	}

	// Resolve Jobs
	jobs, err := DetermineJobs(jobsFlag, config)
	if err != nil {
		return fmt.Errorf("Error: %w", err)
	}

	// Verbose Override
	verbose := vLong || vShort
	if verbose && jobs > 1 {
		fmt.Fprintln(sys.Stdout, "Verbose is specified, so jobs is treated as 1.")
		jobs = 1
	}

	spinner := ui.NewSpinner(verbose)
	spinner.Start()

	// Validation Phase
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath, verbose); err != nil {
		spinner.Stop()
		return err
	}

	// Output Phase
	rows := CollectStatus(config, jobs, opts.GitPath, verbose, false)

	spinner.Stop()

	RenderStatusTable(sys.Stdout, rows)
	return nil
}
