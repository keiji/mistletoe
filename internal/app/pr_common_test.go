package app

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	conf "mistletoe/internal/config"
)

func TestSortPrs(t *testing.T) {
	tests := []struct {
		name string
		prs  []PrInfo
		want []int // Expected order of numbers
	}{
		{
			name: "Sort by State Priority",
			prs: []PrInfo{
				{Number: 1, State: "MERGED"},
				{Number: 2, State: "OPEN"},
				{Number: 3, State: "CLOSED"},
			},
			want: []int{2, 1, 3}, // Open(0) < Merged(2) < Closed(3)
		},
		{
			name: "Sort Open Draft vs Open",
			prs: []PrInfo{
				{Number: 1, State: "OPEN", IsDraft: true},
				{Number: 2, State: "OPEN", IsDraft: false},
			},
			want: []int{2, 1}, // Open(0) < Draft(1)
		},
		{
			name: "Sort by Number Descending within State",
			prs: []PrInfo{
				{Number: 10, State: "OPEN"},
				{Number: 20, State: "OPEN"},
			},
			want: []int{20, 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SortPrs(tt.prs)
			var got []int
			for _, p := range tt.prs {
				got = append(got, p.Number)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SortPrs() got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderPrStatusTable(t *testing.T) {
	rows := []PrStatusRow{
		{
			StatusRow: StatusRow{
				Repo:           "repo1",
				LocalBranchRev: "main:abc",
				HasUnpushed:    false,
			},
			PrNumber:  "#1",
			PrState:   "OPEN",
			PrURL:     "https://github.com/org/repo1/pull/1",
			PrDisplay: "https://github.com/org/repo1/pull/1 [OPEN]",
			Base:      "main",
		},
		{
			StatusRow: StatusRow{
				Repo:           "repo2",
				LocalBranchRev: "dev:def",
				HasUnpushed:    true,
			},
			PrNumber:  "N/A",
			PrState:   "",
			PrURL:     "",
			PrDisplay: "-",
			Base:      "dev",
		},
	}

	var buf bytes.Buffer
	RenderPrStatusTable(&buf, rows)

	output := buf.String()

	assertContains := func(t *testing.T, out, substr string) {
		t.Helper()
		if !strings.Contains(out, substr) {
			t.Errorf("Expected output to contain %q, but it didn't.", substr)
		}
	}

	assertContains(t, output, "repo1")
	assertContains(t, output, "https://github.com/org/repo1/pull/1")
	assertContains(t, output, "OPEN")
	assertContains(t, output, "repo2")
	assertContains(t, output, "-")

	// repo2 has Unpushed
	assertContains(t, output, StatusSymbolUnpushed)

	assertContains(t, output, "Status Legend:")
}

func TestLoadDependencyGraph(t *testing.T) {
	// Create dummy config
	id := "repo1"
	url := "https://github.com/user/repo1"
	cfg := &conf.Config{
		Repositories: &[]conf.Repository{
			{ID: &id, URL: &url},
		},
	}

	// Create dummy dependency file
	tmpDir := t.TempDir()
	depPath := tmpDir + "/deps.md"
	content := "```mermaid\ngraph TD\nrepo1\n```"
	if err := os.WriteFile(depPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write deps file: %v", err)
	}

	// Test 1: Load Success
	graph, loadedContent, err := LoadDependencyGraph(depPath, cfg)
	if err != nil {
		t.Errorf("LoadDependencyGraph failed: %v", err)
	}
	if loadedContent != content {
		t.Errorf("content mismatch")
	}
	if graph == nil {
		t.Errorf("graph is nil")
	}

	// Test 2: Empty path
	graph, _, err = LoadDependencyGraph("", cfg)
	if err != nil {
		t.Errorf("expected no error for empty path, got %v", err)
	}
	if graph != nil {
		t.Errorf("expected nil graph for empty path")
	}

	// Test 3: Invalid path
	_, _, err = LoadDependencyGraph("nonexistent.md", cfg)
	if err == nil {
		t.Errorf("expected error for nonexistent file")
	}
}
