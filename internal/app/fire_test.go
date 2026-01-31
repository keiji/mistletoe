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

// TestFireHelperProcess is a helper process for mocking exec.Command
func TestFireHelperProcess(_ *testing.T) {
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

	cmd, subArgs := args[0], args[1:]

	// Mock `git`
	if strings.Contains(cmd, "git") {
		// handleFire calls:
		// checkout -b ...
		// add .
		// commit -m ...
		// push -u ...

		// Return success for all
		return
	}

	// Fail anything else
	fmt.Fprintf(os.Stderr, "Unknown command %q %v\n", cmd, subArgs)
	os.Exit(2)
}

// TestFireHelperProcessRetry mocks retry scenario
func TestFireHelperProcessRetry(_ *testing.T) {
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

	cmd, subArgs := args[0], args[1:]

	if strings.Contains(cmd, "git") {
		// push -u origin branchName
		if len(subArgs) >= 4 && subArgs[0] == "push" {
			branch := subArgs[3]
			// Fail unless branch ends with "-1" (retry)
			if !strings.HasSuffix(branch, "-1") {
				fmt.Fprintln(os.Stderr, "remote branch exists")
				os.Exit(1)
			}
			return
		}
		// Allow other commands
		return
	}
	os.Exit(2)
}

func TestHandleFire(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "mstl_fire_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp dir
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// Create mistletoe.json
	configContent := `
    {
        "repositories": [
            {
                "url": "https://github.com/example/repo1",
                "id": "repo1"
            }
        ]
    }
    `
	// fire looks for .mstl/config.json or similar.
	mstlDir := filepath.Join(tempDir, ".mstl")
	if err := os.Mkdir(mstlDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mstlDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create repo directory
	if err := os.Mkdir(filepath.Join(tempDir, "repo1"), 0755); err != nil {
		t.Fatal(err)
	}

	// Mock Stdout/Stderr
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	oldStdout := sys.Stdout
	oldStderr := sys.Stderr
	sys.Stdout = &outBuf
	sys.Stderr = &errBuf
	defer func() {
		sys.Stdout = oldStdout
		sys.Stderr = oldStderr
	}()

	// Mock ExecCommand
	oldExec := sys.ExecCommand
	defer func() { sys.ExecCommand = oldExec }()
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestFireHelperProcess", "--", name}
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
	}

	err = handleFire([]string{}, opts)
	if err != nil {
		t.Errorf("handleFire failed: %v", err)
	}

	output := outBuf.String()
	if !strings.Contains(output, "FIRE command initiated") {
		t.Errorf("Expected initiation message, got: %s", output)
	}
	if !strings.Contains(output, "[repo1] Secured in") {
		t.Errorf("Expected secure message for repo1, got: %s", output)
	}
}

func TestHandleFire_NoConfig(t *testing.T) {
	// Test when no config is found
	tempDir, err := os.MkdirTemp("", "mstl_fire_fail")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// handleFire should fail when it tries to load the non-existent config
	opts := GlobalOptions{GitPath: "git"}
	err = handleFire([]string{}, opts)
	if err == nil {
		t.Errorf("Expected error when no config found, got nil")
	}
}

func TestHandleFire_Retry(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "mstl_fire_test_retry")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// Create mistletoe.json
	configContent := `
    {
        "repositories": [
            {
                "url": "https://github.com/example/repo1",
                "id": "repo1"
            }
        ]
    }
    `
	mstlDir := filepath.Join(tempDir, ".mstl")
	if err := os.Mkdir(mstlDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mstlDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create repo directory
	if err := os.Mkdir(filepath.Join(tempDir, "repo1"), 0755); err != nil {
		t.Fatal(err)
	}

	// Mock Stdout/Stderr
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	oldStdout := sys.Stdout
	oldStderr := sys.Stderr
	sys.Stdout = &outBuf
	sys.Stderr = &errBuf
	defer func() {
		sys.Stdout = oldStdout
		sys.Stderr = oldStderr
	}()

	// Mock ExecCommand with Retry Helper
	oldExec := sys.ExecCommand
	defer func() { sys.ExecCommand = oldExec }()
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestFireHelperProcessRetry", "--", name}
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
	}

	err = handleFire([]string{}, opts)
	if err != nil {
		t.Errorf("handleFire failed: %v", err)
	}

	output := outBuf.String()
	// Should see "Retrying..." in output? No, that's in Stderr.
	// We capture Stderr too.
	errOutput := errBuf.String()
	if !strings.Contains(errOutput, "Retrying with new branch") {
		t.Errorf("Expected retry message in stderr, got: %s", errOutput)
	}

	// Should see final success
	if !strings.Contains(output, "[repo1] Secured in") {
		t.Errorf("Expected secure message for repo1, got: %s", output)
	}
}
