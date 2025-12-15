package app

import (
	"errors"
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

			config, err := loadConfig(filename, nil)

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

func TestIDDerivationAndDuplicates(t *testing.T) {
	tests := []struct {
		name     string
		repos    []Repository
		wantErr  bool
		checkIDs map[int]string // Index -> Expected ID
	}{
		{
			name: "Explicit ID OK",
			repos: []Repository{
				{URL: ptr("u1"), ID: ptr("id1")},
				{URL: ptr("u2"), ID: ptr("id2")},
			},
			wantErr: false,
		},
		{
			name: "Implicit ID Derivation (.git)",
			repos: []Repository{
				{URL: ptr("https://github.com/foo/bar.git")},
			},
			wantErr:  false,
			checkIDs: map[int]string{0: "bar"},
		},
		{
			name: "Implicit ID Derivation (no .git)",
			repos: []Repository{
				{URL: ptr("https://github.com/foo/baz")},
			},
			wantErr:  false,
			checkIDs: map[int]string{0: "baz"},
		},
		{
			name: "Implicit ID Derivation (mixed)",
			repos: []Repository{
				{URL: ptr("https://github.com/foo/alpha.git")},
				{URL: ptr("https://github.com/foo/beta")},
			},
			wantErr:  false,
			checkIDs: map[int]string{0: "alpha", 1: "beta"},
		},
		{
			name: "Duplicate Explicit IDs",
			repos: []Repository{
				{URL: ptr("u1"), ID: ptr("dup")},
				{URL: ptr("u2"), ID: ptr("dup")},
			},
			wantErr: true,
		},
		{
			name: "Duplicate Explicit and Implicit",
			repos: []Repository{
				{URL: ptr("https://github.com/foo/bar.git")}, // -> bar
				{URL: ptr("u2"), ID: ptr("bar")},
			},
			wantErr: true,
		},
		{
			name: "Duplicate Implicit",
			repos: []Repository{
				{URL: ptr("https://github.com/foo/bar.git")}, // -> bar
				{URL: ptr("https://gitlab.com/other/bar")},   // -> bar
			},
			wantErr: true,
		},
		{
			name: "Invalid ID (Special characters)",
			repos: []Repository{
				{URL: ptr("u1"), ID: ptr("foo/bar")},
			},
			wantErr: true,
		},
		{
			name: "Invalid ID (Special characters 2)",
			repos: []Repository{
				{URL: ptr("u1"), ID: ptr("foo*bar")},
			},
			wantErr: true,
		},
		{
			name: "Invalid ID (Dot)",
			repos: []Repository{
				{URL: ptr("u1"), ID: ptr(".")},
			},
			wantErr: true,
		},
		{
			name: "Invalid ID (Double Dot)",
			repos: []Repository{
				{URL: ptr("u1"), ID: ptr("..")},
			},
			wantErr: true,
		},
		{
			name: "Invalid URL (ext::)",
			repos: []Repository{
				{URL: ptr("ext::sh -c evil")},
			},
			wantErr: true,
		},
		{
			name: "Invalid URL (Control char)",
			repos: []Repository{
				{URL: ptr("https://example.com/repo\n.git")},
			},
			wantErr: true,
		},
		{
			name: "Invalid Branch (Start with -)",
			repos: []Repository{
				{URL: ptr("u1"), ID: ptr("valid"), Branch: ptr("-flags")},
			},
			wantErr: true,
		},
		{
			name: "Invalid Branch (Special char)",
			repos: []Repository{
				{URL: ptr("u1"), ID: ptr("valid"), Branch: ptr("foo;bar")},
			},
			wantErr: true,
		},
		{
			name: "Valid Branch (Slash OK)",
			repos: []Repository{
				{URL: ptr("u1"), ID: ptr("valid"), Branch: ptr("feature/new-ui")},
			},
			wantErr: false,
		},
		{
			name: "Invalid Revision (Start with -)",
			repos: []Repository{
				{URL: ptr("u1"), ID: ptr("valid"), Revision: ptr("-flags")},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepositories(tt.repos)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRepositories() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && tt.checkIDs != nil {
				for idx, expectedID := range tt.checkIDs {
					if tt.repos[idx].ID == nil || *tt.repos[idx].ID != expectedID {
						got := "<nil>"
						if tt.repos[idx].ID != nil {
							got = *tt.repos[idx].ID
						}
						t.Errorf("Repo[%d] ID = %s, want %s", idx, got, expectedID)
					}
				}
			}
		})
	}
}

func ptr(s string) *string {
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
						"branch": "main"
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
