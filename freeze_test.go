package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary
	if runtime.GOOS == "windows" {
		binaryPath = filepath.Join(os.TempDir(), "gitc-test.exe")
	} else {
		binaryPath = filepath.Join(os.TempDir(), "gitc-test")
	}

	// Build command
	// We are running this from the repo root usually, so "." should work.
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to build binary: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	os.Remove(binaryPath)
	os.Exit(code)
}

// Helper to create a fully set up dummy git repo
func setupDummyRepo(t *testing.T, dir, remoteURL, branchName string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create dir %s: %v", dir, err)
	}

	cmds := [][]string{
		{"init"},
		{"remote", "add", "origin", remoteURL},
		{"checkout", "-b", branchName},
		// Need a commit to have a valid HEAD for rev-parse
		{"commit", "--allow-empty", "-m", "initial commit"},
	}

	for _, args := range cmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		// Git requires user config to commit
		if args[0] == "commit" {
			cmd.Env = append(os.Environ(),
				"GIT_AUTHOR_NAME=Test",
				"GIT_AUTHOR_EMAIL=test@example.com",
				"GIT_COMMITTER_NAME=Test",
				"GIT_COMMITTER_EMAIL=test@example.com",
			)
		}
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to run git %v in %s: %v", args, dir, err)
		}
	}
}

func TestFreeze(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "gitc-freeze-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Setup repos in tmpDir
	repo1Dir := filepath.Join(tmpDir, "repo1")
	repo1URL := "https://github.com/example/repo1.git"
	repo1Branch := "main"
	setupDummyRepo(t, repo1Dir, repo1URL, repo1Branch)

	repo2Dir := filepath.Join(tmpDir, "repo2")
	repo2URL := "https://github.com/example/repo2.git"
	repo2Branch := "develop"
	setupDummyRepo(t, repo2Dir, repo2URL, repo2Branch)

	// Create a non-git dir
	if err := os.Mkdir(filepath.Join(tmpDir, "not-git"), 0755); err != nil {
		t.Fatalf("failed to create non-git dir: %v", err)
	}

	// Run freeze
	outputFile := "frozen.json"

	cmd := exec.Command(binaryPath, "freeze", "-f", outputFile)
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("freeze command failed: %v\nOutput: %s", err, out)
	}

	// Verify output file exists
	outputFilePath := filepath.Join(tmpDir, outputFile)
	if _, err := os.Stat(outputFilePath); os.IsNotExist(err) {
		t.Fatalf("output file was not created")
	}

	// Read and verify content
	data, err := os.ReadFile(outputFilePath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	config, err := ParseConfig(data)
	if err != nil {
		t.Fatalf("failed to parse output json: %v", err)
	}

	if len(config.Repositories) != 2 {
		t.Errorf("expected 2 repos, got %d", len(config.Repositories))
	}

	repoMap := make(map[string]Repository)
	for _, r := range config.Repositories {
		if r.ID == nil {
			t.Error("repo ID is nil")
			continue
		}
		repoMap[*r.ID] = r
	}

	// Check repo1
	// Note: r.ID is the directory name. Since we created "repo1" inside tmpDir,
	// and ran freeze inside tmpDir, the dir name scanned is "repo1".
	r1, ok := repoMap["repo1"]
	if !ok {
		t.Errorf("repo1 not found in %v", repoMap)
	} else {
		if r1.URL != repo1URL {
			t.Errorf("repo1 URL mismatch: got %s, want %s", r1.URL, repo1URL)
		}
		if r1.Branch == nil || *r1.Branch != repo1Branch {
			got := "<nil>"
			if r1.Branch != nil {
				got = *r1.Branch
			}
			t.Errorf("repo1 branch mismatch: got %s, want %s", got, repo1Branch)
		}
		if len(r1.Labels) != 0 {
			t.Errorf("repo1 labels not empty")
		}
	}

	r2, ok := repoMap["repo2"]
	if !ok {
		t.Errorf("repo2 not found")
	} else {
		if r2.URL != repo2URL {
			t.Errorf("repo2 URL mismatch: got %s, want %s", r2.URL, repo2URL)
		}
		if r2.Branch == nil || *r2.Branch != repo2Branch {
			got := "<nil>"
			if r2.Branch != nil {
				got = *r2.Branch
			}
			t.Errorf("repo2 branch mismatch: got %s, want %s", got, repo2Branch)
		}
	}
}

func TestFreeze_FileExists(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "gitc-freeze-fail")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create existing output file
	outputFile := "existing.json"
	fullOutputPath := filepath.Join(tmpDir, outputFile)
	if err := os.WriteFile(fullOutputPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create existing output file: %v", err)
	}

	cmd := exec.Command(binaryPath, "freeze", "-f", outputFile)
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()

	// Expect failure
	if err == nil {
		t.Errorf("expected command to fail when file exists, but it succeeded. Output: %s", out)
	}
}
