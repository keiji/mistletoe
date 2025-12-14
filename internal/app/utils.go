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
