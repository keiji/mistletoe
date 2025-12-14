package app

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

func handleVersion(opts GlobalOptions) {
	v := AppVersion
	if CommitHash != "" {
		v = fmt.Sprintf("%s-%s", AppVersion, CommitHash)
	}
	fmt.Printf("%s version %s\n", AppName, v)
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
	case "pr":
		if AppName != "Mistletoe-gh" {
			fmt.Printf("Unknown subcommand: %s.\n", subcmdName)
			os.Exit(1)
		}
		handlePr(subcmdArgs, opts)
	case "help":
		handleHelp(subcmdArgs, opts)
	case "version":
		handleVersion(opts)
	case "":
		handleHelp(subcmdArgs, opts)
	default:
		fmt.Printf("Unknown subcommand: %s.\n", subcmdName)
		os.Exit(1)
	}
}
