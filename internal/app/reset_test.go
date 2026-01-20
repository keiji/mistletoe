package app

import (
	conf "mistletoe/internal/config"
	"mistletoe/internal/sys"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestHandleReset_Success(t *testing.T) {
	// Mock config
	configJSON := `
{
	"repositories": [
		{ "id": "repo1", "url": "https://example.com/repo1.git", "branch": "main" },
		{ "id": "repo2", "url": "https://example.com/repo2.git", "revision": "abcdef" }
	]
}
`
	configFile, cleanup := createTempConfig(t, configJSON)
	defer cleanup()

	// Mock git command
	oldExec := sys.ExecCommand
	defer func() { sys.ExecCommand = oldExec }()

	sys.ExecCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" {
			// Mock config --get remote.origin.url (called by ValidateRepositoriesIntegrity)
			if len(args) >= 3 && args[0] == "config" && args[1] == "--get" && args[2] == "remote.origin.url" {
				// We need to know which repo is calling.
				// Since we can't easily access the dir from here without parsing args context (which sys.Exec doesn't give easily except maybe via environment or just assumption)
				// Wait, RunGit sets cmd.Dir.
				// But mock doesn't set it unless we inspect it.
				// However, `sys.ExecCommand` returns *exec.Cmd struct which hasn't been run yet.
				// The actual execution is `cmd.Run()` or `cmd.Output()`.
				// We return a command that echos the right thing.
				// For this test we can just return the URL based on simple logic or assume sequential calls?
				// But tests run in parallel or loop.
				// Let's make it smarter.
				// We can't know the directory easily here because sys.ExecCommand(name, args...) doesn't take dir.
				// RunGit sets cmd.Dir AFTER calling sys.ExecCommand.

				// HACK: Return both URLs? Or just generic valid string?
				// ValidateRepositoriesIntegrity checks if output matches config URL.
				// This is tricky to mock perfectly without knowing the context.
				// Maybe we can skip integrity check by mocking ValidateRepositoriesIntegrity?
				// No, that's a function in the same package.

				// Let's assume the order or make the validation permissive in test?
				// No, code is strict.

				// Alternative: Mock os.Stat to fail/succeed?
				// Or... just echo the URL of the repository we are processing?
				// But we don't know which one.

				// Let's assume repo1 is processed first?
				// Or use a custom mock wrapper that can inspect the command *before* Run?
				// `sys.ExecCommand` creates the command. It doesn't run it.
				// `RunGit` does: cmd := sys.ExecCommand(...); cmd.Dir = dir; err := cmd.Run()
				// So we can return a mock command that when Run() checks its own Dir?
				// But `exec.Command` returns a struct that runs a REAL binary (e.g. "echo").
				// We can't inject Go logic into `echo`.

				// We can use the helper process pattern!
				// `TestCheckoutHelperProcess` mentioned in memory.
				cmd := exec.Command(os.Args[0], "-test.run=TestResetHelperProcess", "--", "config", args[2])
				cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
				return cmd
			}

			// Mock rev-parse --verify (check existence)
			if len(args) >= 3 && args[0] == "rev-parse" && args[1] == "--verify" {
				return exec.Command("echo", "hash123")
			}
			// Mock merge-base (check compatibility)
			if len(args) >= 3 && args[0] == "merge-base" {
				return exec.Command("echo", "commonbase")
			}
			// Mock reset (mixed)
			if len(args) >= 2 && args[0] == "reset" && args[1] != "--hard" {
				// args[1] is the target
				return exec.Command("echo", "reset ok")
			}
			// Mock rev-parse HEAD
			if len(args) >= 2 && args[0] == "rev-parse" && args[1] == "HEAD" {
				return exec.Command("echo", "currenthead")
			}
		}
		return exec.Command("echo", "unknown")
	}

	opts := GlobalOptions{GitPath: "git"}
	err := handleReset([]string{"-f", configFile}, opts)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
}

func TestHandleReset_Failure_Check(t *testing.T) {
	// Mock config
	configJSON := `
{
	"repositories": [
		{ "id": "repo1", "url": "https://example.com/repo1.git", "branch": "missing-branch" }
	]
}
`
	configFile, cleanup := createTempConfig(t, configJSON)
	defer cleanup()

	// Mock git command
	oldExec := sys.ExecCommand
	defer func() { sys.ExecCommand = oldExec }()

	sys.ExecCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" {
			if len(args) >= 3 && args[0] == "config" {
				cmd := exec.Command(os.Args[0], "-test.run=TestResetHelperProcess", "--", "config", "remote.origin.url")
				cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
				return cmd
			}
			// Fail rev-parse verify (local check)
			if len(args) >= 3 && args[0] == "rev-parse" && args[1] == "--verify" {
				cmd := exec.Command("false")
				return cmd
			}
			// Fail fetch
			if len(args) >= 2 && args[0] == "fetch" {
				cmd := exec.Command("false")
				return cmd
			}
		}
		return exec.Command("true")
	}

	opts := GlobalOptions{GitPath: "git"}
	err := handleReset([]string{"-f", configFile}, opts)
	if err == nil {
		t.Error("Expected error due to missing branch, got nil")
	}
}

func TestHandleReset_IncompatibleHistory(t *testing.T) {
	// Mock config
	configJSON := `
{
	"repositories": [
		{ "id": "repo1", "url": "https://example.com/repo1.git", "branch": "main" }
	]
}
`
	configFile, cleanup := createTempConfig(t, configJSON)
	defer cleanup()

	// Mock git command
	oldExec := sys.ExecCommand
	defer func() { sys.ExecCommand = oldExec }()

	sys.ExecCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" {
			if len(args) >= 3 && args[0] == "config" {
				cmd := exec.Command(os.Args[0], "-test.run=TestResetHelperProcess", "--", "config", "remote.origin.url")
				cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
				return cmd
			}
			// Success rev-parse verify
			if len(args) >= 3 && args[0] == "rev-parse" && args[1] == "--verify" {
				return exec.Command("echo", "hash123")
			}
			// Success rev-parse HEAD
			if len(args) >= 2 && args[0] == "rev-parse" && args[1] == "HEAD" {
				return exec.Command("echo", "currenthead")
			}
			// Fail merge-base (incompatible)
			if len(args) >= 3 && args[0] == "merge-base" {
				cmd := exec.Command("false")
				return cmd
			}
		}
		return exec.Command("true")
	}

	opts := GlobalOptions{GitPath: "git"}
	err := handleReset([]string{"-f", configFile}, opts)
	if err == nil {
		t.Error("Expected error due to incompatible history, got nil")
	}
}

func TestResolveResetTarget(t *testing.T) {
	// Test priority: Revision > BaseBranch > Branch
	rev := "rev1"
	base := "base1"
	branch := "branch1"

	repo := conf.Repository{
		ID:         nil,
		Revision:   &rev,
		BaseBranch: &base,
		Branch:     &branch,
	}

	target, err := resolveResetTarget(repo)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if target != rev {
		t.Errorf("Expected revision %s, got %s", rev, target)
	}

	// Test BaseBranch
	repo.Revision = nil
	target, err = resolveResetTarget(repo)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if target != base {
		t.Errorf("Expected base-branch %s, got %s", base, target)
	}

	// Test Branch
	repo.BaseBranch = nil
	target, err = resolveResetTarget(repo)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if target != branch {
		t.Errorf("Expected branch %s, got %s", branch, target)
	}

	// Test None
	repo.Branch = nil
	_, err = resolveResetTarget(repo)
	if err == nil {
		t.Error("Expected error when no target specified, got nil")
	}
}

// Helper process for mocking git commands that need context
func TestResetHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// Logic to return correct URL based on CWD
	cwd, _ := os.Getwd()
	// Check if cwd ends with repo1 or repo2
	if len(os.Args) >= 4 && os.Args[3] == "config" {
		// Relaxed check
		if strings.Contains(cwd, "repo1") {
			sys.Stdout.Write([]byte("https://example.com/repo1.git\n"))
			os.Exit(0)
		}
		if strings.Contains(cwd, "repo2") {
			sys.Stdout.Write([]byte("https://example.com/repo2.git\n"))
			os.Exit(0)
		}
		sys.Stdout.Write([]byte("unknown: " + cwd + "\n"))
		os.Exit(1)
	}
	os.Exit(0)
}

// createTempConfig helper
func createTempConfig(t *testing.T, content string) (string, func()) {
	t.Helper()
	f, err := os.CreateTemp("", "config_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	f.Close()

	os.Mkdir("repo1", 0755)
	os.Mkdir("repo1/.git", 0755) // Valid git repo structure
	os.Mkdir("repo2", 0755)
	os.Mkdir("repo2/.git", 0755)

	return f.Name(), func() {
		os.Remove(f.Name())
		os.RemoveAll("repo1")
		os.RemoveAll("repo2")
	}
}
