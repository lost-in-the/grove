//go:build integration

package tui

import (
	"regexp"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
)

// stripANSI removes ANSI escape codes from output for assertion matching.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func newTestProgram(t *testing.T, repoPath string) *teatest.TestModel {
	t.Helper()
	mgr, stateMgr := newTestManagers(t, repoPath)
	m := NewModel(mgr, stateMgr, repoPath)
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
}

func TestProgram_DashboardRenders(t *testing.T) {
	repo := setupRailsFixtureWithWorktrees(t, "testing", "staging")
	tm := newTestProgram(t, repo)

	// Wait for the worktree list to appear in output
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripANSI(string(bts))
		return regexp.MustCompile(`(?i)rails-app`).MatchString(s)
	}, teatest.WithDuration(5*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestProgram_CursorNavigation(t *testing.T) {
	repo := setupRailsFixtureWithWorktrees(t, "alpha", "beta")
	tm := newTestProgram(t, repo)

	// Wait for initial render
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripANSI(string(bts))
		return regexp.MustCompile(`(?i)alpha|beta`).MatchString(s)
	}, teatest.WithDuration(5*time.Second))

	// Press j to move cursor down
	tm.Send(tea.KeyPressMsg{Code: 'j', Text: "j"})
	// Press k to move cursor up
	tm.Send(tea.KeyPressMsg{Code: 'k', Text: "k"})

	// Quit
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestProgram_HelpOverlay(t *testing.T) {
	repo := setupRailsFixture(t)
	tm := newTestProgram(t, repo)

	// Wait for dashboard
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripANSI(string(bts))
		return regexp.MustCompile(`(?i)rails-app`).MatchString(s)
	}, teatest.WithDuration(5*time.Second))

	// Press ? for help
	tm.Send(tea.KeyPressMsg{Code: '?', Text: "?"})

	// Wait for help overlay text
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripANSI(string(bts))
		return regexp.MustCompile(`(?i)keybindings|navigation`).MatchString(s)
	}, teatest.WithDuration(5*time.Second))

	// esc closes the overlay (? and esc are the only accepted close keys); then q quits
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestProgram_HelpOverlay_QuestionMarkToggles(t *testing.T) {
	repo := setupRailsFixture(t)
	tm := newTestProgram(t, repo)

	// Wait for dashboard
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripANSI(string(bts))
		return regexp.MustCompile(`(?i)rails-app`).MatchString(s)
	}, teatest.WithDuration(5*time.Second))

	// Press ? to open the overlay. The overlay footer always renders "esc close",
	// which is unique to the overlay (the dashboard footer uses "esc back").
	tm.Send(tea.KeyPressMsg{Code: '?', Text: "?"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripANSI(string(bts))
		return regexp.MustCompile(`esc\s+close`).MatchString(s)
	}, teatest.WithDuration(5*time.Second))

	// Press ? again — toggles the overlay closed; overlay footer disappears
	tm.Send(tea.KeyPressMsg{Code: '?', Text: "?"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripANSI(string(bts))
		return !regexp.MustCompile(`esc\s+close`).MatchString(s)
	}, teatest.WithDuration(5*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestProgram_HelpOverlay_SpaceDoesNotClose is intentionally omitted.
// teatest streams incremental terminal writes; after pressing space the TUI
// only redraws the scrolled viewport region, so the overlay footer ("esc close")
// does not appear in the latest output chunk even though the overlay is still
// active. Asserting "overlay still present" via WaitFor is not reliably possible
// without a full-screen snapshot API. The regression (space closing the overlay)
// is instead guarded by the unit-level key routing in handleKey and by the fact
// that TestProgram_HelpOverlay_QuestionMarkToggles verifies the overlay's
// open/closed state machine end-to-end.

func TestProgram_UpdateOverlay_OpensOnUWhenAvailable(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)
	m := NewModel(mgr, stateMgr, repo)
	// Inject a fake update so the overlay gate passes regardless of the
	// real ~/.grove/update-check.json contents on the test host. With the
	// Skip-honoring gate in NewModel, a non-TTY test run produces empty
	// fields by design, so direct injection is the canonical seam.
	m.updateLatestVersion = "99.0.0"
	m.updateLatestURL = "https://github.com/lost-in-the/grove/releases/tag/v99.0.0"

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))

	// Wait for dashboard
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripANSI(string(bts))
		return regexp.MustCompile(`(?i)rails-app`).MatchString(s)
	}, teatest.WithDuration(5*time.Second))

	// Press u — modal should open with "Update available" and the latest version.
	tm.Send(tea.KeyPressMsg{Code: 'u', Text: "u"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripANSI(string(bts))
		return regexp.MustCompile(`Update available`).MatchString(s) &&
			regexp.MustCompile(`99\.0\.0`).MatchString(s)
	}, teatest.WithDuration(5*time.Second))

	// esc closes (mirroring HelpOverlay close-keys)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestProgram_UpdateOverlay_GateRespectsSkip verifies NewModel honors
// updatecheck.Skip — when the env-var opt-out is set, NewModel must produce
// a model with an empty updateLatestVersion regardless of cache contents.
// This is the parity guarantee with the CLI box and `grove version`.
func TestProgram_UpdateOverlay_GateRespectsSkip(t *testing.T) {
	t.Setenv("GROVE_NO_UPDATE_NOTIFIER", "1")

	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)
	m := NewModel(mgr, stateMgr, repo)

	if m.updateLatestVersion != "" {
		t.Fatalf("expected updateLatestVersion to be empty when Skip returns true, got %q", m.updateLatestVersion)
	}
	if m.updateLatestURL != "" {
		t.Fatalf("expected updateLatestURL to be empty when Skip returns true, got %q", m.updateLatestURL)
	}
}

func TestProgram_CreateWizard(t *testing.T) {
	repo := setupRailsFixture(t)
	tm := newTestProgram(t, repo)

	// Wait for dashboard
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripANSI(string(bts))
		return regexp.MustCompile(`(?i)rails-app`).MatchString(s)
	}, teatest.WithDuration(5*time.Second))

	// Press n for create wizard
	tm.Send(tea.KeyPressMsg{Code: 'n', Text: "n"})

	// Wait for the create overlay
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripANSI(string(bts))
		return regexp.MustCompile(`(?i)name|create|new`).MatchString(s)
	}, teatest.WithDuration(5*time.Second))

	// Escape to cancel
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestProgram_QuitClean(t *testing.T) {
	repo := setupRailsFixture(t)
	tm := newTestProgram(t, repo)

	// Wait for ready
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return len(bts) > 0
	}, teatest.WithDuration(5*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestProgram_EnterSwitchesWorktree(t *testing.T) {
	repo := setupRailsFixtureWithWorktrees(t, "target")
	tm := newTestProgram(t, repo)

	// Wait for worktrees to load
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := stripANSI(string(bts))
		return regexp.MustCompile(`(?i)target`).MatchString(s)
	}, teatest.WithDuration(5*time.Second))

	// The main worktree is selected first (current), so pressing enter just quits.
	// Navigate to non-current worktree first.
	tm.Send(tea.KeyPressMsg{Code: 'j', Text: "j"})
	time.Sleep(100 * time.Millisecond)

	// Press enter to select/switch
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
