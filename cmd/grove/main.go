package main

import (
	"fmt"
	"os"

	"github.com/LeahArmstrong/grove-cli/cmd/grove/commands"
	timePlugin "github.com/LeahArmstrong/grove-cli/plugins/time"
)

func main() {
	// Initialize plugins
	if err := initializePlugins(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: plugin initialization failed: %v\n", err)
		// Continue anyway - plugins are optional
	}

	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func initializePlugins() error {
	// Initialize time tracking plugin
	if err := timePlugin.InitializePlugin(); err != nil {
		return fmt.Errorf("time plugin: %w", err)
	}

	return nil
}
