package app

import (
	"mistletoe/internal/sys"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdatePrDescriptions_HelperProcess(_ *testing.T) {
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
		// api user
		if len(subCmd) >= 3 && subCmd[0] == "api" && subCmd[1] == "user" {
			os.Stdout.WriteString("me") // currentUser
			return
		}

		// api graphql
		if len(subCmd) >= 2 && subCmd[0] == "api" && subCmd[1] == "graphql" {
			// Return JSON response for viewerCanEditFiles etc.
			// Check arguments if needed, but for coverage simple return is enough
			resp := `{"data": {"repository": {"pullRequest": {"body": "Old Body", "viewerCanEditFiles": true, "author": {"login": "me"}}}}}`
			os.Stdout.WriteString(resp)
			return
		}

		// pr edit
		if len(subCmd) >= 2 && subCmd[0] == "pr" && subCmd[1] == "edit" {
			return // success
		}
	}

	os.Exit(1)
}

func TestUpdatePrDescriptions(t *testing.T) {
	// Mock ExecCommand
	oldExec := sys.ExecCommand
	defer func() { sys.ExecCommand = oldExec }()
	sys.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestUpdatePrDescriptions_HelperProcess", "--", name}
		cs = append(cs, arg...)

		testBin, err := filepath.Abs(os.Args[0])
		if err != nil {
			testBin = os.Args[0] // Fallback
		}

		cmd := exec.Command(testBin, cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	prMap := map[string][]PrInfo{
		"repo1": {{URL: "https://github.com/org/repo1/pull/1", State: "OPEN"}},
	}

	err := updatePrDescriptions(prMap, 1, "gh", false, "{}", "snapshot.json", nil, "", false)
	if err != nil {
		t.Errorf("updatePrDescriptions failed: %v", err)
	}
}
