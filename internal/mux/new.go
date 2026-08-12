package mux

import (
	"os"

	"github.com/lost-in-the/grove/internal/tmux"
)

// ProbeEnv inspects the running environment for backend resolution.
func ProbeEnv() Env {
	return Env{
		InsideTmux:     tmux.IsInsideTmux(),
		InsideHerdr:    os.Getenv("HERDR_ENV") == "1",
		TmuxAvailable:  tmux.IsTmuxAvailable(),
		HerdrAvailable: herdrAvailable(),
	}
}

// New returns the multiplexer for a configured preference, resolving "auto"
// against the current environment. It never returns nil: an unavailable or
// disabled backend yields OffBackend, whose calls are no-ops.
func New(pref Backend) Multiplexer {
	return newFor(ResolveBackend(pref, ProbeEnv()))
}

func newFor(b Backend) Multiplexer {
	switch b {
	case BackendTmux:
		return NewTmux()
	case BackendHerdr:
		return NewHerdr()
	default:
		return NewOff()
	}
}
