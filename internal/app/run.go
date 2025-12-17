package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Parallel processing constants.
const (
	MinParallel     = 1
	MaxParallel     = 128
	DefaultParallel = 1
)

// Global application variables.
var (
	AppName    string
	AppVersion string
	CommitHash string
)

// GlobalOptions holds global command-line options.
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
func Run(appType Type, version, hash string, args []string) {
	if appType == TypeMstlGh {
		AppName = AppNameMstlGh
	} else {
		AppName = AppNameMstl
	}
	AppVersion = version
	CommitHash = hash

	subcmdName, subcmdArgs, err := parseArgs(args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	gitPath := getGitPath()
	gitErr := validateGit(gitPath)

	isPermissive := subcmdName == CmdHelp || subcmdName == CmdVersion || subcmdName == ""

	if gitErr != nil && !isPermissive {
		fmt.Printf("Error: Git is not callable at '%s'. (%v)\n", gitPath, gitErr)
		os.Exit(1)
	}

	ghPath := "gh"
	if appType == TypeMstlGh {
		ghPath = getGhPath()
	}

	opts := GlobalOptions{
		GitPath: gitPath,
		GhPath:  ghPath,
	}

	switch subcmdName {
	case CmdInit:
		handleInit(subcmdArgs, opts)
	case CmdSnapshot:
		handleSnapshot(subcmdArgs, opts)
	case CmdSwitch:
		handleSwitch(subcmdArgs, opts)
	case CmdStatus:
		handleStatus(subcmdArgs, opts)
	case CmdSync:
		handleSync(subcmdArgs, opts)
	case CmdPush:
		handlePush(subcmdArgs, opts)
	case CmdPr:
		if appType != TypeMstlGh {
			fmt.Printf("Unknown subcommand: %s.\n", subcmdName)
			os.Exit(1)
		}
		handlePr(subcmdArgs, opts)
	case CmdHelp:
		handleHelp(subcmdArgs, opts)
	case CmdVersion:
		if appType == TypeMstlGh {
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
