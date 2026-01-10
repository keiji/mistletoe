// Package app implements the core logic of the application.
package app

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func printCommonVersionInfo(opts GlobalOptions) {
	v := AppVersion
	if CommitHash != "" {
		v = fmt.Sprintf("%s-%s", AppVersion, CommitHash)
	}
	fmt.Printf("%s %s\n", AppName, v)
	fmt.Println("Copyright 2025-2026 ARIYAMA Keiji")
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

	out, err := RunGit("", opts.GitPath, false, "--version")
	if err != nil {
		fmt.Println("Error getting git version")
		return
	}
	lines := strings.Split(out, "\n")
	if len(lines) > 0 {
		fmt.Println(lines[0])
	}
}
