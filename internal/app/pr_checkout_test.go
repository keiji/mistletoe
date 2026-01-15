package app


import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"strings"
)

// TestCheckoutHelperProcess is a helper process for mocking exec.Command
func TestCheckoutHelperProcess(_ *testing.T) {
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
	if strings.HasSuffix(cmd, "gh") {
		if len(subCmd) >= 2 && subCmd[0] == "auth" && subCmd[1] == "status" {
			// Success
			return
		}
		if len(subCmd) >= 3 && subCmd[0] == "pr" && subCmd[1] == "view" {
			// Mock `gh pr view <url> --json body [-q .body]`
			// Check if json body requested
			jsonRequested := false
			queryBody := false
			for _, arg := range subCmd {
				if arg == "body" {
					jsonRequested = true
				}
				if arg == ".body" {
					queryBody = true
				}
			}

			if jsonRequested {
				// Return a fake body with Mistletoe block
				body := `
Some description...

------------------
## Mistletoe
<details>
<summary>mistletoe-snapshot-1234.json</summary>

` + "```json" + `
{
  "repositories": [
    {
      "url": "https://github.com/example/repo1",
      "revision": "hash1",
      "branch": "feature/a"
    }
  ]
}
` + "```" + `
</details>

------------------
`
				if queryBody {
					// Output raw body
					fmt.Print(body)
				} else {
					// Output JSON wrapped
					fmt.Printf("{\"body\": %q}", body)
				}
				return
			}
		}
	}

	// Mock `git`
	if strings.HasSuffix(cmd, "git") {
		// Just succeed for typical git commands in this test context
		return
	}

	// Fail anything else
	fmt.Fprintf(os.Stderr, "Unknown command %q\n", args)
	os.Exit(2)
}

func TestHandlePrCheckout(t *testing.T) {
	// Swap ExecCommand
	ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCheckoutHelperProcess", "--", name}
		cs = append(cs, arg...)

		// Ensure executable path is absolute to handle RunGit changing cmd.Dir
		testBin, err := filepath.Abs(os.Args[0])
		if err != nil {
			testBin = os.Args[0] // Fallback
		}

		cmd := exec.Command(testBin, cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	defer func() { ExecCommand = exec.Command }()

	// We verify parsing logic via public ParseMistletoeBlock
	// Note: ParseMistletoeBlock requires separators.
	body := `
Some description...

------------------
## Mistletoe
<details>
<summary>mistletoe-snapshot-1234.json</summary>

` + "```json" + `
{
  "repositories": [
    {
      "url": "https://github.com/example/repo1",
      "revision": "hash1",
      "branch": "feature/a"
    }
  ]
}
` + "```" + `
</details>

------------------
`
	config, _, _, found := ParseMistletoeBlock(body)
	if !found {
		t.Fatalf("ParseMistletoeBlock failed: not found")
	}
	if len(*config.Repositories) != 1 {
		t.Errorf("Expected 1 repo, got %d", len(*config.Repositories))
	}
	repo := (*config.Repositories)[0]
	if *repo.URL != "https://github.com/example/repo1" {
		t.Errorf("Unexpected URL: %s", *repo.URL)
	}

	// We also verify Related PR JSON parsing if present
	// We inject related PR JSON *before* the bottom separator.
	bodyRelated := `
Some description...

------------------
## Mistletoe
<details>
<summary>mistletoe-snapshot-1234.json</summary>

` + "```json" + `
{
  "repositories": [
    {
      "url": "https://github.com/example/repo1",
      "revision": "hash1",
      "branch": "feature/a"
    }
  ]
}
` + "```" + `
</details>

<details>
<summary>mistletoe-related-pr-1234.json</summary>

` + "```json" + `
{
	"dependencies": ["http://a.com"]
}
` + "```" + `
</details>

------------------
`
	config2, related, _, found2 := ParseMistletoeBlock(bodyRelated)
	if !found2 {
		t.Fatalf("ParseMistletoeBlock failed: not found")
	}
	if len(*config2.Repositories) != 1 {
		t.Errorf("Expected 1 repo")
	}
	if len(related) == 0 {
		t.Errorf("Expected related JSON")
	}
	var relMap map[string]interface{}
	if err := json.Unmarshal(related, &relMap); err != nil {
		t.Errorf("Invalid related JSON")
	}
}

func TestWriteDependencyFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mstl_test_dep")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	content := "A --> B\nB --> C"

	// Create a dummy mstl dir
	mstlDir := filepath.Join(tempDir, ".mstl")
	if err := os.Mkdir(mstlDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := writeDependencyFile(mstlDir, content); err != nil {
		t.Fatalf("writeDependencyFile failed: %v", err)
	}

	depFile := filepath.Join(mstlDir, "dependency-graph.md")
	data, err := os.ReadFile(depFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	writtenContent := string(data)
	expectedPrefix := "```mermaid\n"

	if !strings.HasPrefix(writtenContent, expectedPrefix) {
		t.Errorf("Content should start with mermaid block. Got:\n%q", writtenContent)
	}
	if !strings.Contains(writtenContent, content) {
		t.Errorf("Content should contain original graph. Got:\n%q", writtenContent)
	}
	if !strings.HasSuffix(strings.TrimSpace(writtenContent), "```") {
		t.Errorf("Content should end with mermaid block. Got:\n%q", writtenContent)
	}
}

func TestPrCheckoutCommand_Flags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantError     bool
		errorContains string
	}{
		{
			name:          "Invalid flag",
			args:          []string{"--invalid-flag"},
			wantError:     true,
			errorContains: "flag provided but not defined",
		},
		{
			name:          "No URL",
			args:          []string{},
			wantError:     true,
			errorContains: "URL required",
		},
		{
			name:          "Duplicate flags (alias mismatch)",
			args:          []string{"-j", "1", "--jobs", "2"},
			wantError:     true,
			errorContains: "options --jobs and -j cannot be specified with different values",
		},
		{
			name:          "Valid flags - minimal (execution fails at gh check)",
			args:          []string{"-u", "http://github.com/pr/1"},
			wantError:     true,
			errorContains: "command not found", // gh not found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a dummy gh path to ensure failure regardless of environment
			ghPath := "gh"
			if tt.errorContains == "command not found" {
				ghPath = "dummy-gh-executable"
			}
			err := prCheckoutCommand(tt.args, GlobalOptions{GitPath: "git", GhPath: ghPath})
			if (err != nil) != tt.wantError {
				t.Errorf("prCheckoutCommand() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if err != nil && tt.errorContains != "" {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("error = %v, want error containing %q", err, tt.errorContains)
				}
			}
		})
	}
}
