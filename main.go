package main

import (
	"encoding/json"
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

func parseArgs() (string, string, []string) {
	var configFile string
	var args []string

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--file" || arg == "-f" {
			if i+1 < len(os.Args) {
				configFile = os.Args[i+1]
				i++
			} else {
				fmt.Println("Error: --file argument missing")
				os.Exit(1)
			}
		} else if strings.HasPrefix(arg, "--file=") {
			configFile = strings.TrimPrefix(arg, "--file=")
		} else if strings.HasPrefix(arg, "-f=") {
			configFile = strings.TrimPrefix(arg, "-f=")
		} else {
			args = append(args, arg)
		}
	}

	if len(args) == 0 {
		return configFile, "", nil
	}
	return configFile, args[0], args[1:]
}

func handleInit(args []string, config *Config) {
	fmt.Printf("init command called with args: %v\n", args)
	// Placeholder for init logic
}

func handlePrint(args []string, config *Config) {
	for _, repo := range config.Repositories {
		for _, label := range repo.Labels {
			fmt.Printf("%s,%s, %s\n", repo.Repo, repo.Branch, label)
		}
	}
}

func main() {
	configFile, cmdName, cmdArgs := parseArgs()

	if configFile == "" {
		fmt.Println("Error: Please specify a configuration file using --file or -f")
		os.Exit(1)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	config, err := ParseConfig(data)
	if err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	switch cmdName {
	case "init":
		handleInit(cmdArgs, config)
	case "print":
		handlePrint(cmdArgs, config)
	case "":
		// Default to print if no command provided
		handlePrint(cmdArgs, config)
	default:
		fmt.Printf("Unknown command: %s\n", cmdName)
		os.Exit(1)
	}
}
