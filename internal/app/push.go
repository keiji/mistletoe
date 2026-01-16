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

func handlePush(args []string, opts GlobalOptions) {
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
		fmt.Fprintln(sys.Stderr, "error parsing flags:", err)
		osExit(1)
		return
	}

	if err := CheckFlagDuplicates(fs, [][2]string{
		{"file", "f"},
		{"jobs", "j"},
		{"verbose", "v"},
		{"yes", "y"},
	}); err != nil {
		fmt.Fprintln(sys.Stderr, "Error:", err)
		osExit(1)
		return
	}

	configFile, jobsFlag, configData, err := ResolveCommonValues(fLong, fShort, jVal, jValShort, ignoreStdin)
	if err != nil {
		fmt.Fprintf(sys.Stderr, "error: %v\n", err)
		osExit(1)
		return
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
		fmt.Fprintln(sys.Stderr, err)
		osExit(1)
		return
	}

	// Resolve Jobs
	jobs, err := DetermineJobs(jobsFlag, config)
	if err != nil {
		fmt.Fprintf(sys.Stderr, "Error: %v\n", err)
		osExit(1)
		return
	}

	// Verbose Override
	verbose := vLong || vShort
	if verbose && jobs > 1 {
		fmt.Fprintln(sys.Stdout, "Verbose is specified, so jobs is treated as 1.")
		jobs = 1
	}

	spinner := ui.NewSpinner(verbose)

	fail := func(format string, a ...interface{}) {
		spinner.Stop()
		fmt.Fprintf(sys.Stderr, format, a...)
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

	RenderStatusTable(sys.Stdout, rows)

	// Validate status
	if err := ValidateStatusForAction(rows, true); err != nil {
		fail("%v\n", err)
		return
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
		return
	}

	reader := bufio.NewReader(sys.Stdin)
	confirmed, err := ui.AskForConfirmation(reader, "Push updates? (yes/no): ", yesFlag)
	if err != nil {
		fmt.Fprintf(sys.Stderr, "Error reading input: %v\n", err)
		osExit(1)
		return
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
}
