package main

import (
	"fmt"
	"os"
)

func handlePrint(_ []string, opts GlobalOptions) {
	config, err := loadConfig(opts.ConfigFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, repo := range config.Repositories {
		for _, label := range repo.Labels {
			branch := ""
			if repo.Branch != nil {
				branch = *repo.Branch
			}
			fmt.Printf("%s,%s, %s\n", repo.URL, branch, label)
		}
	}
}
