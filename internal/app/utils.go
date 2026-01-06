package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// ExecCommand is a variable that holds exec.Command to allow mocking in tests.
var ExecCommand = exec.Command

// verboseLogMu ensures atomic execution and logging when verbose mode is enabled.
var verboseLogMu sync.Mutex

// formatDuration formats a duration in milliseconds with comma separators (e.g., "1,234ms").
func formatDuration(d time.Duration) string {
	ms := d.Milliseconds()
	p := message.NewPrinter(language.English)
	return p.Sprintf("%dms", ms)
}

// --- Git Helpers ---

// RunGit runs a git command in the specified directory and returns its output (stdout).
// Leading/trailing whitespace is trimmed.
func RunGit(dir string, gitPath string, verbose bool, args ...string) (string, error) {
	if verbose {
		// Lock execution to ensure logs (start command + end duration) are printed sequentially
		// without interleaving from other goroutines.
		// defer Unlock() unlocks at the end of the function, holding the lock during execution.
		verboseLogMu.Lock()
		defer verboseLogMu.Unlock()
	}

	start := time.Now()
	cmdStr := fmt.Sprintf("%s %s", gitPath, strings.Join(args, " "))
	if verbose {
		fmt.Fprintf(os.Stderr, "[CMD] %s ", cmdStr)
	}
	defer func() {
		if verbose {
			fmt.Fprintf(os.Stderr, "(%s)\n", formatDuration(time.Since(start)))
		}
	}()

	cmd := ExecCommand(gitPath, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RunGitInteractive runs a git command connected to os.Stdout/Stderr.
func RunGitInteractive(dir string, gitPath string, verbose bool, args ...string) error {
	if verbose {
		// Lock execution to ensure logs are printed sequentially.
		verboseLogMu.Lock()
		defer verboseLogMu.Unlock()
	}

	start := time.Now()
	cmdStr := fmt.Sprintf("%s %s", gitPath, strings.Join(args, " "))
	if verbose {
		fmt.Fprintf(os.Stderr, "[CMD] %s ", cmdStr)
	}
	defer func() {
		if verbose {
			fmt.Fprintf(os.Stderr, "(%s)\n", formatDuration(time.Since(start)))
		}
	}()

	cmd := ExecCommand(gitPath, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// --- GitHub CLI Helpers ---

// RunGh runs a gh command and returns its output (stdout).
func RunGh(ghPath string, verbose bool, args ...string) (string, error) {
	if verbose {
		// Lock execution to ensure logs are printed sequentially.
		verboseLogMu.Lock()
		defer verboseLogMu.Unlock()
	}

	start := time.Now()
	cmdStr := fmt.Sprintf("%s %s", ghPath, strings.Join(args, " "))
	if verbose {
		fmt.Fprintf(os.Stderr, "[CMD] %s ", cmdStr)
	}
	defer func() {
		if verbose {
			fmt.Fprintf(os.Stderr, "(%s)\n", formatDuration(time.Since(start)))
		}
	}()

	cmd := ExecCommand(ghPath, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// --- Editor Helpers ---

// RunEditor opens the user's preferred editor (or a default) to edit a temporary file.
// It returns the content of the file after the editor is closed.
// If the content is empty, it returns an error.
func RunEditor() (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		if _, err := exec.LookPath("vi"); err == nil {
			editor = "vi"
		} else if _, err := exec.LookPath("notepad"); err == nil {
			editor = "notepad"
		} else {
			return "", fmt.Errorf("no suitable editor found. Please set the EDITOR environment variable")
		}
	}

	tmpFile, err := os.CreateTemp("", "mstl-gh-pr-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close() // Close immediately, let editor open it

	cmd := ExecCommand(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run editor: %w", err)
	}

	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read temporary file: %w", err)
	}

	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return "", fmt.Errorf("empty message, aborted")
	}

	return trimmed, nil
}

// --- Flag Helpers ---

// ResolveCommonValues resolves the configuration file path and parallel count
// from the various flag inputs.
// It also checks for stdin input if no config file is provided.
func ResolveCommonValues(fLong, fShort string, pVal, pValShort int) (string, int, []byte, error) {
	// Parallel
	parallel := DefaultParallel
	if pVal != DefaultParallel && pVal != 0 {
		parallel = pVal
	} else if pValShort != DefaultParallel && pValShort != 0 {
		parallel = pValShort
	}

	if parallel < MinParallel {
		return "", 0, nil, fmt.Errorf("Parallel must be at least %d.", MinParallel)
	}
	if parallel > MaxParallel {
		return "", 0, nil, fmt.Errorf("Parallel must be at most %d.", MaxParallel)
	}

	// Config File
	defaultConfig := ".mstl/config.json"
	var configFile string
	var isDefault bool

	if fLong != defaultConfig {
		configFile = fLong
		isDefault = false
	} else if fShort != defaultConfig {
		configFile = fShort
		isDefault = false
	} else {
		configFile = defaultConfig
		isDefault = true
	}

	// If user supplied empty string explicitly, we treat it as "no file", so we look for stdin.
	if configFile == "" {
		isDefault = false // effectively treated as manual override to nothing
	}

	var configData []byte

	// Check Stdin if we are using defaults OR if configFile is explicitly empty
	checkStdin := false
	if configFile == "" {
		checkStdin = true
	} else if isDefault {
		// If using default file, we also check if there is piped input to prioritize it?
		// Or should we only check stdin if default file doesn't exist? No, explicit pipe should win.
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			checkStdin = true
		}
	}

	if checkStdin {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Data is being piped to stdin
			inputData, err := io.ReadAll(os.Stdin)
			if err != nil {
				return "", 0, nil, fmt.Errorf("failed to read from stdin: %w", err)
			}
			configData = inputData

			// If we successfully read from stdin, we clear configFile so loadConfig uses data
			// UNLESS user explicitly asked for a file?
			// If isDefault is true, we prefer stdin.
			// If isDefault is false (user passed -f ""), we prefer stdin.
			// If user passed -f "somefile" (isDefault false), we shouldn't be here (checkStdin false).
			configFile = ""
		}
	}

	return configFile, parallel, configData, nil
}

// --- Spinner ---

// Spinner shows a simple progress indicator.
type Spinner struct {
	stop     chan struct{}
	done     chan struct{}
	disabled bool
}

// NewSpinner creates a new Spinner instance.
// If verbose is true, the spinner is disabled (no-op).
func NewSpinner(verbose bool) *Spinner {
	return &Spinner{
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		disabled: verbose,
	}
}

// Start starts the spinner in a separate goroutine.
func (s *Spinner) Start() {
	if s.disabled {
		return
	}
	go func() {
		defer close(s.done)
		chars := []string{"/", "-", "\\", "|"}
		i := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				fmt.Print("\r\033[K") // Clear line
				return
			case <-ticker.C:
				fmt.Printf("\rProcessing... %s", chars[i])
				i = (i + 1) % len(chars)
			}
		}
	}()
}

// Stop stops the spinner and clears the line.
func (s *Spinner) Stop() {
	if s.disabled {
		return
	}
	select {
	case s.stop <- struct{}{}:
		<-s.done
	default:
	}
}
