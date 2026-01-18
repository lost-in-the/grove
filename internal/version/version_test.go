package version

import "testing"

func TestGetVersion(t *testing.T) {
	v := GetVersion()
	if v == "" {
		t.Error("GetVersion() returned empty string")
	}
}

func TestGetFullVersion(t *testing.T) {
	v := GetFullVersion()
	if v == "" {
		t.Error("GetFullVersion() returned empty string")
	}
	// Should contain at least the version
	if len(v) < len(Version) {
		t.Error("GetFullVersion() is shorter than Version")
	}
}
