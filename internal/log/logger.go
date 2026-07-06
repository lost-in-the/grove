package log

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// timestampFormat is the per-line timestamp layout used by Printf and the
// session banner.
const timestampFormat = "15:04:05.000"

// Logger is a mutex-guarded, append-only debug log gated by an environment
// variable. It backs both this package's default CLI log (GROVE_LOG) and
// internal/tuilog's TUI debug log (GROVE_DEBUG) so the two stay in sync
// instead of drifting as copy-pasted implementations.
//
// The env var's value selects the destination: "1"/"true" use DefaultPath,
// anything else is treated as an explicit file path, empty disables logging.
type Logger struct {
	mu      sync.Mutex
	file    *os.File
	enabled bool

	// EnvVar gates logging and optionally carries an explicit path.
	EnvVar string
	// Label names the log in stderr warnings (e.g. "log", "debug log").
	Label string
	// Banner is the session-start banner text (e.g. "grove session").
	Banner string
	// DefaultPath returns the log path used when EnvVar is "1"/"true".
	// It should create any required parent directories.
	DefaultPath func() (string, error)
}

// Init opens the log file if the gating env var is set.
// Call once at startup; safe to call multiple times.
func (l *Logger) Init() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return
	}

	val := os.Getenv(l.EnvVar)
	if val == "" {
		return
	}

	path := val
	if path == "1" || path == "true" {
		p, err := l.DefaultPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "grove: warning: cannot resolve default %s path: %v\n", l.Label, err)
			return
		}
		path = p
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grove: warning: failed to open %s %s: %v\n", l.Label, path, err)
		return
	}

	l.file = f
	l.enabled = true

	ts := time.Now().Format(timestampFormat)
	_, _ = fmt.Fprintf(l.file, "%s  === %s started at %s ===\n", ts, l.Banner, time.Now().Format(time.RFC3339))
}

// Close flushes and closes the log file.
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
		l.enabled = false
	}
}

// Printf writes a formatted message to the log.
func (l *Logger) Printf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.enabled {
		return
	}

	ts := time.Now().Format(timestampFormat)
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(l.file, "%s  %s\n", ts, msg)
}

// Enabled returns true when logging is active.
func (l *Logger) Enabled() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enabled
}
