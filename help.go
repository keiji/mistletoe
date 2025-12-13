package main

import (
	"fmt"
)

func handleHelp(_ []string, _ GlobalOptions) {
	fmt.Println("Usage: mstl <command> [options] [arguments]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  init       Initialize repositories")
	fmt.Println("  freeze     Freeze current state to config")
	fmt.Println("  switch     Switch branch")
	fmt.Println("  status     Show status")
	fmt.Println("  sync       Sync repositories")
	fmt.Println("  push       Push changes")
	fmt.Println("  version    Show version")
	fmt.Println("  help       Show this help message")
}
