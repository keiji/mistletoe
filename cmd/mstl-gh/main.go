// Package main is the entry point for the mstl-gh application.
package main

import (
	"os"

	"mistletoe/internal/app"
)

var (
	// Version is injected at build time
	Version = "dev"
	// CommitHash is injected at build time
	CommitHash = "none"
)

func main() {
	app.Run(app.AppTypeMstlGh, "0.0.2", CommitHash, os.Args)
}
