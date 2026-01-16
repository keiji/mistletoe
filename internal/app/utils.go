package app


import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	conf "mistletoe/internal/config"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// ExecCommand is a variable that holds exec.Command to allow mocking in tests.
var ExecCommand = exec.Command

// Stdin is a variable that holds os.Stdin to allow mocking in tests.
var Stdin io.Reader = os.Stdin

// verboseLogMu ensures atomic execution and logging when verbose mode is enabled.
var verboseLogMu sync.Mutex

// formatDuration formats a duration in milliseconds with comma separators (e.g., "1,234ms").
func formatDuration(d time.Duration) string {
	ms := d.Milliseconds()
	p := message.NewPrinter(language.English)
	return p.Sprintf("%dms", ms)
}

// AskForConfirmation prompts the user for a yes/no confirmation.
// If yesFlag is true, it automatically assumes "yes" and returns true without prompting.
// Returns true if the user enters 'y' or 'yes' (case-insensitive), false otherwise.
func AskForConfirmation(reader *bufio.Reader, prompt string, yesFlag bool) (bool, error) {
	fmt.Print(prompt)

	if yesFlag {
		fmt.Println("yes (assumed via flag)")
		return true, nil
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}

// AskForConfirmationRequired prompts the user for a yes/no confirmation.
// It requires explicit "yes" or "no" input (case-insensitive).
// It repeats the prompt until valid input is received.
// If yesFlag is true, it returns true immediately without prompting.
func AskForConfirmationRequired(reader *bufio.Reader, prompt string, yesFlag bool) (bool, error) {
	if yesFlag {
		return true, nil
	}

	for {
		fmt.Print(prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}

		input = strings.TrimSpace(strings.ToLower(input))
		if input == "y" || input == "yes" {
			return true, nil
		}
		if input == "n" || input == "no" {
			return false, nil
		}
		// Loop for invalid or empty input
	}
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
		fmt.Fprintf(Stderr, "[CMD] %s ", cmdStr)
	}
	defer func() {
		if verbose {
			fmt.Fprintf(Stderr, "(%s)\n", formatDuration(time.Since(start)))
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

// RunGitInteractive runs a git command connected to Stdout/Stderr.
func RunGitInteractive(dir string, gitPath string, verbose bool, args ...string) error {
	if verbose {
		// Lock execution to ensure logs are printed sequentially.
		verboseLogMu.Lock()
		defer verboseLogMu.Unlock()
	}

	start := time.Now()
	cmdStr := fmt.Sprintf("%s %s", gitPath, strings.Join(args, " "))
	if verbose {
		fmt.Fprintf(Stderr, "[CMD] %s ", cmdStr)
	}
	defer func() {
		if verbose {
			fmt.Fprintf(Stderr, "(%s)\n", formatDuration(time.Since(start)))
		}
	}()

	cmd := ExecCommand(gitPath, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = Stdout
	cmd.Stderr = Stderr
	cmd.Stdin = Stdin
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
		fmt.Fprintf(Stderr, "[CMD] %s ", cmdStr)
	}
	defer func() {
		if verbose {
			fmt.Fprintf(Stderr, "(%s)\n", formatDuration(time.Since(start)))
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
	cmd.Stdin = Stdin
	cmd.Stdout = Stdout
	cmd.Stderr = Stderr

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

// ResolveCommonValues resolves the configuration file path and jobs count
// from the various flag inputs.
// It also checks for stdin input if no config file is provided.
// If ignoreStdin is true, standard input is never read.
// For jobs flags, pass -1 to indicate they were not set.
func ResolveCommonValues(fLong, fShort string, jVal, jValShort int, ignoreStdin bool) (string, int, []byte, error) {
	// Jobs
	// We use -1 to indicate "unset" (default)
	jobs := -1
	if jVal != -1 {
		jobs = jVal
	} else if jValShort != -1 {
		jobs = jValShort
	}

	// Only validate if jobs was set.
	if jobs != -1 {
		if jobs < MinJobs {
			return "", 0, nil, fmt.Errorf("Jobs must be at least %d.", MinJobs)
		}
		if jobs > MaxJobs {
			return "", 0, nil, fmt.Errorf("Jobs must be at most %d.", MaxJobs)
		}
	}

	// conf.Config File
	defaultConfig := DefaultConfigFile
	var configFile string

	if fLong != defaultConfig {
		configFile = fLong
	} else if fShort != defaultConfig {
		configFile = fShort
	} else {
		configFile = defaultConfig
	}

	var configData []byte

	// Check Stdin if:
	// 1. ignoreStdin is FALSE
	// 2. AND (
	//    a. Using defaults (isDefault=true)
	//    b. OR User explicitly passed empty string (configFile="")
	//    c. OR User passed non-default file (we need to check for conflict with stdin)
	// )

	checkStdin := false
	stdinAvailable := false

	if !ignoreStdin {
		// If Stdin is os.Stdin, we can check Stat to see if it's a pipe.
		// If it's mocked, we assume it's available if it has data (handled by ReadAll later usually,
		// but for the logic "is data being piped?", we might need to rely on the type).
		// For consistency in tests, we rely on the fact that if Stdin is NOT a terminal, it's available.

		if f, ok := Stdin.(*os.File); ok {
			stat, _ := f.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				stdinAvailable = true
			}
		} else {
			// If not *os.File (e.g. bytes.Buffer in test), assume available
			stdinAvailable = true
		}

		if stdinAvailable {
			checkStdin = true
		} else if configFile == "" {
			// Explicit empty string -> force stdin
			checkStdin = true
		}
	}

	if checkStdin {
		if stdinAvailable {
			// Data is being piped to stdin
			inputData, err := io.ReadAll(Stdin)
			if err != nil {
				return "", 0, nil, fmt.Errorf("failed to read from stdin: %w", err)
			}
			configData = inputData

			// Clear configFile so loadConfig uses data
			configFile = ""
		}
	}

	return configFile, jobs, configData, nil
}

// DetermineJobs resolves the final jobs count based on flag input and configuration.
// If jobsFlag is -1 (unset), it checks the configuration.
// If both are unset, it returns DefaultJobs.
// It also enforces Min/Max bounds.
func DetermineJobs(jobsFlag int, config *conf.Config) (int, error) {
	jobs := jobsFlag

	if jobs == -1 {
		if config != nil && config.Jobs != nil {
			jobs = *config.Jobs
		} else {
			jobs = DefaultJobs
		}
	}

	if jobs < MinJobs {
		return 0, fmt.Errorf("Jobs must be at least %d.", MinJobs)
	}
	if jobs > MaxJobs {
		return 0, fmt.Errorf("Jobs must be at most %d.", MaxJobs)
	}

	return jobs, nil
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
				fmt.Fprint(Stderr, "\r\033[K") // Clear line
				return
			case <-ticker.C:
				fmt.Fprintf(Stderr, "\rProcessing... %s", chars[i])
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
