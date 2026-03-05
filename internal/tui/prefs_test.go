package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUIPrefsRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	groveDir := filepath.Join(tmpDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Initially no prefs file
	prefs := loadUIPrefs(tmpDir)
	if prefs != nil {
		t.Fatal("expected nil prefs when file doesn't exist")
	}

	// Save compact_mode = true
	val := true
	if err := saveUIPrefs(tmpDir, &UIPrefs{CompactMode: &val}); err != nil {
		t.Fatalf("saveUIPrefs failed: %v", err)
	}

	// Load it back
	prefs = loadUIPrefs(tmpDir)
	if prefs == nil {
		t.Fatal("expected non-nil prefs after save")
	}
	if prefs.CompactMode == nil || !*prefs.CompactMode {
		t.Error("expected CompactMode=true after save")
	}

	// Save compact_mode = false
	val = false
	if err := saveUIPrefs(tmpDir, &UIPrefs{CompactMode: &val}); err != nil {
		t.Fatalf("saveUIPrefs failed: %v", err)
	}

	prefs = loadUIPrefs(tmpDir)
	if prefs == nil {
		t.Fatal("expected non-nil prefs after second save")
	}
	if prefs.CompactMode == nil || *prefs.CompactMode {
		t.Error("expected CompactMode=false after second save")
	}
}

func TestUIPrefsCorruptFile(t *testing.T) {
	tmpDir := t.TempDir()
	groveDir := filepath.Join(tmpDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write garbage
	path := filepath.Join(groveDir, prefsFileName)
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should return nil, not panic
	prefs := loadUIPrefs(tmpDir)
	if prefs != nil {
		t.Error("expected nil prefs for corrupt file")
	}
}
