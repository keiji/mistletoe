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
	oldExec := sys.ExecCommand
	sys.ExecCommand = mockExecFire
	defer func() { sys.ExecCommand = oldExec }()

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
		case "checkout":
			// git checkout -b mstl-fire-...
			// 1. First attempt (no suffix): Checkout succeeds
			// 2. We will simulate PUSH fail for this
			// 3. Retry with suffix -1: Checkout succeeds
			// 4. PUSH succeeds
			if len(args) >= 4 && args[2] == "-b" {
				branch := args[3]
				// We expect original OR suffix -1
				if !strings.HasSuffix(branch, "-1") && !strings.Contains(branch, "-") {
					// Logic check: if no suffix, it's fine.
					// But we need to ensure it IS the fire branch
				}

				// Just check prefix
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
				branch := args[4]
				// 1. Original (no suffix): Fail (exit 1) -> This triggers retry
				// 2. Suffix -1: Succeed (exit 0)
				if !strings.HasSuffix(branch, "-1") {
					os.Exit(1)
				}
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
