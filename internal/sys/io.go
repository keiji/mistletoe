package sys

import (
	"io"
	"os"
)

var (
	// Stdin is the standard input reader.
	Stdin io.Reader = os.Stdin
	// Stdout is the standard output writer.
	Stdout io.Writer = os.Stdout
	// Stderr is the standard error writer.
	Stderr io.Writer = os.Stderr
)
