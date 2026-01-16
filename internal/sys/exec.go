// Package sys provides system-level abstractions and wrappers,
// such as execution commands and standard I/O, to facilitate testing and mocking.
package sys

import (
	"os/exec"
)

// ExecCommand is a variable that holds exec.Command to allow mocking in tests.
var ExecCommand = exec.Command
