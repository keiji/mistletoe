package app

import (
	conf "mistletoe/internal/config"
)

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

func handlePush(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var pVal, pValShort int
	var vLong, vShort bool

	fs := flag.NewFlagSet("push", flag.ExitOnError)
	fs.StringVar(&fLong, "file", DefaultConfigFile, "Configuration file path")
	fs.StringVar(&fShort, "f", DefaultConfigFile, "Configuration file path (shorthand)")
	fs.IntVar(&pVal, "parallel", -1, "number of parallel processes")
	fs.IntVar(&pValShort, "p", -1, "number of parallel processes (shorthand)")
	var ignoreStdin bool
	fs.BoolVar(&ignoreStdin, "ignore-stdin", false, "Ignore standard input")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("error parsing flags:", err)
		os.Exit(1)
	}

	configFile, parallel, configData, err := ResolveCommonValues(fLong, fShort, pVal, pValShort, ignoreStdin)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	var config *conf.Config
	if configFile != "" {
		config, err = conf.LoadConfigFile(configFile)
	} else {
		config, err = conf.LoadConfigData(configData)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Resolve Parallel (Config fallback)
	if parallel == -1 {
		if config.Parallel != nil {
			parallel = *config.Parallel
		} else {
			parallel = DefaultParallel
		}
	}

	// Verbose Override
	verbose := vLong || vShort
	if verbose && parallel > 1 {
		fmt.Println("Verbose is specified, so parallel is treated as 1.")
		parallel = 1
	}

	// Final Validation
	if parallel < MinParallel {
		fmt.Printf("Error: Parallel must be at least %d.\n", MinParallel)
		os.Exit(1)
	}
	if parallel > MaxParallel {
		fmt.Printf("Error: Parallel must be at most %d.\n", MaxParallel)
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

	RenderStatusTable(Stdout, rows)

	ValidateStatusForAction(rows, true)

	// Identify repositories to push
	var pushable []StatusRow
	for _, row := range rows {
		if row.HasUnpushed {
			pushable = append(pushable, row)
		}
	}

	if len(pushable) == 0 {
		fmt.Println("No repositories to push.")
		return
	}

	fmt.Print("Push updates? (yes/no): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	fmt.Println()
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "y" || input == "yes" {
		for _, row := range pushable {
			fmt.Printf("Pushing %s (branch: %s)...\n", row.Repo, row.BranchName)
			// git push origin [branchname]
			if err := RunGitInteractive(row.RepoDir, opts.GitPath, verbose, "push", "origin", row.BranchName); err != nil {
				fmt.Printf("Failed to push %s: %v.\n", row.Repo, err)
			}
		}
	}
}
