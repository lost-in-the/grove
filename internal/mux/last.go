package mux

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lost-in-the/grove/internal/fsutil"
)

// lastSessionFile is grove's own record of the session the user came from, so
// `grove last` can bounce back to it. It is grove state rather than
// multiplexer state, which is why it lives here and not in a backend.
func lastSessionFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "grove", "last_session"), nil
}

// StoreLastSession records the session name grove is switching away from.
func StoreLastSession(name string) error {
	path, err := lastSessionFile()
	if err != nil {
		return err
	}
	// Atomic write-then-rename so a crash mid-write can't leave a truncated
	// name. AtomicWriteFile's unique temp names also keep concurrent grove
	// processes (agents drive several at once) from sharing one temp file and
	// renaming each other's partial content into place.
	if err := fsutil.AtomicWriteFile(path, []byte(name), 0o644); err != nil {
		return fmt.Errorf("save last session: %w", err)
	}
	return nil
}

// GetLastSession returns the stored session name.
func GetLastSession() (string, error) {
	path, err := lastSessionFile()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no last session stored")
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
