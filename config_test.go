package main

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
