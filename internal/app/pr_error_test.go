package app

import (
	"fmt"
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
	repo := Repository{ID: &id, URL: &url}
	config := &Config{Repositories: &[]Repository{repo}}
	rows := []StatusRow{{Repo: id, BranchName: "main"}}
	knownPRs := map[string]string{id: url}

	prRows := CollectPrStatus(rows, config, 1, "gh", false, knownPRs)

	if len(prRows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(prRows))
	}
	if prRows[0].PrState != "Error" {
		t.Errorf("Expected PrState 'Error', got '%s'", prRows[0].PrState)
	}
	if prRows[0].PrNumber != "Error" {
		t.Errorf("Expected PrNumber 'Error', got '%s'", prRows[0].PrNumber)
	}

	// Case 2: RunGh Error (gh pr view fails)
	os.Setenv("MOCK_GH_VIEW_FAIL", "1")
	defer os.Unsetenv("MOCK_GH_VIEW_FAIL")
	// Unset invalid json to test command fail logic distinctively (though mock priority matters)
	os.Unsetenv("MOCK_GH_VIEW_INVALID_JSON")

	prRowsFail := CollectPrStatus(rows, config, 1, "gh", false, knownPRs)
	if len(prRowsFail) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(prRowsFail))
	}
	if prRowsFail[0].PrState != "Error" {
		t.Errorf("Expected PrState 'Error', got '%s'", prRowsFail[0].PrState)
	}
	if prRowsFail[0].PrDisplay != fmt.Sprintf("%s [Error]", url) {
		t.Errorf("Expected PrDisplay '%s [Error]', got '%s'", url, prRowsFail[0].PrDisplay)
	}
}
