package app

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"mistletoe/internal/sys"
)

// TestSetupHelperProcess is a helper process for mocking exec.Command for setup checks
func TestSetupHelperProcess(_ *testing.T) {
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

	switch cmd {
	case "git":
		// git --version
		return
	case "gh":
		// gh --version
		if len(args) > 1 && args[1] == "--version" {
			return
		}
		// gh auth status
		if len(args) > 2 && args[1] == "auth" && args[2] == "status" {
			if os.Getenv("MOCK_GH_AUTH_FAIL") == "1" {
				os.Exit(1)
			}
			return
		}
	case "fail-cmd":
		os.Exit(1)
	}

	// Default behavior? Fail if not handled
	os.Exit(0)
}

func TestValidateGit(t *testing.T) {
	// Mock execCommand
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestSetupHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	defer func() { sys.ExecCommand = exec.Command }()

	if err := validateGit("git"); err != nil {
		t.Errorf("validateGit(git) failed: %v", err)
	}

	// Test failure
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestSetupHelperProcess", "--", "fail-cmd"}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	if err := validateGit("git"); err == nil {
		t.Error("validateGit(git) expected error, got nil")
	}
}

func TestValidateGh(t *testing.T) {
	// Mock execCommand
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestSetupHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	defer func() { sys.ExecCommand = exec.Command }()

	if err := validateGh("gh"); err != nil {
		t.Errorf("validateGh(gh) failed: %v", err)
	}

	// Test failure
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestSetupHelperProcess", "--", "fail-cmd"}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	if err := validateGh("gh"); err == nil {
		t.Error("validateGh(gh) expected error, got nil")
	}
}

func TestValidateGhAuth(t *testing.T) {
	// Mock execCommand
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestSetupHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	defer func() { sys.ExecCommand = exec.Command }()

	if err := validateGhAuth("gh"); err != nil {
		t.Errorf("validateGhAuth(gh) failed: %v", err)
	}

	// Test auth failure
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestSetupHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "MOCK_GH_AUTH_FAIL=1")
		return cmd
	}
	if err := validateGhAuth("gh"); err == nil {
		t.Error("validateGhAuth(gh) expected error, got nil")
	}
}
