package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Repository struct {
	Repo   string   `json:"repo"`
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
			if arg == "--file" || arg == "-f" {
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
	case "print":
		handlePrint(subcmdArgs, opts)
	case "":
		// Default to print if no command provided
		handlePrint(subcmdArgs, opts)
	default:
		fmt.Printf("Unknown subcommand: %s\n", subcmdName)
		os.Exit(1)
	}
}
