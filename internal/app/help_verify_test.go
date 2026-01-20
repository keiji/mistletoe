package app

import (
	"bytes"
	"io"
	"mistletoe/internal/sys"
	"os"
	"strings"
	"testing"
)

func TestHandleHelp_AllCommandsListed(t *testing.T) {
	// Setup capture of stdout
	oldStdout := sys.Stdout
	r, w, _ := os.Pipe()
	sys.Stdout = w

	// We also need to capture os.Stdout because handleHelp uses fmt.Printf directly.
	// NOTE: In the current implementation handleHelp uses fmt.Printf, which prints to os.Stdout.
	// Changing sys.Stdout variable won't affect fmt.Printf unless we redirect os.Stdout
	// or change handleHelp to use sys.Stdout.
	// Since I cannot easily change os.Stdout file descriptor in this environment (it might affect the agent's output),
	// I will check if handleHelp uses sys.Stdout or fmt.Printf.
	// The read_file of help.go showed:
	// fmt.Printf("Usage: %s <command> [options] [arguments]\n", AppName)
	// This prints to the real stdout.

	// However, I can temporarily swap os.Stdout in the test.
	realStdout := os.Stdout
	os.Stdout = w

	defer func() {
		sys.Stdout = oldStdout
		os.Stdout = realStdout
	}()

	// Case 1: mstl
	AppName = AppNameMstl
	err := handleHelp(nil, GlobalOptions{})
	if err != nil {
		t.Fatalf("handleHelp failed: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	requiredCommands := []string{
		CmdInit,
		CmdSnapshot,
		CmdSwitch,
		CmdStatus,
		CmdSync,
		CmdPush,
		CmdReset,
		CmdFire,
		CmdVersion,
		CmdHelp,
	}

	for _, cmd := range requiredCommands {
		if !strings.Contains(output, cmd) {
			t.Errorf("mstl help missing command: %s", cmd)
		}
	}

	if strings.Contains(output, CmdPr) {
		t.Errorf("mstl help should not contain command: %s", CmdPr)
	}
}

func TestHandleHelp_GhCommandsListed(t *testing.T) {
	// Setup capture of stdout
	oldStdout := sys.Stdout
	r, w, _ := os.Pipe()
	sys.Stdout = w

	realStdout := os.Stdout
	os.Stdout = w

	defer func() {
		sys.Stdout = oldStdout
		os.Stdout = realStdout
	}()

	// Case 2: mstl-gh
	AppName = AppNameMstlGh
	err := handleHelp(nil, GlobalOptions{})
	if err != nil {
		t.Fatalf("handleHelp failed: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	requiredCommands := []string{
		CmdInit,
		CmdSnapshot,
		CmdSwitch,
		CmdStatus,
		CmdSync,
		CmdPush,
		CmdReset,
		CmdFire,
		CmdVersion,
		CmdHelp,
		CmdPr, // Should be present
	}

	for _, cmd := range requiredCommands {
		if !strings.Contains(output, cmd) {
			t.Errorf("mstl-gh help missing command: %s", cmd)
		}
	}
}
