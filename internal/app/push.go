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

	if err := CheckFlagDuplicates(fs, [][2]string{
		{"file", "f"},
		{"jobs", "j"},
		{"verbose", "v"},
	}); err != nil {
		fmt.Fprintln(Stderr, "Error:", err)
		osExit(1)
		return
	}

	configFile, jobsFlag, configData, err := ResolveCommonValues(fLong, fShort, jVal, jValShort, ignoreStdin)
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

	// Output Phase
	rows := CollectStatus(config, jobs, opts.GitPath, verbose, false)

	spinner.Stop()

	RenderStatusTable(Stdout, rows)

	// Validate status
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
			// git push -u origin [branchname]
			if err := RunGitInteractive(row.RepoDir, opts.GitPath, verbose, "push", "-u", "origin", row.BranchName); err != nil {
				fmt.Fprintf(Stdout, "Failed to push %s: %v.\n", row.Repo, err)
			}
		}
	}
}
