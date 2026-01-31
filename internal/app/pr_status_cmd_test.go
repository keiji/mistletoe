package app

import (
	"bytes"
	"mistletoe/internal/sys"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrStatusCommand(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "mstl_pr_status_test")
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
	// Needs .git
	if err := os.Mkdir(filepath.Join(tempDir, "repo1", ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Mock Stdout
	var buf bytes.Buffer
	oldStdout := sys.Stdout
	sys.Stdout = &buf
	defer func() { sys.Stdout = oldStdout }()

    // Mock lookPath
	oldLookPath := lookPath
	lookPath = func(file string) (string, error) {
		return "/mock/gh", nil
	}
	defer func() { lookPath = oldLookPath }()

	// Mock ExecCommand using TestPrCheckoutFullHelperProcess (reusing the one from pr_checkout_exec_test.go)
    // Since they are in the same package 'app', we can refer to it.
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

	opts := GlobalOptions{
		GitPath: "git",
		GhPath:  "gh",
	}

	err = prStatusCommand([]string{}, opts)
	if err != nil {
		t.Errorf("prStatusCommand failed: %v", err)
	}

	// Verify output
	out := buf.String()
	if !strings.Contains(out, "repo1") {
		t.Errorf("Expected table with repo1, got:\n%s", out)
	}
}
