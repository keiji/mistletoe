package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	conf "mistletoe/internal/config"
)

func TestCheckRootDirectorySafety(t *testing.T) {
	// Setup temp directory with a "dirty" state
	tmpDir, err := os.MkdirTemp("", "mstl-safety-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a random file to trigger the safety check
	if err := os.WriteFile(filepath.Join(tmpDir, "dirty.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock Config
	repoID := "repo1"
	repoURL := "https://example.com/repo1.git"
	config := &conf.Config{
		Repositories: &[]conf.Repository{
			{ID: &repoID, URL: &repoURL},
		},
	}

	tests := []struct {
		name          string
		yesFlag       bool
		input         string
		wantErr       bool
		wantErrString string
	}{
		{
			name:    "AutoApprove (Yes Flag)",
			yesFlag: true,
			input:   "", // Should not be read
			wantErr: false,
		},
		{
			name:    "Manual Approve (y)",
			yesFlag: false,
			input:   "y\n",
			wantErr: false,
		},
		{
			name:    "Manual Approve (yes)",
			yesFlag: false,
			input:   "yes\n",
			wantErr: false,
		},
		{
			name:          "Manual Reject (n)",
			yesFlag:       false,
			input:         "n\n",
			wantErr:       true,
			wantErrString: "initialization aborted by user",
		},
		{
			name:          "Manual Reject (no)",
			yesFlag:       false,
			input:         "no\n",
			wantErr:       true,
			wantErrString: "initialization aborted by user",
		},
		{
			name:    "Retry on Empty Input",
			yesFlag: false,
			input:   "\n\ny\n", // Enter, Enter, y
			wantErr: false,
		},
		{
			name:          "Retry on Invalid Input then Reject",
			yesFlag:       false,
			input:         "what\nno\n",
			wantErr:       true,
			wantErrString: "initialization aborted by user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock Stdin
			oldStdin := Stdin
			defer func() { Stdin = oldStdin }()

			// We can't easily mock os.Stdin for bufio.NewReader(Stdin) inside the function
			// if Stdin variable is just an io.Reader, because bufio.NewReader takes io.Reader.
			// In `init.go`: reader := bufio.NewReader(Stdin).
			// So setting app.Stdin = strings.NewReader(...) works.

			Stdin = strings.NewReader(tt.input)

			// Capture Stdout to verify prompt?
			// It's hard to capture stdout since it prints to os.Stdout directly in `init.go`.
			// But we are testing logic here.

			err := checkRootDirectorySafety(config, "", tmpDir, tt.yesFlag)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkRootDirectorySafety() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if err.Error() != tt.wantErrString {
					t.Errorf("checkRootDirectorySafety() error = %v, want %v", err, tt.wantErrString)
				}
			}
		})
	}
}
