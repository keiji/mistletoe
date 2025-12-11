package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// validateEnvironment checks if the current directory state is consistent with the configuration.
func validateEnvironment(repos []Repository, gitPath string) error {
	for _, repo := range repos {
		targetDir := getRepoDir(repo)
		info, err := os.Stat(targetDir)
		if os.IsNotExist(err) {
			continue // Directory doesn't exist, safe to clone
		}
		if err != nil {
			return fmt.Errorf("error checking directory %s: %v", targetDir, err)
		}

		if !info.IsDir() {
			return fmt.Errorf("target %s exists and is not a directory", targetDir)
		}

		// Check if it is a git repo
		gitDir := filepath.Join(targetDir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			// It's a git repo. Check remote.
			cmd := exec.Command(gitPath, "-C", targetDir, "config", "--get", "remote.origin.url")
			out, err := cmd.Output()
			if err != nil {
				// Failed to get remote origin (maybe none configured).
				return fmt.Errorf("directory %s is a git repo but failed to get remote origin: %v", targetDir, err)
			}
			currentURL := strings.TrimSpace(string(out))
			if currentURL != repo.URL {
				return fmt.Errorf("directory %s exists with different remote origin: %s (expected %s)", targetDir, currentURL, repo.URL)
			}
			// Match -> OK.
		} else {
			// Not a git repo. Check if empty.
			err := func() error {
				f, err := os.Open(targetDir)
				if err != nil {
					return fmt.Errorf("failed to open directory %s: %v", targetDir, err)
				}
				defer f.Close()

				_, err = f.Readdirnames(1)
				if err == nil {
					// No error means we found at least one file/dir, so it's not empty.
					return fmt.Errorf("directory %s exists, is not a git repo, and is not empty", targetDir)
				}
				// io.EOF is expected if empty.
				return nil
			}()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func handleInit(args []string, opts GlobalOptions) {
	var fShort, fLong string
	var depth int
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")
	fs.IntVar(&depth, "depth", 0, "Create a shallow clone with a history truncated to the specified number of commits")

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

	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := validateEnvironment(config.Repositories, opts.GitPath); err != nil {
		fmt.Println("Error validating environment:", err)
		os.Exit(1)
	}

	for _, repo := range config.Repositories {
		// 1. Git Clone
		// We prefer external git command.
		// "urlでgit cloneする。IDが指定されていればチェックアウト先のディレクトリ名としてidを採用する"
		gitArgs := []string{"clone"}
		if depth > 0 {
			gitArgs = append(gitArgs, "--depth", fmt.Sprintf("%d", depth))
		}
		gitArgs = append(gitArgs, repo.URL)
		targetDir := getRepoDir(repo)

		// Explicitly pass target directory to avoid ambiguity and to know where to checkout later.
		gitArgs = append(gitArgs, targetDir)

		// Check if directory already exists and is a git repo.
		// validateEnvironment already checked that if it exists, it's safe (matching remote).
		shouldClone := true
		if info, err := os.Stat(targetDir); err == nil && info.IsDir() {
			gitDir := filepath.Join(targetDir, ".git")
			if _, err := os.Stat(gitDir); err == nil {
				fmt.Printf("Repository %s already exists. Skipping clone.\n", targetDir)
				shouldClone = false
			}
		}

		if shouldClone {
			fmt.Printf("Cloning %s into %s...\n", repo.URL, targetDir)
			cmd := exec.Command(opts.GitPath, gitArgs...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Printf("Error cloning %s: %v\n", repo.URL, err)
				// Skip checkout if clone failed
				continue
			}
		}

		// 2. Switch Branch
		// "チェックアウト後、各要素についてbranchで示されたブランチに切り替える。"
		if repo.Branch != "" {
			fmt.Printf("Switching %s to branch %s...\n", targetDir, repo.Branch)
			checkoutCmd := exec.Command(opts.GitPath, "-C", targetDir, "checkout", repo.Branch)
			checkoutCmd.Stdout = os.Stdout
			checkoutCmd.Stderr = os.Stderr
			if err := checkoutCmd.Run(); err != nil {
				fmt.Printf("Error switching branch for %s: %v\n", targetDir, err)
			}
		}
	}
}
