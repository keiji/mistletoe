package app

import (
	"fmt"
	"os"
	"path/filepath"

	"mistletoe/internal/sys"
)

// Jobs processing constants.
const (
	MinJobs     = 1
	MaxJobs     = 128
	DefaultJobs = 1
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
	cmd := sys.ExecCommand(gitPath, "--version")
	return cmd.Run()
}

func validateGh(ghPath string) error {
	cmd := sys.ExecCommand(ghPath, "--version")
	return cmd.Run()
}

func validateGhAuth(ghPath string) error {
	cmd := sys.ExecCommand(ghPath, "auth", "status")
	return cmd.Run()
}

// Run is the entry point for the application logic.
func Run(appType Type, version, hash string, args []string, extraHandler func(string, []string, GlobalOptions) bool) {
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

	// Determine if command allows skipping checks (e.g. help, version)
	isPermissive := subcmdName == CmdHelp || subcmdName == CmdVersion || subcmdName == ""

	// 1. Verify Git (Required for both mstl and mstl-gh)
	gitPath := getGitPath()
	gitErr := validateGit(gitPath)

	if gitErr != nil && !isPermissive {
		fmt.Printf("Error: Git is not callable at '%s'. (%v)\n", gitPath, gitErr)
		os.Exit(1)
	}

	// 2. Verify Gh (Required for mstl-gh only)
	ghPath := "gh"
	if appType == TypeMstlGh {
		ghPath = getGhPath()

		if !isPermissive {
			ghErr := validateGh(ghPath)
			if ghErr != nil {
				fmt.Printf("Error: GitHub CLI (gh) is not callable at '%s'. (%v)\n", ghPath, ghErr)
				os.Exit(1)
			}

			ghAuthErr := validateGhAuth(ghPath)
			if ghAuthErr != nil {
				fmt.Printf("Error: GitHub CLI (gh) is not logged in. Please run 'gh auth login'. (%v)\n", ghAuthErr)
				os.Exit(1)
			}
		}
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
		if extraHandler != nil && extraHandler(subcmdName, subcmdArgs, opts) {
			return
		}
		fmt.Printf("Unknown subcommand: %s.\n", subcmdName)
		os.Exit(1)
	}
}
