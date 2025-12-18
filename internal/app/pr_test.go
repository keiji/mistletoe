package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"strings"
)

// Mock execCommand
// Based on standard Go testing pattern for os/exec:
// https://npf.io/2015/06/testing-exec-command/

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)

	// Ensure executable path is absolute to handle RunGit changing cmd.Dir
	testBin, err := filepath.Abs(os.Args[0])
	if err != nil {
		testBin = os.Args[0] // Fallback
	}

	cmd := exec.Command(testBin, cs...)
	// Pass environment variables to identify specific test cases
	// Important: Append to os.Environ() to preserve PATH and other settings
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "MOCK_GIT_LS_REMOTE_MISSING="+os.Getenv("MOCK_GIT_LS_REMOTE_MISSING"))
	// Pipe stderr to see errors from the helper process during debugging
	cmd.Stderr = os.Stderr
	return cmd
}

func TestHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

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

	cmd, subargs := args[0], args[1:]

	switch cmd {
	case "gh":
		handleGhMock(subargs)
	case "git":
		handleGitMock(subargs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n", cmd)
		os.Exit(2)
	}
	os.Exit(0)
}

func handleGhMock(args []string) {
	if len(args) == 0 {
		os.Exit(0)
	}
	sub := args[0]
	switch sub {
	case "auth":
		os.Exit(0)
	case "repo":
		if len(args) > 1 && args[1] == "view" {
			// Mock response: {"viewerPermission": "ADMIN"}
			// args: repo view <url> --json viewerPermission -q .viewerPermission
			fmt.Print("ADMIN")
			os.Exit(0)
		}
	case "pr":
		// pr list
		if len(args) > 1 && args[1] == "list" {
			// args: pr list --repo <url> --head <branch> --json url -q .[0].url
			// Check if we want to return an existing PR based on env var maybe?
			// Or just return empty string by default.
			if os.Getenv("MOCK_PR_EXISTS") == "1" {
				fmt.Print("https://github.com/user/repo/pull/1")
			}
			os.Exit(0)
		}
		// pr create
		if len(args) > 1 && args[1] == "create" {
			// Output the URL
			fmt.Println("https://github.com/user/repo/pull/2")
			os.Exit(0)
		}
		// pr view
		if len(args) > 1 && args[1] == "view" {
			// Output body
			fmt.Print("Original Body")
			os.Exit(0)
		}
		// pr edit
		if len(args) > 1 && args[1] == "edit" {
			// Success
			os.Exit(0)
		}
	}
	// Default fail
	os.Exit(1)
}

func handleGitMock(args []string) {
	// Handle commands
	if len(args) >= 2 && args[0] == "rev-parse" {
		// rev-parse --abbrev-ref HEAD -> branch name
		if len(args) >= 3 && args[1] == "--abbrev-ref" && args[2] == "HEAD" {
			fmt.Print("master")
			os.Exit(0)
		}
		// rev-parse HEAD -> sha
		if len(args) >= 2 && args[1] == "HEAD" {
			fmt.Print("1234567890abcdef")
			os.Exit(0)
		}
	}

	if len(args) >= 3 && args[0] == "push" {
		// push origin master
		os.Exit(0)
	}

	if len(args) >= 4 && args[2] == "ls-remote" {
		// git -C repoDir ls-remote --heads origin <branch>
		// args: -C repoDir ls-remote --heads origin <branch>
		if os.Getenv("MOCK_GIT_LS_REMOTE_MISSING") == "1" {
			// Return empty
			os.Exit(0)
		}
		// Return dummy ref
		fmt.Println("hash\trefs/heads/branch")
		os.Exit(0)
	}

	// config
	if len(args) >= 3 && args[0] == "config" && args[1] == "--get" {
		fmt.Print("https://github.com/user/repo.git")
		os.Exit(0)
	}

	// Default success for unhandled commands to match real git behavior (which might exit 0 if config key missing? No, 1. But for mocks we want stability)
	// Actually, if we return success, it might proceed.
	os.Exit(0)
}

func TestCheckGhAvailability(t *testing.T) {
	oldExec := ExecCommand
	ExecCommand = fakeExecCommand
	oldLookPath := lookPath
	lookPath = func(_ string) (string, error) { return "/usr/bin/gh", nil }
	defer func() {
		ExecCommand = oldExec
		lookPath = oldLookPath
	}()

	// Test Success
	if err := checkGhAvailability("gh", false); err != nil {
		t.Errorf("Expected success, got %v", err)
	}
}

func TestVerifyGithubRequirements_Success(t *testing.T) {
	oldExec := ExecCommand
	ExecCommand = fakeExecCommand
	defer func() { ExecCommand = oldExec }()

	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)

	url := "https://github.com/user/repo.git"
	id := tmpDir // Use tmpDir as ID so getRepoDir returns it
	repo := Repository{ID: &id, URL: &url}
	repos := []Repository{repo}

	// Mock gh to return success
	existing, err := verifyGithubRequirements(repos, 1, "git", "gh", false, nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(existing) != 0 {
		t.Errorf("Expected no existing PRs, got %v", existing)
	}
}

func TestVerifyGithubRequirements_ExistingPR(t *testing.T) {
	oldExec := ExecCommand
	ExecCommand = fakeExecCommand
	defer func() { ExecCommand = oldExec }()

	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)

	url := "https://github.com/user/repo.git"
	id := tmpDir
	repo := Repository{ID: &id, URL: &url}
	repos := []Repository{repo}

	// Set env to mock existing PR
	os.Setenv("MOCK_PR_EXISTS", "1")
	defer os.Unsetenv("MOCK_PR_EXISTS")

	existing, err := verifyGithubRequirements(repos, 1, "git", "gh", false, nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if url, ok := existing[tmpDir]; !ok || url != "https://github.com/user/repo/pull/1" {
		t.Errorf("Expected existing PR URL, got %v", existing)
	}
}

func TestVerifyGithubRequirements_MissingBaseBranch(t *testing.T) {
	oldExec := ExecCommand
	ExecCommand = fakeExecCommand
	defer func() { ExecCommand = oldExec }()

	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)

	url := "https://github.com/user/repo.git"
	id := tmpDir
	branch := "missing-branch"
	repo := Repository{ID: &id, URL: &url, Branch: &branch}
	repos := []Repository{repo}

	// Set env to mock missing base branch for ls-remote
	os.Setenv("MOCK_GIT_LS_REMOTE_MISSING", "1")
	defer os.Unsetenv("MOCK_GIT_LS_REMOTE_MISSING")

	_, err := verifyGithubRequirements(repos, 1, "git", "gh", false, nil)
	if err == nil {
		t.Error("Expected error due to missing base branch, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "does not exist on remote") {
		t.Errorf("Expected error message about missing base branch, got: %v", err)
	}
}
