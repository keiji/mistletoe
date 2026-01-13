package app

import (
	conf "mistletoe/internal/config"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Reusing checkCommitCount from init_depth_test.go if compiled together.
// To avoid "redeclared" error if they are in the same package scope during `go test ./...`,
// we will verify if we can use the one from init_depth_test.go.
// Since we are creating a new test file in package `app`, we have access to other non-test files in `app`,
// but test files (`_test.go`) are only visible if included in the compilation.
// `go test ./...` compiles all `_test.go` files in the package.
// So `checkCommitCount` from `init_depth_test.go` might conflict if I redefine it, or be available if I don't.
// `checkCommitCount` in `init_depth_test.go` is NOT exported (lowercase).
// But it IS in the same package `app`.
// Go allows accessing unexported symbols from other files in the same package.
// However, `init_depth_test.go` is a test file. Symbols in test files are only available to other test files in the same package (usually).
// So `checkCommitCount` should be visible if we just use it.
// BUT, if I redefine it, it's a compile error.
// To be safe and avoid dependency on `init_depth_test.go` being present/unchanged, I will use a different name.

func checkCommitCountForCheckout(t *testing.T, dir string, expected int) {
	cmd := exec.Command("git", "-C", dir, "rev-list", "--count", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to count commits in %s: %v", dir, err)
	}
	count := strings.TrimSpace(string(out))
	if count != fmt.Sprintf("%d", expected) {
		t.Errorf("expected %d commits, got %s", expected, count)
	}
}

// TestHandlePrCheckoutDepth tests the --depth flag in pr checkout.
func TestHandlePrCheckoutDepth(t *testing.T) {
	// 1. Build mstl-gh binary
	binPath := buildMstlGh(t)

	// 2. Setup Remote Repo with 5 commits
	repoURL, _ := setupRemoteAndContent(t, 5)

	// 3. Prepare Snapshot
	repoID := "checkout-repo"
	master := "master"
	snapshotConfig := conf.Config{
		Repositories: &[]conf.Repository{
			{URL: &repoURL, ID: &repoID, Branch: &master},
		},
	}
	snapshotBytes, _ := json.Marshal(snapshotConfig)

	snapshotFilename := "mistletoe-snapshot-test.json"

	// Mistletoe Block Construction
	// Using backticks for code blocks.
	backtick := "`"
	jsonBlock := backtick + backtick + backtick + "json" + "\n" + string(snapshotBytes) + "\n" + backtick + backtick + backtick

	mistletoeBlock := fmt.Sprintf(`
Check this out.
----
## Mistletoe
<details>
<summary>%s</summary>

%s

</details>
----
`, snapshotFilename, jsonBlock)

	// 4. Create Mock gh
	mockGh := filepath.Join(t.TempDir(), "gh")
	if os.Getenv("GOOS") == "windows" {
		mockGh += ".exe"
	}

	writeGoMockGhBase64(t, mockGh, mistletoeBlock)

	// 5. Run pr checkout
	workDir := t.TempDir()

	// Add mock gh to PATH
	env := os.Environ()
	env = prependPath(env, filepath.Dir(mockGh))

	// Run
	cmd := exec.Command(binPath, "pr", "checkout", "--url", "https://github.com/org/repo/pull/1", "--depth", "2", "-v")
	cmd.Dir = workDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("mstl-gh pr checkout failed: %v", err)
	}

	// 6. Verify Depth
	targetRepo := filepath.Join(workDir, repoID)
	checkCommitCountForCheckout(t, targetRepo, 2)
}

func buildMstlGh(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	binName := "mstl-gh"
	if os.Getenv("GOOS") == "windows" {
		binName += ".exe"
	}
	outputPath := filepath.Join(tempDir, binName)

	// Build cmd/mstl-gh
	// We assume we are in internal/app/
	cmd := exec.Command("go", "build", "-o", outputPath, "../../cmd/mstl-gh")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build mstl-gh: %v", err)
	}
	return outputPath
}

func writeGoMockGhBase64(t *testing.T, path string, bodyResponse string) {
	// Encode bodyResponse to Base64 to avoid escaping issues
	encoded := base64.StdEncoding.EncodeToString([]byte(bodyResponse))

	src := fmt.Sprintf(`package main
import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)
func main() {
	args := strings.Join(os.Args[1:], " ")
	if strings.Contains(args, "pr view") {
		if strings.Contains(args, "body") {
			data, _ := base64.StdEncoding.DecodeString("%s")
			fmt.Print(string(data))
			return
		}
		if strings.Contains(args, "state") {
			fmt.Println("OPEN")
			return
		}
	}
	// Default success
	os.Exit(0)
}
`, encoded)

	srcFile := path + ".go"
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "build", "-o", path, srcFile)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}

func prependPath(env []string, newPath string) []string {
	found := false
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = "PATH=" + newPath + string(os.PathListSeparator) + e[5:]
			found = true
			break
		}
	}
	if !found {
		env = append(env, "PATH="+newPath)
	}
	return env
}
