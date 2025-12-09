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

	// Skip the first argument as it is the program name
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--file" || arg == "-f" {
			if i+1 < len(args) {
				configFile = args[i+1]
				i++
			} else {
				return "", "", nil, errors.New("Error: --file argument missing")
			}
		} else if strings.HasPrefix(arg, "--file=") {
			configFile = strings.TrimPrefix(arg, "--file=")
		} else if strings.HasPrefix(arg, "-f=") {
			configFile = strings.TrimPrefix(arg, "-f=")
		} else {
			subcmdArgs = append(subcmdArgs, arg)
		}
	}

	if len(subcmdArgs) == 0 {
		return configFile, "", nil, nil
	}
	return configFile, subcmdArgs[0], subcmdArgs[1:], nil
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
	return config, nil
}

func handleInit(args []string, configFile string) {
	fmt.Printf("init command called with args: %v\n", args)
	// Placeholder for init logic.
	// In the future, this might create the config file at 'configFile' or default location.
}

func handlePrint(args []string, configFile string) {
	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, repo := range config.Repositories {
		for _, label := range repo.Labels {
			fmt.Printf("%s,%s, %s\n", repo.Repo, repo.Branch, label)
		}
	}
}

func main() {
	configFile, subcmdName, subcmdArgs, err := parseArgs(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	switch subcmdName {
	case "init":
		handleInit(subcmdArgs, configFile)
	case "print":
		handlePrint(subcmdArgs, configFile)
	case "":
		// Default to print if no command provided
		handlePrint(subcmdArgs, configFile)
	default:
		fmt.Printf("Unknown subcommand: %s\n", subcmdName)
		os.Exit(1)
	}
}
