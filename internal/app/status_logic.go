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
	RemoteColor    int
	BranchName     string
	HasUnpushed    bool
	IsPullable     bool
	HasConflict    bool
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
		currentURL, err := RunGit(targetDir, gitPath, "config", "--get", "remote.origin.url")
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
	branchName, err := RunGit(targetDir, gitPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		branchName = ""
	}
	isDetached := false
	if branchName == "HEAD" {
		isDetached = true
	}

	// 2. Get Short SHA
	shortSHA, err := RunGit(targetDir, gitPath, "rev-parse", "--short", "HEAD")
	if err != nil {
		shortSHA = ""
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
	localHeadFull, err := RunGit(targetDir, gitPath, "rev-parse", "HEAD")
	if err != nil {
		localHeadFull = ""
	}

	// Remote Logic
	remoteHeadFull := ""
	remoteDisplay := ""
	remoteColor := ColorNone

	if !isDetached {
		// Check if remote branch exists
		// git ls-remote origin <branchName>
		output, err := RunGit(targetDir, gitPath, "ls-remote", "origin", "refs/heads/"+branchName)
		if err == nil && output != "" {
			// Output format: <SHA>\trefs/heads/<branch>
			fields := strings.Fields(output)
			if len(fields) > 0 {
				remoteHeadFull = fields[0]

				// Construct display: branchName/shortSHA
				shortRemote := remoteHeadFull
				if len(shortRemote) >= 7 {
					shortRemote = shortRemote[:7]
				} else {
					shortRemote = remoteHeadFull
				}
				remoteDisplay = fmt.Sprintf("%s/%s", branchName, shortRemote)

				// Check Pushability (Coloring for Remote Column)
				// If local..remote is not 0, it implies remote has commits local doesn't (pull needed or diverged)
				// -> "push impossible" -> Yellow
				if localHeadFull != "" {
					count, err := RunGit(targetDir, gitPath, "rev-list", "--count", localHeadFull+".."+remoteHeadFull)
					if err == nil && count != "0" {
						remoteColor = ColorYellow
					}
				}
			}
		}
	}

	// Status Logic
	hasUnpushed := false
	isPullable := false
	hasConflict := false

	if remoteHeadFull != "" && localHeadFull != "" {
		// Check if we have the remote object locally
		_, err := RunGit(targetDir, gitPath, "cat-file", "-e", remoteHeadFull)
		objectMissing := (err != nil)

		if objectMissing {
			// Fetch to ensure we can calculate status accurately
			_, err := RunGit(targetDir, gitPath, "fetch", "origin", branchName)
			if err == nil {
				// Check if object exists now
				_, err = RunGit(targetDir, gitPath, "cat-file", "-e", remoteHeadFull)
				if err == nil {
					objectMissing = false
				}
			}
		}

		// Check Unpushed (Ahead)
		// git rev-list --count remote..local
		if remoteHeadFull != localHeadFull {
			// If object is still missing, this will fail and return err, hasUnpushed remains false.
			count, err := RunGit(targetDir, gitPath, "rev-list", "--count", remoteHeadFull+".."+localHeadFull)
			if err == nil && count != "0" {
				hasUnpushed = true
			}
		}

		// Check Pullable (Behind)
		// Only if current branch matches config branch (Existing logic preserved for Status Symbol)
		if repo.Branch != nil && *repo.Branch != "" && *repo.Branch == branchName {
			if remoteHeadFull != localHeadFull {
				if objectMissing {
					// Still missing after fetch attempt? Then we can't really tell, but usually means it's pullable (remote has something we don't have).
					isPullable = true
				} else {
					// Object exists locally, check ancestry
					// git rev-list --count local..remote
					count, err := RunGit(targetDir, gitPath, "rev-list", "--count", localHeadFull+".."+remoteHeadFull)
					if err == nil && count != "0" {
						isPullable = true
					}
				}

				if isPullable && !objectMissing {
					// Check for conflicts
					// 2. Merge Base
					base, err := RunGit(targetDir, gitPath, "merge-base", localHeadFull, remoteHeadFull)
					if err == nil && base != "" {
						base = strings.TrimSpace(base)
						// 3. Merge Tree
						output, err := RunGit(targetDir, gitPath, "merge-tree", base, localHeadFull, remoteHeadFull)
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
	}
}

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
			statusStr += FgGreen + ">" + Reset
		}

		if row.HasConflict {
			statusStr += FgYellow + "!" + Reset
		} else if row.IsPullable {
			statusStr += FgYellow + "<" + Reset
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
	fmt.Println("Status Legend: < Pullable, > Unpushed, ! Conflict")
}
