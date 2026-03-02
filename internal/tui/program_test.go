//go:build integration

package tui

import (
	"regexp"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest"
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

	// Any key closes help, then q quits
	tm.Send(tea.KeyPressMsg{Code: ' ', Text: " "})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
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
