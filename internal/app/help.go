package app

import (
	"fmt"
)

func handleHelp(_ []string, _ GlobalOptions) {
	fmt.Printf("Usage: %s <command> [options] [arguments]\n", AppName)
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Printf("  %-10s Initialize repositories\n", CmdInit)
	fmt.Printf("  %-10s Create snapshot of current state\n", CmdSnapshot)
	fmt.Printf("  %-10s Switch branch\n", CmdSwitch)
	fmt.Printf("  %-10s Show status\n", CmdStatus)
	fmt.Printf("  %-10s Sync repositories\n", CmdSync)
	fmt.Printf("  %-10s Push changes\n", CmdPush)
	fmt.Printf("  %-10s Show version\n", CmdVersion)
	fmt.Printf("  %-10s Show this help message\n", CmdHelp)
}
