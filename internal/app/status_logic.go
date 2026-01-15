package app

import (
	conf "mistletoe/internal/config"
)

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

var osExit = os.Exit

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
	RemoteHeadFull string
}

// ValidateRepositoriesIntegrity checks if repositories exist and are valid.
func ValidateRepositoriesIntegrity(config *conf.Config, gitPath string, verbose bool) error {
	// Debug
	// fmt.Fprintf(Stderr, "DEBUG: ValidateRepositoriesIntegrity BaseDir=%s\n", config.BaseDir)

	for _, repo := range *config.Repositories {
		targetDir := config.GetRepoPath(repo)
		info, err := os.Stat(targetDir)
		if os.IsNotExist(err) {
			// fmt.Fprintf(Stderr, "DEBUG: skipping %s (not exist)\n", targetDir)
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking directory %s: %v", targetDir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("error: target %s exists and is not a directory", targetDir)
		}

		// Check if Git repository
		gitDir := filepath.Join(targetDir, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			return fmt.Errorf("error: directory %s exists but is not a git repository", targetDir)
		}

		// Check remote origin
		currentURL, err := RunGit(targetDir, gitPath, verbose, "config", "--get", "remote.origin.url")
		if err != nil {
			return fmt.Errorf("error: directory %s is a git repo but failed to get remote origin: %v", targetDir, err)
		}

		// fmt.Fprintf(Stderr, "DEBUG: targetDir=%s, currentURL='%s', expectedURL='%s'\n", targetDir, currentURL, *repo.URL)

		if currentURL != *repo.URL {
			return fmt.Errorf("error: directory %s exists with different remote origin: %s (expected %s)", targetDir, currentURL, *repo.URL)
		}
	}
	return nil
}

// CollectStatus collects status for all repositories.
func CollectStatus(config *conf.Config, jobs int, gitPath string, verbose bool, noFetch bool) []StatusRow {
	var rows []StatusRow
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)

	for _, repo := range *config.Repositories {
		wg.Add(1)
		go func(repo conf.Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			row := getRepoStatus(repo, config.BaseDir, gitPath, verbose, noFetch)
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

func getRepoStatus(repo conf.Repository, baseDir, gitPath string, verbose bool, noFetch bool) *StatusRow {
	targetDir := filepath.Join(baseDir, conf.GetRepoDirName(repo))
	repoName := conf.GetRepoDirName(repo)

	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return nil
	}

	// 1. Get Local Status (Short SHA, Full SHA, Branch Status)
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
		localBranchRev = fmt.Sprintf("%s %s", branchName, shortSHA)
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
		var fetchErr error
		if !noFetch {
			_, fetchErr = RunGit(targetDir, gitPath, verbose, "fetch", "origin", branchName)
		}

		// Check upstream configuration and unset if invalid
		currentUpstream, err := RunGit(targetDir, gitPath, verbose, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
		if err == nil {
			currentUpstream = strings.TrimSpace(currentUpstream)
			if currentUpstream != "" {
				// Condition 1: Local branch name and upstream name are different
				// We assume remote is always "origin" per ValidateRepositoriesIntegrity
				if currentUpstream != "origin/"+branchName {
					msg := fmt.Sprintf("Unsetting upstream for %s because the configuration is invalid (differs from origin/%s).\n", repoName, branchName)
					fmt.Fprint(Stderr, msg)
					_, _ = RunGit(targetDir, gitPath, verbose, "branch", "--unset-upstream")
				} else {
					// Condition 2: Remote branch does not exist
					// If fetch succeeded, the branch exists.
					// If fetch failed, verify with ls-remote.
					if fetchErr != nil {
						lsOut, lsErr := RunGit(targetDir, gitPath, verbose, "ls-remote", "--heads", "origin", branchName)
						// If ls-remote succeeded (network ok) but returned no output, branch is missing.
						if lsErr == nil && lsOut == "" {
							msg := fmt.Sprintf("Unsetting upstream for %s because the remote branch does not exist. It will be set again if you push.\n", repoName)
							fmt.Fprint(Stderr, msg)
							_, _ = RunGit(targetDir, gitPath, verbose, "branch", "--unset-upstream")
						}
					}
				}
			}
		}

		output, err := RunGit(targetDir, gitPath, verbose, "rev-parse", "refs/remotes/origin/"+branchName)
		if err == nil && output != "" {
			remoteHeadFull = strings.TrimSpace(output)

			shortRemote := remoteHeadFull
			if len(shortRemote) >= 7 {
				shortRemote = shortRemote[:7]
			} else {
				shortRemote = remoteHeadFull
			}
			remoteDisplay = fmt.Sprintf("%s %s", branchName, shortRemote)

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
		if remoteHeadFull != localHeadFull {
			count, err := RunGit(targetDir, gitPath, verbose, "rev-list", "--count", remoteHeadFull+".."+localHeadFull)
			if err == nil && count != "0" {
				hasUnpushed = true
			}
		}

		if repo.Branch != nil && *repo.Branch != "" && *repo.Branch == branchName {
			if remoteHeadFull != localHeadFull {
				count, err := RunGit(targetDir, gitPath, verbose, "rev-list", "--count", localHeadFull+".."+remoteHeadFull)
				if err == nil && count != "0" {
					isPullable = true
				}

				if isPullable {
					base, err := RunGit(targetDir, gitPath, verbose, "merge-base", localHeadFull, remoteHeadFull)
					if err == nil && base != "" {
						base = strings.TrimSpace(base)
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
		RemoteHeadFull: remoteHeadFull,
	}
}

// RenderStatusTable renders the status table to the provided writer (usually stdout).
func RenderStatusTable(w io.Writer, rows []StatusRow) {
	table := tablewriter.NewTable(w,
		tablewriter.WithHeaderAutoFormat(tw.Off),
		tablewriter.WithRowAutoWrap(tw.WrapNone),
		tablewriter.WithAlignment(tw.MakeAlign(5, tw.AlignLeft)),
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

		remoteStr := row.RemoteRev
		if row.RemoteColor == ColorYellow {
			remoteStr = FgYellow + remoteStr + Reset
		}

		_ = table.Append(row.Repo, row.ConfigRef, row.LocalBranchRev, remoteStr, statusStr)
	}
	if err := table.Render(); err != nil {
		fmt.Fprintf(w, "Error rendering table: %v\n", err)
	}
	fmt.Fprintf(w, "Status Legend: %s Pullable, %s Unpushed, %s Conflict\n", StatusSymbolPullable, StatusSymbolUnpushed, StatusSymbolConflict)
}

// ValidateStatusForAction checks if repositories are in a safe state for operations.
func ValidateStatusForAction(rows []StatusRow, checkPullable bool) {
	var behindRepos []string
	for _, row := range rows {
		if checkPullable && row.IsPullable {
			behindRepos = append(behindRepos, row.Repo)
		}
		if row.HasConflict {
			fmt.Fprintf(Stderr, "error: repository '%s' has conflicts. Cannot proceed.\n", row.Repo)
			osExit(1)
		}
		if row.BranchName == "HEAD" {
			fmt.Fprintf(Stderr, "error: repository '%s' is in a detached HEAD state. Cannot proceed.\n", row.Repo)
			osExit(1)
		}
	}

	if len(behindRepos) > 0 {
		fmt.Fprintf(Stderr, "error: the following repositories are behind remote and require a pull:\n")
		for _, r := range behindRepos {
			fmt.Fprintf(Stderr, " - %s\n", r)
		}
		fmt.Fprintln(Stderr, "Please pull changes before proceeding.")
		osExit(1)
	}
}
