package main

import (
	"os"

	"github.com/LeahArmstrong/grove-cli/cmd/grove/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
