package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

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

func handleInit(args []string, opts GlobalOptions) {
	var fShort, fLong string
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")

	if err := fs.Parse(args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	configFile := opts.ConfigFile
	if fLong != "" {
		configFile = fLong
	} else if fShort != "" {
		configFile = fShort
	}

	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := validateRepositories(config.Repositories); err != nil {
		fmt.Println("Error validating configuration:", err)
		os.Exit(1)
	}

	for _, repo := range config.Repositories {
		// 1. Git Clone
		// We prefer external git command.
		// "urlでgit cloneする。IDが指定されていればチェックアウト先のディレクトリ名としてidを採用する"
		gitArgs := []string{"clone", repo.URL}
		targetDir := getRepoDir(repo)

		// Explicitly pass target directory to avoid ambiguity and to know where to checkout later.
		gitArgs = append(gitArgs, targetDir)

		fmt.Printf("Cloning %s into %s...\n", repo.URL, targetDir)
		cmd := exec.Command("git", gitArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error cloning %s: %v\n", repo.URL, err)
			// Skip checkout if clone failed
			continue
		}

		// 2. Switch Branch
		// "チェックアウト後、各要素についてbranchで示されたブランチに切り替える。"
		if repo.Branch != "" {
			fmt.Printf("Switching %s to branch %s...\n", targetDir, repo.Branch)
			checkoutCmd := exec.Command("git", "-C", targetDir, "checkout", repo.Branch)
			checkoutCmd.Stdout = os.Stdout
			checkoutCmd.Stderr = os.Stderr
			if err := checkoutCmd.Run(); err != nil {
				fmt.Printf("Error switching branch for %s: %v\n", targetDir, err)
			}
		}
	}
}
