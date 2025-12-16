package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	MinParallel     = 1
	MaxParallel     = 128
	DefaultParallel = 1
)

var (
	AppName    string
	AppVersion string
	CommitHash string
)

type GlobalOptions struct {
	GitPath string
	GhPath  string
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


func getGitPath() string {
	if envPath := os.Getenv("GIT_EXEC_PATH"); envPath != "" {
		return filepath.Join(envPath, "git")
	}
	return "git"
}

func getGhPath() string {
	if envPath := os.Getenv("GH_EXEC_PATH"); envPath != "" {
		return filepath.Join(envPath, "gh")
	}
	return "gh"
}

func validateGit(gitPath string) error {
	cmd := exec.Command(gitPath, "--version")
	return cmd.Run()
}

// Run is the entry point for the application logic.
func Run(name, version, hash string, args []string) {
	AppName = name
	AppVersion = version
	CommitHash = hash

	subcmdName, subcmdArgs, err := parseArgs(args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	gitPath := getGitPath()
	gitErr := validateGit(gitPath)

	isPermissive := subcmdName == "help" || subcmdName == "version" || subcmdName == ""

	if gitErr != nil && !isPermissive {
		fmt.Printf("Error: Git is not callable at '%s'. (%v)\n", gitPath, gitErr)
		os.Exit(1)
	}

	ghPath := "gh"
	if AppName == "Mistletoe-gh" {
		ghPath = getGhPath()
	}

	opts := GlobalOptions{
		GitPath: gitPath,
		GhPath:  ghPath,
	}

	switch subcmdName {
	case "init":
		handleInit(subcmdArgs, opts)
	case "snapshot":
		handleSnapshot(subcmdArgs, opts)
	case "switch":
		handleSwitch(subcmdArgs, opts)
	case "status":
		handleStatus(subcmdArgs, opts)
	case "sync":
		handleSync(subcmdArgs, opts)
	case "push":
		handlePush(subcmdArgs, opts)
	case "pr":
		if AppName != "Mistletoe-gh" {
			fmt.Printf("Unknown subcommand: %s.\n", subcmdName)
			os.Exit(1)
		}
		handlePr(subcmdArgs, opts)
	case "help":
		handleHelp(subcmdArgs, opts)
	case "version":
		if AppName == "Mistletoe-gh" {
			handleVersionGh(opts)
		} else {
			handleVersionMstl(opts)
		}
	case "":
		handleHelp(subcmdArgs, opts)
	default:
		fmt.Printf("Unknown subcommand: %s.\n", subcmdName)
		os.Exit(1)
	}
}
