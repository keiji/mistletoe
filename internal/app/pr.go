package app

import (
	"fmt"
	"os"
)

// HandlePr handles the 'pr' subcommand.
func HandlePr(args []string, opts GlobalOptions) {
	if len(args) == 0 {
		fmt.Println("Usage: mstl-gh pr <subcommand> [options]")
		fmt.Println("Available subcommands: create, update, checkout, status")
		os.Exit(1)
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case CmdCreate:
		handlePrCreate(subArgs, opts)
	case CmdCheckout:
		handlePrCheckout(subArgs, opts)
	case CmdStatus:
		handlePrStatus(subArgs, opts)
	case CmdUpdate:
		handlePrUpdate(subArgs, opts)
	default:
		fmt.Printf("Unknown pr subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}
