package tui

import (
	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/tuilog"
)

// sessionSwitchUsable reports whether the exit may move the user by
// switching or attaching a session: the gates handleTmuxSwitch checks before
// it tries.
func sessionSwitchUsable(cfg *config.Config, mx mux.Multiplexer) bool {
	tmuxMode := "auto"
	if cfg != nil && cfg.Tmux.Mode != "" {
		tmuxMode = cfg.Tmux.Mode
	}
	return tmuxMode != "off" && !sessionsSuppressed(cfg) && mx.Available()
}

// canMoveUser reports whether a "take me there" exit can move the user at
// all: through a session, or through the grove shell function.
func canMoveUser(cfg *config.Config, mx mux.Multiplexer) bool {
	return sessionSwitchUsable(cfg, mx) || cli.CanCd()
}

// checkCanMove decides, once at start, whether the dashboard can move the
// user, so it can say so before they press enter.
func (m Model) checkCanMove() Model {
	m.setCannotMove(!canMoveUser(m.cfg, muxFor(m.cfg)))
	return m
}

func (m *Model) setCannotMove(v bool) {
	m.cannotMove = v
	if m.helpOverlay != nil {
		m.helpOverlay.CannotMove = v
	}
}

// moveAfterExit moves the user to path after the dashboard closed: a
// session switch first, else a cd through the shell function. When neither
// can happen it says why on stderr, with the cd to run by hand.
func (m *Model) moveAfterExit(path string, stderr *cli.Writer) {
	// A session switch must win: a cd as well would change the OLD
	// session's directory.
	if m.handleTmuxSwitch(path) {
		return
	}
	// Route through cli.CdDirective: it prefers a pre-existing
	// GROVE_CD_FILE (never recreating a stale one) and falls back to a cd:
	// line on stdout under GROVE_SHELL.
	separateFromAltScreen()
	if cli.CdDirective(path) {
		tuilog.Printf("moveAfterExit: emitted cd directive for %q", path)
		return
	}
	tuilog.Printf("moveAfterExit: no session and no shell integration, cannot move to %q", path)
	cli.ExplainCannotMove(stderr, path)
}

// moveNoticeText is the dashboard's standing line when it cannot move you.
const moveNoticeText = "⚠ grove cannot move you from here: shell integration is not loaded"

func moveNotice() string { return Styles.WarningText.Render(moveNoticeText) }

// helpSectionCannotMove leads the key reference when grove cannot move you.
var helpSectionCannotMove = helpSection{
	title: "⚠ grove cannot move you from here",
	note:  "No session can be switched, and shell integration is not loaded.",
	items: []helpEntry{
		{`eval "$(grove install zsh)"`, "Add to ~/.zshrc (bash: ~/.bashrc), then open a new shell"},
		{"grove", "Run it by name, not by its path: the path skips the shell function"},
		{"enter", "Still quits, and prints the cd to run"},
	},
}
