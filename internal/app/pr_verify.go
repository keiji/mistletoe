package app

import (
	conf "mistletoe/internal/config"
)

import (
	"fmt"
	"strings"
)

// VerifyRevisionsUnchanged checks if the current HEAD revision of repositories matches the status collected earlier.
func VerifyRevisionsUnchanged(config *conf.Config, originalRows []StatusRow, gitPath string, verbose bool) error {
	statusMap := make(map[string]StatusRow)
	for _, row := range originalRows {
		statusMap[row.Repo] = row
	}

	for _, repo := range *config.Repositories {
		repoName := getRepoName(repo)
		originalRow, ok := statusMap[repoName]
		if !ok {
			// If not in original rows, maybe it was skipped or failed?
			// Ideally should be there if CollectStatus succeeded.
			continue
		}

		targetDir := config.GetRepoPath(repo)
		output, err := RunGit(targetDir, gitPath, verbose, "rev-parse", "HEAD")
		if err != nil {
			return fmt.Errorf("failed to get current revision for %s: %v", repoName, err)
		}
		currentRev := strings.TrimSpace(output)

		if currentRev != originalRow.LocalHeadFull {
			return fmt.Errorf("repository '%s' has changed since status collection (expected %s, got %s). Aborting.", repoName, originalRow.LocalHeadFull, currentRev)
		}
	}
	return nil
}
