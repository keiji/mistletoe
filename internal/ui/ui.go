// Package ui provides user interface utilities for CLI interaction,
// including spinners, confirmation prompts, and other display helpers.
package ui

import (
	"bufio"
	"fmt"
	"mistletoe/internal/sys"
	"strings"
	"time"
)

// AskForConfirmation prompts the user for a yes/no confirmation.
// If yesFlag is true, it automatically assumes "yes" and returns true without prompting.
// Returns true if the user enters 'y' or 'yes' (case-insensitive), false otherwise.
func AskForConfirmation(reader *bufio.Reader, prompt string, yesFlag bool) (bool, error) {
	fmt.Fprint(sys.Stdout, prompt)

	if yesFlag {
		fmt.Fprintln(sys.Stdout, "yes (assumed via flag)")
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
		fmt.Fprint(sys.Stdout, prompt)
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
				fmt.Fprint(sys.Stderr, "\r\033[K") // Clear line
				return
			case <-ticker.C:
				fmt.Fprintf(sys.Stderr, "\rProcessing... %s", chars[i])
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
