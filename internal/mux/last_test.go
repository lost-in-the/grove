package mux

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreAndGetLastSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := StoreLastSession("grove-testing"); err != nil {
		t.Fatalf("StoreLastSession() error = %v", err)
	}

	got, err := GetLastSession()
	if err != nil {
		t.Fatalf("GetLastSession() error = %v", err)
	}
	if got != "grove-testing" {
		t.Errorf("GetLastSession() = %q, want %q", got, "grove-testing")
	}
}

func TestGetLastSessionWhenUnset(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, err := GetLastSession(); err == nil {
		t.Error("GetLastSession() expected an error when nothing is stored")
	}
}

func TestStoreLastSessionOverwrites(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	for _, name := range []string{"grove-one", "grove-two"} {
		if err := StoreLastSession(name); err != nil {
			t.Fatalf("StoreLastSession(%q) error = %v", name, err)
		}
	}

	got, err := GetLastSession()
	if err != nil {
		t.Fatalf("GetLastSession() error = %v", err)
	}
	if got != "grove-two" {
		t.Errorf("GetLastSession() = %q, want the most recent value", got)
	}
}

func TestStoreLastSessionLeavesNoTempFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := StoreLastSession("grove-testing"); err != nil {
		t.Fatalf("StoreLastSession() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, ".config", "grove", "last_session.tmp")); !os.IsNotExist(err) {
		t.Error("StoreLastSession left its temp file behind")
	}
}
