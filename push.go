package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func handlePush(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var pVal, pValShort int
	var lLong, lShort string

	fs := flag.NewFlagSet("push", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")
	fs.StringVar(&lLong, "labels", "", "comma-separated list of labels to filter repositories")
	fs.StringVar(&lShort, "l", "", "labels (short)")
	fs.IntVar(&pVal, "parallel", DefaultParallel, "number of parallel processes")
	fs.IntVar(&pValShort, "p", DefaultParallel, "number of parallel processes (short)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	parallel := DefaultParallel
	if pVal != DefaultParallel {
		parallel = pVal
	} else if pValShort != DefaultParallel {
		parallel = pValShort
	}

	if parallel < MinParallel {
		fmt.Printf("Error: parallel must be at least %d\n", MinParallel)
		os.Exit(1)
	}
	if parallel > MaxParallel {
		fmt.Printf("Error: parallel must be at most %d\n", MaxParallel)
		os.Exit(1)
	}

	configFile := opts.ConfigFile
	if fLong != "" {
		configFile = fLong
	} else if fShort != "" {
		configFile = fShort
	}

	labels := lLong
	if lShort != "" {
		labels = lShort
	}

	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Spinner control
	spinnerStop := make(chan struct{})
	spinnerDone := make(chan struct{})

	startSpinner := func() {
		go func() {
			defer close(spinnerDone)
			chars := []string{"/", "-", "\\", "|"}
			i := 0
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-spinnerStop:
					fmt.Print("\r\033[K") // Clear line
					return
				case <-ticker.C:
					fmt.Printf("\rProcessing... %s", chars[i])
					i = (i + 1) % len(chars)
				}
			}
		}()
	}

	stopSpinner := func() {
		// Non-blocking send to stop
		select {
		case spinnerStop <- struct{}{}:
			<-spinnerDone
		default:
			// Already stopped or not started
		}
	}

	fail := func(format string, a ...interface{}) {
		stopSpinner()
		fmt.Printf(format, a...)
		os.Exit(1)
	}

	startSpinner()

	// Validation Phase
	if err := ValidateRepositories(*config.Repositories, opts.GitPath); err != nil {
		fail("%v\n", err)
	}

	// Filter Repositories
	repos := FilterRepositories(*config.Repositories, labels)

	// Output Phase
	rows := CollectStatus(repos, parallel, opts.GitPath)

	stopSpinner()

	RenderStatusTable(rows)

	// Identify repositories to push
	var pushable []StatusRow
	for _, row := range rows {
		if row.HasUnpushed {
			pushable = append(pushable, row)
		}
	}

	if len(pushable) == 0 {
		fmt.Println("There are no repositories to push.")
		return
	}

	fmt.Print("Do you want to push? (y/yes): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	fmt.Println()
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "y" || input == "yes" {
		for _, row := range pushable {
			fmt.Printf("Pushing %s (branch: %s)...\n", row.Repo, row.BranchName)
			// git push origin [branchname]
			cmd := exec.Command(opts.GitPath, "-C", row.RepoDir, "push", "origin", row.BranchName)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Printf("Failed to push %s: %v\n", row.Repo, err)
			}
		}
	}
}
