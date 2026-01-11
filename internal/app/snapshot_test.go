package app

import (
	conf "mistletoe/internal/config"
)

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"encoding/json"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary
	if runtime.GOOS == "windows" {
		binaryPath = filepath.Join(os.TempDir(), "mstl-test.exe")
	} else {
		binaryPath = filepath.Join(os.TempDir(), "mstl-test")
	}

	// Build command
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		fmt.Printf("Failed to get root dir: %v\n", err)
		os.Exit(1)
	}
	cmdPath := filepath.Join(rootDir, "cmd", "mstl")
	cmd := exec.Command("go", "build", "-o", binaryPath, cmdPath)
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

	cmd := exec.Command(binaryPath, "snapshot", "-o", outputFile, "-f", "")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("snapshot command failed: %v\nOutput: %s", err, out)
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

	// Check repo1
	// Note: r.ID is the directory name. Since we created "repo1" inside tmpDir,
	// and ran snapshot inside tmpDir, the dir name scanned is "repo1".
	r1, ok := repoMap["repo1"]
	if !ok {
		t.Errorf("repo1 not found in %v", repoMap)
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
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "mstl-snapshot-default")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Setup repos in tmpDir
	repo1Dir := filepath.Join(tmpDir, "repo1")
	repo1URL := "https://github.com/example/repo1.git"
	repo1Branch := "main"
	setupDummyRepo(t, repo1Dir, repo1URL, repo1Branch)

	// Run snapshot without -o
	cmd := exec.Command(binaryPath, "snapshot", "-f", "")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("snapshot command failed: %v\nOutput: %s", err, out)
	}

	// We don't know the exact ID easily without duplicating logic, but we can search for the file pattern
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
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "mstl-snapshot-fail")
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

	cmd := exec.Command(binaryPath, "snapshot", "-o", outputFile)
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()

	// Expect failure
	if err == nil {
		t.Errorf("expected command to fail when file exists, but it succeeded. Output: %s", out)
	}
}

// TestGenerateSnapshot_ExcludesJobs verifies that the generated snapshot JSON does not contain the "jobs" field.
func TestGenerateSnapshot_ExcludesJobs(t *testing.T) {
	// 1. Create a dummy config with jobs set (this simulates loading a config that has it)
	// Although GenerateSnapshot creates a NEW config, passing a config to it is used for BaseBranch resolution.
	// But the Jobs field in the input config doesn't matter for the output structure directly,
	// UNLESS GenerateSnapshot mistakenly copies it.

	// However, GenerateSnapshot constructs a fresh conf.Config struct.
	// The Jobs field in conf.Config is a pointer (*int). If it is nil, json omitempty hides it.
	// If GenerateSnapshot doesn't set it, it is nil.

	// We will call GenerateSnapshotVerbose directly to verify the output bytes.

	// Create a dummy repo so we have something to snapshot
	tmpDir, err := os.MkdirTemp("", "mstl-gen-snapshot-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "repo1")
	repoURL := "https://github.com/example/repo1.git"
	repoBranch := "main"
	setupDummyRepo(t, repoDir, repoURL, repoBranch)

	// Input config
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
		BaseDir: tmpDir, // Ensure it looks in tmpDir
	}

	// Call GenerateSnapshotVerbose
	jsonBytes, _, err := GenerateSnapshotVerbose(inputConfig, "git", false)
	if err != nil {
		t.Fatalf("GenerateSnapshotVerbose failed: %v", err)
	}

	// Verify "jobs" key is NOT present in jsonBytes
	// We can decode into a map[string]interface{}
	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.Fatalf("failed to unmarshal generated json: %v", err)
	}

	if _, ok := result["jobs"]; ok {
		t.Errorf("generated snapshot contains 'jobs' key, but it should be excluded")
	}
}
