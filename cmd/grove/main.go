package main

import (
	"fmt"
	"os"

	"github.com/LeahArmstrong/grove-cli/cmd/grove/commands"
	"github.com/LeahArmstrong/grove-cli/internal/log"
)

func main() {
	log.Init()
	defer log.Close()

	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
