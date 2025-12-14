// Package main is the entry point for the mstl tool.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	MinParallel     = 1
	MaxParallel     = 128
	DefaultParallel = 1
)

var commitHash string

type GlobalOptions struct {
	GitPath string
}

func parseArgs(args []string) (string, []string, error) {
	// Skip the first argument as it is the program name
	if len(args) < 2 {
		return "", nil, nil
	}

	subcmdName := args[1]
	subcmdArgs := args[2:]

	return subcmdName, subcmdArgs, nil
}

func handleVersion(opts GlobalOptions) {
	v := appVersion
	if commitHash != "" {
		v = fmt.Sprintf("%s-%s", appVersion, commitHash)
	}
	fmt.Printf("%s version %s\n", appName, v)
	fmt.Println("https://github.com/keiji/mistletoe")
	fmt.Println()

	// In handleVersion, we check existence again or assume the one passed in GlobalOptions
	// If the startup validation failed for help/version, opts.GitPath is still set to what was attempted.

	if err := validateGit(opts.GitPath); err != nil {
		fmt.Println("Git binary not found")
		return
	}

	// Resolve absolute path for display if possible, mostly for clarity
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
	// git --version output typically includes a newline. We only want the first line.
	lines := strings.Split(string(out), "\n")
	if len(lines) > 0 {
		fmt.Println(lines[0])
	}
}

func getGitPath() string {
	if envPath := os.Getenv("GIT_EXEC_PATH"); envPath != "" {
		return filepath.Join(envPath, "git")
	}
	return "git"
}

func validateGit(gitPath string) error {
	cmd := exec.Command(gitPath, "--version")
	return cmd.Run()
}

func main() {
	subcmdName, subcmdArgs, err := parseArgs(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	gitPath := getGitPath()
	gitErr := validateGit(gitPath)

	// If git is not found/valid, we error out unless the command is help or version (or default/empty which is help)
	isPermissive := subcmdName == "help" || subcmdName == "version" || subcmdName == ""

	if gitErr != nil && !isPermissive {
		fmt.Printf("Error: Git is not callable at '%s'. (%v)\n", gitPath, gitErr)
		os.Exit(1)
	}

	opts := GlobalOptions{
		GitPath: gitPath,
	}

	switch subcmdName {
	case "init":
		handleInit(subcmdArgs, opts)
	case "freeze":
		handleFreeze(subcmdArgs, opts)
	case "switch":
		handleSwitch(subcmdArgs, opts)
	case "status":
		handleStatus(subcmdArgs, opts)
	case "sync":
		handleSync(subcmdArgs, opts)
	case "push":
		handlePush(subcmdArgs, opts)
	case "help":
		handleHelp(subcmdArgs, opts)
	case "version":
		handleVersion(opts)
	case "":
		// Default to help if no command provided
		handleHelp(subcmdArgs, opts)
	default:
		fmt.Printf("Unknown subcommand: %s.\n", subcmdName)
		os.Exit(1)
	}
}
