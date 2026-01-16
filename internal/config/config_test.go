package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// strPtr is a helper to return a pointer to a string.
func strPtr(s string) *string { return &s }

func TestLoadConfigFile(t *testing.T) {
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
		wantErrMsg  string        // Expected error message (substring)
	}{
		{
			name: "File does not exist",
			setup: func() string {
				return "non_existent_file.json"
			},
			wantConfig: false,
			wantErrMsg: "Configuration file non_existent_file.json not found.",
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

			cfg, err := LoadConfigFile(filename)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("LoadConfigFile() expected error %v, got nil", tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("LoadConfigFile() error = %v, want %v", err, tt.wantErr)
				}
			} else if tt.wantErrMsg != "" {
				if err == nil {
					t.Errorf("LoadConfigFile() expected error containing %q, got nil", tt.wantErrMsg)
				} else if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("LoadConfigFile() error = %v, want error containing %q", err, tt.wantErrMsg)
				}
			} else {
				if err != nil {
					t.Errorf("LoadConfigFile() unexpected error: %v", err)
				}
				if tt.wantConfig && cfg == nil {
					t.Error("LoadConfigFile() expected config, got nil")
				}
			}
		})
	}
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

func TestLoadConfigData_Validation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name: "Duplicate IDs",
			input: `{
				"repositories": [
					{"id": "repo1", "url": "http://a"},
					{"id": "repo1", "url": "http://b"}
				]
			}`,
			wantErr: ErrDuplicateID,
		},
		{
			name: "Duplicate Auto-Generated IDs",
			input: `{
				"repositories": [
					{"url": "http://example.com/repo.git"},
					{"url": "http://other.com/repo.git"}
				]
			}`,
			wantErr: ErrDuplicateID,
		},
		{
			name: "Invalid ID with spaces",
			input: `{
				"repositories": [
					{"id": "repo 1", "url": "http://a"}
				]
			}`,
			wantErr: ErrInvalidID,
		},
		{
			name: "Invalid ID dot",
			input: `{
				"repositories": [
					{"id": ".", "url": "http://a"}
				]
			}`,
			wantErr: ErrInvalidID,
		},
		{
			name: "Invalid ID dotdot",
			input: `{
				"repositories": [
					{"id": "..", "url": "http://a"}
				]
			}`,
			wantErr: ErrInvalidID,
		},
		{
			name: "Invalid URL ext protocol",
			input: `{
				"repositories": [
					{"url": "ext::sh /tmp/pwn"}
				]
			}`,
			wantErr: ErrInvalidURL,
		},
		{
			name: "Invalid URL control char",
			input: `{
				"repositories": [
					{"id": "valid-id", "url": "http://example.com/\n"}
				]
			}`,
			wantErr: ErrInvalidURL,
		},
		{
			name: "Invalid Branch",
			input: `{
				"repositories": [
					{"url": "http://a", "branch": "-flag"}
				]
			}`,
			wantErr: ErrInvalidGitRef,
		},
		{
			name: "Invalid BaseBranch",
			input: `{
				"repositories": [
					{"url": "http://a", "base-branch": "invalid~Char"}
				]
			}`,
			wantErr: ErrInvalidGitRef,
		},
		{
			name: "Invalid Revision",
			input: `{
				"repositories": [
					{"url": "http://a", "revision": "invalid:Char"}
				]
			}`,
			wantErr: ErrInvalidGitRef,
		},
		{
			name: "Valid Configuration",
			input: `{
				"repositories": [
					{"id": "repo1", "url": "http://example.com/r1"},
					{"url": "http://example.com/r2"}
				]
			}`,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfigData([]byte(tt.input))
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("LoadConfigData() expected error %v, got nil", tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("LoadConfigData() error = %v, want %v", err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("LoadConfigData() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetRepoDirName(t *testing.T) {
	tests := []struct {
		name string
		repo Repository
		want string
	}{
		{
			name: "With Explicit ID",
			repo: Repository{ID: strPtr("my-repo"), URL: strPtr("http://example.com/ignored")},
			want: "my-repo",
		},
		{
			name: "With URL only",
			repo: Repository{URL: strPtr("http://example.com/foo.git")},
			want: "foo",
		},
		{
			name: "With URL no extension",
			repo: Repository{URL: strPtr("http://example.com/bar")},
			want: "bar",
		},
		{
			name: "With URL trailing slash",
			repo: Repository{URL: strPtr("http://example.com/baz/")},
			want: "baz",
		},
		{
			name: "Empty URL and ID",
			repo: Repository{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetRepoDirName(tt.repo); got != tt.want {
				t.Errorf("GetRepoDirName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetRepoPath(t *testing.T) {
	config := Config{
		BaseDir: filepath.FromSlash("/base/dir"),
	}

	repo := Repository{ID: strPtr("sub"), URL: strPtr("http://a.com/sub")}

	want := filepath.Join(config.BaseDir, "sub")

	got := config.GetRepoPath(repo)
	if got != want {
		t.Errorf("GetRepoPath() = %v, want %v", got, want)
	}
}
