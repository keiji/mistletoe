package main

import (
	"os"

	"mistletoe/internal/app"
)

var (
	appName    = "Mistletoe-gh"
	appVersion = "0.0.2"
	commitHash string
)

func main() {
	app.Run(appName, appVersion, commitHash, os.Args)
}
