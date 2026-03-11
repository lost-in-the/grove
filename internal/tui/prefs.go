package tui

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/lost-in-the/grove/internal/tuilog"
)

// UIPrefs stores user preferences that persist across TUI sessions.
type UIPrefs struct {
	CompactMode *bool `json:"compact_mode,omitempty"`
}

// prefsFileName is the file within .grove/ that stores UI preferences.
const prefsFileName = "ui_prefs.json"

// loadUIPrefs reads UI preferences from .grove/ui_prefs.json.
// Returns nil if the file doesn't exist or can't be parsed (non-fatal).
func loadUIPrefs(projectRoot string) *UIPrefs {
	path := uiPrefsPath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var prefs UIPrefs
	if err := json.Unmarshal(data, &prefs); err != nil {
		tuilog.Printf("warning: failed to parse %s: %v", path, err)
		return nil
	}
	return &prefs
}

// saveUIPrefs writes UI preferences to .grove/ui_prefs.json.
func saveUIPrefs(projectRoot string, prefs *UIPrefs) error {
	path := uiPrefsPath(projectRoot)
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// uiPrefsPath returns the full path to the UI preferences file.
func uiPrefsPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".grove", prefsFileName)
}
