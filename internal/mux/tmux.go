package mux

import (
	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/tmux"
)

// TmuxBackend drives tmux. It delegates to internal/tmux, which owns the
// command construction and exact-target quoting rules.
//
// tmux addresses sessions by name, so every method keys on Target.Name and
// Target.Path is used only when creating a session.
type TmuxBackend struct{}

// NewTmux returns the tmux backend.
func NewTmux() *TmuxBackend { return &TmuxBackend{} }

// Compile-time proof that tmux keeps its optional capabilities.
var (
	_ Multiplexer      = (*TmuxBackend)(nil)
	_ Popuper          = (*TmuxBackend)(nil)
	_ ControlModer     = (*TmuxBackend)(nil)
	_ AttachDirectiver = (*TmuxBackend)(nil)
)

// Backend returns BackendTmux.
func (b *TmuxBackend) Backend() Backend { return BackendTmux }

// Available reports whether tmux is installed.
func (b *TmuxBackend) Available() bool { return tmux.IsTmuxAvailable() }

// Inside reports whether grove is running inside a tmux client.
func (b *TmuxBackend) Inside() bool { return tmux.IsInsideTmux() }

// Ensure creates the session when missing. Idempotent.
func (b *TmuxBackend) Ensure(t Target) error {
	return tmux.CreateSession(t.Name, t.Path)
}

// EnsureWithCommand creates the session running command. Idempotent: an
// existing session is left alone and the command is not run.
func (b *TmuxBackend) EnsureWithCommand(t Target, command string) error {
	return tmux.CreateSessionWithCommand(t.Name, t.Path, command)
}

// Exists reports whether the session exists.
func (b *TmuxBackend) Exists(t Target) (bool, error) {
	return tmux.SessionExists(t.Name)
}

// List returns every tmux session. Path is left empty — tmux has no notion of
// a session's checkout, so Index falls back to name matching.
func (b *TmuxBackend) List() ([]Session, error) {
	raw, err := tmux.ListSessions()
	if err != nil {
		return nil, err
	}
	sessions := make([]Session, 0, len(raw))
	for _, s := range raw {
		sessions = append(sessions, Session{
			Name:    s.Name,
			ID:      s.Name,
			Status:  attachStatus(s.Attached),
			Agent:   AgentUnreported,
			Windows: s.Windows,
		})
	}
	return sessions, nil
}

// Current returns the name of the tmux session grove is running inside.
func (b *TmuxBackend) Current() (string, error) { return tmux.GetCurrentSession() }

// AttachHint returns the tmux command that attaches to the session.
func (b *TmuxBackend) AttachHint(t Target) string {
	return "tmux attach -t " + t.Name
}

// Attach attaches to the session, blocking until detach.
func (b *TmuxBackend) Attach(t Target) error { return tmux.AttachSession(t.Name) }

// Switch moves the current tmux client to the session.
func (b *TmuxBackend) Switch(t Target) error { return tmux.SwitchSession(t.Name) }

// Rename renames the session.
func (b *TmuxBackend) Rename(from, to Target) error {
	return tmux.RenameSession(from.Name, to.Name)
}

// Kill destroys the session, leaving the checkout untouched.
func (b *TmuxBackend) Kill(t Target) error { return tmux.KillSession(t.Name) }

// PaneInfo returns the session's active pane state.
func (b *TmuxBackend) PaneInfo(t Target) (*PaneInfo, error) {
	info, err := tmux.GetPaneInfo(t.Name)
	if err != nil {
		return nil, err
	}
	return &PaneInfo{CurrentPath: info.CurrentPath, CurrentCommand: info.CurrentCommand}, nil
}

// SendCommand types a command line into the session's active pane.
func (b *TmuxBackend) SendCommand(t Target, command string) error {
	return tmux.SendKeys(t.Name, command)
}

// Popup shows the session in a tmux display-popup overlay.
func (b *TmuxBackend) Popup(t Target, width, height string) error {
	return tmux.DisplayPopup(t.Name, width, height)
}

// UseControlMode reports whether tmux -CC applies: config permits it and the
// terminal supports it (iTerm2).
func (b *TmuxBackend) UseControlMode(cfg *bool) bool {
	return tmux.ShouldUseControlMode(cfg)
}

// AttachControlMode attaches with tmux -CC.
func (b *TmuxBackend) AttachControlMode(t Target) error {
	return tmux.AttachSessionControlMode(t.Name)
}

// AttachDirective hands attachment to the shell wrapper.
func (b *TmuxBackend) AttachDirective(t Target, controlMode bool) bool {
	cli.TmuxAttachDirective(t.Name, controlMode)
	return true
}

func attachStatus(attached bool) Status {
	if attached {
		return StatusAttached
	}
	return StatusDetached
}
