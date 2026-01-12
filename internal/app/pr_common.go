package app

import (
	conf "mistletoe/internal/config"
)

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
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
	Author             Author     `json:"author"`
	ViewerCanEditFiles bool       `json:"viewerCanEditFiles"`
	Body               string     `json:"body"`
	HeadRepository     conf.Repository `json:"headRepository"`
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
// knownPRs is an optional map of [RepoID] -> []PrInfo to skip querying existing PRs.
func CollectPrStatus(statusRows []StatusRow, config *conf.Config, jobs int, ghPath string, verbose bool, knownPRs map[string][]PrInfo) []PrStatusRow {
	repoMap := make(map[string]conf.Repository)
	for _, r := range *config.Repositories {
		repoMap[getRepoName(r)] = r
	}

	prRows := make([]PrStatusRow, len(statusRows))
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
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
				if items, ok := knownPRs[r.Repo]; ok && len(items) > 0 {
					isKnown = true
					// Use known items directly without network call
					// Assuming items are already sorted by relevance if coming from pr_create
					// However, if coming from partial data (like just created), we need to handle that.

					// We'll trust the items provided.
					// If they are missing fields (like Number), we try to parse from URL.

					var validItems []PrInfo
					var displays []string

					for _, pr := range items {
						// Fill missing Number if 0 but URL exists
						if pr.Number == 0 && pr.URL != "" {
							if _, _, num, err := parsePrURL(pr.URL); err == nil {
								pr.Number = num
							}
						}
						// Default State if missing
						if pr.State == "" {
							pr.State = GitHubPrStateOpen
						}

						validItems = append(validItems, pr)

						displayState := getPrDisplayState(pr)
						line := fmt.Sprintf("%s [%s]", pr.URL, displayState)
						if displayState == DisplayPrStateMerged || displayState == DisplayPrStateClosed {
							line = AnsiFgGray + line + AnsiReset
						}
						displays = append(displays, line)
					}

					prRow.PrItems = validItems
					prRow.PrDisplay = strings.Join(displays, "\n")

					if len(validItems) > 0 {
						// Set Top fields based on first
						topPr := validItems[0]
						prRow.PrURL = topPr.URL
						if topPr.Number != 0 {
							prRow.PrNumber = fmt.Sprintf("#%d", topPr.Number)
						} else {
							prRow.PrNumber = "N/A"
						}
						prRow.PrState = topPr.State
						if topPr.BaseRefName != "" {
							prRow.Base = topPr.BaseRefName
						}
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
					// Check for upstream (parent) repository in case of fork
					repoURL := *conf.URL
					configCanonicalURL := *conf.URL

					outParent, errParent := RunGh(ghPath, verbose, "repo", "view", repoURL, "--json", "url,parent", "-q", ".")
					if errParent == nil {
						type RepoView struct {
							URL    string `json:"url"`
							Parent *struct {
								URL string `json:"url"`
							} `json:"parent"`
						}
						var rv RepoView
						if json.Unmarshal([]byte(outParent), &rv) == nil {
							// Use the canonical URL from GitHub for comparison
							if rv.URL != "" {
								configCanonicalURL = rv.URL
							}
							// Use parent URL for query if exists
							if rv.Parent != nil && rv.Parent.URL != "" {
								repoURL = rv.Parent.URL
							}
						}
					}

					args := []string{"pr", "list", "--repo", repoURL, "--head", r.BranchName, "--state", "all", "--json", "number,state,isDraft,url,baseRefName,headRefOid,author,body,headRepository"}
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
								// Filter by HeadRepository matching conf.Config URL (canonical)
								if isPrFromConfiguredRepo(pr, configCanonicalURL) {
									if strings.EqualFold(pr.State, GitHubPrStateOpen) || (pr.IsDraft && strings.EqualFold(pr.State, GitHubPrStateOpen)) {
										hasOpenPR = true
										break
									}
								}
							}

							// Filter PRs
							var filteredPrs []PrInfo
							for _, pr := range prs {
								// Apply same repo filter
								if !isPrFromConfiguredRepo(pr, configCanonicalURL) {
									continue
								}

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
func RenderPrStatusTable(w io.Writer, rows []PrStatusRow) {
	table := tablewriter.NewTable(w,
		tablewriter.WithRenderer(renderer.NewMarkdown()),
		tablewriter.WithAlignment(tw.MakeAlign(5, tw.AlignLeft)),
	)
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
		fmt.Fprintf(w, "Error rendering table: %v\n", err)
	}
	fmt.Fprintf(w, "Status Legend: %s Pullable, %s Unpushed, %s Conflict\n", StatusSymbolPullable, StatusSymbolUnpushed, StatusSymbolConflict)
}

// executePush pushes changes for the given repositories.
func executePush(repos []conf.Repository, baseDir string, rows []StatusRow, jobs int, gitPath string, verbose bool) error {
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
	var mu sync.Mutex
	var errs []string

	statusMap := make(map[string]StatusRow)
	for _, r := range rows {
		statusMap[r.Repo] = r
	}

	for _, repo := range repos {
		wg.Add(1)
		go func(r conf.Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoDir := filepath.Join(baseDir, conf.GetRepoDirName(r))
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

func updatePrDescriptions(prMap map[string][]PrInfo, jobs int, ghPath string, verbose bool, snapshotData, snapshotFilename string, deps *DependencyGraph, depContent string, overwrite bool) error {
	if len(prMap) == 0 {
		return nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
	var errs []string

	// Flatten tasks
	type task struct {
		repoID string
		url    string
		item   PrInfo
	}
	var tasks []task
	for id, items := range prMap {
		for _, item := range items {
			// Filter out Merged or Closed PRs
			if strings.EqualFold(item.State, GitHubPrStateMerged) || strings.EqualFold(item.State, GitHubPrStateClosed) {
				continue
			}
			tasks = append(tasks, task{repoID: id, url: item.URL, item: item})
		}
	}

	// We need current user for validation
	currentUser, err := GetGhUser(ghPath, verbose)
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	for _, t := range tasks {
		wg.Add(1)
		go func(tsk task) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			targetURL := tsk.url
			repoID := tsk.repoID

			// Get current body and permissions via GraphQL
			// We use GraphQL because 'gh pr view' JSON output might miss viewerCanEditFiles key in some contexts,
			// leading to false negatives in permission checks.
			owner, repo, number, err := parsePrURL(targetURL)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("failed to parse PR URL %s: %v", targetURL, err))
				mu.Unlock()
				return
			}

			query := `query($owner: String!, $name: String!, $number: Int!) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      body
      viewerCanEditFiles
      author {
        login
      }
    }
  }
}`

			out, err := RunGh(ghPath, verbose, "api", "graphql",
				"-F", "owner="+owner,
				"-F", "name="+repo,
				"-F", "number="+strconv.Itoa(number),
				"-f", "query="+query)

			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("failed to fetch PR details via GraphQL for %s: %v", targetURL, err))
				mu.Unlock()
				return
			}

			// Parse GraphQL Response
			type GqlResponse struct {
				Data struct {
					Repository struct {
						PullRequest struct {
							Body               string `json:"body"`
							ViewerCanEditFiles bool   `json:"viewerCanEditFiles"`
							Author             struct {
								Login string `json:"login"`
							} `json:"author"`
						} `json:"pullRequest"`
					} `json:"repository"`
				} `json:"data"`
			}

			var resp GqlResponse
			if err := json.Unmarshal([]byte(out), &resp); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("failed to parse GraphQL response for %s: %v", targetURL, err))
				mu.Unlock()
				return
			}

			prData := resp.Data.Repository.PullRequest

			// Update tsk.item with latest info
			tsk.item.Body = prData.Body
			tsk.item.ViewerCanEditFiles = prData.ViewerCanEditFiles
			tsk.item.Author = Author{Login: prData.Author.Login}
			originalBody := prData.Body

			// Validate
			if err := ValidatePrPermissionAndOverwrite(repoID, tsk.item, currentUser, overwrite); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("skipping %s: %v", targetURL, err))
				mu.Unlock()
				return
			}

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

func parsePrURL(url string) (owner, repo string, number int, err error) {
	// Matches https://github.com/OWNER/REPO/pull/NUMBER
	// Also handles http, no .git (PR URLs usually don't have .git)
	re := regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/pull/(\d+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) != 4 {
		return "", "", 0, fmt.Errorf("invalid PR URL format: %s", url)
	}
	owner = matches[1]
	repo = matches[2]
	numStr := matches[3]
	number, err = strconv.Atoi(numStr)
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid PR number in URL: %s", url)
	}
	return owner, repo, number, nil
}

func getRepoName(r conf.Repository) string {
	if r.ID != nil && *r.ID != "" {
		return *r.ID
	}
	// Fallback to dir name
	return conf.GetRepoDirName(r)
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
func verifyGithubRequirements(repos []conf.Repository, baseDir string, rows []StatusRow, jobs int, gitPath, ghPath string, verbose bool, knownPRs map[string][]string) (map[string]string, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
	var errs []string
	existingPRs := make(map[string]string)

	statusMap := make(map[string]StatusRow)
	for _, r := range rows {
		statusMap[r.Repo] = r
	}

	for _, repo := range repos {
		wg.Add(1)
		go func(r conf.Repository) {
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
				repoDir := filepath.Join(baseDir, conf.GetRepoDirName(r))
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
			repoDir := filepath.Join(baseDir, conf.GetRepoDirName(r))
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

// LoadDependencyGraph loads and parses the dependency graph from the specified file.
// If the path is empty, it returns nil and no error.
func LoadDependencyGraph(depPath string, config *conf.Config) (*DependencyGraph, string, error) {
	if depPath == "" {
		return nil, "", nil
	}

	contentBytes, errRead := os.ReadFile(depPath)
	if errRead != nil {
		return nil, "", fmt.Errorf("error reading dependency file: %v", errRead)
	}
	depContent := string(contentBytes)

	var validIDs []string
	for _, r := range *config.Repositories {
		validIDs = append(validIDs, getRepoName(r))
	}

	deps, errDep := ParseDependencies(depContent, validIDs)
	if errDep != nil {
		return nil, "", fmt.Errorf("error loading dependencies: %v", errDep)
	}

	return deps, depContent, nil
}
