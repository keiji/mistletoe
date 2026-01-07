package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// PrInfo holds information about a Pull Request.
type PrInfo struct {
	Number             int    `json:"number"`
	State              string `json:"state"`
	IsDraft            bool   `json:"isDraft"`
	URL                string `json:"url"`
	BaseRefName        string `json:"baseRefName"`
	HeadRefOid         string `json:"headRefOid"`
	Author             Author `json:"author"`
	ViewerCanEditFiles bool   `json:"viewerCanEditFiles"`
	Body               string `json:"body"`
}

// Author represents a GitHub user.
type Author struct {
	Login string `json:"login"`
}

// PrStatusRow represents a row in the PR status table.
type PrStatusRow struct {
	StatusRow
	PrNumber  string
	PrState   string
	PrURL     string
	PrItems   []PrInfo
	PrDisplay string
	Base      string
}

// CollectPrStatus collects Pull Request status for the given repositories.
// knownPRs is an optional map of [RepoID] -> []URL to skip querying existing PRs.
func CollectPrStatus(statusRows []StatusRow, config *Config, parallel int, ghPath string, verbose bool, knownPRs map[string][]string) []PrStatusRow {
	repoMap := make(map[string]Repository)
	for _, r := range *config.Repositories {
		repoMap[getRepoName(r)] = r
	}

	prRows := make([]PrStatusRow, len(statusRows))
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var mu sync.Mutex

	for i, row := range statusRows {
		wg.Add(1)
		go func(idx int, r StatusRow) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			prRow := PrStatusRow{StatusRow: r}

			isKnown := false
			if knownPRs != nil {
				if urls, ok := knownPRs[r.Repo]; ok && len(urls) > 0 {
					isKnown = true
					// Pick the first as representative for singular fields (Top)
					url := urls[0]
					prRow.PrURL = url

					var items []PrInfo
					var displays []string

					for _, u := range urls {
						args := []string{"pr", "view", u, "--json", "number,state,isDraft,baseRefName,headRefOid,author,viewerCanEditFiles,body"}
						out, err := RunGh(ghPath, verbose, args...)
						if err == nil {
							var pr PrInfo
							if err := json.Unmarshal([]byte(out), &pr); err == nil {
								pr.URL = u
								items = append(items, pr)

								displayState := getPrDisplayState(pr)
								line := fmt.Sprintf("%s [%s]", pr.URL, displayState)
								if displayState == DisplayPrStateMerged || displayState == DisplayPrStateClosed {
									line = AnsiFgGray + line + AnsiReset
								}
								displays = append(displays, line)
							}
						}
					}

					prRow.PrItems = items
					prRow.PrDisplay = strings.Join(displays, "\n")

					if len(items) > 0 {
						// Set Top fields based on first
						topPr := items[0]
						prRow.PrNumber = fmt.Sprintf("#%d", topPr.Number)
						prRow.PrState = topPr.State
						if topPr.BaseRefName != "" {
							prRow.Base = topPr.BaseRefName
						}
					} else {
						// Fallback if all lookups failed
						prRow.PrDisplay = fmt.Sprintf("%s [Error]", url) // Show first url
						prRow.PrState = "Error"
						prRow.PrNumber = "N/A"
					}
				}
			}

			conf, ok := repoMap[r.Repo]
			if ok && conf.URL != nil {
				baseBranch := ""
				if conf.BaseBranch != nil && *conf.BaseBranch != "" {
					baseBranch = *conf.BaseBranch
				}
				if baseBranch != "" {
					prRow.Base = baseBranch
				}

				if !isKnown && r.RepoDir != "" && r.BranchName != "HEAD" && r.BranchName != "" {
					args := []string{"pr", "list", "--repo", *conf.URL, "--head", r.BranchName, "--state", "all", "--json", "number,state,isDraft,url,baseRefName,headRefOid,author,viewerCanEditFiles,body"}
					if baseBranch != "" {
						args = append(args, "--base", baseBranch)
					}

					out, err := RunGh(ghPath, verbose, args...)
					if verbose {
						fmt.Printf("[%s] gh pr list output: %s\n", r.Repo, out)
					}
					if err == nil {
						var prs []PrInfo
						if err := json.Unmarshal([]byte(out), &prs); err == nil && len(prs) > 0 {
							// Check for Open PRs
							hasOpenPR := false
							for _, pr := range prs {
								if strings.EqualFold(pr.State, GitHubPrStateOpen) || (pr.IsDraft && strings.EqualFold(pr.State, GitHubPrStateOpen)) {
									hasOpenPR = true
									break
								}
							}

							// Filter PRs
							var filteredPrs []PrInfo
							for _, pr := range prs {
								if strings.EqualFold(pr.State, GitHubPrStateOpen) || (pr.IsDraft && strings.EqualFold(pr.State, GitHubPrStateOpen)) {
									filteredPrs = append(filteredPrs, pr)
								} else {
									// Closed or Merged
									// Include if (HeadRefOid matches LocalHeadFull) OR (There is an Open PR)
									matchHead := r.LocalHeadFull != "" && pr.HeadRefOid == r.LocalHeadFull
									if matchHead || hasOpenPR {
										filteredPrs = append(filteredPrs, pr)
									}
								}
							}

							if len(filteredPrs) == 0 {
								prRow.PrNumber = "N/A"
							} else {
								// Sort PRs
								SortPrs(filteredPrs)

								// Format PR column & Collect Items
								var prLines []string
								var items []PrInfo
								for _, pr := range filteredPrs {
									displayState := getPrDisplayState(pr)
									line := fmt.Sprintf("%s [%s]", pr.URL, displayState)
									if displayState == DisplayPrStateMerged || displayState == DisplayPrStateClosed {
										line = AnsiFgGray + line + AnsiReset
									}
									prLines = append(prLines, line)
									items = append(items, pr)
								}
								prRow.PrDisplay = strings.Join(prLines, "\n")
								prRow.PrItems = items

								// Set other fields based on the first (most relevant) PR
								topPr := filteredPrs[0]
								prRow.PrURL = topPr.URL
								prRow.PrNumber = fmt.Sprintf("#%d", topPr.Number)
								prRow.PrState = topPr.State // Raw state

								if prRow.Base == "" {
									prRow.Base = topPr.BaseRefName
								}
							}
						} else {
							prRow.PrNumber = "N/A"
						}
					} else {
						prRow.PrNumber = "N/A"
					}
				} else if !isKnown {
					prRow.PrNumber = "N/A"
				}
			}

			mu.Lock()
			prRows[idx] = prRow
			mu.Unlock()

		}(i, row)
	}
	wg.Wait()

	return prRows
}

func getPrDisplayState(pr PrInfo) string {
	if pr.IsDraft && pr.State == GitHubPrStateOpen {
		return DisplayPrStateDraft
	}
	switch pr.State {
	case GitHubPrStateOpen:
		return DisplayPrStateOpen
	case GitHubPrStateMerged:
		return DisplayPrStateMerged
	case GitHubPrStateClosed:
		return DisplayPrStateClosed
	default:
		return pr.State
	}
}

// SortPrs sorts a list of PrInfo objects.
func SortPrs(prs []PrInfo) {
	stateRank := func(pr PrInfo) int {
		// Handle Draft explicitly
		if pr.IsDraft && strings.ToUpper(pr.State) == GitHubPrStateOpen {
			return 1
		}

		switch strings.ToUpper(pr.State) {
		case GitHubPrStateOpen:
			return 0
		case GitHubPrStateMerged:
			return 2
		case GitHubPrStateClosed:
			return 3
		default:
			return 4
		}
	}

	sort.Slice(prs, func(i, j int) bool {
		rankI := stateRank(prs[i])
		rankJ := stateRank(prs[j])

		if rankI != rankJ {
			return rankI < rankJ
		}
		// Same state, sort by number descending
		return prs[i].Number > prs[j].Number
	})
}

// RenderPrStatusTable renders the PR status table.
func RenderPrStatusTable(rows []PrStatusRow) {
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
	// Change Header Order: Repository, Base, Branch/Rev, Status, PR
	table.Header("Repository", "Base", "Branch/Rev", "Status", "PR")

	for _, row := range rows {
		statusStr := ""
		if row.HasUnpushed {
			statusStr += AnsiFgGreen + StatusSymbolUnpushed + AnsiReset
		}

		if row.HasConflict {
			statusStr += AnsiFgYellow + StatusSymbolConflict + AnsiReset
		} else if row.IsPullable {
			statusStr += AnsiFgYellow + StatusSymbolPullable + AnsiReset
		}

		if statusStr == "" {
			statusStr = "-"
		}

		prContent := row.PrDisplay
		if prContent == "" {
			prContent = "-"
		}

		_ = table.Append(row.Repo, row.Base, row.LocalBranchRev, statusStr, prContent)
	}
	if err := table.Render(); err != nil {
		fmt.Printf("Error rendering table: %v\n", err)
	}
	fmt.Printf("Status Legend: %s Pullable, %s Unpushed, %s Conflict\n", StatusSymbolPullable, StatusSymbolUnpushed, StatusSymbolConflict)
}

// executePush pushes changes for the given repositories.
func executePush(repos []Repository, rows []StatusRow, parallel int, gitPath string, verbose bool) error {
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var mu sync.Mutex
	var errs []string

	statusMap := make(map[string]StatusRow)
	for _, r := range rows {
		statusMap[r.Repo] = r
	}

	for _, repo := range repos {
		wg.Add(1)
		go func(r Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoDir := GetRepoDir(r)
			repoName := getRepoName(r)

			branchName := ""
			if row, ok := statusMap[repoName]; ok && row.BranchName != "" {
				branchName = row.BranchName
			} else {
				// Fallback
				b, err := RunGit(repoDir, gitPath, verbose, "rev-parse", "--abbrev-ref", "HEAD")
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("[%s] failed to get branch: %v", repoName, err))
					mu.Unlock()
					return
				}
				branchName = b
			}

			fmt.Printf("[%s] Pushing to origin/%s...\n", repoName, branchName)
			if _, err := RunGit(repoDir, gitPath, verbose, "push", "origin", branchName); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("[%s] push failed: %v", repoName, err))
				mu.Unlock()
				return
			}
		}(repo)
	}
	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred during push:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

func updatePrDescriptions(prMap map[string][]PrInfo, parallel int, ghPath string, verbose bool, snapshotData, snapshotFilename string, deps *DependencyGraph, depContent string) error {
	if len(prMap) == 0 {
		return nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var errs []string

	// Flatten tasks
	type task struct {
		repoID string
		url    string
	}
	var tasks []task
	for id, items := range prMap {
		for _, item := range items {
			tasks = append(tasks, task{repoID: id, url: item.URL})
		}
	}

	for _, t := range tasks {
		wg.Add(1)
		go func(tsk task) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			targetURL := tsk.url
			repoID := tsk.repoID

			// Check PR State
			stateOut, err := RunGh(ghPath, verbose, "pr", "view", targetURL, "--json", "state", "-q", ".state")
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("failed to check state for PR %s: %v", targetURL, err))
				mu.Unlock()
				return
			}
			state := strings.TrimSpace(string(stateOut))
			if state == "MERGED" || state == "CLOSED" {
				return
			}

			// Get current body
			bodyOut, err := RunGh(ghPath, verbose, "pr", "view", targetURL, "--json", "body", "-q", ".body")
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("failed to view PR %s: %v", targetURL, err))
				mu.Unlock()
				return
			}
			originalBody := strings.TrimSpace(string(bodyOut))

			// Generate new Mistletoe block
			newBlock := GenerateMistletoeBody(snapshotData, snapshotFilename, repoID, prMap, deps, depContent)

			// Update body
			newBody := EmbedMistletoeBody(originalBody, newBlock)

			// Update
			_, err = RunGh(ghPath, verbose, "pr", "edit", targetURL, "--body", newBody)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("failed to edit PR %s: %v", targetURL, err))
				mu.Unlock()
				return
			}
		}(t)
	}
	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("errors updating descriptions:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

func getRepoName(r Repository) string {
	if r.ID != nil && *r.ID != "" {
		return *r.ID
	}
	// Fallback to dir name
	return GetRepoDir(r)
}

// resolveRemoteBranchHash tries to resolve the remote branch hash locally first,
// and falls back to ls-remote if necessary.
func resolveRemoteBranchHash(repoDir, gitPath, branchName string, verbose bool) (string, error) {
	// 1. Try local ref (fast)
	// checks refs/remotes/origin/<branchName>
	out, err := RunGit(repoDir, gitPath, verbose, "rev-parse", "--verify", "refs/remotes/origin/"+branchName)
	if err == nil && out != "" {
		return strings.TrimSpace(out), nil
	}

	// 2. Fallback to ls-remote (network, slow)
	lsOut, err := RunGit(repoDir, gitPath, verbose, "ls-remote", "--heads", "origin", branchName)
	if err != nil {
		return "", err
	}

	lines := strings.Split(lsOut, "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			// exact match for branch
			if parts[1] == "refs/heads/"+branchName {
				return parts[0], nil
			}
		}
	}

	return "", nil
}

// Mockable lookPath for testing
var lookPath = exec.LookPath

func checkGhAvailability(ghPath string, verbose bool) error {
	_, err := lookPath(ghPath)
	if err != nil {
		return errors.New("error: 'gh' command not found. Please install GitHub CLI")
	}
	_, err = RunGh(ghPath, verbose, "auth", "status")
	if err != nil {
		return errors.New("error: 'gh' is not authenticated. Please run 'gh auth login'")
	}
	return nil
}

// verifyGithubRequirements checks GitHub URL, permissions, base branch existence, and existing PRs.
// It returns a map of RepoName -> Existing PR URL.
// Accepts knownPRs map[string][]string (ID -> []URL) to optimize existing PR check.
func verifyGithubRequirements(repos []Repository, rows []StatusRow, parallel int, gitPath, ghPath string, verbose bool, knownPRs map[string][]string) (map[string]string, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	var errs []string
	existingPRs := make(map[string]string)

	statusMap := make(map[string]StatusRow)
	for _, r := range rows {
		statusMap[r.Repo] = r
	}

	for _, repo := range repos {
		wg.Add(1)
		go func(r Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoName := getRepoName(r)

			// 1. Check if URL is GitHub
			if r.URL == nil || !strings.Contains(*r.URL, "github.com") {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("repository %s is not a GitHub repository", repoName))
				mu.Unlock()
				return
			}

			// 2. Check Permission
			out, err := RunGh(ghPath, verbose, "repo", "view", *r.URL, "--json", "viewerPermission", "-q", ".viewerPermission")
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("failed to check permission for %s: %v", repoName, err))
				mu.Unlock()
				return
			}
			perm := strings.TrimSpace(out)
			if perm != "ADMIN" && perm != "MAINTAIN" && perm != "WRITE" {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("insufficient permission for %s: %s (need WRITE or better)", repoName, perm))
				mu.Unlock()
				return
			}

			// 3. Check Base Branch Existence
			baseBranch := ""
			if r.BaseBranch != nil && *r.BaseBranch != "" {
				baseBranch = *r.BaseBranch
			} else if r.Branch != nil && *r.Branch != "" {
				baseBranch = *r.Branch
			}

			if baseBranch != "" {
				repoDir := GetRepoDir(r)
				remoteHash, err := resolveRemoteBranchHash(repoDir, gitPath, baseBranch, verbose)
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("[%s] failed to check base branch '%s': %v", repoName, baseBranch, err))
					mu.Unlock()
					return
				}
				if remoteHash == "" {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("[%s] base branch '%s' does not exist on remote", repoName, baseBranch))
					mu.Unlock()
					return
				}
			}

			// 4. Check for existing PR
			if knownPRs != nil {
				if urls, ok := knownPRs[repoName]; ok && len(urls) > 0 {
					mu.Lock()
					existingPRs[repoName] = urls[0]
					mu.Unlock()
					return
				}
			}

			// Fallback to query
			repoDir := GetRepoDir(r)
			branchName := ""

			if row, ok := statusMap[repoName]; ok && row.BranchName != "" {
				branchName = row.BranchName
			} else {
				// Redundant fallback
				b, err := RunGit(repoDir, gitPath, verbose, "rev-parse", "--abbrev-ref", "HEAD")
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("[%s] failed to get branch for PR check: %v", repoName, err))
					mu.Unlock()
					return
				}
				branchName = b
			}

			out, errCheck := RunGh(ghPath, verbose, "pr", "list", "--repo", *r.URL, "--head", branchName, "--json", "url", "-q", ".[0].url")
			if errCheck != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("[%s] failed to check for existing PR: %v", repoName, errCheck))
				mu.Unlock()
				return
			}
			prURL := strings.TrimSpace(out)

			if prURL != "" {
				mu.Lock()
				existingPRs[repoName] = prURL
				mu.Unlock()
			}

		}(repo)
	}
	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("GitHub validation failed:\n%s", strings.Join(errs, "\n"))
	}
	return existingPRs, nil
}

// GetGhUser returns the current authenticated GitHub user's login.
func GetGhUser(ghPath string, verbose bool) (string, error) {
	out, err := RunGh(ghPath, verbose, "api", "user", "--jq", ".login")
	if err != nil {
		return "", fmt.Errorf("failed to get GitHub user: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// ValidatePrPermissionAndOverwrite checks if we can overwrite an existing PR.
// It returns nil if allowed, or an error if permission denied or overwrite flag required.
func ValidatePrPermissionAndOverwrite(repoID string, pr PrInfo, currentUser string, overwrite bool) error {
	// 1. Permission Check
	if !pr.ViewerCanEditFiles {
		return fmt.Errorf("permission denied: you do not have edit permission for PR %s (Repo: %s)", pr.URL, repoID)
	}

	// 2. Overwrite Logic
	// Check for Mistletoe block
	_, _, found := ParseMistletoeBlock(pr.Body)
	if found {
		// Existing block found -> Safe to overwrite
		return nil
	}

	// No Mistletoe block
	if strings.EqualFold(pr.Author.Login, currentUser) {
		// Creator is me -> Safe to overwrite (append)
		return nil
	}

	// Creator is NOT me
	if overwrite {
		// Overwrite flag set -> Safe
		return nil
	}

	return fmt.Errorf("PR %s (Repo: %s) was created by %s and does not have a Mistletoe block. Use --overwrite (-w) to force update", pr.URL, repoID, pr.Author.Login)
}
