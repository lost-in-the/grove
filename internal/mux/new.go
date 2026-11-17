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
		InsideHerdrApp: InsideHerdrApp(),
		TmuxAvailable:  tmux.IsTmuxAvailable(),
		HerdrAvailable: herdrAvailable(),
	}
}

// InsideHerdrApp reports whether this process runs in a pane of the herdr
// desktop app. The app runs on herdr-core, which sets HC_TERMINAL_ID and none
// of the HERDR_ variables the herdr server sets.
func InsideHerdrApp() bool {
	return os.Getenv("HC_TERMINAL_ID") != ""
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
