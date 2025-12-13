package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

func handleSync(args []string, opts GlobalOptions) {
	var fShort, fLong string

	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	configFile, _, err := ResolveCommonValues(fLong, fShort, opts.ConfigFile, DefaultParallel, DefaultParallel)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	spinner := NewSpinner()

	fail := func(format string, a ...interface{}) {
		spinner.Stop()
		fmt.Printf(format, a...)
		os.Exit(1)
	}

	spinner.Start()

	// Validation Phase
	if err := ValidateRepositoriesIntegrity(config, opts.GitPath); err != nil {
		fail("%v\n", err)
	}

	// Status Phase
	// Using default parallel (1) as sync doesn't specify parallelism
	rows := CollectStatus(config, 1, opts.GitPath)

	spinner.Stop()

	// Analyze Status
	hasConflict := false
	needsPull := false

	for _, row := range rows {
		if row.HasConflict {
			hasConflict = true
		}
		if row.IsPullable {
			needsPull = true
		}
	}

	if hasConflict {
		RenderStatusTable(rows)
		fmt.Println("pullが必要なリポジトリがあるがコンフリクトしているので処理を中止する")
		os.Exit(1)
	}

	argsPull := []string{"pull"}

	if needsPull {
		fmt.Println("pullが必要なリポジトリがある。")
		fmt.Print("originのコミットをmergeするか、rebaseするか、処理を中止（abort）するか？ [merge/rebase/abort]: ")

		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			input := strings.ToLower(strings.TrimSpace(scanner.Text()))
			switch input {
			case "merge", "m":
				// Default argsPull is good
			case "rebase", "r":
				argsPull = append(argsPull, "--rebase")
			case "abort", "a", "q":
				fmt.Println("中止します。")
				os.Exit(0)
			default:
				fmt.Println("不明な入力です。中止します。")
				os.Exit(1)
			}
		} else {
			// EOF or error
			os.Exit(1)
		}
	}

	// Execute Pull
	for _, row := range rows {
		fmt.Printf("Syncing %s...\n", row.Repo)
		if err := RunGitInteractive(row.RepoDir, opts.GitPath, argsPull...); err != nil {
			fmt.Printf("Error pulling %s: %v\n", row.Repo, err)
			os.Exit(1) // Abort on error as per "Sequentially pull" typical strict behavior or "abort" logic
		}
	}
}
