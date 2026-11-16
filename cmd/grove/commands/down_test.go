package commands

import (
	"path/filepath"
	"testing"
)

func TestSharedStackDownNotice(t *testing.T) {
	const current = "/work/myapp-feature"
	name := filepath.Base

	tests := []struct {
		name       string
		shared     bool
		configured string
		ok         bool
		want       string
	}{
		{name: "configured for another worktree", shared: true, configured: "/work/myapp-main", ok: true,
			want: "Stopping the stack. It is on 'myapp-main', not here."},
		{name: "configured for the current worktree", shared: true, configured: current, ok: true, want: ""},
		{name: "env var unset", shared: true, want: ""},
		{name: "not the shared stack", shared: false, configured: "/work/myapp-main", ok: true, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sharedStackDownNotice(tt.shared, tt.configured, tt.ok, current, name)
			if got != tt.want {
				t.Errorf("notice = %q, want %q", got, tt.want)
			}
		})
	}
}
