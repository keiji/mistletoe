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
	if err := RunApp(appType, version, hash, args, extraHandler); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// RunApp is the internal entry point that returns an error instead of exiting.
func RunApp(appType Type, version, hash string, args []string, extraHandler func(string, []string, GlobalOptions) bool) error {
	if appType == TypeMstlGh {
		AppName = AppNameMstlGh
	} else {
		AppName = AppNameMstl
	}
	AppVersion = version
	CommitHash = hash

	subcmdName, subcmdArgs, err := parseArgs(args)
	if err != nil {
		return err
	}

	// Determine if command allows skipping checks (e.g. help, version)
	isPermissive := subcmdName == CmdHelp || subcmdName == CmdVersion || subcmdName == ""

	// 1. Verify Git (Required for both mstl and mstl-gh)
	gitPath := getGitPath()
	gitErr := validateGit(gitPath)

	if gitErr != nil && !isPermissive {
		return fmt.Errorf("Error: Git is not callable at '%s'. (%v)", gitPath, gitErr)
	}

	// 2. Verify Gh (Required for mstl-gh only)
	ghPath := "gh"
	if appType == TypeMstlGh {
		ghPath = getGhPath()

		if !isPermissive {
			ghErr := validateGh(ghPath)
			if ghErr != nil {
				return fmt.Errorf("Error: GitHub CLI (gh) is not callable at '%s'. (%v)", ghPath, ghErr)
			}

			ghAuthErr := validateGhAuth(ghPath)
			if ghAuthErr != nil {
				return fmt.Errorf("Error: GitHub CLI (gh) is not logged in. Please run 'gh auth login'. (%v)", ghAuthErr)
			}
		}
	}

	opts := GlobalOptions{
		GitPath: gitPath,
		GhPath:  ghPath,
	}

	switch subcmdName {
	case CmdInit:
		return handleInit(subcmdArgs, opts)
	case CmdSnapshot:
		return handleSnapshot(subcmdArgs, opts)
	case CmdSwitch:
		return handleSwitch(subcmdArgs, opts)
	case CmdStatus:
		return handleStatus(subcmdArgs, opts)
	case CmdSync:
		return handleSync(subcmdArgs, opts)
	case CmdPush:
		return handlePush(subcmdArgs, opts)
	case CmdReset:
		return handleReset(subcmdArgs, opts)
	case CmdFire:
		return handleFire(subcmdArgs, opts)
	case CmdHelp:
		return handleHelp(subcmdArgs, opts)
	case CmdVersion:
		if appType == TypeMstlGh {
			return handleVersionGh(opts)
		}
		return handleVersionMstl(opts)
	case "":
		return handleHelp(subcmdArgs, opts)
	default:
		if extraHandler != nil && extraHandler(subcmdName, subcmdArgs, opts) {
			return nil
		}
		return fmt.Errorf("Unknown subcommand: %s.", subcmdName)
	}
}
