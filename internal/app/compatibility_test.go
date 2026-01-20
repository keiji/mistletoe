package app


import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildBinaryForTest builds the specified binary (mstl or mstl-gh) from cmd/ and returns its path.
func buildBinaryForTest(t *testing.T, cmdName string) string {
	binPath := filepath.Join(t.TempDir(), cmdName)
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	// Determine root dir relative to internal/app
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to get root dir: %v", err)
	}
	cmdSrcPath := filepath.Join(rootDir, "cmd", cmdName)

	// Check if source exists
	if _, err := os.Stat(cmdSrcPath); os.IsNotExist(err) {
		t.Fatalf("source for %s not found at %s", cmdName, cmdSrcPath)
	}

	// Inject a version so we can test the output format "mstl v..."
	ldflags := "-X main.appVersion=v0.0.0-test"
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", binPath, cmdSrcPath)
	// Inherit env to ensure we have cache etc.
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build %s: %v\nOutput: %s", cmdName, err, output)
	}
	return binPath
}

// setupMockGh builds a dummy gh executable that always succeeds.
// It returns the directory containing the executable, suitable for GH_EXEC_PATH.
func setupMockGh(t *testing.T) string {
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	content := `package main
import "os"
func main() { os.Exit(0) }
`
	if err := os.WriteFile(src, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	binName := "gh"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(dir, binName)

	cmd := exec.Command("go", "build", "-o", binPath, src)
	// Use same env as test process (needed for go build cache etc)
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build mock gh: %v", err)
	}
	return dir
}

func TestMstlAndMstlGhCompatibility(t *testing.T) {
	// 1. Build binaries
	binMstl := buildBinaryForTest(t, "mstl")
	binMstlGh := buildBinaryForTest(t, "mstl-gh")

	// 2. Test 'version' command
	t.Run("version", func(t *testing.T) {
		outMstl, err := exec.Command(binMstl, "version").Output()
		if err != nil {
			t.Fatalf("mstl version failed: %v", err)
		}
		if !strings.Contains(string(outMstl), "mstl v") {
			t.Errorf("mstl version output unexpected: %s", outMstl)
		}

		outMstlGh, err := exec.Command(binMstlGh, "version").Output()
		if err != nil {
			t.Fatalf("mstl-gh version failed: %v", err)
		}
		if !strings.Contains(string(outMstlGh), "mstl-gh v") {
			t.Errorf("mstl-gh version output unexpected: %s", outMstlGh)
		}
	})

	// 3. Test 'help' command structure
	t.Run("help", func(t *testing.T) {
		outMstl, _ := exec.Command(binMstl, "help").Output()
		outMstlGh, _ := exec.Command(binMstlGh, "help").Output()

		strMstl := string(outMstl)
		strMstlGh := string(outMstlGh)

		// Basic check: both should contain common commands
		commonCmds := []string{"init", "status", "sync", "push", "reset", "fire"}
		for _, cmd := range commonCmds {
			if !strings.Contains(strMstl, cmd) {
				t.Errorf("mstl help missing %s", cmd)
			}
			if !strings.Contains(strMstlGh, cmd) {
				t.Errorf("mstl-gh help missing %s", cmd)
			}
		}

		// Specific check: mstl-gh should have 'pr', mstl should not
		if strings.Contains(strMstl, " pr ") {
			t.Error("mstl help should NOT contain 'pr' command")
		}
		if !strings.Contains(strMstlGh, " pr ") {
			t.Error("mstl-gh help SHOULD contain 'pr' command")
		}
	})

	// 4. Test 'init' and 'status' (Integration)
	t.Run("init_and_status", func(t *testing.T) {
		// Setup remote
		remoteURL, _ := setupRemoteAndContent(t, 1)

		// Define config content
		configContent := fmt.Sprintf(`{
            "repositories": [
                {
                    "url": "%s",
                    "id": "repo-test"
                }
            ]
        }`, remoteURL)

		// Run mstl init
		dirMstl := t.TempDir()
		// Use same dir for config to ensure BaseDir resolves correctly to CWD/TestDir
		configFileMstl := filepath.Join(dirMstl, "repos.json")
		if err := os.WriteFile(configFileMstl, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		// Note: passing --ignore-stdin to prevent CI environment issues
		cmd := exec.Command(binMstl, "init", "-f", configFileMstl, "--ignore-stdin")
		// init creates repos in config dir (BaseDir) or CWD. Here config is in dirMstl.
		cmd.Dir = dirMstl
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mstl init failed: %v\n%s", err, out)
		}

		// Verify mstl status
		cmdStatus := exec.Command(binMstl, "status", "-f", configFileMstl, "--ignore-stdin")
		cmdStatus.Dir = dirMstl
		if out, err := cmdStatus.CombinedOutput(); err != nil {
			t.Fatalf("mstl status failed: %v\n%s", err, out)
		} else {
			if !strings.Contains(string(out), "repo-test") {
				t.Errorf("mstl status output missing repo-test: %s", out)
			}
		}

		// Prepare mock gh for mstl-gh
		mockGhDir := setupMockGh(t)

		// Run mstl-gh init
		dirMstlGh := t.TempDir()
		configFileMstlGh := filepath.Join(dirMstlGh, "repos.json")
		if err := os.WriteFile(configFileMstlGh, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cmdGh := exec.Command(binMstlGh, "init", "-f", configFileMstlGh, "--ignore-stdin")
		cmdGh.Dir = dirMstlGh
		cmdGh.Env = append(os.Environ(), "GH_EXEC_PATH="+mockGhDir)
		if out, err := cmdGh.CombinedOutput(); err != nil {
			t.Fatalf("mstl-gh init failed: %v\n%s", err, out)
		}

		// Verify mstl-gh status
		cmdStatusGh := exec.Command(binMstlGh, "status", "-f", configFileMstlGh, "--ignore-stdin")
		cmdStatusGh.Dir = dirMstlGh
		cmdStatusGh.Env = append(os.Environ(), "GH_EXEC_PATH="+mockGhDir)
		if out, err := cmdStatusGh.CombinedOutput(); err != nil {
			t.Fatalf("mstl-gh status failed: %v\n%s", err, out)
		} else {
			if !strings.Contains(string(out), "repo-test") {
				t.Errorf("mstl-gh status output missing repo-test: %s", out)
			}
		}

		// Check if repo exists in both
		if _, err := os.Stat(filepath.Join(dirMstl, "repo-test", ".git")); os.IsNotExist(err) {
			t.Errorf("mstl failed to clone repo")
		}
		if _, err := os.Stat(filepath.Join(dirMstlGh, "repo-test", ".git")); os.IsNotExist(err) {
			t.Errorf("mstl-gh failed to clone repo")
		}
	})
}
