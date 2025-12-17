// Package main is the entry point for the mstl-gh application.
package main

import (
	"os"

	"mistletoe/internal/app"
)

var (
	appVersion = "0.0.2"
	commitHash string
)

func main() {
	app.Run(app.TypeMstlGh, appVersion, commitHash, os.Args)
}
