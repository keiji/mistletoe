// Package main is the entry point for the gitc tool.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const appVersion = "0.0.1"

const (
	MinParallel     = 1
	MaxParallel     = 128
	DefaultParallel = 1
)

var commitHash string

type GlobalOptions struct {
	ConfigFile string
	GitPath    string
}

func parseArgs(args []string) (string, string, []string, error) {
	var configFile string
	var subcmdArgs []string
	parsingGlobalOptions := true

	// Skip the first argument as it is the program name
	for i := 1; i < len(args); i++ {
		arg := args[i]

		if parsingGlobalOptions {
			if arg == "--version" || arg == "-v" {
				return "", "version", nil, nil
			} else if arg == "--file" || arg == "-f" {
				if i+1 < len(args) {
					configFile = args[i+1]
					i++
				} else {
					return "", "", nil, errors.New("Error: --file argument missing")
				}
				continue
			} else if strings.HasPrefix(arg, "--file=") {
				configFile = strings.TrimPrefix(arg, "--file=")
				continue
			} else if strings.HasPrefix(arg, "-f=") {
				configFile = strings.TrimPrefix(arg, "-f=")
				continue
			}
		}

		parsingGlobalOptions = false
		subcmdArgs = append(subcmdArgs, arg)
	}

	if len(subcmdArgs) == 0 {
		return configFile, "", nil, nil
	}
	return configFile, subcmdArgs[0], subcmdArgs[1:], nil
}

func handleVersion(opts GlobalOptions) {
	v := appVersion
	if commitHash != "" {
		v = fmt.Sprintf("%s-%s", appVersion, commitHash)
	}
	fmt.Printf("gitc version %s\n", v)

	// In handleVersion, we check existence again or assume the one passed in GlobalOptions
	// If the startup validation failed for print/version, opts.GitPath is still set to what was attempted.

	if err := validateGit(opts.GitPath); err != nil {
		fmt.Println("git binary is not found")
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
		fmt.Println("git version: error getting version")
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
	configFile, subcmdName, subcmdArgs, err := parseArgs(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	gitPath := getGitPath()
	gitErr := validateGit(gitPath)

	// If git is not found/valid, we error out unless the command is print or version (or default/empty which is print)
	isPermissive := subcmdName == "print" || subcmdName == "version" || subcmdName == ""

	if gitErr != nil && !isPermissive {
		fmt.Printf("Error: git is not callable at '%s'. (%v)\n", gitPath, gitErr)
		os.Exit(1)
	}

	opts := GlobalOptions{
		ConfigFile: configFile,
		GitPath:    gitPath,
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
	case "push":
		handlePush(subcmdArgs, opts)
	case "print":
		handlePrint(subcmdArgs, opts)
	case "version":
		handleVersion(opts)
	case "":
		// Default to print if no command provided
		handlePrint(subcmdArgs, opts)
	default:
		fmt.Printf("Unknown subcommand: %s\n", subcmdName)
		os.Exit(1)
	}
}
