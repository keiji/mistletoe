package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/olekukonko/tablewriter"
)

func handleStatus(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var pVal, pValShort int

	fs := flag.NewFlagSet("status", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")
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
	for _, repo := range config.Repositories {
		targetDir := getRepoDir(repo)
		info, err := os.Stat(targetDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			fail("Error checking directory %s: %v\n", targetDir, err)
		}
		if !info.IsDir() {
			fail("Error: target %s exists and is not a directory\n", targetDir)
		}

		// Check if Git repository
		gitDir := filepath.Join(targetDir, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			fail("Error: directory %s exists but is not a git repository\n", targetDir)
		}

		// Check remote origin
		cmd := exec.Command(opts.GitPath, "-C", targetDir, "config", "--get", "remote.origin.url")
		out, err := cmd.Output()
		if err != nil {
			fail("Error: directory %s is a git repo but failed to get remote origin: %v\n", targetDir, err)
		}
		currentURL := strings.TrimSpace(string(out))
		if currentURL != repo.URL {
			fail("Error: directory %s exists with different remote origin: %s (expected %s)\n", targetDir, currentURL, repo.URL)
		}
	}

	// Output Phase
	type RowData struct {
		Repo           string
		ConfigRef      string
		LocalBranchRev string
		RemoteRev      string
		Status         string
		Color          []int
	}
	var rows []RowData
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)

	for _, repo := range config.Repositories {
		wg.Add(1)
		go func(repo Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			targetDir := getRepoDir(repo)
			repoName := targetDir
			if repo.ID != nil && *repo.ID != "" {
				repoName = *repo.ID
			}

			if _, err := os.Stat(targetDir); os.IsNotExist(err) {
				return
			}

			// 1. Get Branch Name (abbrev-ref)
			cmd := exec.Command(opts.GitPath, "-C", targetDir, "rev-parse", "--abbrev-ref", "HEAD")
			out, err := cmd.Output()
			branchName := ""
			isDetached := false
			if err == nil {
				branchName = strings.TrimSpace(string(out))
			}
			if branchName == "HEAD" {
				isDetached = true
			}

			// 2. Get Short SHA
			cmd = exec.Command(opts.GitPath, "-C", targetDir, "rev-parse", "--short", "HEAD")
			out, err = cmd.Output()
			shortSHA := ""
			if err == nil {
				shortSHA = strings.TrimSpace(string(out))
			}

			// 3. Construct LocalBranchRev
			localBranchRev := ""
			if branchName != "" && shortSHA != "" {
				localBranchRev = fmt.Sprintf("%s/%s", branchName, shortSHA)
			} else if shortSHA != "" {
				localBranchRev = shortSHA
			}

			// 4. ConfigRef
			configRef := ""
			if repo.Branch != nil && *repo.Branch != "" {
				configRef = *repo.Branch
			}
			if repo.Revision != nil && *repo.Revision != "" {
				if configRef != "" {
					configRef += "/" + *repo.Revision
				} else {
					configRef = *repo.Revision
				}
			}

			// Local HEAD Rev (Full SHA for check)
			cmd = exec.Command(opts.GitPath, "-C", targetDir, "rev-parse", "HEAD")
			out, err = cmd.Output()
			localHeadFull := ""
			if err == nil {
				localHeadFull = strings.TrimSpace(string(out))
			}

			// Remote Rev
			remoteHeadFull := ""
			remoteHeadDisplay := ""
			if !isDetached {
				// Check if remote branch exists
				// git ls-remote origin <branchName>
				// branchName is safe because we checked it's not empty/HEAD in !isDetached (mostly)
				// well, branchName could be empty if err != nil above, but usually fine.

				cmd = exec.Command(opts.GitPath, "-C", targetDir, "ls-remote", "origin", "refs/heads/"+branchName)
				out, err = cmd.Output()
				if err == nil {
					output := strings.TrimSpace(string(out))
					if output != "" {
						// Output format: <SHA>\trefs/heads/<branch>
						fields := strings.Fields(output)
						if len(fields) > 0 {
							remoteHeadFull = fields[0]
							if len(remoteHeadFull) >= 7 {
								remoteHeadDisplay = remoteHeadFull[:7]
							} else {
								remoteHeadDisplay = remoteHeadFull
							}
						}
					}
				}
			}

			// Status (Unpushed?)
			statusVal := "-"
			hasUnpushed := false

			if remoteHeadFull != "" && localHeadFull != "" {
				if remoteHeadFull != localHeadFull {
					// Check if unpushed
					cmd = exec.Command(opts.GitPath, "-C", targetDir, "rev-list", "--count", remoteHeadFull+".."+localHeadFull)
					out, err := cmd.Output()
					if err == nil {
						count := strings.TrimSpace(string(out))
						if count != "0" {
							hasUnpushed = true
						}
					}
				}
			} else if !isDetached && remoteHeadFull == "" {
				// Remote branch doesn't exist? Means all local commits are unpushed
				hasUnpushed = true
			}

			var color []int
			if hasUnpushed {
				statusVal = "s"
				color = []int{tablewriter.FgYellowColor}
			}

			mu.Lock()
			rows = append(rows, RowData{
				Repo:           repoName,
				ConfigRef:      configRef,
				LocalBranchRev: localBranchRev,
				RemoteRev:      remoteHeadDisplay,
				Status:         statusVal,
				Color:          color,
			})
			mu.Unlock()
		}(repo)
	}
	wg.Wait()

	stopSpinner()

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Repo < rows[j].Repo
	})

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Repository", "Branch/Rev", "Local Branch/Rev", "Remote Rev", "Status"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.SetColumnSeparator("|")
	table.SetRowSeparator("-")
	table.SetAutoFormatHeaders(false)
	table.SetAutoWrapText(false)

	for _, row := range rows {
		colors := []tablewriter.Colors{
			{}, {}, {}, {}, tablewriter.Colors(row.Color),
		}
		table.Rich([]string{row.Repo, row.ConfigRef, row.LocalBranchRev, row.RemoteRev, row.Status}, colors)
	}
	table.Render()
}
