package app

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

func handleSync(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var pVal, pValShort int
	var vLong, vShort bool

	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "number of parallel processes (short)")
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	configFile, parallel, configData, err := ResolveCommonValues(fLong, fShort, pVal, pValShort)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	verbose := vLong || vShort

	var config *Config
	if configFile != "" {
		config, err = loadConfigFile(configFile)
	} else {
		config, err = loadConfigData(configData)
	}

	if err != nil {
		fmt.Println(err)
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

	// Status Phase
	rows := CollectStatus(config, parallel, opts.GitPath, verbose, false)

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
			fmt.Println("Updates available.")
			fmt.Print("Merge, rebase, or abort? [merge/rebase/abort]: ")

			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				input := strings.ToLower(strings.TrimSpace(scanner.Text()))
				switch input {
				case "merge", "m":
					argsPull = append(argsPull, "--no-rebase")
				case "rebase", "r":
					argsPull = append(argsPull, "--rebase")
				case "abort", "a", "q":
					fmt.Println("Aborted.")
					os.Exit(0)
				default:
					fmt.Println("Invalid input. Aborted.")
					os.Exit(1)
				}
			} else {
				// EOF or error
				os.Exit(1)
			}
		} else {
			fmt.Println("Updates available. Pulling...")
		}
	}

	// Execute Pull
	for _, row := range rows {
		if row.RemoteRev == "" {
			fmt.Printf("Skipping %s: Remote branch not found.\n", row.Repo)
			continue
		}

		fmt.Printf("Syncing %s...\n", row.Repo)
		if err := RunGitInteractive(row.RepoDir, opts.GitPath, verbose, argsPull...); err != nil {
			fmt.Printf("Error pulling %s: %v\n", row.Repo, err)
			os.Exit(1) // Abort on error as per "Sequentially pull" typical strict behavior or "abort" logic
		}
	}
}
