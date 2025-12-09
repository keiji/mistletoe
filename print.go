package main

import (
	"errors"
	"fmt"
	"os"
)

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

func handlePrint(args []string, opts GlobalOptions) {
	config, err := loadConfig(opts.ConfigFile)
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
