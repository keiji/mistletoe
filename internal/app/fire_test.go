package app

import (
	"fmt"
	conf "mistletoe/internal/config"
	"mistletoe/internal/sys"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestFireCommand verifies the git sequence for the fire command.
func TestFireCommand(t *testing.T) {
	// Setup helper process for git
	sys.ExecCommand = mockExecFire
	defer func() { sys.ExecCommand = nil }()

	// We need to set USER or USERNAME for consistent branch naming in test
	os.Setenv("USER", "testuser")
	defer os.Unsetenv("USER")

	// Helper vars
	jobs := 1
	id := "repo1"

	// Create a temporary directory for the repo to simulate path
	tmpDir, err := os.MkdirTemp("", "mstl-fire-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	repoPath := strings.TrimSuffix(tmpDir, "/") // just in case

	config := &conf.Config{
		Jobs: &jobs,
		Repositories: &[]conf.Repository{
			{
				ID:   &id,
			},
		},
		BaseDir: repoPath,
	}

	// Ensure repo1 dir exists
	os.Mkdir(repoPath+"/repo1", 0755)

	opts := GlobalOptions{
		GitPath: "git",
	}

	// Let's set an env var to tell the helper process we are in "fire mode"
	os.Setenv("GO_TEST_FIRE_MODE", "true")
	defer os.Unsetenv("GO_TEST_FIRE_MODE")

	err = fireCommand(config, opts)
	if err != nil {
		t.Errorf("fireCommand returned error: %v", err)
	}
}

func mockExecFire(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessFire", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

// TestHelperProcessFire is the mock entry point.
// It must start with Test to be callable by -test.run
func TestHelperProcessFire(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	// Check if we are running the fire test
	if os.Getenv("GO_TEST_FIRE_MODE") != "true" {
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

	cmd := args[0]
	subCmd := ""
	if len(args) > 1 {
		subCmd = args[1]
	}

	// We expect git commands
	if cmd == "git" {
		switch subCmd {
		case "ls-remote":
			// git ls-remote --exit-code --heads origin <branch>
			// We want to simulate that the FIRST branch name exists (exit 0)
			// and the SECOND branch name (suffix -1) does not exist (exit 2).

			// Argument structure: git ls-remote --exit-code --heads origin <branch>
			// args[0] is git (consumed)
			// subCmd is ls-remote
			// args: ls-remote, --exit-code, --heads, origin, <branch>
			// So index 2: --exit-code, 3: --heads, 4: origin, 5: <branch>
			if len(args) >= 6 {
				branch := args[5]
				// If branch ends with -1, we say it's new (exit 2)
				if strings.HasSuffix(branch, "-1") {
					os.Exit(2)
				}
				// Otherwise (original name), we say it exists (exit 0)
				os.Exit(0)
			}
			os.Exit(2) // fallback

		case "checkout":
			// git checkout -b mstl-fire-...
			if len(args) >= 4 && args[2] == "-b" {
				branch := args[3]
				// We expect the RETRIED branch name (suffix -1)
				if !strings.HasSuffix(branch, "-1") {
					fmt.Fprintf(os.Stderr, "Expected branch with -1 suffix, got: %s\n", branch)
					os.Exit(1)
				}
				// We relax the test to match just the prefix since the username depends on env
				if !strings.HasPrefix(branch, "mstl-fire-repo1-") {
					fmt.Fprintf(os.Stderr, "Unexpected branch format: %s\n", branch)
					os.Exit(1)
				}
				// Success
				os.Exit(0)
			}
		case "add":
			// git add .
			if len(args) >= 3 && args[2] == "." {
				os.Exit(0)
			}
		case "commit":
			// git commit -m ... --no-gpg-sign
			// check for --no-gpg-sign
			hasNoGpg := false
			for _, a := range args {
				if a == "--no-gpg-sign" {
					hasNoGpg = true
				}
			}
			if !hasNoGpg {
				fmt.Fprintf(os.Stderr, "Missing --no-gpg-sign\n")
				os.Exit(1)
			}
			os.Exit(0)
		case "push":
			// git push -u origin <branch>
			if len(args) >= 5 && args[2] == "-u" && args[3] == "origin" {
				os.Exit(0)
			}
		}
		// If we are here, we might have got a command we didn't explicitly handle or verify
		// strictly, or it's a version check.
		// Allow version/help commands
		if subCmd == "--version" {
			os.Exit(0)
		}

		fmt.Fprintf(os.Stderr, "Unexpected git command: %v\n", args)
		os.Exit(1)
	}

	os.Exit(0)
}
