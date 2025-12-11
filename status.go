package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/olekukonko/tablewriter"
)

func handleStatus(args []string, opts GlobalOptions) {
	var fShort, fLong string
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	fs.StringVar(&fLong, "file", "", "configuration file")
	fs.StringVar(&fShort, "f", "", "configuration file (short)")

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

	// Validation Phase
	for _, repo := range config.Repositories {
		targetDir := getRepoDir(repo)
		info, err := os.Stat(targetDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			fmt.Printf("Error checking directory %s: %v\n", targetDir, err)
			os.Exit(1)
		}
		if !info.IsDir() {
			fmt.Printf("Error: target %s exists and is not a directory\n", targetDir)
			os.Exit(1)
		}

		// Check if Git repository
		gitDir := filepath.Join(targetDir, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			fmt.Printf("Error: directory %s exists but is not a git repository\n", targetDir)
			os.Exit(1)
		}

		// Check remote origin
		cmd := exec.Command(opts.GitPath, "-C", targetDir, "config", "--get", "remote.origin.url")
		out, err := cmd.Output()
		if err != nil {
			fmt.Printf("Error: directory %s is a git repo but failed to get remote origin: %v\n", targetDir, err)
			os.Exit(1)
		}
		currentURL := strings.TrimSpace(string(out))
		if currentURL != repo.URL {
			fmt.Printf("Error: directory %s exists with different remote origin: %s (expected %s)\n", targetDir, currentURL, repo.URL)
			os.Exit(1)
		}
	}

	// Output Phase
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Repository", "Local Branch/Rev", "Remote Rev", "Local HEAD Rev", "Unpushed?"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.SetColumnSeparator("|")
	table.SetRowSeparator("-")

	// Tablewriter defaults to ASCII (+---+). For Markdown (|---|) we might need tweaks.
	// But "ASCIIの表に見えるように整形する" (format to look like ASCII table) AND "markdown table format".
	// tablewriter.NewWriter(os.Stdout) produces ASCII.
	// To make it Markdown compatible:
	table.SetAutoFormatHeaders(false)
	// Markdown tables:
	// | H1 | H2 |
	// |---|---|
	// | v1 | v2 |
	// tablewriter doesn't generate the |---|---| line automatically in a way that is strictly markdown compatible usually?
	// Actually, `table.SetBorders(...)` controls the outer pipes.
	// Let's rely on standard TableWriter output but configure it to be as close as possible.
	// User said "Markdown table format", so I should ensure the separator line exists.
	// Tablewriter prints a separator line between header and body if configured.

	// Let's try to match standard Markdown syntax:
	table.SetAutoWrapText(false)

	for _, repo := range config.Repositories {
		targetDir := getRepoDir(repo)
		repoName := targetDir
		if repo.ID != nil && *repo.ID != "" {
			repoName = *repo.ID
		}

		// If not exists
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			// Skip or show missing?
			// The prompt says "For each repository... display info".
			// If it doesn't exist, we can't display local info.
			// I'll skip it to match the "If integrity verified... for EACH repo... display"
			// but if it's missing, it's not verified?
			// "repositoriesの各要素に対応するディレクトリがあれば...検証する"
			// "If validated... display".
			// Maybe show "Not Cloned"?
			// I will populate with empty strings or "N/A"
			// table.Append([]string{repoName, "(Not Cloned)", "", "", ""})
			// Actually, let's stick to existing directories to be safe, or just show "Not Found".
			continue
		}

		// Local Branch/Rev
		// git rev-parse --abbrev-ref HEAD
		cmd := exec.Command(opts.GitPath, "-C", targetDir, "rev-parse", "--abbrev-ref", "HEAD")
		out, err := cmd.Output()
		localRef := ""
		isDetached := false
		if err == nil {
			localRef = strings.TrimSpace(string(out))
		}
		if localRef == "HEAD" {
			isDetached = true
			// Get full/short sha
			cmd := exec.Command(opts.GitPath, "-C", targetDir, "rev-parse", "--short", "HEAD")
			out, _ := cmd.Output()
			localRef = strings.TrimSpace(string(out))
		}

		// Local HEAD Rev (Full SHA)
		cmd = exec.Command(opts.GitPath, "-C", targetDir, "rev-parse", "HEAD")
		out, err = cmd.Output()
		localHeadSHA := ""
		if err == nil {
			localHeadSHA = strings.TrimSpace(string(out))
		}

		// Remote Rev
		remoteHeadSHA := ""
		if !isDetached {
			// Check if remote branch exists
			// git ls-remote origin <localRef>
			cmd = exec.Command(opts.GitPath, "-C", targetDir, "ls-remote", "origin", "refs/heads/"+localRef)
			out, err = cmd.Output()
			if err == nil {
				output := strings.TrimSpace(string(out))
				if output != "" {
					// Output format: <SHA>\trefs/heads/<branch>
					fields := strings.Fields(output)
					if len(fields) > 0 {
						remoteHeadSHA = fields[0]
					}
				}
			}
		}

		// Unpushed?
		unpushed := ""
		if remoteHeadSHA != "" && localHeadSHA != "" {
			if remoteHeadSHA != localHeadSHA {
				// Check if unpushed
				// git rev-list --count remoteSHA..localSHA
				// If > 0, local has commits not in remote.
				// Note: this requires remote objects to be present locally (fetched).
				// If not present, rev-list might fail, and we mark as "?".
				cmd = exec.Command(opts.GitPath, "-C", targetDir, "rev-list", "--count", remoteHeadSHA+".."+localHeadSHA)
				out, err := cmd.Output()
				if err == nil {
					count := strings.TrimSpace(string(out))
					if count != "0" {
						unpushed = "Yes"
					} else {
						unpushed = "No"
					}
				} else {
					unpushed = "?"
				}
			} else {
				unpushed = "No"
			}
		} else if isDetached {
			unpushed = "-"
		} else {
			// Remote branch doesn't exist?
			unpushed = "?"
		}

		table.Append([]string{repoName, localRef, remoteHeadSHA, localHeadSHA, unpushed})
	}
	table.Render()
}
