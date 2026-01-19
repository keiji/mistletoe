package app

import (
	"encoding/json"
	conf "mistletoe/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunApp_UnknownSubcommand(t *testing.T) {
	err := RunApp(TypeMstl, "v1", "hash", []string{"mstl", "unknowncmd"}, nil)
	if err == nil {
		t.Fatal("Expected error for unknown subcommand, got nil")
	}
	if !strings.Contains(err.Error(), "Unknown subcommand: unknowncmd") {
		t.Errorf("Expected unknown subcommand error, got: %v", err)
	}
}

func TestRunApp_InvalidFlags(t *testing.T) {
	// List of subcommands to test.
	// We test that passing an invalid flag format (like /x) results in an error.
	// For commands that do not accept positional args (status, sync, push, snapshot),
	// /x is treated as a positional arg, so we expect "unexpected argument" or similar error.
	subcommands := []string{
		CmdStatus,
		CmdSync,
		CmdPush,
		CmdSnapshot,
	}

	for _, cmd := range subcommands {
		t.Run(cmd, func(t *testing.T) {
			// Using /x as the invalid flag/argument
			err := RunApp(TypeMstl, "v1", "hash", []string{"mstl", cmd, "/x"}, nil)
			if err == nil {
				t.Fatalf("Expected error for %s with invalid flag /x, got nil", cmd)
			}
			// We expect an error message indicating unexpected argument or similar
			// Since we added checks "len(fs.Args()) > 0" for these commands.
			if !strings.Contains(err.Error(), "does not accept positional arguments") &&
			   !strings.Contains(err.Error(), "Unexpected argument") {
				t.Errorf("Expected positional argument error for %s, got: %v", cmd, err)
			}
		})
	}
}

func TestRunApp_Switch_InvalidFlag(t *testing.T) {
	// Create a dummy config file to pass the config loading stage
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mstl.json")
	config := conf.Config{
		Repositories: &[]conf.Repository{},
	}
	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0644)

	// Change working directory to tmpDir so it picks up if we relied on default,
	// but here we will pass -f explicitly to be safe.
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Switch takes 1 argument.
	// If we pass /x, it is ambiguous (could be branch name).
	// But if we pass TWO arguments, it should error.
	// mstl switch branch /x
	// We must pass -f config.json to avoid "config not found" error.
	err := RunApp(TypeMstl, "v1", "hash", []string{"mstl", "switch", "branch", "/x", "-f", configPath}, nil)
	if err == nil {
		t.Fatal("Expected error for switch with extra argument, got nil")
	}
	if !strings.Contains(err.Error(), "Too many arguments") {
		t.Errorf("Expected too many arguments error, got: %v", err)
	}
}

func TestRunApp_Init_InvalidFlag(t *testing.T) {
    // Init takes optional destination.
    // If we pass explicit invalid flags like -? or --unknown, it should fail.
    err := RunApp(TypeMstl, "v1", "hash", []string{"mstl", "init", "--unknown-flag"}, nil)
    if err == nil {
        t.Fatal("Expected error for init with unknown flag, got nil")
    }
}
