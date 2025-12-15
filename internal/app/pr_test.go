package app

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"strings"
)

// Mock execCommand
// Based on standard Go testing pattern for os/exec:
// https://npf.io/2015/06/testing-exec-command/

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	// Pass environment variables to identify specific test cases
	// Important: Append to os.Environ() to preserve PATH and other settings
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "MOCK_GIT_LS_REMOTE_MISSING="+os.Getenv("MOCK_GIT_LS_REMOTE_MISSING"))
	// Pipe stderr to see errors from the helper process during debugging
	cmd.Stderr = os.Stderr
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// Debug
	// fmt.Fprintf(os.Stderr, "DEBUG: Args: %v\n", os.Args)

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
	// We only mock git ls-remote in this helper, as other git commands are executed via RunGit (utils.go) which uses real git.
	// But in verifyGithubRequirements, we explicitly call execCommand for ls-remote, so it comes here.
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
	// Fallback to real git? No, we shouldn't execute real git from the mock if we don't intend to.
	// If verifyGithubRequirements called execCommand for other git commands, we would need to handle them.
	// But it only uses it for ls-remote.
	os.Exit(1)
}

func TestCheckGhAvailability(t *testing.T) {
	oldExec := execCommand
	execCommand = fakeExecCommand
	oldLookPath := lookPath
	lookPath = func(file string) (string, error) { return "/usr/bin/gh", nil }
	defer func() {
		execCommand = oldExec
		lookPath = oldLookPath
	}()

	// Test Success
	if err := checkGhAvailability("gh"); err != nil {
		t.Errorf("Expected success, got %v", err)
	}
}

func TestVerifyGithubRequirements_Success(t *testing.T) {
	oldExec := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExec }()

	tmpDir := t.TempDir()
	_, err := exec.Command("git", "init", tmpDir).Output()
	if err != nil {
		t.Fatal(err)
	}
	// Commit something so HEAD exists
	cmd := exec.Command("git", "-C", tmpDir, "commit", "--allow-empty", "-m", "init")
	// Set config for commit to work
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	url := "https://github.com/user/repo.git"
	id := tmpDir // Use tmpDir as ID so getRepoDir returns it
	repo := Repository{ID: &id, URL: &url}
	repos := []Repository{repo}

	// Mock gh to return success
	existing, err := verifyGithubRequirements(repos, 1, "git", "gh")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(existing) != 0 {
		t.Errorf("Expected no existing PRs, got %v", existing)
	}
}

func TestVerifyGithubRequirements_ExistingPR(t *testing.T) {
	oldExec := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExec }()

	tmpDir := t.TempDir()
	exec.Command("git", "init", tmpDir).Run()
	cmd := exec.Command("git", "-C", tmpDir, "commit", "--allow-empty", "-m", "init")
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	cmd.Run()

	url := "https://github.com/user/repo.git"
	id := tmpDir
	repo := Repository{ID: &id, URL: &url}
	repos := []Repository{repo}

	// Set env to mock existing PR
	os.Setenv("MOCK_PR_EXISTS", "1")
	defer os.Unsetenv("MOCK_PR_EXISTS")

	existing, err := verifyGithubRequirements(repos, 1, "git", "gh")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if url, ok := existing[tmpDir]; !ok || url != "https://github.com/user/repo/pull/1" {
		t.Errorf("Expected existing PR URL, got %v", existing)
	}
}

func TestVerifyGithubRequirements_MissingBaseBranch(t *testing.T) {
	oldExec := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExec }()

	tmpDir := t.TempDir()
	exec.Command("git", "init", tmpDir).Run()
	cmd := exec.Command("git", "-C", tmpDir, "commit", "--allow-empty", "-m", "init")
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	cmd.Run()

	url := "https://github.com/user/repo.git"
	id := tmpDir
	branch := "missing-branch"
	repo := Repository{ID: &id, URL: &url, Branch: &branch}
	repos := []Repository{repo}

	// Set env to mock missing base branch for ls-remote
	os.Setenv("MOCK_GIT_LS_REMOTE_MISSING", "1")
	defer os.Unsetenv("MOCK_GIT_LS_REMOTE_MISSING")

	_, err := verifyGithubRequirements(repos, 1, "git", "gh")
	if err == nil {
		t.Error("Expected error due to missing base branch, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "does not exist on remote") {
		t.Errorf("Expected error message about missing base branch, got: %v", err)
	}
}
