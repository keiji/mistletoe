package app

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshot_DetachedHead(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "mstl-snapshot-detached-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Setup normal repo (branch)
	repoBranchDir := filepath.Join(tmpDir, "repo_branch")
	repoBranchURL := "https://github.com/example/repo_branch.git"
	setupDummyRepo(t, repoBranchDir, repoBranchURL, "main")

	// Setup detached repo
	repoDetachedDir := filepath.Join(tmpDir, "repo_detached")
	repoDetachedURL := "https://github.com/example/repo_detached.git"
	setupDummyRepo(t, repoDetachedDir, repoDetachedURL, "main")

	// Detach HEAD in repo_detached
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDetachedDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}
	hash := strings.TrimSpace(string(out))

	cmd = exec.Command("git", "checkout", hash)
	cmd.Dir = repoDetachedDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout hash: %v", err)
	}

	// Run snapshot
	outputFile := "snapshot.json"
	// binaryPath is defined in snapshot_test.go and populated by TestMain
	cmd = exec.Command(binaryPath, "snapshot", "-f", outputFile)
	cmd.Dir = tmpDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("snapshot command failed: %v\nOutput: %s", err, out)
	}

	// Read output
	data, err := os.ReadFile(filepath.Join(tmpDir, outputFile))
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	// Unmarshal into map to check for presence/absence of fields
	var rawConfig map[string][]map[string]interface{}
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		t.Fatalf("failed to parse output json to map: %v", err)
	}

	repos := rawConfig["repositories"]
	repoMap := make(map[string]map[string]interface{})
	for _, r := range repos {
		if id, ok := r["id"].(string); ok {
			repoMap[id] = r
		}
	}

	// Check repo_branch
	rBranch, ok := repoMap["repo_branch"]
	if !ok {
		t.Fatal("repo_branch not found in output")
	}
	if b, ok := rBranch["branch"]; !ok || b != "main" {
		t.Errorf("repo_branch: expected branch 'main', got %v", b)
	}
	if r, ok := rBranch["revision"]; ok {
		t.Errorf("repo_branch: unexpected revision field: %v", r)
	}

	// Check repo_detached
	rDetached, ok := repoMap["repo_detached"]
	if !ok {
		t.Fatal("repo_detached not found in output")
	}
	if b, ok := rDetached["branch"]; ok {
		t.Errorf("repo_detached: unexpected branch field: %v", b)
	}
	if rev, ok := rDetached["revision"]; !ok || rev != hash {
		t.Errorf("repo_detached: expected revision '%s', got %v", hash, rev)
	}
}
