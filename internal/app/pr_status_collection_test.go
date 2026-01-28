package app

import (
	conf "mistletoe/internal/config"
	"mistletoe/internal/sys"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestPrStatusHelperProcess mocks the gh command for PR status collection.
func TestPrStatusHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd := args[0]
	// args[1:] are sub-commands/flags

	if cmd == "gh" {
		// Mock "gh repo view ... --json url,parent"
		if len(args) > 3 && args[1] == "repo" && args[2] == "view" {
			// Return simple repo info (no parent)
			fmt.Println(`{"url": "https://github.com/owner/repo"}`)
			return
		}

		// Mock "gh pr list ... --state all"
		if len(args) > 2 && args[1] == "pr" && args[2] == "list" {
			// Check if --state all is present
			hasStateAll := false
			for _, arg := range args {
				if arg == "all" {
					hasStateAll = true
				}
			}

			if hasStateAll {
				// Return Open, Merged, and Closed PRs
				prs := []map[string]interface{}{
					{
						"number":      101,
						"state":       "OPEN",
						"isDraft":     false,
						"url":         "https://github.com/owner/repo/pull/101",
						"baseRefName": "main",
						"headRefOid":  "sha_open",
						"author":      map[string]string{"login": "user1"},
						"headRepository": map[string]string{
							"url": "https://github.com/owner/repo",
						},
					},
					{
						"number":      100,
						"state":       "MERGED",
						"isDraft":     false,
						"url":         "https://github.com/owner/repo/pull/100",
						"baseRefName": "main",
						"headRefOid":  "sha_merged",
						"author":      map[string]string{"login": "user1"},
						"headRepository": map[string]string{
							"url": "https://github.com/owner/repo",
						},
					},
					{
						"number":      99,
						"state":       "CLOSED",
						"isDraft":     false,
						"url":         "https://github.com/owner/repo/pull/99",
						"baseRefName": "main",
						"headRefOid":  "sha_closed",
						"author":      map[string]string{"login": "user1"},
						"headRepository": map[string]string{
							"url": "https://github.com/owner/repo",
						},
					},
				}
				data, _ := json.Marshal(prs)
				fmt.Println(string(data))
				return
			}
		}
	}

	// Default: success
	os.Exit(0)
}

func TestCollectPrStatus_IncludesMergedAndClosed(t *testing.T) {
	// Mock sys.ExecCommand
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestPrStatusHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	defer func() { sys.ExecCommand = exec.Command }()

	// Setup Config
	repoID := "repo1"
	repoURL := "https://github.com/owner/repo"
	baseBranch := "main"
	repo := conf.Repository{
		ID:         &repoID,
		URL:        &repoURL,
		BaseBranch: &baseBranch,
	}
	config := &conf.Config{
		Repositories: &[]conf.Repository{repo},
	}

	// Setup StatusRow
	statusRows := []StatusRow{
		{
			Repo:          repoID,
			RepoDir:       "/tmp/repo1",
			BranchName:    "feature/branch",
			LocalHeadFull: "sha_current", // Doesn't match any PR head (new commit pushed)
		},
	}

	// Run CollectPrStatus
	prRows := CollectPrStatus(statusRows, config, 1, "gh", false, nil)

	if len(prRows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(prRows))
	}

	row := prRows[0]
	if len(row.PrItems) != 3 {
		t.Errorf("Expected 3 PR items (Open, Merged, Closed), got %d", len(row.PrItems))
	}

	// Verify items
	states := make(map[string]bool)
	for _, pr := range row.PrItems {
		states[strings.ToUpper(pr.State)] = true
	}

	if !states["OPEN"] {
		t.Error("Missing OPEN PR")
	}
	if !states["MERGED"] {
		t.Error("Missing MERGED PR")
	}
	if !states["CLOSED"] {
		t.Error("Missing CLOSED PR")
	}
}
