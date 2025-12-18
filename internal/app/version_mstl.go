// Package app implements the core logic of the application.
package app

import (
	"flag"
	"fmt"
	"os"
)

func handleVersionMstl(args []string, opts GlobalOptions) {
	// Parse flags for version command
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	var vLong, vShort bool
	fs.BoolVar(&vLong, "verbose", false, "Enable verbose output")
	fs.BoolVar(&vShort, "v", false, "Enable verbose output (shorthand)")

	// We use ParseFlagsFlexible or just fs.Parse depending on if we expect args.
	// version typically has no args.
	if err := ParseFlagsFlexible(fs, args); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}
	verbose := vLong || vShort

	printCommonVersionInfo(opts, verbose)
}
