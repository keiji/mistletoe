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
	app.Run(app.AppTypeMstlGh, appVersion, commitHash, os.Args)
}
