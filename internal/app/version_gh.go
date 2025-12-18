// Package app implements the core logic of the application.
package app

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func handleVersionGh(args []string, opts GlobalOptions) {
	// Parse flags for version command
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	var vLong, vShort bool
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}
	verbose := vLong || vShort

	printCommonVersionInfo(opts, verbose)

	fmt.Println()

	displayGhPath := opts.GhPath
	if resolved, err := exec.LookPath(opts.GhPath); err == nil {
		displayGhPath = resolved
	} else if filepath.IsAbs(opts.GhPath) {
		displayGhPath = opts.GhPath
	}
	fmt.Printf("gh path: %s\n", displayGhPath)

	outGh, err := RunGh(opts.GhPath, verbose, "--version")
	if err == nil {
		linesGh := strings.Split(outGh, "\n")
		if len(linesGh) > 0 {
			fmt.Println(linesGh[0])
		}
	} else {
		fmt.Println("Error getting gh version (gh might not be installed)")
	}
}
