package app

import (
	conf "mistletoe/internal/config"
)

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"strings"
)

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

func runHandleSnapshot(t *testing.T, args []string, workDir string) (string, string, int) {
	var stdoutBuf, stderrBuf bytes.Buffer
	originalStdout, originalStderr := Stdout, Stderr
	originalOsExit := osExit
	defer func() {
		Stdout, Stderr = originalStdout, originalStderr
		osExit = originalOsExit
	}()
	Stdout = &stdoutBuf
	Stderr = &stderrBuf

	// Mock Stdin
	Stdin = strings.NewReader("")

	exitCode := 0
	osExit = func(code int) {
		exitCode = code
		panic("os.Exit called")
	}
	defer func() { recover() }()

	cwd, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(cwd)

	// Append --ignore-stdin
	fullArgs := append(args, "--ignore-stdin")
	handleSnapshot(fullArgs, GlobalOptions{GitPath: "git"})

	return stdoutBuf.String(), stderrBuf.String(), exitCode
}

func TestSnapshot(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "mstl-snapshot-test")
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

	// Run snapshot
	outputFile := "snapshot.json"

	_, _, code := runHandleSnapshot(t, []string{"-o", outputFile, "-f", ""}, tmpDir)
	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
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

	config, err := conf.ParseConfig(data)
	if err != nil {
		t.Fatalf("failed to parse output json: %v", err)
	}

	if len(*config.Repositories) != 2 {
		t.Errorf("expected 2 repos, got %d", len(*config.Repositories))
	}

	repoMap := make(map[string]conf.Repository)
	for _, r := range *config.Repositories {
		if r.ID == nil {
			t.Error("repo ID is nil")
			continue
		}
		repoMap[*r.ID] = r
	}

	r1, ok := repoMap["repo1"]
	if !ok {
		t.Errorf("repo1 not found")
	} else {
		if *r1.URL != repo1URL {
			t.Errorf("repo1 URL mismatch: got %s, want %s", *r1.URL, repo1URL)
		}
		if r1.Branch == nil || *r1.Branch != repo1Branch {
			got := "<nil>"
			if r1.Branch != nil {
				got = *r1.Branch
			}
			t.Errorf("repo1 branch mismatch: got %s, want %s", got, repo1Branch)
		}
	}

	r2, ok := repoMap["repo2"]
	if !ok {
		t.Errorf("repo2 not found")
	} else {
		if *r2.URL != repo2URL {
			t.Errorf("repo2 URL mismatch: got %s, want %s", *r2.URL, repo2URL)
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

func TestSnapshot_DefaultFilename(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mstl-snapshot-default")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo1Dir := filepath.Join(tmpDir, "repo1")
	repo1URL := "https://github.com/example/repo1.git"
	repo1Branch := "main"
	setupDummyRepo(t, repo1Dir, repo1URL, repo1Branch)

	_, _, code := runHandleSnapshot(t, []string{"-f", ""}, tmpDir)
	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}

	found := false
	for _, f := range files {
		if !f.IsDir() {
			name := f.Name()
			if len(name) > 19 && name[:19] == "mistletoe-snapshot-" && name[len(name)-5:] == ".json" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("default snapshot file not found in %v", files)
	}
}

func TestSnapshot_FileExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mstl-snapshot-fail")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outputFile := "existing.json"
	fullOutputPath := filepath.Join(tmpDir, outputFile)
	if err := os.WriteFile(fullOutputPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create existing output file: %v", err)
	}

	_, stderr, code := runHandleSnapshot(t, []string{"-o", outputFile}, tmpDir)

	if code != 1 {
		t.Errorf("Expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "exists") {
		t.Errorf("Expected exists error")
	}
}

// Keep TestGenerateSnapshot_ExcludesJobs as is (it calls GenerateSnapshotVerbose directly)
func TestGenerateSnapshot_ExcludesJobs(t *testing.T) {
	// ... (Existing implementation was correct, calling exported function)

	tmpDir, err := os.MkdirTemp("", "mstl-gen-snapshot-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "repo1")
	repoURL := "https://github.com/example/repo1.git"
	repoBranch := "main"
	setupDummyRepo(t, repoDir, repoURL, repoBranch)

	repoID := "repo1"
	repoURLPtr := &repoURL
	jobsVal := 5

	inputConfig := &conf.Config{
		Jobs: &jobsVal,
		Repositories: &[]conf.Repository{
			{
				ID: &repoID,
				URL: repoURLPtr,
			},
		},
		BaseDir: tmpDir,
	}

	jsonBytes, _, err := GenerateSnapshotVerbose(inputConfig, "git", false)
	if err != nil {
		t.Fatalf("GenerateSnapshotVerbose failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.Fatalf("failed to unmarshal generated json: %v", err)
	}

	if _, ok := result["jobs"]; ok {
		t.Errorf("generated snapshot contains 'jobs' key, but it should be excluded")
	}
}
