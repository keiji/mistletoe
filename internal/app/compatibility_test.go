package app

import (
)

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

	cmd := exec.Command("go", "build", "-o", binPath, cmdSrcPath)
	// Inherit env to ensure we have cache etc.
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build %s: %v\nOutput: %s", cmdName, err, output)
	}
	return binPath
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
		if !strings.Contains(string(outMstl), "mstl version") {
			t.Errorf("mstl version output unexpected: %s", outMstl)
		}

		outMstlGh, err := exec.Command(binMstlGh, "version").Output()
		if err != nil {
			t.Fatalf("mstl-gh version failed: %v", err)
		}
		if !strings.Contains(string(outMstlGh), "Mistletoe-gh version") {
			t.Errorf("mstl-gh version output unexpected: %s", outMstlGh)
		}
	})

	// 3. Test 'help' command structure
	t.Run("help", func(t *testing.T) {
		outMstl, _ := exec.Command(binMstl, "help").Output()
		outMstlGh, _ := exec.Command(binMstlGh, "help").Output()

		linesMstl := strings.Split(string(outMstl), "\n")
		linesMstlGh := strings.Split(string(outMstlGh), "\n")

		if len(linesMstl) != len(linesMstlGh) {
			t.Fatalf("Line count mismatch in help output. mstl: %d, mstl-gh: %d", len(linesMstl), len(linesMstlGh))
		}

		// Skip first line (Usage: ...) and compare the rest
		for i := 1; i < len(linesMstl); i++ {
			if linesMstl[i] != linesMstlGh[i] {
				t.Errorf("Mismatch at line %d:\nmstl   : %q\nmstl-gh: %q", i, linesMstl[i], linesMstlGh[i])
			}
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
		configDirMstl := t.TempDir()
		configFileMstl := filepath.Join(configDirMstl, "repos.json")
		if err := os.WriteFile(configFileMstl, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cmd := exec.Command(binMstl, "init", "-f", configFileMstl)
		// init creates repos in CWD unless configured otherwise.
		cmd.Dir = dirMstl
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mstl init failed: %v\n%s", err, out)
		}

		// Verify mstl status
		cmdStatus := exec.Command(binMstl, "status", "-f", configFileMstl)
		cmdStatus.Dir = dirMstl
		if out, err := cmdStatus.CombinedOutput(); err != nil {
			t.Fatalf("mstl status failed: %v\n%s", err, out)
		} else {
			if !strings.Contains(string(out), "repo-test") {
				t.Errorf("mstl status output missing repo-test: %s", out)
			}
		}

		// Run mstl-gh init
		dirMstlGh := t.TempDir()
		configDirMstlGh := t.TempDir()
		configFileMstlGh := filepath.Join(configDirMstlGh, "repos.json")
		if err := os.WriteFile(configFileMstlGh, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cmdGh := exec.Command(binMstlGh, "init", "-f", configFileMstlGh)
		cmdGh.Dir = dirMstlGh
		if out, err := cmdGh.CombinedOutput(); err != nil {
			t.Fatalf("mstl-gh init failed: %v\n%s", err, out)
		}

		// Verify mstl-gh status
		cmdStatusGh := exec.Command(binMstlGh, "status", "-f", configFileMstlGh)
		cmdStatusGh.Dir = dirMstlGh
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
