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

func prefsRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".grove"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestUIPrefs_PinsRoundTrip(t *testing.T) {
	root := prefsRoot(t)
	if err := updateUIPrefs(root, func(p *UIPrefs) { p.Pins = map[string]string{"2": "fix-bug"} }); err != nil {
		t.Fatal(err)
	}
	if got := loadUIPrefs(root); got == nil || got.Pins["2"] != "fix-bug" {
		t.Errorf("pins not saved: %+v", got)
	}
}

// One setting's save never drops another.
func TestUIPrefs_UpdateKeepsOtherFields(t *testing.T) {
	root := prefsRoot(t)
	_ = updateUIPrefs(root, func(p *UIPrefs) { p.Pins = map[string]string{"1": "root"} })
	no := false
	_ = updateUIPrefs(root, func(p *UIPrefs) { p.CompactMode = &no })
	got := loadUIPrefs(root)
	if got == nil || got.Pins["1"] != "root" || got.CompactMode == nil || *got.CompactMode {
		t.Errorf("prefs = %+v, want pins and compact_mode false", got)
	}
}

func TestToggleCompact_KeepsPins(t *testing.T) {
	root := prefsRoot(t)
	_ = updateUIPrefs(root, func(p *UIPrefs) { p.Pins = map[string]string{"3": "testing"} })
	m := newTestModel(withItems(3), withSize(120, 40))
	m.projectRoot = root
	m = sendKey(m, "v")
	if got := loadUIPrefs(root); got == nil || got.Pins["3"] != "testing" {
		t.Errorf("v dropped the pins: %+v", got)
	}
}
