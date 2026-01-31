// Package app implements the core logic of the application.
package app

import (
	"fmt"
	"mistletoe/internal/sys"
	"os/exec"
	"path/filepath"
	"strings"
)

func handleVersionGh(opts GlobalOptions) error {
	printCommonVersionInfo(opts)

	fmt.Fprintln(sys.Stdout)

	displayGhPath := opts.GhPath
	if resolved, err := exec.LookPath(opts.GhPath); err == nil {
		displayGhPath = resolved
	} else if filepath.IsAbs(opts.GhPath) {
		displayGhPath = opts.GhPath
	}
	fmt.Fprintf(sys.Stdout, "gh path: %s\n", displayGhPath)

	outGh, err := RunGh(opts.GhPath, false, "--version")
	if err == nil {
		linesGh := strings.Split(outGh, "\n")
		if len(linesGh) > 0 {
			fmt.Fprintln(sys.Stdout, linesGh[0])
		}
	} else {
		fmt.Fprintln(sys.Stdout, "Error getting gh version (gh might not be installed)")
	}
	return nil
}
