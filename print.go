package main

import (
	"flag"
	"fmt"
	"os"
)

func handlePrint(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var lLong, lShort string

	fs := flag.NewFlagSet("print", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")
	fs.StringVar(&lLong, "labels", "", "comma-separated list of labels to filter repositories")
	fs.StringVar(&lShort, "l", "", "labels (short)")

	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	configFile := opts.ConfigFile
	if fLong != "" {
		configFile = fLong
	} else if fShort != "" {
		configFile = fShort
	}

	labels := lLong
	if lShort != "" {
		labels = lShort
	}

	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	repos := FilterRepositories(*config.Repositories, labels)

	for _, repo := range repos {
		for _, label := range repo.Labels {
			branch := ""
			if repo.Branch != nil {
				branch = *repo.Branch
			}
			fmt.Printf("%s,%s, %s\n", *repo.URL, branch, label)
		}
	}
}
