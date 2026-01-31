package app

import (
	"fmt"
	"mistletoe/internal/sys"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetGhUser(t *testing.T) {
	// Mock ExecCommand
	oldExec := sys.ExecCommand
	defer func() { sys.ExecCommand = oldExec }()
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestPrCommonHelperProcess", "--", name}
		cs = append(cs, arg...)

		testBin, err := filepath.Abs(os.Args[0])
		if err != nil {
			testBin = os.Args[0] // Fallback
		}

		cmd := exec.Command(testBin, cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	user, err := GetGhUser("gh", false)
	if err != nil {
		t.Errorf("GetGhUser failed: %v", err)
	}
	if user != "testuser" {
		t.Errorf("Expected testuser, got %s", user)
	}
}

func TestPrCommonHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	// Parse args...
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
		if len(subCmd) >= 2 && subCmd[0] == "api" && subCmd[1] == "user" {
			fmt.Print("testuser")
			return
		}
	}
	fmt.Fprintf(os.Stderr, "Unknown command %q %v\n", cmd, subCmd)
	os.Exit(1)
}
