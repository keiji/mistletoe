package app

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushCmd(t *testing.T) {
	// Build mstl binary
	binPath := buildMstl(t)

	t.Run("No Push Needed", func(t *testing.T) {
		workDir := t.TempDir()
		remote1, _ := setupRemoteAndContent(t, 2)
		id1 := "repo1"
		exec.Command("git", "clone", remote1, filepath.Join(workDir, id1)).Run()

		config := Config{
			Repositories: &[]Repository{
				{ID: &id1, URL: &remote1},
			},
		}
		configFile := filepath.Join(workDir, "repos.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		cmd := exec.Command(binPath, "push", "-f", configFile)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("push command failed: %v", err)
		}
		output := string(out)
		if !strings.Contains(output, "No repositories to push.") {
			t.Errorf("Expected 'No repositories to push.', got: %s", output)
		}
	})

	t.Run("Push Needed - User Yes", func(t *testing.T) {
		workDir := t.TempDir()
		remote1, _ := setupRemoteAndContent(t, 2)
		id1 := "repo1"
		repoPath := filepath.Join(workDir, id1)
		exec.Command("git", "clone", remote1, repoPath).Run()

		// Add local commit
		configureGitUser(t, repoPath)
		fname := filepath.Join(repoPath, "new.txt")
		os.WriteFile(fname, []byte("new"), 0644)
		exec.Command("git", "-C", repoPath, "add", ".").Run()
		exec.Command("git", "-C", repoPath, "commit", "-m", "unpushed").Run()

		config := Config{
			Repositories: &[]Repository{
				{ID: &id1, URL: &remote1},
			},
		}
		configFile := filepath.Join(workDir, "repos.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		cmd := exec.Command(binPath, "push", "-f", configFile)
		cmd.Dir = workDir

		// Pipe input "y"
		cmd.Stdin = strings.NewReader("y\n")

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("push command failed: %v\nOutput: %s", err, string(out))
		}
		output := string(out)

		// Check output for "Pushing repo1..." (or similar)
		if !strings.Contains(output, "Pushing repo1") {
			t.Errorf("Expected output to indicate pushing repo1, got: %s", output)
		}

		// Verify remote has the commit
		cmdCheck := exec.Command("git", "-C", repoPath, "ls-remote", "origin", "HEAD")
		outCheck, _ := cmdCheck.Output()
		remoteSHA := strings.Fields(string(outCheck))[0]

		cmdLocal := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
		outLocal, _ := cmdLocal.Output()
		localSHA := strings.TrimSpace(string(outLocal))

		if remoteSHA != localSHA {
			t.Errorf("Remote HEAD (%s) does not match Local HEAD (%s) after push", remoteSHA, localSHA)
		}
	})

	t.Run("Push Needed - User No", func(t *testing.T) {
		workDir := t.TempDir()
		remote1, _ := setupRemoteAndContent(t, 2)
		id1 := "repo1"
		repoPath := filepath.Join(workDir, id1)
		exec.Command("git", "clone", remote1, repoPath).Run()

		// Add local commit
		configureGitUser(t, repoPath)
		fname := filepath.Join(repoPath, "new.txt")
		os.WriteFile(fname, []byte("new"), 0644)
		exec.Command("git", "-C", repoPath, "add", ".").Run()
		exec.Command("git", "-C", repoPath, "commit", "-m", "unpushed").Run()

		// Capture SHA before push attempt
		cmdLocal := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
		outLocal, _ := cmdLocal.Output()
		localSHA := strings.TrimSpace(string(outLocal))

		config := Config{
			Repositories: &[]Repository{
				{ID: &id1, URL: &remote1},
			},
		}
		configFile := filepath.Join(workDir, "repos.json")
		data, _ := json.Marshal(config)
		os.WriteFile(configFile, data, 0644)

		cmd := exec.Command(binPath, "push", "-f", configFile)
		cmd.Dir = workDir

		// Pipe input "n"
		cmd.Stdin = strings.NewReader("n\n")

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("push command failed: %v\nOutput: %s", err, string(out))
		}
		output := string(out)

		if strings.Contains(output, "Pushing repo1") {
			t.Errorf("Did not expect 'Pushing repo1' when user said no")
		}

		// Verify remote HEAD is NOT localSHA
		cmdCheck := exec.Command("git", "-C", repoPath, "ls-remote", "origin", "HEAD")
		outCheck, _ := cmdCheck.Output()
		remoteSHA := strings.Fields(string(outCheck))[0]

		if remoteSHA == localSHA {
			t.Errorf("Remote HEAD (%s) matched Local HEAD (%s) but push should have been skipped", remoteSHA, localSHA)
		}
	})
}
