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
)

// ErrNoRepoRoot reports whether err means "Target.Repo was required but unset".
func ErrNoRepoRoot(err error) bool { return errors.Is(err, errNoRepoRoot) }

// ErrNotInside reports whether err means "grove is not inside a session".
func ErrNotInside(err error) bool { return errors.Is(err, errNotInside) }

// ErrNoSession reports whether err means "the target has no live session".
func ErrNoSession(err error) bool { return errors.Is(err, errNoSession) }
