// Package main is the entry point for the gitc tool.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

const appVersion = "0.0.1"

var commitHash string

type Repository struct {
	ID       *string  `json:"id"`
	URL      string   `json:"url"`
	Branch   string   `json:"branch,omitempty"`
	Revision string   `json:"revision,omitempty"`
	Labels   []string `json:"labels"`
}

type Config struct {
	Repositories []Repository `json:"repositories"`
}

type GlobalOptions struct {
	ConfigFile string
	GitPath    string
}

func ParseConfig(data []byte) (*Config, error) {
	var config Config
	err := json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// validateRepositories checks for duplicate IDs in the repository list.
// IDs that are nil are ignored.
func validateRepositories(repos []Repository) error {
	seenIDs := make(map[string]bool)
	for _, repo := range repos {
		if repo.ID != nil && *repo.ID != "" {
			if seenIDs[*repo.ID] {
				return fmt.Errorf("duplicate repository ID found: %s", *repo.ID)
			}
			seenIDs[*repo.ID] = true
		}
	}
	return nil
}

// getRepoDir determines the checkout directory name.
// If ID is present and not empty, it is used. Otherwise, it is derived from the URL.
func getRepoDir(repo Repository) string {
	if repo.ID != nil && *repo.ID != "" {
		return *repo.ID
	}
	// Derive from URL using path.Base because URLs use forward slashes
	url := strings.TrimRight(repo.URL, "/")
	base := path.Base(url)
	return strings.TrimSuffix(base, ".git")
}

func loadConfig(configFile string) (*Config, error) {
	if configFile == "" {
		return nil, errors.New("Error: Please specify a configuration file using --file or -f")
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("Error reading file: %v", err)
	}

	config, err := ParseConfig(data)
	if err != nil {
		return nil, fmt.Errorf("Error parsing JSON: %v", err)
	}

	if err := validateRepositories(config.Repositories); err != nil {
		return nil, fmt.Errorf("Error validating configuration: %v", err)
	}

	return config, nil
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
