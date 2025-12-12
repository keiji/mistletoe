package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

const (
	ColorNone   = 0
	ColorYellow = 1
	ColorGreen  = 2
)

// StatusRow represents the status of a single repository.
type StatusRow struct {
	Repo           string
	ConfigRef      string
	LocalBranchRev string
	RemoteRev      string
	Status         string
	Color          int
	BranchName     string
	HasUnpushed    bool
	RepoDir        string
}

// ValidateRepositoriesIntegrity checks if repositories exist and are valid.
func ValidateRepositoriesIntegrity(config *Config, gitPath string) error {
	for _, repo := range *config.Repositories {
		targetDir := getRepoDir(repo)
		info, err := os.Stat(targetDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("Error checking directory %s: %v", targetDir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("Error: target %s exists and is not a directory", targetDir)
		}

		// Check if Git repository
		gitDir := filepath.Join(targetDir, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			return fmt.Errorf("Error: directory %s exists but is not a git repository", targetDir)
		}

		// Check remote origin
		cmd := exec.Command(gitPath, "-C", targetDir, "config", "--get", "remote.origin.url")
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("Error: directory %s is a git repo but failed to get remote origin: %v", targetDir, err)
		}
		currentURL := strings.TrimSpace(string(out))
		if currentURL != *repo.URL {
			return fmt.Errorf("Error: directory %s exists with different remote origin: %s (expected %s)", targetDir, currentURL, *repo.URL)
		}
	}
	return nil
}

// CollectStatus collects status for all repositories.
func CollectStatus(config *Config, parallel int, gitPath string) []StatusRow {
	var rows []StatusRow
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)

	for _, repo := range *config.Repositories {
		wg.Add(1)
		go func(repo Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			row := getRepoStatus(repo, gitPath)
			if row != nil {
				mu.Lock()
				rows = append(rows, *row)
				mu.Unlock()
			}
		}(repo)
	}
	wg.Wait()

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Repo < rows[j].Repo
	})

	return rows
}

func getRepoStatus(repo Repository, gitPath string) *StatusRow {
	targetDir := getRepoDir(repo)
	repoName := targetDir
	if repo.ID != nil && *repo.ID != "" {
		repoName = *repo.ID
	}

	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return nil
	}

	// 1. Get Branch Name (abbrev-ref)
	cmd := exec.Command(gitPath, "-C", targetDir, "rev-parse", "--abbrev-ref", "HEAD")
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
	cmd = exec.Command(gitPath, "-C", targetDir, "rev-parse", "--short", "HEAD")
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
	cmd = exec.Command(gitPath, "-C", targetDir, "rev-parse", "HEAD")
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
		cmd = exec.Command(gitPath, "-C", targetDir, "ls-remote", "origin", "refs/heads/"+branchName)
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

	// Status Logic
	hasUnpushed := false
	isPullable := false

	if remoteHeadFull != "" && localHeadFull != "" {
		// Check Unpushed (Ahead)
		// git rev-list --count remote..local
		if remoteHeadFull != localHeadFull {
			cmd = exec.Command(gitPath, "-C", targetDir, "rev-list", "--count", remoteHeadFull+".."+localHeadFull)
			out, err := cmd.Output()
			if err == nil {
				count := strings.TrimSpace(string(out))
				if count != "0" {
					hasUnpushed = true
				}
			}
		}

		// Check Pullable (Behind)
		// Only if current branch matches config branch
		if repo.Branch != nil && *repo.Branch != "" && *repo.Branch == branchName {
			if remoteHeadFull != localHeadFull {
				// Check if we have the remote object locally
				cmdObj := exec.Command(gitPath, "-C", targetDir, "cat-file", "-e", remoteHeadFull)
				if err := cmdObj.Run(); err != nil {
					// Object missing locally, so it's likely a new commit on remote (we haven't fetched)
					isPullable = true
				} else {
					// Object exists locally, check ancestry
					// git rev-list --count local..remote
					cmd = exec.Command(gitPath, "-C", targetDir, "rev-list", "--count", localHeadFull+".."+remoteHeadFull)
					out, err := cmd.Output()
					if err == nil {
						count := strings.TrimSpace(string(out))
						if count != "0" {
							isPullable = true
						}
					}
				}
			}
		}

	} else if !isDetached && remoteHeadFull == "" {
		// Remote branch doesn't exist? Means all local commits are unpushed
		hasUnpushed = true
	}

	statusVal := ""
	var color = ColorNone

	if hasUnpushed {
		statusVal += "*"
		color = ColorYellow
	}

	if isPullable {
		statusVal += "+"
		if color == ColorNone {
			color = ColorGreen
		}
	}

	if statusVal == "" {
		statusVal = "-"
	}

	return &StatusRow{
		Repo:           repoName,
		ConfigRef:      configRef,
		LocalBranchRev: localBranchRev,
		RemoteRev:      remoteHeadDisplay,
		Status:         statusVal,
		Color:          color,
		BranchName:     branchName,
		HasUnpushed:    hasUnpushed,
		RepoDir:        targetDir,
	}
}

func RenderStatusTable(rows []StatusRow) {
	// Replicating v0.0.5 style
	// SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	// SetCenterSeparator("|")
	// SetColumnSeparator("|")
	// SetRowSeparator("-")
	// SetAutoFormatHeaders(false)
	// SetAutoWrapText(false)

	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeaderAutoFormat(tw.Off),
		tablewriter.WithRowAutoWrap(tw.WrapNone), // Using WrapNone to mimic false
		tablewriter.WithRendition(tw.Rendition{
			Borders: tw.Border{Left: tw.On, Top: tw.Off, Right: tw.On, Bottom: tw.Off},
			Settings: tw.Settings{
				Separators: tw.Separators{BetweenColumns: tw.On, BetweenRows: tw.Off},
			},
			Symbols: tw.NewSymbolCustom("v0.0.5-like").
				WithColumn("|").
				WithRow("-").
				WithCenter("|").
				WithHeaderMid("-").
				WithTopMid("-").     // If top border was on, but it is off
				WithBottomMid("-"), // If bottom border was on, but it is off
		}),
	)
	table.Header("Repository", "Branch/Rev", "Local Branch/Rev", "Remote Rev", "Status")

	const (
		Reset   = "\033[0m"
		FgGreen = "\033[32m"
		FgYellow = "\033[33m"
	)

	for _, row := range rows {
		status := row.Status
		if row.Color == ColorYellow {
			status = FgYellow + status + Reset
		} else if row.Color == ColorGreen {
			status = FgGreen + status + Reset
		}

		_ = table.Append(row.Repo, row.ConfigRef, row.LocalBranchRev, row.RemoteRev, status)
	}
	if err := table.Render(); err != nil {
		fmt.Printf("Error rendering table: %v\n", err)
	}
}
