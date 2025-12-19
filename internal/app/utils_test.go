package app

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestRunGit_Real tests RunGit against the real git command (if available)
// or mocks it if we want unit isolation.
func TestRunGit(t *testing.T) {
	// Simple test to ensure RunGit calls exec correctly
	// We'll assume git is available in the environment since this project depends on it.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	out, err := RunGit("", "git", false, "--version")
	if err != nil {
		t.Fatalf("RunGit failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("RunGit returned empty output")
	}
}

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
		t.Fatalf("ResolveCommonValues() unexpected error: %v", err)
	}

	if gotConfig != "" {
		t.Errorf("ResolveCommonValues() config = %v, want empty", gotConfig)
	}
	if gotParallel != DefaultParallel {
		t.Errorf("ResolveCommonValues() parallel = %v, want %v", gotParallel, DefaultParallel)
	}
	if string(gotData) != testConfig {
		t.Errorf("ResolveCommonValues() data = %v, want %v", string(gotData), testConfig)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{time.Duration(0), "0ms"},
		{time.Duration(100 * time.Millisecond), "100ms"},
		{time.Duration(999 * time.Millisecond), "999ms"},
		{time.Duration(1000 * time.Millisecond), "1,000ms"},
		{time.Duration(1234 * time.Millisecond), "1,234ms"},
		{time.Duration(1234567 * time.Millisecond), "1,234,567ms"},
		{time.Duration(1000000 * time.Millisecond), "1,000,000ms"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.input)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestRunGit_VerboseLog(t *testing.T) {
	// Skip if no echo command (e.g. strict windows env without sh)
	// But usually available.
	if _, err := exec.LookPath("echo"); err != nil {
		t.Skip("echo command not found")
	}

	// Capture stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = w

	defer func() {
		os.Stderr = oldStderr
	}()

	// RunGit with verbose=true
	// We use "echo" as gitPath to avoid git dependency issues in this specific test
	// and ensure it runs quickly.
	_, _ = RunGit("", "echo", true, "hello")

	w.Close()

	out, _ := io.ReadAll(r)
	output := string(out)

	// Check format: [CMD] echo hello (0ms) or similar
	if !strings.Contains(output, "[CMD] echo hello (") {
		t.Errorf("Log output format incorrect or missing: %q", output)
	}
	if !strings.HasSuffix(strings.TrimSpace(output), "ms)") {
		t.Errorf("Log output should end with ms): %q", output)
	}
}
