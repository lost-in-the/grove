// Package log provides runtime debug logging for grove CLI commands.
//
// Enable with GROVE_LOG=1 or GROVE_LOG=/path/to/file.
// Default log path: ~/.grove/grove.log
package log

import (
	"fmt"
	"os"
	"path/filepath"
)

// std is the package-level CLI logger instance.
var std = &Logger{
	EnvVar: "GROVE_LOG",
	Label:  "log",
	Banner: "grove session",
	DefaultPath: func() (string, error) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home dir: %w", err)
		}
		groveDir := filepath.Join(home, ".grove")
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			return "", fmt.Errorf("cannot create %s: %w", groveDir, err)
		}
		return filepath.Join(groveDir, "grove.log"), nil
	},
}

// Init opens the log file if GROVE_LOG is set.
// Call once at startup; safe to call multiple times.
func Init() { std.Init() }

// Close flushes and closes the log file.
func Close() { std.Close() }

// Printf writes a formatted message to the log.
func Printf(format string, args ...any) { std.Printf(format, args...) }

// Enabled returns true when logging is active.
func Enabled() bool { return std.Enabled() }
