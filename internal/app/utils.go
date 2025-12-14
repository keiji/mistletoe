package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// --- Git Helpers ---

// RunGit runs a git command in the specified directory and returns its output (stdout).
// Leading/trailing whitespace is trimmed.
func RunGit(dir string, gitPath string, args ...string) (string, error) {
	cmd := exec.Command(gitPath, args...)
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
func RunGitInteractive(dir string, gitPath string, args ...string) error {
	cmd := exec.Command(gitPath, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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

	cmd := exec.Command(editor, tmpFile.Name())
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
func ResolveCommonValues(fLong, fShort string, pVal, pValShort int) (string, int, error) {
	// Parallel
	parallel := DefaultParallel
	if pVal != DefaultParallel {
		parallel = pVal
	} else if pValShort != DefaultParallel {
		parallel = pValShort
	}

	if parallel < MinParallel {
		return "", 0, fmt.Errorf("Parallel must be at least %d.", MinParallel)
	}
	if parallel > MaxParallel {
		return "", 0, fmt.Errorf("Parallel must be at most %d.", MaxParallel)
	}

	// Config File
	configFile := fLong
	if configFile == "" {
		configFile = fShort
	}

	return configFile, parallel, nil
}

// --- Spinner ---

type Spinner struct {
	stop chan struct{}
	done chan struct{}
}

func NewSpinner() *Spinner {
	return &Spinner{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (s *Spinner) Start() {
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

func (s *Spinner) Stop() {
	select {
	case s.stop <- struct{}{}:
		<-s.done
	default:
	}
}
