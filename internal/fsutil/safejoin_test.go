package fsutil

import (
	"testing"
)

func TestSafeJoin(t *testing.T) {
	base := "/work/myproject-testing"

	tests := []struct {
		name    string
		rel     string
		want    string
		wantErr bool
	}{
		{
			name: "plain relative path inside base",
			rel:  "config/app.yml",
			want: "/work/myproject-testing/config/app.yml",
		},
		{
			name: "single component",
			rel:  "secrets",
			want: "/work/myproject-testing/secrets",
		},
		{
			name:    "dotdot escape attempt",
			rel:     "../../.ssh/id_rsa",
			wantErr: true,
		},
		{
			name:    "dotdot in middle",
			rel:     "config/../../.ssh/id_rsa",
			wantErr: true,
		},
		{
			name:    "absolute path",
			rel:     "/etc/passwd",
			wantErr: true,
		},
		{
			name: "same dir dot",
			rel:  ".",
			want: base,
		},
		{
			name: "traversal that resolves inside base",
			rel:  "a/../b",
			want: "/work/myproject-testing/b",
		},
		{
			name: "nested path",
			rel:  "vendor/bundle/ruby",
			want: "/work/myproject-testing/vendor/bundle/ruby",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeJoin(base, tt.rel)
			if tt.wantErr {
				if err == nil {
					t.Errorf("SafeJoin(%q, %q) = %q, want error", base, tt.rel, got)
				}
				return
			}
			if err != nil {
				t.Errorf("SafeJoin(%q, %q) unexpected error: %v", base, tt.rel, err)
				return
			}
			if got != tt.want {
				t.Errorf("SafeJoin(%q, %q) = %q, want %q", base, tt.rel, got, tt.want)
			}
		})
	}
}
