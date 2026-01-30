// Package tuilog provides debug logging for the TUI.
//
// Enable with GROVE_DEBUG=1 or GROVE_DEBUG=/path/to/file.
// Default log path: $HOME/.grove-debug.log
package tuilog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu      sync.Mutex
	logFile *os.File
	enabled bool
)

// Init opens the debug log if GROVE_DEBUG is set.
// Call once at TUI startup; safe to call multiple times.
func Init() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		return
	}

	val := os.Getenv("GROVE_DEBUG")
	if val == "" {
		return
	}

	path := val
	if path == "1" || path == "true" {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		path = filepath.Join(home, ".grove-debug.log")
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}

	logFile = f
	enabled = true

	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(logFile, "%s  === grove TUI session started at %s ===\n", ts, time.Now().Format(time.RFC3339))
}

// Close flushes and closes the log file.
func Close() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
		enabled = false
	}
}

// Printf writes a formatted message to the debug log.
func Printf(format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()

	if !enabled {
		return
	}

	ts := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(logFile, "%s  %s\n", ts, msg)
}

// Enabled returns true when debug logging is active.
func Enabled() bool {
	mu.Lock()
	defer mu.Unlock()
	return enabled
}
