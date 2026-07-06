// Package tuilog provides debug logging for the TUI.
//
// Enable with GROVE_DEBUG=1 or GROVE_DEBUG=/path/to/file.
// Default log path: $HOME/.grove-debug.log
package tuilog

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lost-in-the/grove/internal/log"
)

// std is the TUI debug logger — a second instance of the shared
// log.Logger with its own env var and default path.
var std = &log.Logger{
	EnvVar: "GROVE_DEBUG",
	Label:  "debug log",
	Banner: "grove TUI session",
	DefaultPath: func() (string, error) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home dir: %w", err)
		}
		return filepath.Join(home, ".grove-debug.log"), nil
	},
}

// Init opens the debug log if GROVE_DEBUG is set.
// Call once at TUI startup; safe to call multiple times.
func Init() { std.Init() }

// Close flushes and closes the log file.
func Close() { std.Close() }

// Printf writes a formatted message to the debug log.
func Printf(format string, args ...any) { std.Printf(format, args...) }

// Enabled returns true when debug logging is active.
func Enabled() bool { return std.Enabled() }
