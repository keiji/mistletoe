// Package app implements the core logic of the application.
package app

import (
	"fmt"
	"mistletoe/internal/sys"
	"path/filepath"
	"strings"
)

func printCommonVersionInfo(opts GlobalOptions) {
	v := AppVersion
	if CommitHash != "" {
		v = fmt.Sprintf("%s-%s", AppVersion, CommitHash)
	}
	fmt.Fprintf(sys.Stdout, "%s %s\n", AppName, v)
	fmt.Fprintln(sys.Stdout, "Copyright 2025-2026 ARIYAMA Keiji")
	fmt.Fprintln(sys.Stdout, "https://github.com/keiji/mistletoe")
	fmt.Fprintln(sys.Stdout)

	if err := validateGit(opts.GitPath); err != nil {
		fmt.Fprintln(sys.Stdout, "Git binary not found")
		return
	}

	displayPath := opts.GitPath
	if resolved, err := lookPath(opts.GitPath); err == nil {
		displayPath = resolved
	} else if filepath.IsAbs(opts.GitPath) {
		displayPath = opts.GitPath
	}
	fmt.Fprintf(sys.Stdout, "git path: %s\n", displayPath)

	out, err := RunGit("", opts.GitPath, false, "--version")
	if err != nil {
		fmt.Fprintln(sys.Stdout, "Error getting git version")
		return
	}
	lines := strings.Split(out, "\n")
	if len(lines) > 0 {
		fmt.Fprintln(sys.Stdout, lines[0])
	}
}
