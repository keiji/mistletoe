package app

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// Mock execCommand
// Based on standard Go testing pattern for os/exec:
// https://npf.io/2015/06/testing-exec-command/

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	// Pass environment variables to identify specific test cases
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
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
	if err := checkGhAvailability(); err != nil {
		t.Errorf("Expected success, got %v", err)
	}
}

func TestVerifyGithubRequirements_Success(t *testing.T) {
	oldExec := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExec }()

	// Need to use RunGit which uses real git... verifyGithubRequirements uses RunGit to get branch.
	// But `RunGit` uses `exec.Command` directly in `utils.go`.
	// We cannot mock `RunGit` easily unless we refactor `utils.go` too or use a dummy repo.
	// `pr.go` imports `RunGit` from same package `app`.

	// Since we are in the same package `app`, we can't shadow `RunGit` unless we define it as a var.
	// `RunGit` is a function.
	// But `verifyGithubRequirements` takes `gitPath`.
	// If we provide a fake git path, `RunGit` will fail unless that fake path is executable.

	// Strategy: Create a dummy git repo for the test, so real `git` works.
	// We only need `rev-parse --abbrev-ref HEAD` to work.

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
	existing, err := verifyGithubRequirements(repos, 1, "git")
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

	existing, err := verifyGithubRequirements(repos, 1, "git")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if url, ok := existing[tmpDir]; !ok || url != "https://github.com/user/repo/pull/1" {
		t.Errorf("Expected existing PR URL, got %v", existing)
	}
}

func TestFilterRepositories(t *testing.T) {
	url := "http://example.com"
	id1 := "repo1"
	id2 := "repo2"
	r1 := Repository{ID: &id1, URL: &url}
	r2 := Repository{ID: &id2, URL: &url}

	config := &Config{
		Repositories: &[]Repository{r1, r2},
	}

	ignored := map[string]bool{"repo1": true}

	filtered := filterRepositories(config, ignored)

	if len(filtered) != 1 {
		t.Errorf("Expected 1 repo, got %d", len(filtered))
	}
	if *filtered[0].ID != "repo2" {
		t.Errorf("Expected repo2, got %s", *filtered[0].ID)
	}
}
