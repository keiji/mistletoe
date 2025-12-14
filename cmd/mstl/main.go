package main

import (
	"os"

	"mistletoe/internal/app"
)

var (
	appName    = "mstl"
	appVersion = "0.0.1"
	commitHash string
)

func main() {
	app.Run(appName, appVersion, commitHash, os.Args)
}
