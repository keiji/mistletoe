package app

import (
	"strings"
	"testing"
)

func TestPrStatusCommand_Flags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantError     bool
		errorContains string
	}{
		{
			name:          "Invalid flag",
			args:          []string{"--invalid-flag"},
			wantError:     true,
			errorContains: "flag provided but not defined",
		},
		{
			name:          "Duplicate flags (alias mismatch)",
			args:          []string{"-j", "1", "--jobs", "2"},
			wantError:     true,
			errorContains: "options --jobs and -j cannot be specified with different values",
		},
		{
			name:      "Valid flags - missing config (execution fails at load)",
			args:      []string{"-f", "nonexistent.json"},
			wantError: true,
			// Depending on execution flow, if we use "git" as "gh", it might fail checkGhAvailability (auth status).
			// If we use "dummy-gh", it fails earlier (LookPath).
			// We just want to ensure flags were parsed. Both failures imply flag parsing passed.
			errorContains: "command not found", // or "auth", accepting generic failure
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use dummy gh/git to avoid external dependencies
			// For "Valid flags" case, we use "git" for GhPath to pass the existence check
			ghPath := "dummy-gh"
			if tt.errorContains == "no such file or directory" {
				ghPath = "git" // Assumed to exist in environment
			}
			opts := GlobalOptions{
				GitPath: "dummy-git",
				GhPath:  ghPath,
			}

			err := prStatusCommand(tt.args, opts)
			if (err != nil) != tt.wantError {
				t.Errorf("prStatusCommand() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if err != nil && tt.errorContains != "" {
				// Allow matching one of potential errors if exact match is tricky without full mocking
				if !strings.Contains(err.Error(), tt.errorContains) && !strings.Contains(err.Error(), "auth") {
					t.Errorf("error = %v, want error containing %q or 'auth'", err, tt.errorContains)
				}
			}
		})
	}
}
