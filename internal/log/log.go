// Package log provides runtime debug logging for grove CLI commands.
//
// Enable with GROVE_LOG=1 or GROVE_LOG=/path/to/file.
// Default log path: ~/.grove/grove.log
package log

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

// Init opens the log file if GROVE_LOG is set.
// Call once at startup; safe to call multiple times.
func Init() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		return
	}

	val := os.Getenv("GROVE_LOG")
	if val == "" {
		return
	}

	path := val
	if path == "1" || path == "true" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "grove: warning: cannot resolve home dir for log: %v\n", err)
			return
		}
		groveDir := filepath.Join(home, ".grove")
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "grove: warning: cannot create ~/.grove for log: %v\n", err)
			return
		}
		path = filepath.Join(groveDir, "grove.log")
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grove: warning: failed to open log %s: %v\n", path, err)
		return
	}

	logFile = f
	enabled = true

	ts := time.Now().Format("15:04:05.000")
	_, _ = fmt.Fprintf(logFile, "%s  === grove session started at %s ===\n", ts, time.Now().Format(time.RFC3339))
}

// Close flushes and closes the log file.
func Close() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
		enabled = false
	}
}

// Printf writes a formatted message to the log.
func Printf(format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()

	if !enabled {
		return
	}

	ts := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(logFile, "%s  %s\n", ts, msg)
}

// Enabled returns true when logging is active.
func Enabled() bool {
	mu.Lock()
	defer mu.Unlock()
	return enabled
}
