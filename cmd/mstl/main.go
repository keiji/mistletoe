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
	app.Run(app.AppTypeMstl, appVersion, commitHash, os.Args)
}
