package docker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

func TestParseInspectMounts(t *testing.T) {
	tests := []struct {
		name      string
		blob      string
		mountDest string
		wantSrc   string
		wantErr   bool
	}{
		{
			name:      "single bind-mount matches dest",
			blob:      `[{"Type":"bind","Source":"/Users/leah/work/proj-foo","Destination":"/app"}]`,
			mountDest: "/app",
			wantSrc:   "/Users/leah/work/proj-foo",
		},
		{
			name:      "trailing slash on Destination still matches",
			blob:      `[{"Type":"bind","Source":"/work/proj","Destination":"/app/"}]`,
			mountDest: "/app",
			wantSrc:   "/work/proj",
		},
		{
			name:      "trailing slash on probe still matches",
			blob:      `[{"Type":"bind","Source":"/work/proj","Destination":"/app"}]`,
			mountDest: "/app/",
			wantSrc:   "/work/proj",
		},
		{
			name:      "multiple mounts — first match wins",
			blob:      `[{"Source":"/vendor","Destination":"/vendor"},{"Source":"/work/proj","Destination":"/app"},{"Source":"/cache","Destination":"/cache"}]`,
			mountDest: "/app",
			wantSrc:   "/work/proj",
		},
		{
			name:      "no mount at dest returns error",
			blob:      `[{"Source":"/vendor","Destination":"/vendor"}]`,
			mountDest: "/app",
			wantErr:   true,
		},
		{
			name:      "malformed JSON returns error",
			blob:      `not json`,
			mountDest: "/app",
			wantErr:   true,
		},
		{
			name:      "empty array returns error",
			blob:      `[]`,
			mountDest: "/app",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInspectMounts([]byte(tt.blob), tt.mountDest)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error mismatch: got %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.wantSrc {
				t.Errorf("source = %q, want %q", got, tt.wantSrc)
			}
		})
	}
}

func TestSamePath(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"/foo/bar", "/foo/bar", true},
		{"/foo/bar/", "/foo/bar", true},
		{"/foo/./bar", "/foo/bar", true},
		{"/foo/bar/../baz", "/foo/baz", true},
		{"/foo/bar", "/foo/baz", false},
		{"", "/foo", false},
		{"/foo", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		if got := samePath(tt.a, tt.b); got != tt.want {
			t.Errorf("samePath(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestResolveEnvPath(t *testing.T) {
	tests := []struct {
		name        string
		composePath string
		envValue    string
		want        string
	}{
		{"empty env value", "/compose", "", ""},
		{"absolute env value passes through", "/compose", "/abs/path", "/abs/path"},
		{"relative resolves against compose", "/compose", "./proj-foo", "/compose/proj-foo"},
		{"dot-dot resolves", "/compose/sub", "../up", "/compose/up"},
		{"trailing slash cleaned", "/compose", "./proj-foo/", "/compose/proj-foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEnvPath(tt.composePath, tt.envValue)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShortID(t *testing.T) {
	if got := shortID("0123456789abcdef0000"); got != "0123456789ab" {
		t.Errorf("expected 12-char prefix, got %q", got)
	}
	if got := shortID("short"); got != "short" {
		t.Errorf("expected short string passed through, got %q", got)
	}
}

func TestMountDriftConfigFromExternal(t *testing.T) {
	t.Run("nil ext returns nil", func(t *testing.T) {
		if got := MountDriftConfigFromExternal(nil, "/compose"); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})
	t.Run("empty services returns nil", func(t *testing.T) {
		ext := &config.ExternalComposeConfig{EnvVar: "X"}
		if got := MountDriftConfigFromExternal(ext, "/compose"); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})
	t.Run("populated config", func(t *testing.T) {
		ext := &config.ExternalComposeConfig{
			EnvVar:    "PROJECT_DIR",
			EnvFile:   ".env.local",
			Services:  []string{"web", "worker"},
			MountDest: "/srv/app",
		}
		got := MountDriftConfigFromExternal(ext, "/compose")
		if got == nil {
			t.Fatal("expected non-nil config")
		}
		if got.ComposePath != "/compose" {
			t.Errorf("ComposePath = %q, want %q", got.ComposePath, "/compose")
		}
		if got.EnvFileName != ".env.local" {
			t.Errorf("EnvFileName = %q, want %q", got.EnvFileName, ".env.local")
		}
		if got.MountDest != "/srv/app" {
			t.Errorf("MountDest = %q, want %q", got.MountDest, "/srv/app")
		}
	})
	t.Run("defaults mount_dest to /app when unset", func(t *testing.T) {
		ext := &config.ExternalComposeConfig{EnvVar: "X", Services: []string{"web"}}
		got := MountDriftConfigFromExternal(ext, "/compose")
		if got.MountDest != "/app" {
			t.Errorf("MountDest = %q, want %q (default)", got.MountDest, "/app")
		}
	})
}

// TestParseInspectMounts_RealisticOutput sanity-checks the parser against an
// output shape closer to what `docker inspect --format '{{json .Mounts}}'`
// actually emits, including extra fields the struct doesn't model.
func TestParseInspectMounts_RealisticOutput(t *testing.T) {
	blob := []byte(`[
		{"Type":"bind","Source":"/Users/leah/work/proj-feature-x","Destination":"/app","Mode":"","RW":true,"Propagation":"rprivate"},
		{"Type":"volume","Name":"node_modules","Source":"/var/lib/docker/volumes/node_modules/_data","Destination":"/app/node_modules","Driver":"local","Mode":"z","RW":true,"Propagation":""}
	]`)
	got, err := parseInspectMounts(blob, "/app")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := filepath.Clean("/Users/leah/work/proj-feature-x")
	if got != want {
		t.Errorf("source = %q, want %q", got, want)
	}
}

// Regression test for the drift-check path reintroducing issue #98: a lone
// --env-file makes compose v2 ignore .env, so the args must layer .env
// underneath the configured env file via composeEnvFileArgs.
func TestComposePsQArgs_LayersDefaultEnvFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("FOO=bar\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	got := composePsQArgs(dir, ".env.local", "web")
	want := []string{"compose", "--env-file", ".env", "--env-file", ".env.local", "ps", "-q", "web"}
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestComposePsQArgs_DefaultEnvFileOmitsFlag(t *testing.T) {
	dir := t.TempDir()

	got := composePsQArgs(dir, ".env", "web")
	want := []string{"compose", "ps", "-q", "web"}
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}
