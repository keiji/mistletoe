package app

import (
	"io"
	"os"
)

var (
	// Stdout is the standard output writer.
	Stdout io.Writer = os.Stdout
	// Stderr is the standard error writer.
	Stderr io.Writer = os.Stderr
)
