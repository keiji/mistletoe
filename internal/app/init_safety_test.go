package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	conf "mistletoe/internal/config"
)

func TestCheckRootDirectorySafety(t *testing.T) {
	// Helper to create a temp dir with specific files
	createTempDir := func(files ...string) string {
		dir, err := os.MkdirTemp("", "init-safety-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		for _, f := range files {
			path := filepath.Join(dir, f)
			if strings.HasSuffix(f, "/") {
				os.MkdirAll(path, 0755)
			} else {
				os.WriteFile(path, []byte("test"), 0644)
			}
		}
		return dir
	}

	// Mock Stdin
	mockStdin := func(input string) {
		Stdin = strings.NewReader(input)
	}
	defer func() { Stdin = os.Stdin }()

	// Config
	repoID := "repo1"
	repoURL := "https://example.com/repo1.git"
	config := &conf.Config{
		Repositories: &[]conf.Repository{
			{ID: &repoID, URL: &repoURL},
		},
	}

	tests := []struct {
		name        string
		files       []string // files in temp dir
		configFile  string   // name of config file (assumed in temp dir)
		input       string   // user input (y/n)
		yes         bool
		expectError bool
		expectPrompt bool // if we expect the logic to hit the prompt
	}{
		{
			name:        "Clean directory (only whitelist)",
			files:       []string{".git/", ".mstl/", "repo1/", "config.json"},
			configFile:  "config.json",
			expectError: false,
			expectPrompt: false,
		},
		{
			name:        "Unknown file, User says Yes",
			files:       []string{"repo1/", "unknown.txt"},
			input:       "y\n",
			expectError: false,
			expectPrompt: true,
		},
		{
			name:        "Unknown file, User says No",
			files:       []string{"repo1/", "unknown.txt"},
			input:       "n\n",
			expectError: true,
			expectPrompt: true,
		},
		{
			name:        "Unknown file, Yes flag",
			files:       []string{"repo1/", "unknown.txt"},
			yes:         true,
			expectError: false,
			expectPrompt: true, // It hits the prompt block but auto-confirms
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := createTempDir(tt.files...)
			defer os.RemoveAll(dir)

			// Resolve absolute path for config file to match application logic
			confPath := ""
			if tt.configFile != "" {
				confPath = filepath.Join(dir, tt.configFile)
			}

			if tt.input != "" {
				mockStdin(tt.input)
			}

			// We pass 'dir' explicitly as targetDir
			err := checkRootDirectorySafety(config, confPath, dir, tt.yes)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
