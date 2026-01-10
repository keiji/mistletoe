package app

import (
	conf "mistletoe/internal/config"
)

import (
	"os"
	"testing"
)

func TestCollectPrStatus_ErrorHandling(t *testing.T) {
	oldExec := ExecCommand
	ExecCommand = fakeExecCommand
	defer func() { ExecCommand = oldExec }()

	// Case 1: JSON Unmarshal Error (gh pr view returns invalid json)
	os.Setenv("MOCK_GH_VIEW_INVALID_JSON", "1")
	defer os.Unsetenv("MOCK_GH_VIEW_INVALID_JSON")

	id := "."
	url := "https://github.com/user/repo/pull/1"
	repo := conf.Repository{ID: &id, URL: &url}
	config := &conf.Config{Repositories: &[]conf.Repository{repo}}
	rows := []StatusRow{{Repo: id, BranchName: "main"}}

	knownPRs := map[string][]PrInfo{id: {{URL: url}}}

	// Verify that it handles partial info gracefully
	prRows := CollectPrStatus(rows, config, 1, "gh", false, knownPRs)

	if len(prRows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(prRows))
	}
	// With the new logic, just passing URL results in assuming OPEN state.
	// It does NOT fail.
	if prRows[0].PrState != "OPEN" {
		t.Errorf("Expected PrState 'OPEN' (default), got '%s'", prRows[0].PrState)
	}
	// And PrNumber should be parsed from URL
	if prRows[0].PrNumber != "#1" {
		t.Errorf("Expected PrNumber '#1', got '%s'", prRows[0].PrNumber)
	}
}
