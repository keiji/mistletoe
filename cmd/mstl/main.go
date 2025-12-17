// Package main is the entry point for the mstl application.
package main

import (
	"os"

	"mistletoe/internal/app"
)

var (
	appVersion = "0.0.1"
	commitHash string
)

func main() {
	app.Run(app.TypeMstl, appVersion, commitHash, os.Args, nil)
}
