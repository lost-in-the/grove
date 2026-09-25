// Package mux abstracts the terminal multiplexer grove drives for worktree
// sessions. Two backends exist: tmux (the historical default) and herdr.
//
// The two disagree about identity. tmux addresses sessions by name, so grove's
// canonical {project}-{name} is the key. herdr addresses workspaces by opaque
// id and treats the label as cosmetic, so the checkout path is the key. Target
// therefore carries both, and every backend picks the field it can key on.
package mux

// Target identifies one worktree session in backend-neutral terms.
//
// Name is the canonical tmux session name / herdr workspace label — always the
// {project}-{name} form from worktree.TmuxSessionName, regardless of the
// directory naming pattern. Path is the worktree checkout path. Repo is the
// repository's main checkout.
//
// Name and Path must always be populated: tmux keys on Name, herdr keys on
// Path. Repo is required by herdr when creating or adopting a session, because
// herdr resolves the *source repository* separately from the worktree being
// opened, and rejects a linked worktree in that role. Operations on a session
// that already exists (kill, focus, rename) do not need it.
type Target struct {
	Name string
	Path string
	Repo string
	// Short is the worktree's display name — what the user typed and what
	// `grove ls` shows. Backends use it to name the window or tab the worktree
	// occupies, where the project prefix in Name would be noise. Optional;
	// DisplayName falls back to Name.
	Short string
}

// DisplayName returns the name to show a human for this target: the worktree's
// short name when known, otherwise the canonical session name.
func (t Target) DisplayName() string {
	if t.Short != "" {
		return t.Short
	}
	return t.Name
}

// Status is the attachment state of a session.
type Status string

const (
	// StatusAttached means a client is currently viewing the session (tmux).
	StatusAttached Status = "attached"
	// StatusDetached means the session exists but no client is viewing it
	// (tmux).
	StatusDetached Status = "detached"
	// StatusActive means the workspace is the server's focused one (herdr).
	//
	// herdr cannot say whether anyone is looking: `focused` is a single
	// server-wide "current workspace" that stays set with no client attached
	// at all, and several clients may each view a different workspace. So
	// herdr reports active/open rather than borrowing tmux's attached/detached,
	// which would claim a viewer that may not exist.
	StatusActive Status = "active"
	// StatusOpen means the workspace exists but is not the focused one (herdr).
	StatusOpen Status = "open"
	// StatusNone means no session exists for the target.
	StatusNone Status = "none"
)

// Foreground reports whether the status marks the session in front: attached
// under tmux, active under herdr.
func (s Status) Foreground() bool { return s == StatusAttached || s == StatusActive }

// Background reports whether the session exists but is not in front:
// detached under tmux, open under herdr.
func (s Status) Background() bool { return s == StatusDetached || s == StatusOpen }

// AgentStatus is the coding-agent lifecycle state a backend reports for a
// session. Only herdr can report these; the tmux backend always returns
// AgentUnreported.
type AgentStatus string

const (
	// AgentUnreported means the backend cannot observe agent state at all.
	AgentUnreported AgentStatus = ""
	// AgentIdle means the agent is ready for input and has been seen.
	AgentIdle AgentStatus = "idle"
	// AgentWorking means the agent is actively running.
	AgentWorking AgentStatus = "working"
	// AgentBlocked means the agent is waiting on input or approval.
	AgentBlocked AgentStatus = "blocked"
	// AgentDone means background work finished and has not been seen yet.
	AgentDone AgentStatus = "done"
	// AgentUnknown means an agent is present but could not be classified.
	// It does not prove completion.
	AgentUnknown AgentStatus = "unknown"
)

// Observed reports whether the backend actually saw a coding agent.
//
// AgentUnknown does not count. herdr rolls a workspace up to "unknown"
// whenever nothing identifiable is running — a plain shell pane reports it —
// so treating it as a sighting would put a meaningless word on every row.
func (a AgentStatus) Observed() bool {
	switch a {
	case AgentIdle, AgentWorking, AgentBlocked, AgentDone:
		return true
	default:
		return false
	}
}

// Session is a live multiplexer session belonging to a worktree.
type Session struct {
	// Name is the session name (tmux) or workspace label (herdr).
	Name string
	// Path is the session's checkout path. Empty when the backend cannot
	// report it — the tmux backend leaves this unset.
	Path string
	// Repo is the repository's main checkout, when the backend reports it
	// (herdr's worktree provenance). The tmux backend leaves this unset.
	Repo string
	// ID is the backend's own handle, used for follow-up calls. For tmux this
	// equals Name; for herdr it is the opaque workspace id (e.g. "w1").
	ID string
	// Status is the attachment state.
	Status Status
	// Agent is the rolled-up agent lifecycle state, or AgentUnreported.
	Agent AgentStatus
	// Windows counts windows (tmux) or panes (herdr).
	Windows int
}

// PaneInfo describes a session's active pane.
type PaneInfo struct {
	CurrentPath    string
	CurrentCommand string
	// HasAgent reports that a recognized coding agent occupies the pane. Only
	// herdr can detect this; tmux leaves it false.
	HasAgent bool
}

// IsShell reports whether the pane's foreground command is a known shell.
// Grove only corrects directory drift when it is — sending `cd` into a running
// editor or agent would type into it instead.
func (p *PaneInfo) IsShell() bool {
	if p.HasAgent {
		return false
	}
	switch p.CurrentCommand {
	case "bash", "zsh", "fish", "sh":
		return true
	default:
		return false
	}
}

// Multiplexer is the session surface grove drives. Implementations must be
// safe to construct even when the backing binary is absent — callers check
// Available first.
type Multiplexer interface {
	// Backend names the implementation ("tmux", "herdr", "off").
	Backend() Backend
	// Available reports whether the backing binary is installed.
	Available() bool
	// Inside reports whether grove is running inside this multiplexer.
	Inside() bool

	// Ensure idempotently creates or adopts the session for a target.
	Ensure(t Target) error
	// EnsureWithCommand is Ensure, running command in the new session's pane.
	// When the session already exists the command is not run.
	EnsureWithCommand(t Target, command string) error
	// Exists reports whether a session exists for the target.
	Exists(t Target) (bool, error)
	// List returns every session the backend knows about.
	List() ([]Session, error)
	// Current returns the name of the session grove is running inside.
	Current() (string, error)
	// AttachHint returns the command a user would type to attach by hand.
	// Shown in manual mode, where grove prepares the session but leaves
	// attaching to the user.
	AttachHint(t Target) string

	// Attach blocks until the user detaches.
	Attach(t Target) error
	// Switch moves the current client to the target. Only valid from Inside.
	Switch(t Target) error
	// Rename moves a session from one target's identity to another's.
	Rename(from, to Target) error
	// Kill destroys the session. It never touches the checkout on disk.
	Kill(t Target) error

	// PaneInfo returns the target session's active pane state.
	PaneInfo(t Target) (*PaneInfo, error)
	// SendCommand runs a shell command line in the session's active pane.
	SendCommand(t Target, command string) error
}

// Popuper is implemented by backends that can display a session in an overlay
// without relocating the client. tmux implements it via display-popup; herdr
// exposes popup placement only through its plugin pane surface, so the herdr
// backend does not implement it and callers fall back to a full attach.
type Popuper interface {
	Popup(t Target, width, height string) error
}

// ControlModer is implemented by backends with a terminal-native control
// protocol — tmux -CC under iTerm2. herdr has no equivalent.
type ControlModer interface {
	// UseControlMode reports whether control mode applies right now, given the
	// config override (nil means "auto-detect").
	UseControlMode(cfg *bool) bool
	// AttachControlMode attaches using the control protocol. Blocks.
	AttachControlMode(t Target) error
}

// SessionLocator is implemented by backends that run several independent
// servers side by side — herdr's named sessions, each with its own workspaces
// and no awareness of the others — so a target's session may live somewhere
// other than the server grove is talking to.
//
// Ensure on such a backend adopts a target's existing session from another
// server instead of opening a duplicate, and Kill closes the target's session
// in every running server. Callers therefore must not gate Kill on Exists,
// which only answers for the server grove is talking to.
type SessionLocator interface {
	// LocatedIn names the other server whose session Ensure adopted for t, or
	// "" when t's session is (or would be) in the one grove is talking to.
	LocatedIn(t Target) string
	// KillEverywhere is Kill, also reporting which other servers it closed
	// t's session in and which it could not check.
	KillEverywhere(t Target) (KillReport, error)
}

// KillReport is what KillEverywhere did beyond the server grove talks to.
type KillReport struct {
	// ClosedIn names the other servers t's session was closed in.
	ClosedIn []string
	// Unchecked names running servers that did not answer in time; a copy of
	// t's session may remain in them.
	Unchecked []string
}

// AttachPreparer is implemented by backends whose attach command cannot name
// its target — herdr's client starts on whichever workspace its server has
// focused. PrepareAttach readies the target so that a user who runs AttachHint
// by hand lands on it.
type AttachPreparer interface {
	PrepareAttach(t Target) error
}

// Adopter is implemented by backends that keep their own record of whether
// grove tracks a worktree — herdr's sidebar marker, set by grove's herdr
// plugin on worktrees created outside grove. Adopted is called after grove
// adopts such a worktree, to bring its session in line.
type Adopter interface {
	Adopted(t Target) error
}

// AttachDirectiver is implemented by backends that can hand attachment off to
// grove's shell wrapper instead of attaching in-process.
type AttachDirectiver interface {
	// AttachDirective emits the shell-integration directive that makes the
	// wrapper attach after grove exits. Returns false when the backend cannot
	// express this, and the caller should attach directly instead.
	AttachDirective(t Target, controlMode bool) bool
}
