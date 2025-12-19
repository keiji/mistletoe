package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// StatusRow represents the status of a single repository.
type StatusRow struct {
	Repo           string
	ConfigRef      string
	LocalBranchRev string
	RemoteRev      string
	RemoteColor    int
	BranchName     string
	HasUnpushed    bool
	IsPullable     bool
	HasConflict    bool
	RepoDir        string
	LocalHeadFull  string
}

// ValidateRepositoriesIntegrity checks if repositories exist and are valid.
func ValidateRepositoriesIntegrity(config *Config, gitPath string, verbose bool) error {
	for _, repo := range *config.Repositories {
		targetDir := GetRepoDir(repo)
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
		currentURL, err := RunGit(targetDir, gitPath, verbose, "config", "--get", "remote.origin.url")
		if err != nil {
			return fmt.Errorf("Error: directory %s is a git repo but failed to get remote origin: %v", targetDir, err)
		}
		if currentURL != *repo.URL {
			return fmt.Errorf("Error: directory %s exists with different remote origin: %s (expected %s)", targetDir, currentURL, *repo.URL)
		}
	}
	return nil
}

// CollectStatus collects status for all repositories.
func CollectStatus(config *Config, parallel int, gitPath string, verbose bool, noFetch bool) []StatusRow {
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

			row := getRepoStatus(repo, gitPath, verbose, noFetch)
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

func getRepoStatus(repo Repository, gitPath string, verbose bool, noFetch bool) *StatusRow {
	targetDir := GetRepoDir(repo)
	repoName := targetDir
	if repo.ID != nil && *repo.ID != "" {
		repoName = *repo.ID
	}

	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return nil
	}

	// 1. Get Local Status (Short SHA, Full SHA, Branch Status)
	// We use git log -1 --format="%h%n%H%n%D" to get all info in one go.
	// %h: Short Hash, %H: Full Hash, %D: Ref names
	output, err := RunGit(targetDir, gitPath, verbose, "log", "-1", "--format=%h%n%H%n%D")

	branchName := ""
	shortSHA := ""
	localHeadFull := ""
	isDetached := false

	if err == nil {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) >= 1 {
			shortSHA = lines[0]
		}
		if len(lines) >= 2 {
			localHeadFull = lines[1]
		}
		if len(lines) >= 3 {
			refs := lines[2]
			if strings.Contains(refs, "HEAD ->") {
				parts := strings.Split(refs, "HEAD ->")
				if len(parts) > 1 {
					remainder := strings.TrimSpace(parts[1])
					branchParts := strings.Split(remainder, ",")
					branchName = strings.TrimSpace(branchParts[0])
				}
			} else {
				isDetached = true
				branchName = "HEAD"
			}
		} else {
			// Detached with no other refs
			isDetached = true
			branchName = "HEAD"
		}
	} else {
		// Fallback for unborn branches (empty repo) where git log fails
		branchName, err = RunGit(targetDir, gitPath, verbose, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			branchName = ""
		}
		if branchName == "HEAD" {
			isDetached = true
		}
	}

	// 3. Construct LocalBranchRev
	localBranchRev := ""
	if branchName != "" && shortSHA != "" {
		localBranchRev = fmt.Sprintf("%s:%s", branchName, shortSHA)
	} else if shortSHA != "" {
		localBranchRev = shortSHA
	} else if branchName != "" {
		localBranchRev = branchName // Unborn branch
	}

	// 4. ConfigRef
	configRef := ""
	if repo.Branch != nil && *repo.Branch != "" {
		configRef = *repo.Branch
	}
	if repo.Revision != nil && *repo.Revision != "" {
		if configRef != "" {
			configRef += ":" + *repo.Revision
		} else {
			configRef = *repo.Revision
		}
	}

	// Remote Logic
	remoteHeadFull := ""
	remoteDisplay := ""
	remoteColor := ColorNone

	if !isDetached {
		// Check if remote branch exists
		// If noFetch is true, skip explicit fetch and rely on existing refs/remotes/origin
		if !noFetch {
			// git fetch origin <branchName>
			// This replaces ls-remote + (maybe) fetch with a single fetch.
			// It ensures we have the latest remote state and objects.
			_, _ = RunGit(targetDir, gitPath, verbose, "fetch", "origin", branchName)
		}

		// Resolve the remote branch tip from refs/remotes/origin/<branchName>
		output, err := RunGit(targetDir, gitPath, verbose, "rev-parse", "refs/remotes/origin/"+branchName)
		if err == nil && output != "" {
			remoteHeadFull = strings.TrimSpace(output)

			// Construct display: branchName/shortSHA
			shortRemote := remoteHeadFull
			if len(shortRemote) >= 7 {
				shortRemote = shortRemote[:7]
			} else {
				shortRemote = remoteHeadFull
			}
			remoteDisplay = fmt.Sprintf("%s:%s", branchName, shortRemote)

			// Check Pushability (Coloring for Remote Column)
			// If local..remote is not 0, it implies remote has commits local doesn't (pull needed or diverged)
			// -> "push impossible" -> Yellow
			if localHeadFull != "" {
				count, err := RunGit(targetDir, gitPath, verbose, "rev-list", "--count", localHeadFull+".."+remoteHeadFull)
				if err == nil && count != "0" {
					remoteColor = ColorYellow
				}
			}
		}
	}

	// Status Logic
	hasUnpushed := false
	isPullable := false
	hasConflict := false

	if remoteHeadFull != "" && localHeadFull != "" {
		// Since we fetched above, we assume we have the objects.
		// Proceed directly to ancestry checks.

		// Check Unpushed (Ahead)
		// git rev-list --count remote..local
		if remoteHeadFull != localHeadFull {
			// If object is still missing, this will fail and return err, hasUnpushed remains false.
			count, err := RunGit(targetDir, gitPath, verbose, "rev-list", "--count", remoteHeadFull+".."+localHeadFull)
			if err == nil && count != "0" {
				hasUnpushed = true
			}
		}

		// Check Pullable (Behind)
		// Only if current branch matches config branch (Existing logic preserved for Status Symbol)
		if repo.Branch != nil && *repo.Branch != "" && *repo.Branch == branchName {
			if remoteHeadFull != localHeadFull {
				// Object exists locally, check ancestry
				// git rev-list --count local..remote
				count, err := RunGit(targetDir, gitPath, verbose, "rev-list", "--count", localHeadFull+".."+remoteHeadFull)
				if err == nil && count != "0" {
					isPullable = true
				}

				if isPullable {
					// Check for conflicts
					// 2. Merge Base
					base, err := RunGit(targetDir, gitPath, verbose, "merge-base", localHeadFull, remoteHeadFull)
					if err == nil && base != "" {
						base = strings.TrimSpace(base)
						// 3. Merge Tree
						output, err := RunGit(targetDir, gitPath, verbose, "merge-tree", base, localHeadFull, remoteHeadFull)
						if err == nil {
							if strings.Contains(output, "<<<<<<<") {
								hasConflict = true
							}
						}
					}
				}
			}
		}

	} else if !isDetached && remoteHeadFull == "" {
		// Remote branch doesn't exist? Means all local commits are unpushed
		hasUnpushed = true
	}

	return &StatusRow{
		Repo:           repoName,
		ConfigRef:      configRef,
		LocalBranchRev: localBranchRev,
		RemoteRev:      remoteDisplay,
		RemoteColor:    remoteColor,
		BranchName:     branchName,
		HasUnpushed:    hasUnpushed,
		IsPullable:     isPullable,
		HasConflict:    hasConflict,
		RepoDir:        targetDir,
		LocalHeadFull:  localHeadFull,
	}
}

// RenderStatusTable renders the status table to stdout.
func RenderStatusTable(rows []StatusRow) {
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeaderAutoFormat(tw.Off),
		tablewriter.WithRowAutoWrap(tw.WrapNone),
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
				WithTopMid("-").
				WithBottomMid("-"),
		}),
	)
	table.Header("Repository", "Config", "Local", "Remote", "Status")

	const (
		Reset    = "\033[0m"
		FgRed    = "\033[31m"
		FgGreen  = "\033[32m"
		FgYellow = "\033[33m"
	)

	for _, row := range rows {
		// Status Column
		statusStr := ""
		if row.HasUnpushed {
			statusStr += FgGreen + StatusSymbolUnpushed + Reset
		}

		if row.HasConflict {
			statusStr += FgYellow + StatusSymbolConflict + Reset
		} else if row.IsPullable {
			statusStr += FgYellow + StatusSymbolPullable + Reset
		}

		if statusStr == "" {
			statusStr = "-"
		}

		// Remote Column
		remoteStr := row.RemoteRev
		if row.RemoteColor == ColorYellow {
			remoteStr = FgYellow + remoteStr + Reset
		}

		_ = table.Append(row.Repo, row.ConfigRef, row.LocalBranchRev, remoteStr, statusStr)
	}
	if err := table.Render(); err != nil {
		fmt.Printf("Error rendering table: %v\n", err)
	}
	fmt.Printf("Status Legend: %s Pullable, %s Unpushed, %s Conflict\n", StatusSymbolPullable, StatusSymbolUnpushed, StatusSymbolConflict)
}
