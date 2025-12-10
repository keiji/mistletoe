// Package main is the entry point for the gitc tool.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const appVersion = "0.0.1"

var commitHash string

type Repository struct {
	ID     *string  `json:"id"`
	URL    string   `json:"url"`
	Branch string   `json:"branch"`
	Labels []string `json:"labels"`
}

type Config struct {
	Repositories []Repository `json:"repositories"`
}

type GlobalOptions struct {
	ConfigFile string
}

func ParseConfig(data []byte) (*Config, error) {
	var config Config
	err := json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
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

func handleVersion() {
	v := appVersion
	if commitHash != "" {
		v = fmt.Sprintf("%s-%s", appVersion, commitHash)
	}
	fmt.Printf("gitc version %s\n", v)

	path, err := exec.LookPath("git")
	if err != nil {
		fmt.Println("git path: not found")
		fmt.Println("git version: not found")
		return
	}
	fmt.Printf("git path: %s\n", path)

	out, err := exec.Command("git", "--version").Output()
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

func main() {
	configFile, subcmdName, subcmdArgs, err := parseArgs(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	opts := GlobalOptions{
		ConfigFile: configFile,
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
		handleVersion()
	case "":
		// Default to print if no command provided
		handlePrint(subcmdArgs, opts)
	default:
		fmt.Printf("Unknown subcommand: %s\n", subcmdName)
		os.Exit(1)
	}
}
