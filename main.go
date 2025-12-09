package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
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

func main() {
	var fileFlag string
	var fFlag string

	flag.StringVar(&fileFlag, "file", "", "Configuration file path")
	flag.StringVar(&fFlag, "f", "", "Configuration file path (shorthand)")
	flag.Parse()

	configFile := fileFlag
	if configFile == "" {
		configFile = fFlag
	}

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

	for _, repo := range config.Repositories {
		for _, label := range repo.Labels {
			fmt.Printf("%s,%s, %s\n", repo.Repo, repo.Branch, label)
		}
	}
}
