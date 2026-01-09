package app

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"mistletoe/internal/config"
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
			wantErr:    config.ErrInvalidDataFormat,
		},
		{
			name: "Missing repositories key",
			setup: func() string {
				return createTempFile(`{}`)
			},
			wantConfig: false,
			wantErr:    config.ErrInvalidDataFormat,
		},
		{
			name: "Repositories key is null",
			setup: func() string {
				return createTempFile(`{"repositories": null}`)
			},
			wantConfig: false,
			wantErr:    config.ErrInvalidDataFormat,
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
			wantErr:    config.ErrInvalidDataFormat,
		},
		{
			name: "Repo URL is null",
			setup: func() string {
				return createTempFile(`{"repositories": [{"url": null}]}`)
			},
			wantConfig: false,
			wantErr:    config.ErrInvalidDataFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := tt.setup()
			if filename != "non_existent_file.json" {
				defer os.Remove(filename)
			}

			cfg, err := loadConfigFile(filename)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("loadConfigFile() expected error %v, got nil", tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("loadConfigFile() error = %v, want %v", err, tt.wantErr)
				}
			} else if tt.wantErrMsg != "" {
				if err == nil {
					t.Errorf("loadConfigFile() expected error containing %q, got nil", tt.wantErrMsg)
				} else if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("loadConfigFile() error = %v, want error containing %q", err, tt.wantErrMsg)
				}
			} else {
				if err != nil {
					t.Errorf("loadConfigFile() unexpected error: %v", err)
				}
				if tt.wantConfig && cfg == nil {
					t.Error("loadConfigFile() expected config, got nil")
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
		want    *config.Config
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
			want: &config.Config{
				Repositories: func() *[]config.Repository {
					s := "user/repo"
					b := "main"
					r := []config.Repository{
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
