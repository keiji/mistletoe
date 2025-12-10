package main

import (
	"testing"
)

func TestValidateRepositories_Duplicates(t *testing.T) {
	id1 := "repo1"
	id2 := "repo2"

	tests := []struct {
		name    string
		repos   []Repository
		wantErr bool
	}{
		{
			name: "No duplicates",
			repos: []Repository{
				{ID: &id1, URL: "http://example.com/1.git"},
				{ID: &id2, URL: "http://example.com/2.git"},
			},
			wantErr: false,
		},
		{
			name: "Duplicates",
			repos: []Repository{
				{ID: &id1, URL: "http://example.com/1.git"},
				{ID: &id1, URL: "http://example.com/2.git"},
			},
			wantErr: true,
		},
		{
			name: "Nil IDs (ignored)",
			repos: []Repository{
				{ID: nil, URL: "http://example.com/1.git"},
				{ID: nil, URL: "http://example.com/2.git"},
				{ID: &id1, URL: "http://example.com/3.git"},
			},
			wantErr: false,
		},
		{
			name: "Nil ID and matching string ID (no collision)",
			repos: []Repository{
				{ID: nil, URL: "http://example.com/1.git"},
				{ID: &id1, URL: "http://example.com/2.git"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateRepositories(tt.repos); (err != nil) != tt.wantErr {
				t.Errorf("validateRepositories() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetRepoDir(t *testing.T) {
	id := "custom-dir"
	tests := []struct {
		name     string
		repo     Repository
		expected string
	}{
		{
			name:     "With ID",
			repo:     Repository{ID: &id, URL: "https://github.com/foo/bar.git"},
			expected: "custom-dir",
		},
		{
			name:     "Without ID, standard git",
			repo:     Repository{ID: nil, URL: "https://github.com/foo/bar.git"},
			expected: "bar",
		},
		{
			name:     "Without ID, no .git",
			repo:     Repository{ID: nil, URL: "https://github.com/foo/baz"},
			expected: "baz",
		},
		{
			name:     "Without ID, trailing slash",
			repo:     Repository{ID: nil, URL: "https://github.com/foo/qux/"},
			expected: "qux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRepoDir(tt.repo)
			if got != tt.expected {
				t.Errorf("getRepoDir() = %v, want %v", got, tt.expected)
			}
		})
	}
}
