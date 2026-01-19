// Package app implements the core logic of the application.
package app

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func handleVersionGh(opts GlobalOptions) error {
	printCommonVersionInfo(opts)

	fmt.Println()

	displayGhPath := opts.GhPath
	if resolved, err := exec.LookPath(opts.GhPath); err == nil {
		displayGhPath = resolved
	} else if filepath.IsAbs(opts.GhPath) {
		displayGhPath = opts.GhPath
	}
	fmt.Printf("gh path: %s\n", displayGhPath)

	outGh, err := RunGh(opts.GhPath, false, "--version")
	if err == nil {
		linesGh := strings.Split(outGh, "\n")
		if len(linesGh) > 0 {
			fmt.Println(linesGh[0])
		}
	} else {
		fmt.Println("Error getting gh version (gh might not be installed)")
	}
	return nil
}
