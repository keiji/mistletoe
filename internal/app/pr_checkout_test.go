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
	config, _, found := ParseMistletoeBlock(body)
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
	config2, related, found2 := ParseMistletoeBlock(bodyRelated)
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
