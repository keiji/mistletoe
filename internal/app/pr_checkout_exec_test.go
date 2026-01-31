package app

import (
	"bytes"
	"fmt"
	"mistletoe/internal/sys"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrCheckoutFullHelperProcess extends the helper process
func TestPrCheckoutFullHelperProcess(_ *testing.T) {
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

	cmd, subCmd := args[0], args[1:]

	// Mock `gh`
	if strings.Contains(cmd, "gh") {
		// gh --version
		if len(subCmd) > 0 && subCmd[0] == "--version" {
			fmt.Println("gh version 2.0.0")
			return
		}

		if len(subCmd) >= 2 && subCmd[0] == "auth" && subCmd[1] == "status" {
			return
		}
		if len(subCmd) >= 3 && subCmd[0] == "pr" && subCmd[1] == "view" {
			// Check flags
			jsonRequested := false
			stateRequested := false
			for i, arg := range subCmd {
				if arg == "--json" && i+1 < len(subCmd) {
					fields := subCmd[i+1]
					if strings.Contains(fields, "body") {
						jsonRequested = true
					}
					if strings.Contains(fields, "state") {
						stateRequested = true
					}
					// Don't break immediately if multiple flags are possible or repeated,
					// but for this mock, one --json is expected.
				}
			}

			if jsonRequested {
				// Return a fake body
				body := `
-------------
## Mistletoe
<details>
<summary>mistletoe-snapshot-123.json</summary>
` + "```json" + `
{
  "repositories": [
    {
      "url": "https://github.com/example/repo1",
      "branch": "feature/a",
      "id": "repo1"
    }
  ]
}
` + "```" + `
</details>
-------------
`
				// -q .body
				// Output raw
				fmt.Print(body)
				return
			}

			if stateRequested {
				fmt.Print("OPEN")
				return
			}
		}
	}

	// Mock `git`
	if strings.Contains(cmd, "git") {
		// PerformInit calls git clone, checkout, etc.
		// CollectStatus calls git status, rev-parse, etc.
		// We can just succeed for everything or be more specific.
		// git rev-parse --is-inside-work-tree
		if len(subCmd) >= 2 && subCmd[0] == "rev-parse" {
			if subCmd[1] == "--is-inside-work-tree" {
				fmt.Println("true")
				return
			}
			if subCmd[1] == "--show-toplevel" {
				wd, _ := os.Getwd()
				fmt.Println(wd)
				return
			}
			if subCmd[1] == "--abbrev-ref" { // current branch
				fmt.Println("feature/a")
				return
			}
		}

		// git remote get-url origin
		if len(subCmd) >= 3 && subCmd[0] == "remote" && subCmd[1] == "get-url" {
			fmt.Println("https://github.com/example/repo1")
			return
		}

		// git config --get remote.origin.url
		if len(subCmd) >= 3 && subCmd[0] == "config" && subCmd[1] == "--get" && subCmd[2] == "remote.origin.url" {
			fmt.Println("https://github.com/example/repo1")
			return
		}

		// git status --porcelain
		if len(subCmd) >= 2 && subCmd[0] == "status" && subCmd[1] == "--porcelain" {
			return // Clean
		}

		// git ls-remote
		if len(subCmd) >= 1 && subCmd[0] == "ls-remote" {
			// Returning hash
			fmt.Println("abc1234\trefs/heads/feature/a")
			return
		}

		// git fetch
		if len(subCmd) >= 1 && subCmd[0] == "fetch" {
			return
		}

		// git checkout
		if len(subCmd) >= 1 && subCmd[0] == "checkout" {
			return
		}

		// git log -1 --format=%h%n%H%n%D
		if len(subCmd) >= 1 && subCmd[0] == "log" {
			fmt.Println("1234567\n1234567890abcdef\nHEAD -> feature/a, origin/feature/a")
			return
		}

		return
	}

	fmt.Fprintf(os.Stderr, "Unknown command %q %v\n", cmd, subCmd)
	os.Exit(2)
}

func TestPrCheckoutCommand_Success(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "mstl_checkout_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// Mock lookPath
	oldLookPath := lookPath
	lookPath = func(file string) (string, error) {
		return "/mock/gh", nil
	}
	defer func() { lookPath = oldLookPath }()

	// Mock Stdout
	var buf bytes.Buffer
	oldStdout := sys.Stdout
	sys.Stdout = &buf
	defer func() { sys.Stdout = oldStdout }()

	// Mock ExecCommand
	oldExec := sys.ExecCommand
	defer func() { sys.ExecCommand = oldExec }()
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestPrCheckoutFullHelperProcess", "--", name}
		cs = append(cs, arg...)

		testBin, err := filepath.Abs(os.Args[0])
		if err != nil {
			testBin = os.Args[0] // Fallback
		}

		cmd := exec.Command(testBin, cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	// Create repo1 directory structure so PerformInit doesn't try to clone (which would do nothing in mock)
	// and fail on subsequent commands.
	if err := os.MkdirAll(filepath.Join(tempDir, "repo1", ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	opts := GlobalOptions{
		GitPath: "git",
		GhPath:  "./gh",
	}

	// -u is required
	args := []string{"-u", "https://github.com/example/repo1/pull/1", "-v"}

	err = prCheckoutCommand(args, opts)
	if err != nil {
		t.Errorf("prCheckoutCommand failed: %v", err)
	}

	// Verify output
	out := buf.String()
	// "Initializing repositories..." is printed to os.Stdout, not sys.Stdout (mocked), so we can't see it.
	// We check for the table output which IS printed to sys.Stdout.
	if !strings.Contains(out, "repo1") {
		t.Errorf("Expected table with repo1, got:\n%s", out)
	}
	if !strings.Contains(out, "feature/a") {
		t.Errorf("Expected table with feature/a, got:\n%s", out)
	}
}

// TestPrCheckoutFailHelperProcess returns a body without Mistletoe block
func TestPrCheckoutFailHelperProcess(_ *testing.T) {
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
		os.Exit(2)
	}

	cmd, subCmd := args[0], args[1:]

	if strings.Contains(cmd, "gh") {
		if len(subCmd) >= 3 && subCmd[0] == "pr" && subCmd[1] == "view" {
			// Return dummy body
			fmt.Println("No Mistletoe block here.")
			return
		}
		// gh --version etc
		if len(subCmd) > 0 && subCmd[0] == "--version" {
			fmt.Println("gh version 2.0.0")
			return
		}
		if len(subCmd) >= 2 && subCmd[0] == "auth" && subCmd[1] == "status" {
			return
		}
	}
	os.Exit(0)
}

func TestPrCheckoutCommand_NoBlock(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "mstl_checkout_fail")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// Mock lookPath
	oldLookPath := lookPath
	lookPath = func(file string) (string, error) {
		return "/mock/gh", nil
	}
	defer func() { lookPath = oldLookPath }()

	// Mock ExecCommand
	oldExec := sys.ExecCommand
	defer func() { sys.ExecCommand = oldExec }()
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestPrCheckoutFailHelperProcess", "--", name}
		cs = append(cs, arg...)

		testBin, err := filepath.Abs(os.Args[0])
		if err != nil {
			testBin = os.Args[0] // Fallback
		}

		cmd := exec.Command(testBin, cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	opts := GlobalOptions{
		GitPath: "git",
		GhPath:  "gh",
	}

	args := []string{"-u", "https://github.com/example/repo1/pull/1"}

	err = prCheckoutCommand(args, opts)
	if err == nil {
		t.Errorf("Expected error when block missing, got nil")
	} else if !strings.Contains(err.Error(), "Mistletoe block not found") {
		t.Errorf("Expected 'Mistletoe block not found', got: %v", err)
	}
}
