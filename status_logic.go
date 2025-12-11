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
)

// StatusRow represents the status of a single repository.
type StatusRow struct {
	Repo           string
	ConfigRef      string
	LocalBranchRev string
	RemoteRev      string
	Status         string
	Color          []int
	BranchName     string
	HasUnpushed    bool
	RepoDir        string
}

// ValidateRepositoriesIntegrity checks if repositories exist and are valid.
func ValidateRepositoriesIntegrity(config *Config, gitPath string) error {
	for _, repo := range config.Repositories {
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
		if currentURL != repo.URL {
			return fmt.Errorf("Error: directory %s exists with different remote origin: %s (expected %s)", targetDir, currentURL, repo.URL)
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

	for _, repo := range config.Repositories {
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

	// Status (Unpushed?)
	statusVal := "-"
	hasUnpushed := false

	if remoteHeadFull != "" && localHeadFull != "" {
		if remoteHeadFull != localHeadFull {
			// Check if unpushed
			cmd = exec.Command(gitPath, "-C", targetDir, "rev-list", "--count", remoteHeadFull+".."+localHeadFull)
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
