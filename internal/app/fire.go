package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	conf "mistletoe/internal/config"
	"mistletoe/internal/sys"
	"os"
	"os/user"
	"strings"
	"sync"
)

// handleFire handles the fire subcommand.
func handleFire(_ []string, opts GlobalOptions) error {
	// "fire" takes no options and implies --yes behavior.
	// It uses the default config file search logic.

	configFile, configData, err := resolveConfigForFire(opts.GitPath)
	if err != nil {
		return err
	}

	var config *conf.Config
	if configFile != "" {
		config, err = conf.LoadConfigFile(configFile)
	} else {
		config, err = conf.LoadConfigData([]byte(configData))
	}

	if err != nil {
		return err
	}

	return fireCommand(config, opts)
}

// resolveConfigForFire finds the configuration file without any flags.
func resolveConfigForFire(gitPath string) (string, string, error) {
	// Default behavior: look for config in current or parent dirs.
	// We mimic ResolveCommonValues but simpler since we have no flags.
	f, err := SearchParentConfig(DefaultConfigFile, []byte(""), gitPath)
	return f, "", err
}

func fireCommand(config *conf.Config, opts GlobalOptions) error {
	// Generate unique branch suffix components once
	username := getSafeUsername()
	uuid := getShortUUID()

	fmt.Fprintf(sys.Stdout, "🔥 FIRE command initiated. Branch suffix: %s-%s\n", username, uuid)

	// We want to run this in parallel for speed.
	jobs := DefaultJobs
	if config.Jobs != nil && *config.Jobs > 0 {
		jobs = *config.Jobs
	}

	// Because config.Repositories is a pointer to a slice
	repos := *config.Repositories

	tasks := make(chan conf.Repository, len(repos))
	var wg sync.WaitGroup

	// Push tasks
	for _, repo := range repos {
		tasks <- repo
	}
	close(tasks)

	// Start workers
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repo := range tasks {
				// Use config helper to resolve path
				path := config.GetRepoPath(repo)

				// ID is a pointer, need to dereference carefully
				id := ""
				if repo.ID != nil {
					id = *repo.ID
				} else {
					// Should have been set by LoadConfig but to be safe
					id = conf.GetRepoDirName(repo)
				}

				processFireRepo(id, path, opts.GitPath, username, uuid)
			}
		}()
	}

	wg.Wait()
	fmt.Fprintln(sys.Stdout, "🔥 FIRE command completed.")
	return nil
}

func processFireRepo(repoID, repoPath, gitPath, username, uuid string) {
	baseBranchName := fmt.Sprintf("mstl-fire-%s-%s-%s", repoID, username, uuid)
	branchName := baseBranchName

	// Retry loop to avoid collision
	for i := 0; i < 5; i++ {
		if i > 0 {
			branchName = fmt.Sprintf("%s-%d", baseBranchName, i)
		}

		// 1. Switch -c <branch> (or checkout -b)
		if err := runGitFire(repoPath, gitPath, "checkout", "-b", branchName); err != nil {
			fmt.Fprintf(sys.Stderr, "[%s] Error creating branch: %v\n", repoID, err)
			return
		}

		// 2. Add .
		if err := runGitFire(repoPath, gitPath, "add", "."); err != nil {
			fmt.Fprintf(sys.Stderr, "[%s] Error staging changes: %v\n", repoID, err)
		}

		// 3. Commit
		msg := fmt.Sprintf("Emergency commit triggered by %s fire command.", AppName)
		if err := runGitFire(repoPath, gitPath, "commit", "-m", msg, "--no-gpg-sign"); err != nil {
			fmt.Fprintf(sys.Stderr, "[%s] Error committing (might be empty): %v\n", repoID, err)
		}

		// 4. Push
		if err := runGitFire(repoPath, gitPath, "push", "-u", "origin", branchName); err != nil {
			fmt.Fprintf(sys.Stderr, "[%s] Error pushing to %s: %v. Retrying with new branch...\n", repoID, branchName, err)
			continue
		}

		fmt.Fprintf(sys.Stdout, "[%s] Secured in %s\n", repoID, branchName)
		return
	}

	fmt.Fprintf(sys.Stderr, "[%s] Failed to find available branch name after retries.\n", repoID)
}

func runGitFire(dir, gitPath string, args ...string) error {
	cmd := sys.ExecCommand(gitPath, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func getSafeUsername() string {
	u, err := user.Current()
	if err != nil {
		// Fallback to env vars
		name := os.Getenv("USER")
		if name == "" {
			name = os.Getenv("USERNAME")
		}
		if name == "" {
			return "unknown"
		}
		return sanitizeName(name)
	}

	// Use username, handle Windows "domain\user"
	name := u.Username
	if parts := strings.Split(name, "\\"); len(parts) > 1 {
		name = parts[len(parts)-1]
	}
	return sanitizeName(name)
}

func sanitizeName(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	return sb.String()
}

func getShortUUID() string {
	b := make([]byte, 4) // 4 bytes = 8 hex chars
	_, err := rand.Read(b)
	if err != nil {
		return "emergency"
	}
	return hex.EncodeToString(b)
}
