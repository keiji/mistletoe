package main

import (
	"flag"
	"fmt"
	"os"
)

func handlePrint(args []string, _ GlobalOptions) {
	var fShort, fLong string

	fs := flag.NewFlagSet("print", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	configFile := fLong
	if configFile == "" {
		configFile = fShort
	}

	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, repo := range *config.Repositories {
		branch := ""
		if repo.Branch != nil {
			branch = *repo.Branch
		}
		fmt.Printf("%s,%s\n", *repo.URL, branch)
	}
}
