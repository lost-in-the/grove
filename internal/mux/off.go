package mux

// OffBackend is the null multiplexer, used when the user disabled session
// management or no supported binary is installed.
//
// It exists so callers can hold a non-nil Multiplexer unconditionally: every
// mutating call is a silent no-op and every query reports "no session", which
// is exactly the behavior grove wants when there is nothing to drive. Guarding
// on Available() stays the caller's job for messaging, not correctness.
type OffBackend struct{}

// NewOff returns the null backend.
func NewOff() *OffBackend { return &OffBackend{} }

var _ Multiplexer = (*OffBackend)(nil)

// Backend returns BackendOff.
func (b *OffBackend) Backend() Backend { return BackendOff }

// Available always reports false.
func (b *OffBackend) Available() bool { return false }

// Inside always reports false.
func (b *OffBackend) Inside() bool { return false }

// Ensure does nothing.
func (b *OffBackend) Ensure(Target) error { return nil }

// EnsureWithCommand does nothing.
func (b *OffBackend) EnsureWithCommand(Target, string) error { return nil }

// Exists always reports false.
func (b *OffBackend) Exists(Target) (bool, error) { return false, nil }

// List returns no sessions.
func (b *OffBackend) List() ([]Session, error) { return nil, nil }

// Current reports that grove is not inside any session.
func (b *OffBackend) Current() (string, error) { return "", errNotInside }

// AttachHint returns nothing to type.
func (b *OffBackend) AttachHint(Target) string { return "" }

// Attach does nothing.
func (b *OffBackend) Attach(Target) error { return nil }

// Switch does nothing.
func (b *OffBackend) Switch(Target) error { return nil }

// Rename does nothing.
func (b *OffBackend) Rename(Target, Target) error { return nil }

// Kill does nothing.
func (b *OffBackend) Kill(Target) error { return nil }

// PaneInfo reports that there is no pane to inspect.
func (b *OffBackend) PaneInfo(Target) (*PaneInfo, error) { return nil, errNoSession }

// SendCommand does nothing.
func (b *OffBackend) SendCommand(Target, string) error { return nil }
