package mux

import "errors"

var (
	// errNotInside is returned by Current when grove is not running inside
	// the backend's multiplexer.
	errNotInside = errors.New("not inside a multiplexer session")
	// errNoSession is returned when a target has no live session.
	errNoSession = errors.New("no session for target")
)

// ErrNotInside reports whether err means "grove is not inside a session".
func ErrNotInside(err error) bool { return errors.Is(err, errNotInside) }

// ErrNoSession reports whether err means "the target has no live session".
func ErrNoSession(err error) bool { return errors.Is(err, errNoSession) }
