package app


import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestRunEditor verifies RunEditor functionality.
// Note: We cannot interactively test an editor, so we mock ExecCommand to simulate an editor
// that writes to the file.
func TestRunEditor(t *testing.T) {
	// Mock ExecCommand
	originalExecCommand := ExecCommand
	defer func() { ExecCommand = originalExecCommand }()

	ExecCommand = func(name string, arg ...string) *exec.Cmd {
		// Mock behavior: write "Edited content" to the file (arg[0])
		return exec.Command(os.Args[0], "-test.run=TestHelperProcess_Editor", "--", arg[0])
	}

	content, err := RunEditor()
	if err != nil {
		t.Fatalf("RunEditor failed: %v", err)
	}

	expected := "Edited content"
	if content != expected {
		t.Errorf("expected '%s', got '%s'", expected, content)
	}
}

// TestHelperProcess_Editor is the helper process for TestRunEditor.
// It acts as the "editor", writing content to the provided filename.
func TestHelperProcess_Editor(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		// This logic is invoked via TestHelperProcess dispatcher in common_test.go
		// But since we are calling os.Args[0] with specific -test.run, we can just return if not targeted.
		// However, standard TestHelperProcess pattern uses GO_WANT_HELPER_PROCESS.
		// Let's rely on standard pattern if we can, but we need custom logic (write to file).

		// If we use the flag approach:
		args := os.Args
		for len(args) > 0 {
			if args[0] == "--" {
				args = args[1:]
				break
			}
			args = args[1:]
		}

		if len(args) == 0 {
			return
		}

		filename := args[0]
		err := os.WriteFile(filename, []byte("Edited content"), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to write file: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}

// Ensure TestHelperProcess exists or create a dispatcher.
// In `common_test.go` or similar, there is usually a TestHelperProcess.
// Since `utils.go` invokes `os.Args[0]`, we need to ensure the test binary handles the call.
// The code above `TestRunEditor` uses `-test.run=TestHelperProcess_Editor`.
// This matches the function name `TestHelperProcess_Editor`.
// So when the subprocess runs, it executes `TestHelperProcess_Editor`.
// However, `go test` runs all tests matching the pattern.
// `TestHelperProcess_Editor` needs to behave like a test (take *testing.T) but perform the action.
// The logic inside `TestHelperProcess_Editor` checks args.
// But `TestHelperProcess_Editor` will be called by `go test`.
// We need to ensure it doesn't run during normal `go test ./...` execution unless arguments match.
// The trick is checking `os.Args` for `--`.

func TestRunEditor_Empty(t *testing.T) {
	// Mock ExecCommand to write nothing
	originalExecCommand := ExecCommand
	defer func() { ExecCommand = originalExecCommand }()

	ExecCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command(os.Args[0], "-test.run=TestHelperProcess_EditorEmpty", "--", arg[0])
	}

	_, err := RunEditor()
	if err == nil {
		t.Fatal("expected error for empty content, got nil")
	}
	expectedError := "empty message, aborted"
	if err.Error() != expectedError {
		t.Errorf("expected error '%s', got '%v'", expectedError, err)
	}
}

func TestHelperProcess_EditorEmpty(t *testing.T) {
	args := os.Args
	foundSeparator := false
	for _, arg := range args {
		if arg == "--" {
			foundSeparator = true
			break
		}
	}
	if !foundSeparator {
		return // Not running as helper
	}

	// Write nothing and exit
	os.Exit(0)
}

func TestSpinner(t *testing.T) {
	// Test Normal Spinner
	s := NewSpinner(false)
	s.Start()
	// Let it run briefly
	s.Stop()

	// Test Verbose Spinner (No-op)
	sVerbose := NewSpinner(true)
	sVerbose.Start()
	// Should do nothing
	sVerbose.Stop()
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0ms"},
		{100 * time.Millisecond, "100ms"},
		{1234 * time.Millisecond, "1,234ms"},
		{1000 * time.Millisecond, "1,000ms"},
		{1234567 * time.Millisecond, "1,234,567ms"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatDuration(tt.d); got != tt.want {
				t.Errorf("formatDuration(%v) = %v, want %v", tt.d, got, tt.want)
			}
		})
	}
}

func TestResolveCommonValues(t *testing.T) {
	originalStdin := Stdin
	defer func() { Stdin = originalStdin }()

	tests := []struct {
		name        string
		fLong       string
		fShort      string
		jVal        int
		jValShort   int
		ignoreStdin bool
		stdinData   string
		wantFile    string
		wantP       int
		wantData    string
		wantErr     bool
	}{
		{
			name:      "Defaults",
			fLong:     DefaultConfigFile,
			fShort:    DefaultConfigFile,
			jVal:      -1, // Simulating unset
			jValShort: -1, // Simulating unset
			wantFile:  "", // With strings.Reader(""), logic assumes Stdin is piped but empty, so clears configFile
			wantP:     -1, // Should return unset
			wantData:  "",
			stdinData: "",
		},
		{
			name:        "Explicit File Long",
			fLong:       "custom.json",
			fShort:      DefaultConfigFile,
			jVal:      -1,
			jValShort: -1,
			ignoreStdin: true, // Simulate TTY/No pipe
			wantFile:    "custom.json",
			wantP:       -1,
		},
		{
			name:        "Explicit File Short",
			fLong:       DefaultConfigFile,
			fShort:      "short.json",
			jVal:      -1,
			jValShort: -1,
			ignoreStdin: true, // Simulate TTY/No pipe
			wantFile:    "short.json",
			wantP:       -1,
		},
		{
			name:        "Jobs Long",
			fLong:       DefaultConfigFile,
			fShort:      DefaultConfigFile,
			jVal:        4,
			jValShort:   -1,
			ignoreStdin: true, // Simulate TTY/No pipe
			wantFile:    DefaultConfigFile,
			wantP:       4,
		},
		{
			name:        "Jobs Short",
			fLong:       DefaultConfigFile,
			fShort:      DefaultConfigFile,
			jVal:        -1,
			jValShort:   8,
			ignoreStdin: true, // Simulate TTY/No pipe
			wantFile:    DefaultConfigFile,
			wantP:       8,
		},
		// Validation tests: ResolveCommonValues only validates if jobs IS SET (!= -1)
		{
			name:      "Jobs Invalid Low",
			jVal:      0, // 0 < MinJobs (1)
			jValShort: -1,
			wantErr:   true,
		},
		{
			name:      "Jobs Invalid High",
			jVal:      200, // 200 > MaxJobs (128)
			jValShort: -1,
			wantErr:   true,
		},
		{
			name:      "Stdin used when default file and stdin available",
			fLong:     DefaultConfigFile,
			fShort:    DefaultConfigFile,
			jVal:      -1,
			jValShort: -1,
			stdinData: `{"repositories": []}`,
			wantFile:  "", // Cleared when using stdin
			wantData:  `{"repositories": []}`,
			wantP:     -1,
		},
		{
			name:      "Stdin ignored when ignoreStdin true",
			fLong:     DefaultConfigFile,
			fShort:    DefaultConfigFile,
			jVal:      -1,
			jValShort: -1,
			ignoreStdin: true,
			stdinData: `{"repositories": []}`,
			wantFile:  DefaultConfigFile, // Uses default file
			wantData:  "",
			wantP:     -1,
		},
		{
			name:      "Conflict: Custom file and Stdin",
			fLong:     "custom.json",
			fShort:    DefaultConfigFile,
			jVal:      -1,
			jValShort: -1,
			stdinData: `{"repositories": []}`,
			wantErr:   true,
		},
		{
			name:      "Explicit empty file forces Stdin",
			fLong:     "",
			fShort:    DefaultConfigFile, // fLong takes precedence as empty
			jVal:      -1,
			jValShort: -1,
			stdinData: `{"repositories": []}`,
			wantFile:  "",
			wantData:  `{"repositories": []}`,
			wantP:     -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.stdinData != "" {
				Stdin = strings.NewReader(tt.stdinData)
			} else {
				Stdin = strings.NewReader("")
			}

			gotFile, gotP, gotData, err := ResolveCommonValues(tt.fLong, tt.fShort, tt.jVal, tt.jValShort, tt.ignoreStdin)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveCommonValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFile != tt.wantFile {
				t.Errorf("ResolveCommonValues() file = %v, want %v", gotFile, tt.wantFile)
			}
			if gotP != tt.wantP {
				t.Errorf("ResolveCommonValues() p = %v, want %v", gotP, tt.wantP)
			}
			if string(gotData) != tt.wantData {
				t.Errorf("ResolveCommonValues() data = %v, want %v", string(gotData), tt.wantData)
			}
		})
	}
}
