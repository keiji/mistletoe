// Package app implements the core logic of the application.
package app

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func handleVersionGh(opts GlobalOptions) {
	printCommonVersionInfo(opts)

	fmt.Println()

	displayGhPath := opts.GhPath
	if resolved, err := exec.LookPath(opts.GhPath); err == nil {
		displayGhPath = resolved
	} else if filepath.IsAbs(opts.GhPath) {
		displayGhPath = opts.GhPath
	}
	fmt.Printf("gh path: %s\n", displayGhPath)

	outGh, err := exec.Command(opts.GhPath, "--version").Output()
	if err == nil {
		linesGh := strings.Split(string(outGh), "\n")
		if len(linesGh) > 0 {
			fmt.Println(linesGh[0])
		}
	} else {
		fmt.Println("Error getting gh version (gh might not be installed)")
	}
}
