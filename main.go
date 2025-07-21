package main

import (
	"os"

	"github.com/annurdien/sim-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}