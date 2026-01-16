package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCommand(t *testing.T) {
	// Create a temp directory for tests
	tmpDir, err := os.MkdirTemp("", "mstl-init-cli-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save original CWD
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	defer func() {
		_ = os.Chdir(origWd)
	}()

	// Switch to temp dir for safer testing (avoid cluttering real CWD)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	opts := GlobalOptions{GitPath: "git", GhPath: "gh"}

	t.Run("Invalid Flag", func(t *testing.T) {
		err := initCommand([]string{"--invalid-flag"}, opts)
		if err == nil {
			t.Error("Expected error for invalid flag, got nil")
		}
	})

	t.Run("Duplicate Flags", func(t *testing.T) {
		err := initCommand([]string{"--file", "a", "-f", "b"}, opts)
		if err == nil {
			t.Error("Expected error for duplicate flags, got nil")
		}
		errMsg := fmt.Sprint(err)
		if !strings.Contains(errMsg, "different values") && !strings.Contains(errMsg, "duplicate") {
			t.Errorf("Expected duplicate/different values error, got: %v", err)
		}
	})

	t.Run("Missing Config", func(t *testing.T) {
		// Expect error because no config file exists and no stdin provided (simulated by failure to load)
		err := initCommand([]string{"--ignore-stdin"}, opts)
		if err == nil {
			t.Error("Expected error when no config provided, got nil")
		}
	})

	t.Run("Valid Init with File", func(t *testing.T) {
		// Setup a sub-workspace
		wsDir := filepath.Join(tmpDir, "ws1")
		if err := os.Mkdir(wsDir, 0755); err != nil {
			t.Fatal(err)
		}
		configFile := filepath.Join(wsDir, "config.json")

		// Create a dummy config
		// Note: PerformInit uses actual git commands. To avoid network/git dependency,
		// we can leave repositories empty or use local file protocol.
		configContent := `{
			"version": "1.0",
			"repositories": []
		}`
		if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Switch to workspace
		if err := os.Chdir(wsDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(tmpDir)

		err := initCommand([]string{"--file", "config.json"}, opts)
		if err != nil {
			t.Errorf("initCommand failed: %v", err)
		}

		// Verify output
		mstlConfig := filepath.Join(".mstl", "config.json")
		if _, err := os.Stat(mstlConfig); os.IsNotExist(err) {
			t.Errorf(".mstl/config.json not created")
		}
	})

	t.Run("Init with Private Repo Filter", func(t *testing.T) {
		wsDir := filepath.Join(tmpDir, "ws_private")
		if err := os.Mkdir(wsDir, 0755); err != nil {
			t.Fatal(err)
		}

		repoIDPublic := "public-repo"
		repoIDPrivate := "private-repo"
		repoURL := "http://example.com/repo.git"
		isPrivate := true

		configStruct := map[string]interface{}{
			"version": "1.0",
			"repositories": []map[string]interface{}{
				{"id": repoIDPublic, "url": repoURL},
				{"id": repoIDPrivate, "url": repoURL, "private": isPrivate},
			},
		}
		configBytes, _ := json.Marshal(configStruct)
		configFile := filepath.Join(wsDir, "config.json")
		os.WriteFile(configFile, configBytes, 0644)

		if err := os.Chdir(wsDir); err != nil { t.Fatal(err) }
		defer os.Chdir(tmpDir)

		// Execute
		err := initCommand([]string{"-f", "config.json"}, opts)
		if err != nil {
			t.Fatalf("initCommand failed: %v", err)
		}

		// Check .mstl/config.json
		resultBytes, err := os.ReadFile(filepath.Join(".mstl", "config.json"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(resultBytes), repoIDPrivate) {
			t.Error("Private repo should be filtered out from .mstl/config.json")
		}
		if !strings.Contains(string(resultBytes), repoIDPublic) {
			t.Error("Public repo should be present in .mstl/config.json")
		}
	})

	t.Run("Destination Flag", func(t *testing.T) {
		wsDir := filepath.Join(tmpDir, "ws_dest_source")
		if err := os.Mkdir(wsDir, 0755); err != nil { t.Fatal(err) }

		configFile := filepath.Join(wsDir, "myconfig.json")
		os.WriteFile(configFile, []byte(`{"version":"1.0","repositories":[]}`), 0644)

		// We will run from tmpDir, pointing to ws_dest_target
		targetDir := filepath.Join(tmpDir, "ws_dest_target")

		// Run command
		// Note: initCommand calls validateAndPrepareInitDest which Chdirs.
		// We need to restore CWD inside this test.
		curDir, _ := os.Getwd() // likely tmpDir or wsDir depending on prev tests
		// Reset to tmpDir to be safe
		os.Chdir(tmpDir)
		defer os.Chdir(curDir)

		err := initCommand([]string{"--file", configFile, "--dest", targetDir}, opts)
		if err != nil {
			t.Fatalf("initCommand failed: %v", err)
		}

		// Check if targetDir created and has .mstl
		if _, err := os.Stat(filepath.Join(targetDir, ".mstl", "config.json")); os.IsNotExist(err) {
			t.Error("Config not created in destination directory")
		}
	})

	t.Run("Dependency File Validation - Success", func(t *testing.T) {
		wsDir := filepath.Join(tmpDir, "ws_dep_ok")
		if err := os.Mkdir(wsDir, 0755); err != nil { t.Fatal(err) }
		os.Chdir(wsDir)
		defer os.Chdir(tmpDir)

		configContent := `{
			"version": "1.0",
			"repositories": [{"id": "repoA", "url": "http://a"}, {"id": "repoB", "url": "http://b"}]
		}`
		os.WriteFile("config.json", []byte(configContent), 0644)

		// Create valid dependency file (assuming Mermaid format or similar handled by ParseDependencies)
		// Looking at code, ParseDependencies likely parses mermaid-like graph.
		// Let's assume simplistic content based on dependency.go (not read here but inferred).
		// Usually: graph TD \n repoA --> repoB
		depContent := "graph TD\n    repoA --> repoB"
		os.WriteFile("deps.md", []byte(depContent), 0644)

		err := initCommand([]string{"-f", "config.json", "--dependencies", "deps.md", "--yes"}, opts)
		if err != nil {
			t.Errorf("Expected success with valid deps, got: %v", err)
		}
	})

	t.Run("Dependency File Validation - Failure (Invalid ID)", func(t *testing.T) {
		wsDir := filepath.Join(tmpDir, "ws_dep_fail")
		if err := os.Mkdir(wsDir, 0755); err != nil { t.Fatal(err) }
		os.Chdir(wsDir)
		defer os.Chdir(tmpDir)

		configContent := `{"version": "1.0", "repositories": [{"id": "repoA", "url": "http://a"}]}`
		os.WriteFile("config.json", []byte(configContent), 0644)

		// repoB does not exist in config
		depContent := "graph TD\n    repoA --> repoB"
		os.WriteFile("deps.md", []byte(depContent), 0644)

		err := initCommand([]string{"-f", "config.json", "--dependencies", "deps.md", "--yes"}, opts)
		if err == nil {
			t.Error("Expected error for invalid dependency ID, got nil")
		}
	})

	t.Run("Dependency File - File Not Found", func(t *testing.T) {
		wsDir := filepath.Join(tmpDir, "ws_dep_missing")
		if err := os.Mkdir(wsDir, 0755); err != nil { t.Fatal(err) }
		os.Chdir(wsDir)
		defer os.Chdir(tmpDir)

		os.WriteFile("config.json", []byte(`{"version":"1.0","repositories":[]}`), 0644)

		err := initCommand([]string{"-f", "config.json", "--dependencies", "missing.md"}, opts)
		if err == nil {
			t.Error("Expected error for missing dependency file, got nil")
		}
	})
}
