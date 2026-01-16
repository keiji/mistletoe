package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateAndPrepareInitDest(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "init_dest_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Case 1: Destination exists and is a file -> Error
	destFile := filepath.Join(tempDir, "file")
	if err := os.WriteFile(destFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	if _, err := validateAndPrepareInitDest(destFile); err == nil {
		t.Errorf("Expected error when dest is a file, got nil")
	}

	// Case 2: Destination does not exist, parent does not exist -> Error
	destDeep := filepath.Join(tempDir, "missing", "missing")
	if _, err := validateAndPrepareInitDest(destDeep); err == nil {
		t.Errorf("Expected error when parent does not exist, got nil")
	}

	// Case 3: Destination does not exist, parent is a file -> Error
	destParentFile := filepath.Join(destFile, "subdir")
	if _, err := validateAndPrepareInitDest(destParentFile); err == nil {
		t.Errorf("Expected error when parent is a file, got nil")
	}

	// Case 4: Destination exists, not empty -> Success (Allowed now, per-repo checks handle safety)
	destNotEmpty := filepath.Join(tempDir, "not_empty")
	if err := os.Mkdir(destNotEmpty, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(destNotEmpty, "dummy"), []byte("dummy"), 0644); err != nil {
		t.Fatalf("Failed to create dummy file: %v", err)
	}
	// Restore CWD after success
	origWd, _ := os.Getwd()
	if _, err := validateAndPrepareInitDest(destNotEmpty); err != nil {
		t.Errorf("Expected success when dest is not empty, got error: %v", err)
	}
	os.Chdir(origWd)

	// Case 5: Destination exists, empty -> Success
	destEmpty := filepath.Join(tempDir, "empty")
	if err := os.Mkdir(destEmpty, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	// We need to restore CWD after success
	origWd, _ = os.Getwd()
	defer os.Chdir(origWd)

	if _, err := validateAndPrepareInitDest(destEmpty); err != nil {
		t.Errorf("Expected success when dest is empty, got error: %v", err)
	}
	// Verify CWD changed
	cwd, _ := os.Getwd()
	// Need to resolve symlinks for Mac/Temp paths usually, but let's check base equality
	evalCwd, _ := filepath.EvalSymlinks(cwd)
	evalDest, _ := filepath.EvalSymlinks(destEmpty)
	if evalCwd != evalDest {
		t.Errorf("Expected CWD to be %s, got %s", evalDest, evalCwd)
	}
	os.Chdir(origWd) // Reset for next case

	// Case 6: Destination does not exist, parent exists -> Success (creates dir)
	destNew := filepath.Join(tempDir, "new_dir")
	if _, err := validateAndPrepareInitDest(destNew); err != nil {
		t.Errorf("Expected success when creating new dir, got error: %v", err)
	}
	// Verify it exists
	if _, err := os.Stat(destNew); os.IsNotExist(err) {
		t.Errorf("Expected dir %s to be created", destNew)
	}
	os.Chdir(origWd)
}
