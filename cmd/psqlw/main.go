package main

import (
	"os"

	"github.com/openclosed-dev/psql-wrapper/internal"
)

func main() {
	var exitCode = internal.Launch("psqlw", "psql", os.Args)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
