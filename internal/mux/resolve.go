package mux

import (
	"fmt"
	"strings"
)

// Backend names a multiplexer implementation.
type Backend string

const (
	// BackendAuto picks a backend from the environment.
	BackendAuto Backend = "auto"
	// BackendTmux drives tmux.
	BackendTmux Backend = "tmux"
	// BackendHerdr drives herdr.
	BackendHerdr Backend = "herdr"
	// BackendOff disables session management entirely.
	BackendOff Backend = "off"
)

// Env is the environment probe ResolveBackend decides from. It is a plain
// struct so resolution stays testable without touching the real environment.
type Env struct {
	InsideTmux     bool
	InsideHerdr    bool
	TmuxAvailable  bool
	HerdrAvailable bool
}

// ParseBackend validates a configured backend name. An empty string means auto.
func ParseBackend(s string) (Backend, error) {
	switch b := Backend(strings.ToLower(strings.TrimSpace(s))); b {
	case "":
		return BackendAuto, nil
	case BackendAuto, BackendTmux, BackendHerdr, BackendOff:
		return b, nil
	default:
		return "", fmt.Errorf("mux.backend must be one of: auto, tmux, herdr, off (got %q)", s)
	}
}

// ResolveBackend turns a configured preference plus an environment probe into
// the backend to use.
//
// An explicit preference always wins, even when the binary is missing — the
// caller reports that through Available rather than silently driving a
// different multiplexer than the user asked for.
//
// Under auto, the multiplexer grove is already running inside wins, because
// relocating a client in a multiplexer the user is not looking at is useless.
// herdr beats tmux there: running herdr inside tmux is a normal setup, and the
// inner multiplexer owns the panes. Outside both, tmux wins ties so existing
// installs keep their behavior when herdr is merely present.
func ResolveBackend(pref Backend, env Env) Backend {
	switch pref {
	case BackendOff, BackendTmux, BackendHerdr:
		return pref
	}

	switch {
	case env.InsideHerdr && env.HerdrAvailable:
		return BackendHerdr
	case env.InsideTmux && env.TmuxAvailable:
		return BackendTmux
	case env.TmuxAvailable:
		return BackendTmux
	case env.HerdrAvailable:
		return BackendHerdr
	default:
		return BackendOff
	}
}
