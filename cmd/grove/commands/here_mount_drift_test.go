package commands

import (
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/plugins/docker"
)

// Issue #148 (1): the header printed before any comparison runs must read as
// neutral, not a verdict. "Mount drift — 'x'" followed by "✓ All services
// mounted" eight lines later reads as a false alarm.
func TestMountCheckHeaderTemplate(t *testing.T) {
	if !strings.HasPrefix(mountCheckHeaderTemplate, "Mount check") {
		t.Errorf("header template = %q, want prefix %q", mountCheckHeaderTemplate, "Mount check")
	}
	if strings.Contains(strings.ToLower(mountCheckHeaderTemplate), "drift") {
		t.Errorf("header template %q reads as a verdict before any comparison runs (issue #148)", mountCheckHeaderTemplate)
	}
}

// Issue #148 (2): the cwd (currentName) and the env file's configured
// worktree (configuredName) can differ, and both are in hand at print time —
// the command must say so instead of printing a clean bill of health about a
// worktree the user isn't standing in.
func TestMountCheckMismatchNote(t *testing.T) {
	tests := []struct {
		name           string
		currentName    string
		configuredName string
		want           string
	}{
		{
			name:           "matching names produce no note",
			currentName:    "feature-a",
			configuredName: "feature-a",
			want:           "",
		},
		{
			name:           "empty configured name produces no note (env unset)",
			currentName:    "feature-b",
			configuredName: "",
			want:           "",
		},
		{
			name:           "mismatch names note cwd and configured worktree",
			currentName:    "feature-b",
			configuredName: "feature-a",
			want:           "note: you are in 'feature-b'; the stack is configured for 'feature-a'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mountCheckMismatchNote(tt.currentName, tt.configuredName)
			if got != tt.want {
				t.Errorf("mountCheckMismatchNote(%q, %q) = %q, want %q", tt.currentName, tt.configuredName, got, tt.want)
			}
		})
	}
}

// Issue #148 (3): --require-current gates on the cwd-vs-configured mismatch
// with a distinct exit code, independent of env-vs-container drift, and
// takes priority over it — if you're not even standing in the configured
// worktree, the drift verdict beneath it isn't the actionable signal.
func TestMountCheckExitCode(t *testing.T) {
	tests := []struct {
		name           string
		mismatch       bool
		requireCurrent bool
		anyDrift       bool
		wantCode       int
		wantExit       bool
	}{
		{
			name:     "no mismatch, no drift: exit 0",
			wantCode: 0,
			wantExit: false,
		},
		{
			name:     "drift without require-current: MountDrift",
			anyDrift: true,
			wantCode: exitcode.MountDrift,
			wantExit: true,
		},
		{
			name:           "mismatch without require-current does not gate",
			mismatch:       true,
			requireCurrent: false,
			wantCode:       0,
			wantExit:       false,
		},
		{
			name:           "mismatch with require-current: distinct code",
			mismatch:       true,
			requireCurrent: true,
			wantCode:       exitcode.MountCheckMismatch,
			wantExit:       true,
		},
		{
			name:           "require-current set but no mismatch: falls through to drift check",
			mismatch:       false,
			requireCurrent: true,
			anyDrift:       true,
			wantCode:       exitcode.MountDrift,
			wantExit:       true,
		},
		{
			name:           "mismatch + drift + require-current: mismatch code wins",
			mismatch:       true,
			requireCurrent: true,
			anyDrift:       true,
			wantCode:       exitcode.MountCheckMismatch,
			wantExit:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode, gotExit := mountCheckExitCode(tt.mismatch, tt.requireCurrent, tt.anyDrift)
			if gotCode != tt.wantCode || gotExit != tt.wantExit {
				t.Errorf("mountCheckExitCode(%v, %v, %v) = (%d, %v), want (%d, %v)",
					tt.mismatch, tt.requireCurrent, tt.anyDrift, gotCode, gotExit, tt.wantCode, tt.wantExit)
			}
		})
	}

	if exitcode.MountCheckMismatch == exitcode.MountDrift {
		t.Fatal("MountCheckMismatch must be distinct from MountDrift (issue #148)")
	}
}

func TestConfiguredSourcePath(t *testing.T) {
	tests := []struct {
		name    string
		reports []docker.MountDriftReport
		want    string
	}{
		{
			name:    "empty reports",
			reports: nil,
			want:    "",
		},
		{
			name: "no report carries a configured source",
			reports: []docker.MountDriftReport{
				{Service: "web"},
			},
			want: "",
		},
		{
			name: "first non-empty wins",
			reports: []docker.MountDriftReport{
				{Service: "web", ConfiguredSource: "/work/proj-feature-a"},
				{Service: "worker", ConfiguredSource: "/work/proj-feature-a"},
			},
			want: "/work/proj-feature-a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := configuredSourcePath(tt.reports); got != tt.want {
				t.Errorf("configuredSourcePath(%+v) = %q, want %q", tt.reports, got, tt.want)
			}
		})
	}
}

func TestDisplayPath(t *testing.T) {
	t.Run("falls back to (unset) when no report carries a source", func(t *testing.T) {
		got := displayPath([]docker.MountDriftReport{{Service: "web"}})
		if got != "(unset)" {
			t.Errorf("displayPath = %q, want %q", got, "(unset)")
		}
	})
	t.Run("returns the configured source when present", func(t *testing.T) {
		got := displayPath([]docker.MountDriftReport{{Service: "web", ConfiguredSource: "/work/proj-feature-a"}})
		if got != "/work/proj-feature-a" {
			t.Errorf("displayPath = %q, want %q", got, "/work/proj-feature-a")
		}
	})
}
