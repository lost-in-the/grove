package config

// EffectiveMuxBackend returns the configured multiplexer backend, folding in
// the legacy `tmux.mode = "off"` switch.
//
// tmux.mode predates mux.backend and is how existing installs turn session
// management off entirely, so it still wins: a user who disabled tmux did not
// ask grove to start driving herdr instead.
func (c *Config) EffectiveMuxBackend() string {
	if c.Tmux.Mode == "off" {
		return "off"
	}
	if c.Mux.Backend == "" {
		return "auto"
	}
	return c.Mux.Backend
}
