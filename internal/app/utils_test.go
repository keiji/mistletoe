package app

import (
	"os"
	"strings"
	"testing"
)

func TestResolveCommonValues(t *testing.T) {
	tests := []struct {
		name         string
		fLong        string
		fShort       string
		pVal         int
		pValShort    int
		wantConfig   string
		wantParallel int
		wantErr      bool
	}{
		{
			name:         "Defaults",
			fLong:        "",
			fShort:       "",
			pVal:         DefaultParallel,
			pValShort:    DefaultParallel,
			wantConfig:   "",
			wantParallel: DefaultParallel,
			wantErr:      false,
		},
		{
			name:         "Config from Long Flag",
			fLong:        "config.json",
			fShort:       "",
			pVal:         DefaultParallel,
			pValShort:    DefaultParallel,
			wantConfig:   "config.json",
			wantParallel: DefaultParallel,
			wantErr:      false,
		},
		{
			name:         "Config from Short Flag",
			fLong:        "",
			fShort:       "short.json",
			pVal:         DefaultParallel,
			pValShort:    DefaultParallel,
			wantConfig:   "short.json",
			wantParallel: DefaultParallel,
			wantErr:      false,
		},
		{
			name:         "Config Long Priority",
			fLong:        "long.json",
			fShort:       "short.json",
			pVal:         DefaultParallel,
			pValShort:    DefaultParallel,
			wantConfig:   "long.json",
			wantParallel: DefaultParallel,
			wantErr:      false,
		},
		{
			name:         "Parallel from Long Flag",
			fLong:        "",
			fShort:       "",
			pVal:         4,
			pValShort:    DefaultParallel,
			wantConfig:   "",
			wantParallel: 4,
			wantErr:      false,
		},
		{
			name:         "Parallel from Short Flag",
			fLong:        "",
			fShort:       "",
			pVal:         DefaultParallel,
			pValShort:    8,
			wantConfig:   "",
			wantParallel: 8,
			wantErr:      false,
		},
		{
			name:         "Parallel Long Priority",
			fLong:        "",
			fShort:       "",
			pVal:         4,
			pValShort:    8,
			wantConfig:   "",
			wantParallel: 4,
			wantErr:      false,
		},
		{
			name:         "Parallel Too Low",
			fLong:        "",
			fShort:       "",
			pVal:         -1,
			pValShort:    DefaultParallel,
			wantConfig:   "",
			wantParallel: 0,
			wantErr:      true,
		},
		{
			name:         "Parallel Too High",
			fLong:        "",
			fShort:       "",
			pVal:         MaxParallel + 1,
			pValShort:    DefaultParallel,
			wantConfig:   "",
			wantParallel: 0,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotConfig, gotParallel, _, err := ResolveCommonValues(tt.fLong, tt.fShort, tt.pVal, tt.pValShort)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveCommonValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotConfig != tt.wantConfig {
					t.Errorf("ResolveCommonValues() config = %v, want %v", gotConfig, tt.wantConfig)
				}
				if gotParallel != tt.wantParallel {
					t.Errorf("ResolveCommonValues() parallel = %v, want %v", gotParallel, tt.wantParallel)
				}
			}
		})
	}
}

func TestResolveCommonValues_WithStdin(t *testing.T) {
	// Backup original stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Create a pipe to simulate stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdin = r

	// Write raw data to the pipe
	testConfig := "test config data"
	go func() {
		defer w.Close()
		_, _ = w.Write([]byte(testConfig))
	}()

	// Call the function
	gotConfig, gotParallel, gotData, err := ResolveCommonValues("", "", DefaultParallel, DefaultParallel)

	if err != nil {
		t.Fatalf("ResolveCommonValues failed: %v", err)
	}
	if gotConfig != "" {
		t.Errorf("Expected empty config file path, got %q", gotConfig)
	}
	if gotParallel != DefaultParallel {
		t.Errorf("Expected default parallel, got %d", gotParallel)
	}
	if string(gotData) != testConfig {
		t.Errorf("Expected config data %q, got %q", testConfig, string(gotData))
	}
}

func TestRunGit(t *testing.T) {
	// Simple test to ensure RunGit calls exec correctly
	out, err := RunGit("", "git", "--version")
	if err != nil {
		t.Fatalf("RunGit failed: %v", err)
	}
	if !strings.HasPrefix(out, "git version") {
		t.Errorf("Expected output starting with 'git version', got %q", out)
	}
}
