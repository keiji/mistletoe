package app

import (
	conf "mistletoe/internal/config"
	"mistletoe/internal/sys"
	"mistletoe/internal/ui"
)

import (
	"bufio"
	"flag"
	"fmt"
)

func handlePush(args []string, opts GlobalOptions) error {
	var (
		fShort, fLong string
		jVal, jValShort int
		vLong, vShort bool
		yes, yesShort bool
	)

	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	fs.SetOutput(sys.Stderr)
	fs.StringVar(&fLong, "file", DefaultConfigFile, "Configuration file path")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "Configuration file path (shorthand)")
	fs.IntVar(&jVal, "jobs", -1, "number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "number of concurrent jobs (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")
	fs.BoolVar(&yes, "yes", false, "Automatically answer 'yes' to all prompts")
	fs.BoolVar(&yesShort, "y", false, "Automatically answer 'yes' to all prompts (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		return fmt.Errorf("error parsing flags: %w", err)
	}

	if len(fs.Args()) > 0 {
		return fmt.Errorf("Error: push command does not accept positional arguments")
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
		return fmt.Errorf("error: %w", err)
	}

	yesFlag := yes || yesShort
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

	// Validate status
	if err := ValidateStatusForAction(rows, true); err != nil {
		return err
	}

	// Identify repositories to push
	var pushable []StatusRow
	for _, row := range rows {
		if row.HasUnpushed {
			pushable = append(pushable, row)
		}
	}

	if len(pushable) == 0 {
		fmt.Fprintln(sys.Stdout, "No repositories to push.")
		return nil
	}

	reader := bufio.NewReader(sys.Stdin)
	confirmed, err := ui.AskForConfirmation(reader, "Push updates? (yes/no): ", yesFlag)
	if err != nil {
		return fmt.Errorf("Error reading input: %w", err)
	}
	if confirmed {
		for _, row := range pushable {
			fmt.Fprintf(sys.Stdout, "Pushing %s (branch: %s)...\n", row.Repo, row.BranchName)
			// git push -u origin [branchname]
			if err := RunGitInteractive(row.RepoDir, opts.GitPath, verbose, "push", "-u", "origin", row.BranchName); err != nil {
				fmt.Fprintf(sys.Stdout, "Failed to push %s: %v.\n", row.Repo, err)
			}
		}
	}
	return nil
}
