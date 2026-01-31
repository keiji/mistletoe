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

const (
	mockGitVersionOutput = "git version 2.30.0"
	mockGhVersionOutput  = "gh version 2.0.0 (2021-01-01)"
)

// TestVersionHelperProcess is a helper process for mocking exec.Command
func TestVersionHelperProcess(_ *testing.T) {
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
		if len(subArgs) >= 1 && subArgs[0] == "--version" {
			fmt.Println(mockGitVersionOutput)
			return
		}
	}

	// Mock `gh`
	if strings.Contains(cmd, "gh") {
		if len(subArgs) >= 1 && subArgs[0] == "--version" {
			fmt.Println(mockGhVersionOutput)
			return
		}
	}

	// Fail anything else
	fmt.Fprintf(os.Stderr, "Unknown command %q %v\n", cmd, subArgs)
	os.Exit(2)
}

func TestHandleVersionMstl(t *testing.T) {
	// Mock Stdout
	var buf bytes.Buffer
	oldStdout := sys.Stdout
	sys.Stdout = &buf
	defer func() { sys.Stdout = oldStdout }()

	// Mock LookPath
	oldLookPath := lookPath
	lookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}
	defer func() { lookPath = oldLookPath }()

	// Mock ExecCommand
	oldExec := sys.ExecCommand
	defer func() { sys.ExecCommand = oldExec }()
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestVersionHelperProcess", "--", name}
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

	err := handleVersionMstl(opts)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, AppName) {
		t.Errorf("expected output to contain AppName, got %s", output)
	}
	if !strings.Contains(output, mockGitVersionOutput) {
		t.Errorf("expected output to contain %q, got %s", mockGitVersionOutput, output)
	}
}

func TestHandleVersionGh(t *testing.T) {
	// Mock Stdout
	var buf bytes.Buffer
	oldStdout := sys.Stdout
	sys.Stdout = &buf
	defer func() { sys.Stdout = oldStdout }()

	// Mock LookPath
	oldLookPath := lookPath
	lookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}
	defer func() { lookPath = oldLookPath }()

	// Mock ExecCommand
	oldExec := sys.ExecCommand
	defer func() { sys.ExecCommand = oldExec }()
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestVersionHelperProcess", "--", name}
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

	err := handleVersionGh(opts)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, AppName) {
		t.Errorf("expected output to contain AppName, got %s", output)
	}
	if !strings.Contains(output, mockGitVersionOutput) {
		t.Errorf("expected output to contain %q, got %s", mockGitVersionOutput, output)
	}
	// mockGhVersionOutput contains the date, but our logic splits lines.
	// RunGh returns output. handleVersionGh splits by newline.
	// If output is "gh version ...", we expect that.
	// Just verify the main part.
	if !strings.Contains(output, "gh version 2.0.0") {
		t.Errorf("expected output to contain gh version 2.0.0, got %s", output)
	}
}
