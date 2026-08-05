package tui

import (
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/mux"
)

// muxFor resolves the multiplexer backend from a dashboard config.
//
// The TUI has no GroveContext, so it resolves per call site rather than
// memoizing on one. Resolution is a couple of env reads plus a cached
// exec.LookPath, so it stays cheap enough for the refresh path.
func muxFor(cfg *config.Config) mux.Multiplexer {
	pref := mux.BackendAuto
	if cfg != nil {
		if parsed, err := mux.ParseBackend(cfg.EffectiveMuxBackend()); err == nil {
			pref = parsed
		}
	}
	return mux.New(pref)
}
