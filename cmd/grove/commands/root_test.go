package commands

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/shell"
	"github.com/lost-in-the/grove/internal/tui"
)

func TestSwitchAfterDashboard_UpRunsGroveToWithUp(t *testing.T) {
	name, opts, ok := switchAfterDashboard(tui.Exit{Path: "/src/myapp-feature", Name: "feature", Up: true})

	if !ok {
		t.Fatal("U on the dashboard must run the shared switch flow")
	}
	if name != "feature" {
		t.Errorf("name = %q, want %q", name, "feature")
	}
	// Path pins the row the user picked: a name alone is ambiguous when two
	// worktrees in different directories share a basename.
	if opts != (switchOptions{Up: true, Path: "/src/myapp-feature"}) {
		t.Errorf("opts = %+v, want grove to --up on the selected path", opts)
	}
}

func TestSwitchAfterDashboard_EnterAndQuitLeaveTheMoveToTheDashboard(t *testing.T) {
	for _, exit := range []tui.Exit{
		{Path: "/src/myapp-feature", Name: "feature"},
		{},
	} {
		if _, _, ok := switchAfterDashboard(exit); ok {
			t.Errorf("switchAfterDashboard(%+v) ran the switch flow; the dashboard already moved you", exit)
		}
	}
}

// Bare `grove` outside a grove project must diagnose (like every
// RequireGroveContext command) rather than silently show help or launch the
// TUI against a non-project directory (#138).
func TestDecideBareAction(t *testing.T) {
	tests := []struct {
		name        string
		isTTY       bool
		tuiDisabled bool
		inProject   bool
		want        bareAction
	}{
		{"non-TTY in project shows help", false, false, true, bareShowHelp},
		{"non-TTY outside project shows help", false, false, false, bareShowHelp},
		{"GROVE_TUI=0 in project shows help", true, true, true, bareShowHelp},
		{"GROVE_TUI=0 outside project shows help", true, true, false, bareShowHelp},
		{"interactive outside project diagnoses", true, false, false, bareDiagnose},
		{"interactive in project launches TUI", true, false, true, bareLaunchTUI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideBareAction(tt.isTTY, tt.tuiDisabled, tt.inProject)
			if got != tt.want {
				t.Errorf("decideBareAction(%v, %v, %v) = %v, want %v",
					tt.isTTY, tt.tuiDisabled, tt.inProject, got, tt.want)
			}
		})
	}
}

// After U the dashboard leaves the move to grove to --up, so the CLI's own
// explanation covers a missing shell function there too.
func TestFinishDashboard_UpWithoutShellIntegrationExplains(t *testing.T) {
	t.Setenv("GROVE_SHELL", "")
	t.Setenv("GROVE_CD_FILE", "")
	ctx := browseTestContext(t)
	ctx.Config.Mux.Backend = "off"
	t.Chdir(ctx.ProjectRoot)

	var err error
	stderr := captureStderr(t, func() {
		err = finishDashboard(ctx, tui.Exit{Path: ctx.ProjectRoot, Name: filepath.Base(ctx.ProjectRoot), Up: true})
	})

	if err != nil {
		t.Fatalf("finishDashboard() error = %v", err)
	}
	for _, want := range []string{
		"grove could not change your directory: shell integration is not loaded.",
		"cd '" + ctx.ProjectRoot + "'",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr is missing %q:\n%s", want, stderr)
		}
	}
}

// The start-up nudge uses the dashboard's words for shell integration, with
// no dash.
func TestWarnOutdatedShellIntegration_Wording(t *testing.T) {
	t.Setenv("GROVE_SHELL_VERSION", "1")
	got := captureStderr(t, warnOutdatedShellIntegration)
	want := fmt.Sprintf("grove: shell integration is outdated (v1, current v%d): run grove setup\n", shell.ShellVersion)
	if got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}

	t.Setenv("GROVE_SHELL_VERSION", strconv.Itoa(shell.ShellVersion))
	if got := captureStderr(t, warnOutdatedShellIntegration); got != "" {
		t.Errorf("stderr = %q for a current shell function, want nothing", got)
	}
}
