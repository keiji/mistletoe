package app

import (
	conf "mistletoe/internal/config"
	"testing"
)

func TestCategorizePrUpdate(t *testing.T) {
	// Helper to create string pointer
	strPtr := func(s string) *string {
		return &s
	}

	repo1 := conf.Repository{ID: strPtr("repo1"), URL: strPtr("url1")}
	repo2 := conf.Repository{ID: strPtr("repo2"), URL: strPtr("url2")}
	repo3 := conf.Repository{ID: strPtr("repo3"), URL: strPtr("url3")}

	repos := []conf.Repository{repo1, repo2, repo3}

	tests := []struct {
		name              string
		prRows            []PrStatusRow
		statusRows        []StatusRow
		wantPushUpdate    []string // IDs
		wantNoPushUpdate  []string // IDs
		wantSkipped       []string // IDs
	}{
		{
			name: "All skipped (No PRs)",
			prRows: []PrStatusRow{
				{StatusRow: StatusRow{Repo: "repo1"}, PrItems: []PrInfo{}},
				{StatusRow: StatusRow{Repo: "repo2"}, PrItems: []PrInfo{}},
				{StatusRow: StatusRow{Repo: "repo3"}, PrItems: []PrInfo{}},
			},
			statusRows: []StatusRow{
				{Repo: "repo1", HasUnpushed: false},
				{Repo: "repo2", HasUnpushed: false},
				{Repo: "repo3", HasUnpushed: false},
			},
			wantPushUpdate:   []string{},
			wantNoPushUpdate: []string{},
			wantSkipped:      []string{"repo1", "repo2", "repo3"},
		},
		{
			name: "Repo1 has Open PR + Unpushed, Repo2 has Open PR + Pushed",
			prRows: []PrStatusRow{
				{
					StatusRow: StatusRow{Repo: "repo1"},
					PrItems: []PrInfo{
						{State: GitHubPrStateOpen, URL: "url1/pr/1"},
					},
				},
				{
					StatusRow: StatusRow{Repo: "repo2"},
					PrItems: []PrInfo{
						{State: GitHubPrStateOpen, URL: "url2/pr/1"},
					},
				},
				{StatusRow: StatusRow{Repo: "repo3"}, PrItems: []PrInfo{}},
			},
			statusRows: []StatusRow{
				{Repo: "repo1", HasUnpushed: true},
				{Repo: "repo2", HasUnpushed: false},
				{Repo: "repo3", HasUnpushed: false},
			},
			wantPushUpdate:   []string{"repo1"},
			wantNoPushUpdate: []string{"repo2"},
			wantSkipped:      []string{"repo3"},
		},
		{
			name: "Repo1 has Closed PR (Should be skipped)",
			prRows: []PrStatusRow{
				{
					StatusRow: StatusRow{Repo: "repo1"},
					PrItems: []PrInfo{
						{State: GitHubPrStateClosed, URL: "url1/pr/1"},
					},
				},
			},
			statusRows: []StatusRow{
				{Repo: "repo1", HasUnpushed: true},
				{Repo: "repo2", HasUnpushed: false},
				{Repo: "repo3", HasUnpushed: false},
			},
			wantPushUpdate:   []string{},
			wantNoPushUpdate: []string{},
			wantSkipped:      []string{"repo1", "repo2", "repo3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, catPushUpdate, catNoPushUpdate, skippedRepos := categorizePrUpdate(&repos, tt.prRows, tt.statusRows)

			checkRepos(t, "PushUpdate", catPushUpdate, tt.wantPushUpdate)
			checkRepos(t, "NoPushUpdate", catNoPushUpdate, tt.wantNoPushUpdate)
			checkStringSlice(t, "Skipped", skippedRepos, tt.wantSkipped)
		})
	}
}

func checkRepos(t *testing.T, category string, gotRepos []conf.Repository, wantIDs []string) {
	t.Helper()
	if len(gotRepos) != len(wantIDs) {
		t.Errorf("%s: got %d repos, want %d", category, len(gotRepos), len(wantIDs))
		return
	}
	for i, r := range gotRepos {
		if *r.ID != wantIDs[i] {
			t.Errorf("%s[%d]: got %s, want %s", category, i, *r.ID, wantIDs[i])
		}
	}
}

func checkStringSlice(t *testing.T, category string, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %d items, want %d", category, len(got), len(want))
		return
	}
	for i, s := range got {
		if s != want[i] {
			t.Errorf("%s[%d]: got %s, want %s", category, i, s, want[i])
		}
	}
}
