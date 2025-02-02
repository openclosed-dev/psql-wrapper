package main

import (
	"os"

	"github.com/openclosed-dev/psql-wrapper/internal"
)

func main() {
	var exitCode = internal.Launch("psql", os.Args[1:])
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
