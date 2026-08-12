package mux

import "errors"

var (
	// errNotInside is returned by Current when grove is not running inside
	// the backend's multiplexer.
	errNotInside = errors.New("not inside a multiplexer session")
	// errNoSession is returned when a target has no live session.
	errNoSession = errors.New("no session for target")
	// errNoRepoRoot is returned when a backend needs Target.Repo and the
	// caller did not supply it.
	errNoRepoRoot = errors.New("target has no repository root")
	// errUnmanaged is returned by Ensure when the backend will not take
	// ownership of a target and no session should be expected for it. The
	// caller degrades to a plain directory switch rather than failing.
	errUnmanaged = errors.New("target is not managed by this backend")
)

// ErrNoSession reports whether err means "the target has no live session".
func ErrNoSession(err error) bool { return errors.Is(err, errNoSession) }

// ErrUnmanaged reports whether err means "this backend will not create a
// session for that target".
//
// It is not a failure. herdr is the case that needs it: grove deliberately
// never creates a workspace for a repository's main checkout — that is herdr's
// to own, and grove imposing one reaches past worktree lifecycle, which is the
// boundary the whole integration is built on. Commands treat it as "no session
// management here" and fall through to changing directory.
func ErrUnmanaged(err error) bool { return errors.Is(err, errUnmanaged) }
