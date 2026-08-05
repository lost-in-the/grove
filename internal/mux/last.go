package mux

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Write-then-rename so a crash mid-write can't leave a truncated name
	// that would send `grove last` somewhere unexpected.
	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(name), 0644); err != nil {
		return fmt.Errorf("write last session: %w", err)
	}
	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
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
