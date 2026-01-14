package app

import (
	conf "mistletoe/internal/config"
)

import (
	"bufio"
	"flag"
	"fmt"
	"strings"
)

func handleSync(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var jVal, jValShort int
	var vLong, vShort bool

	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(Stderr)
	fs.StringVar(&fLong, "file", DefaultConfigFile, "Configuration file path")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "Configuration file path (shorthand)")
	fs.IntVar(&jVal, "jobs", -1, "number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "number of concurrent jobs (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")
	var yes, yesShort bool
	fs.BoolVar(&yes, "yes", false, "Automatically answer 'yes' to all prompts")
	fs.BoolVar(&yesShort, "y", false, "Automatically answer 'yes' to all prompts (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Fprintln(Stderr, "Error parsing flags:", err)
		osExit(1)
		return
	}

	if err := CheckFlagDuplicates(fs, [][2]string{
		{"file", "f"},
		{"jobs", "j"},
		{"verbose", "v"},
		{"yes", "y"},
	}); err != nil {
		fmt.Fprintln(Stderr, "Error:", err)
		osExit(1)
		return
	}

	configFile, jobsFlag, configData, err := ResolveCommonValues(fLong, fShort, jVal, jValShort, ignoreStdin)
	if err != nil {
		fmt.Fprintf(Stderr, "Error: %v\n", err)
		osExit(1)
		return
	}

	yesFlag := yes || yesShort
	configFile, err = SearchParentConfig(configFile, configData, opts.GitPath, yesFlag)
	if err != nil {
		fmt.Fprintf(Stderr, "Error searching parent config: %v\n", err)
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

	// Resolve Jobs
	jobs, err := DetermineJobs(jobsFlag, config)
	if err != nil {
		fmt.Fprintf(Stderr, "Error: %v\n", err)
		osExit(1)
		return
	}

	// Verbose Override
	verbose := vLong || vShort
	if verbose && jobs > 1 {
		fmt.Fprintln(Stdout, "Verbose is specified, so jobs is treated as 1.")
		jobs = 1
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

	// Status Phase
	rows := CollectStatus(config, jobs, opts.GitPath, verbose, false)

	spinner.Stop()

	// Analyze Status
	needsPull := false
	needsStrategy := false

	for _, row := range rows {
		// Only consider pullable if there is no conflict
		if row.IsPullable {
			needsPull = true
			if row.HasUnpushed {
				needsStrategy = true
			}
		}
	}

	argsPull := []string{"pull"}

	if needsPull {
		if needsStrategy {
			fmt.Fprintln(Stdout, "Updates available.")

			if yesFlag {
				fmt.Fprintln(Stdout, "Using default strategy (merge) due to --yes flag.")
				argsPull = append(argsPull, "--no-rebase")
			} else {
				fmt.Fprint(Stdout, "Merge, rebase, or abort? [merge/rebase/abort]: ")
				scanner := bufio.NewScanner(Stdin)
				if scanner.Scan() {
					input := strings.ToLower(strings.TrimSpace(scanner.Text()))
					switch input {
					case "merge", "m":
						argsPull = append(argsPull, "--no-rebase")
					case "rebase", "r":
						argsPull = append(argsPull, "--rebase")
					case "abort", "a", "q":
						fmt.Fprintln(Stdout, "Aborted.")
						osExit(0)
						return
					default:
						fmt.Fprintln(Stdout, "Invalid input. Aborted.")
						osExit(1)
						return
					}
				} else {
					// EOF or error
					osExit(1)
					return
				}
			}
		} else {
			fmt.Fprintln(Stdout, "Updates available. Pulling...")
		}
	}

	// Execute Pull
	for _, row := range rows {
		if row.RemoteRev == "" {
			fmt.Fprintf(Stdout, "Skipping %s: Remote branch not found.\n", row.Repo)
			continue
		}

		fmt.Fprintf(Stdout, "Syncing %s...\n", row.Repo)
		if err := RunGitInteractive(row.RepoDir, opts.GitPath, verbose, argsPull...); err != nil {
			fmt.Fprintf(Stderr, "Error pulling %s: %v\n", row.Repo, err)
			osExit(1) // Abort on error as per "Sequentially pull" typical strict behavior or "abort" logic
			return
		}
	}
}
