// Package app implements the core logic of the application.
package app

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func handleVersion(opts GlobalOptions) {
	v := AppVersion
	if CommitHash != "" {
		v = fmt.Sprintf("%s-%s", AppVersion, CommitHash)
	}
	fmt.Printf("%s version %s\n", AppName, v)
	fmt.Println("Copyright 2025 ARIYAMA Keiji")
	fmt.Println("https://github.com/keiji/mistletoe")
	fmt.Println()

	if err := validateGit(opts.GitPath); err != nil {
		fmt.Println("Git binary not found")
		return
	}

	displayPath := opts.GitPath
	if resolved, err := exec.LookPath(opts.GitPath); err == nil {
		displayPath = resolved
	} else if filepath.IsAbs(opts.GitPath) {
		displayPath = opts.GitPath
	}
	fmt.Printf("git path: %s\n", displayPath)

	out, err := exec.Command(opts.GitPath, "--version").Output()
	if err != nil {
		fmt.Println("Error getting git version")
		return
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) > 0 {
		fmt.Println(lines[0])
	}

	if AppName == "Mistletoe-gh" {
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
}
