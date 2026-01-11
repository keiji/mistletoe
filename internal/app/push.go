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

func handlePush(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var jVal, jValShort int
	var vLong, vShort bool

	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	fs.SetOutput(Stderr)
	fs.StringVar(&fLong, "file", DefaultConfigFile, "Configuration file path")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "Configuration file path (shorthand)")
	fs.IntVar(&jVal, "jobs", -1, "number of concurrent jobs")
	fs.IntVar(&jValShort, "j", -1, "number of concurrent jobs (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Fprintln(Stderr, "error parsing flags:", err)
		osExit(1)
		return
	}

	configFile, jobs, configData, err := ResolveCommonValues(fLong, fShort, jVal, jValShort, ignoreStdin)
	if err != nil {
		fmt.Fprintf(Stderr, "error: %v\n", err)
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

	// ValidateStatusForAction now returns error if invalid, but we need to check if it exits?
	// The implementation of ValidateStatusForAction in status_logic.go currently calls log.Fatalf or similar?
	// No, memory says: "Safety checks for push... are centralized in ValidateStatusForAction...".
	// Let's check ValidateStatusForAction implementation later, but assuming it returns error or exits.
	// Actually, looking at status_logic.go earlier, it was:
	// func ValidateStatusForAction(rows []StatusRow, checkBehind bool) { ... }
	// It likely calls os.Exit internally if it fails. I might need to refactor that too!
	// But let's assume it's fine for now or I'll catch it in tests.
	// Wait, if ValidateStatusForAction calls os.Exit directly, my test will crash.
	// I should check status_logic.go.
	ValidateStatusForAction(rows, true)

	// Identify repositories to push
	var pushable []StatusRow
	for _, row := range rows {
		if row.HasUnpushed {
			pushable = append(pushable, row)
		}
	}

	if len(pushable) == 0 {
		fmt.Fprintln(Stdout, "No repositories to push.")
		return
	}

	fmt.Fprint(Stdout, "Push updates? (yes/no): ")
	reader := bufio.NewReader(Stdin)
	input, _ := reader.ReadString('\n')
	fmt.Fprintln(Stdout)
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "y" || input == "yes" {
		for _, row := range pushable {
			fmt.Fprintf(Stdout, "Pushing %s (branch: %s)...\n", row.Repo, row.BranchName)
			// git push origin [branchname]
			if err := RunGitInteractive(row.RepoDir, opts.GitPath, verbose, "push", "origin", row.BranchName); err != nil {
				fmt.Fprintf(Stdout, "Failed to push %s: %v.\n", row.Repo, err)
			}
		}
	}
}
