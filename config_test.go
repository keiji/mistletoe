package main

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Helper to create temp file
	createTempFile := func(content string) string {
		tmpfile, err := os.CreateTemp("", "config_test_*.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tmpfile.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		if err := tmpfile.Close(); err != nil {
			t.Fatal(err)
		}
		return tmpfile.Name()
	}

	tests := []struct {
		name        string
		setup       func() string // Returns filename
		wantConfig  bool          // whether we expect non-nil config
		wantErr     error         // Expected error target
	}{
		{
			name: "File does not exist",
			setup: func() string {
				return "non_existent_file.json"
			},
			wantConfig: false,
			wantErr:    ErrConfigFileNotFound,
		},
		{
			name: "Valid file",
			setup: func() string {
				return createTempFile(`{"repositories": [{"url": "https://example.com/repo.git"}]}`)
			},
			wantConfig: true,
			wantErr:    nil,
		},
		{
			name: "Invalid JSON",
			setup: func() string {
				return createTempFile(`{ invalid json }`)
			},
			wantConfig: false,
			wantErr:    ErrInvalidDataFormat,
		},
		{
			name: "Missing repositories key",
			setup: func() string {
				return createTempFile(`{}`)
			},
			wantConfig: false,
			wantErr:    ErrInvalidDataFormat,
		},
		{
			name: "Repositories key is null",
			setup: func() string {
				return createTempFile(`{"repositories": null}`)
			},
			wantConfig: false,
			wantErr:    ErrInvalidDataFormat,
		},
		{
			name: "Repositories empty array (valid)",
			setup: func() string {
				return createTempFile(`{"repositories": []}`)
			},
			wantConfig: true,
			wantErr:    nil,
		},
		{
			name: "Repo missing URL",
			setup: func() string {
				return createTempFile(`{"repositories": [{"id": "test"}]}`)
			},
			wantConfig: false,
			wantErr:    ErrInvalidDataFormat,
		},
		{
			name: "Repo URL is null",
			setup: func() string {
				return createTempFile(`{"repositories": [{"url": null}]}`)
			},
			wantConfig: false,
			wantErr:    ErrInvalidDataFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := tt.setup()
			if filename != "non_existent_file.json" {
				defer os.Remove(filename)
			}

			config, err := loadConfig(filename)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("loadConfig() expected error %v, got nil", tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("loadConfig() error = %v, want %v", err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("loadConfig() unexpected error: %v", err)
				}
				if tt.wantConfig && config == nil {
					t.Error("loadConfig() expected config, got nil")
				}
			}
		})
	}
}

func TestParseLabels(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{" a , b ", []string{"a", "b"}},
		{",,", nil},
		{"a,,b", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Input:%q", tt.input), func(t *testing.T) {
			got := ParseLabels(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseLabels(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFilterRepositories(t *testing.T) {
	repos := []Repository{
		{ID: strPtr("repo1"), Labels: []string{"java", "server"}},
		{ID: strPtr("repo2"), Labels: []string{"kotlin", "server"}},
		{ID: strPtr("repo3"), Labels: []string{"java", "client"}},
		{ID: strPtr("repo4"), Labels: []string{}},
	}

	tests := []struct {
		name   string
		labels []string
		want   []string // Expected IDs
	}{
		{
			name:   "No filter (nil)",
			labels: nil,
			want:   []string{"repo1", "repo2", "repo3", "repo4"},
		},
		{
			name:   "No filter (empty)",
			labels: []string{},
			want:   []string{"repo1", "repo2", "repo3", "repo4"},
		},
		{
			name:   "Single match",
			labels: []string{"kotlin"},
			want:   []string{"repo2"},
		},
		{
			name:   "Multiple match (intersection)",
			labels: []string{"java"},
			want:   []string{"repo1", "repo3"},
		},
		{
			name:   "Multi-label filter",
			labels: []string{"server", "client"},
			want:   []string{"repo1", "repo2", "repo3"},
		},
		{
			name:   "No match",
			labels: []string{"ruby"},
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterRepositories(repos, tt.labels)
			if len(got) != len(tt.want) {
				t.Errorf("FilterRepositories() length = %d, want %d", len(got), len(tt.want))
			}
			for i, r := range got {
				if *r.ID != tt.want[i] {
					t.Errorf("FilterRepositories()[%d] ID = %s, want %s", i, *r.ID, tt.want[i])
				}
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Config
		wantErr bool
	}{
		{
			name: "Valid JSON",
			input: `{
				"repositories": [
					{
						"url": "user/repo",
						"branch": "main",
						"labels": ["bug", "feature"]
					}
				]
			}`,
			want: &Config{
				Repositories: func() *[]Repository {
					s := "user/repo"
					b := "main"
					r := []Repository{
						{
							URL:    &s,
							Branch: &b,
							Labels: []string{"bug", "feature"},
						},
					}
					return &r
				}(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConfig([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
