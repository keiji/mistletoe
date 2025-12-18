package app

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

	// Output Phase
	rows := CollectStatus(config, parallel, opts.GitPath, verbose)

	spinner.Stop()

	RenderStatusTable(rows)

	for _, row := range rows {
		if row.HasConflict {
			fail("Conflicts detected. Cannot push.\n")
		}
	}

	for _, row := range rows {
		if row.IsPullable {
			fail("Sync required.\n")
		}
	}

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
