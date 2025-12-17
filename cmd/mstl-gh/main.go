// Package main is the entry point for the mstl-gh application.
package main

import (
	"os"

	"mistletoe/internal/app"
)

var (
	appVersion = "0.1.0-beta"
	commitHash string
)

func main() {
	app.Run(app.TypeMstlGh, appVersion, commitHash, os.Args, func(cmd string, args []string, opts app.GlobalOptions) bool {
		if cmd == app.CmdPr {
			app.HandlePr(args, opts)
			return true
		}
		return false
	})
}
