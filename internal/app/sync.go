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
	"strings"
)

func handleSync(args []string, opts GlobalOptions) error {
	var (
		fShort, fLong string
		jVal, jValShort int
		vLong, vShort bool
		yes, yesShort bool
	)

	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
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
		return fmt.Errorf("Error parsing flags: %w", err)
	}

	if len(fs.Args()) > 0 {
		return fmt.Errorf("Error: sync command does not accept positional arguments")
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
			fmt.Fprintln(sys.Stdout, "Updates available.")

			if yesFlag {
				fmt.Fprintln(sys.Stdout, "Using default strategy (merge) due to --yes flag.")
				argsPull = append(argsPull, "--no-rebase")
			} else {
				fmt.Fprint(sys.Stdout, "Merge, rebase, or abort? [merge/rebase/abort]: ")
				scanner := bufio.NewScanner(sys.Stdin)
				if scanner.Scan() {
					input := strings.ToLower(strings.TrimSpace(scanner.Text()))
					switch input {
					case "merge", "m":
						argsPull = append(argsPull, "--no-rebase")
					case "rebase", "r":
						argsPull = append(argsPull, "--rebase")
					case "abort", "a", "q":
						fmt.Fprintln(sys.Stdout, "Aborted.")
						return nil
					default:
						return fmt.Errorf("Invalid input. Aborted.")
					}
				} else {
					// EOF or error
					return fmt.Errorf("Input error or EOF.")
				}
			}
		} else {
			fmt.Fprintln(sys.Stdout, "Updates available. Pulling...")
		}
	}

	// Execute Pull
	for _, row := range rows {
		if row.RemoteRev == "" {
			fmt.Fprintf(sys.Stdout, "[%s] Skipping: Remote branch not found.\n", row.Repo)
			continue
		}

		// Check if we can skip pulling.
		// We skip if:
		// 1. IsPullable is false (not strictly behind according to config)
		// 2. AND Local matches Remote (no difference)
		//
		// Note: If IsPullable is false but Local != Remote, it could mean:
		// - We are Ahead (Unpushed) -> Pull is no-op, but we let it run (or could optimize later).
		// - We are Behind but 'repo.Branch' config doesn't match current branch -> IsPullable is false (gated).
		//   In this case, we MUST run pull (which behaves like standard git pull).
		if !row.IsPullable && row.LocalHeadFull == row.RemoteHeadFull {
			fmt.Fprintf(sys.Stdout, "[%s] Skipping: Already up to date.\n", row.Repo)
			continue
		}

		fmt.Fprintf(sys.Stdout, "[%s] Syncing...\n", row.Repo)
		if err := RunGitInteractive(row.RepoDir, opts.GitPath, verbose, argsPull...); err != nil {
			return fmt.Errorf("[%s] Error pulling: %w", row.Repo, err)
		}
	}
	return nil
}
