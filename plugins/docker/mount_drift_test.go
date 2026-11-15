package docker

import (
	"errors"
	"fmt"
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
			blob:      `[{"Type":"bind","Source":"/home/dev/projects/myapp-foo","Destination":"/app"}]`,
			mountDest: "/app",
			wantSrc:   "/home/dev/projects/myapp-foo",
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
	t.Run("carries per-service mount_dests through", func(t *testing.T) {
		ext := &config.ExternalComposeConfig{
			EnvVar:     "X",
			Services:   []string{"app", "worker"},
			MountDests: map[string]string{"worker": "/srv/worker"},
		}
		got := MountDriftConfigFromExternal(ext, "/compose")
		if got.MountDests["worker"] != "/srv/worker" {
			t.Errorf("MountDests = %v, want worker=/srv/worker", got.MountDests)
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
		{"Type":"bind","Source":"/home/dev/projects/myapp-feature","Destination":"/app","Mode":"","RW":true,"Propagation":"rprivate"},
		{"Type":"volume","Name":"node_modules","Source":"/var/lib/docker/volumes/node_modules/_data","Destination":"/app/node_modules","Driver":"local","Mode":"z","RW":true,"Propagation":""}
	]`)
	got, err := parseInspectMounts(blob, "/app")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := filepath.Clean("/home/dev/projects/myapp-feature")
	if got != want {
		t.Errorf("source = %q, want %q", got, want)
	}
}

// fakeMountInspector stands in for the docker CLI, the same way fakeStrategy
// stands in for the compose strategy in hook_compose_test.go.
type fakeMountInspector struct {
	ids       map[string]string // service -> container ID ("" = not running)
	idErr     map[string]error  // service -> compose ps failure
	sources   map[string]string // container ID -> mount source
	mountErr  map[string]error  // container ID -> inspect failure
	probed    map[string]string // service -> dest probed
	serviceOf map[string]string // container ID -> service, filled as ids are handed out
}

func (f *fakeMountInspector) ContainerID(_, _, service string) (string, error) {
	if err := f.idErr[service]; err != nil {
		return "", err
	}
	id := f.ids[service]
	if f.serviceOf == nil {
		f.serviceOf = map[string]string{}
	}
	f.serviceOf[id] = service
	return id, nil
}

func (f *fakeMountInspector) MountSource(containerID, dest string) (string, error) {
	if f.probed == nil {
		f.probed = map[string]string{}
	}
	f.probed[f.serviceOf[containerID]] = dest
	if err := f.mountErr[containerID]; err != nil {
		return "", err
	}
	return f.sources[containerID], nil
}

func driftTestConfig(t *testing.T, services ...string) MountDriftConfig {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_DIR=/work/myapp-feature\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	return MountDriftConfig{ComposePath: dir, EnvFileName: ".env", EnvVar: "APP_DIR", Services: services}
}

func TestCheckMountDrift_Statuses(t *testing.T) {
	cfg := driftTestConfig(t, "app", "worker", "db", "cache", "web")
	insp := &fakeMountInspector{
		ids: map[string]string{"app": "aaa", "worker": "bbb", "db": "", "cache": "ddd", "web": "eee"},
		idErr: map[string]error{
			"cache": errors.New("docker compose ps: daemon down"),
		},
		sources: map[string]string{"aaa": "/work/myapp-feature", "bbb": "/work/myapp-main"},
		mountErr: map[string]error{
			"eee": fmt.Errorf("%w: no bind-mount at %q in container", errNoMount, "/app"),
		},
	}

	reports, err := checkMountDrift(cfg, insp)
	if err != nil {
		t.Fatalf("checkMountDrift: %v", err)
	}
	want := map[string]MountStatus{
		"app":    MountVerified,
		"worker": MountDrift,
		"db":     MountNotRunning,
		"cache":  MountError,
		"web":    MountNoMount,
	}
	for _, r := range reports {
		if r.Status != want[r.Service] {
			t.Errorf("%s: status = %q, want %q (detail %q)", r.Service, r.Status, want[r.Service], r.Detail)
		}
	}
}

func TestCheckMountDrift_InspectFailureIsErrorNotNoMount(t *testing.T) {
	cfg := driftTestConfig(t, "app")
	insp := &fakeMountInspector{
		ids:      map[string]string{"app": "aaa"},
		mountErr: map[string]error{"aaa": errors.New("docker inspect: exit status 1")},
	}
	reports, _ := checkMountDrift(cfg, insp)
	if reports[0].Status != MountError {
		t.Errorf("status = %q, want %q", reports[0].Status, MountError)
	}
}

func TestCheckMountDrift_PerServiceMountDest(t *testing.T) {
	cfg := driftTestConfig(t, "app", "worker")
	cfg.MountDest = "/app"
	cfg.MountDests = map[string]string{"worker": "/srv/worker"}
	insp := &fakeMountInspector{
		ids:     map[string]string{"app": "aaa", "worker": "bbb"},
		sources: map[string]string{"aaa": "/work/myapp-feature", "bbb": "/work/myapp-feature"},
	}
	reports, _ := checkMountDrift(cfg, insp)
	if insp.probed["app"] != "/app" || insp.probed["worker"] != "/srv/worker" {
		t.Errorf("probed = %v, want app=/app worker=/srv/worker", insp.probed)
	}
	if reports[1].MountDest != "/srv/worker" || reports[1].Status != MountVerified {
		t.Errorf("worker report = %+v", reports[1])
	}
}

func TestSummarizeMountReports(t *testing.T) {
	mk := func(statuses ...MountStatus) []MountDriftReport {
		var rs []MountDriftReport
		for _, s := range statuses {
			rs = append(rs, MountDriftReport{Status: s})
		}
		return rs
	}
	tests := []struct {
		name         string
		reports      []MountDriftReport
		wantLine     string
		wantAllClear bool
	}{
		{"all verified", mk(MountVerified, MountVerified, MountVerified), "All 3 services mounted from the configured worktree.", true},
		{"single verified", mk(MountVerified), "All 1 service mounted from the configured worktree.", true},
		{"mixed", mk(MountVerified, MountVerified, MountVerified, MountVerified, MountNoMount, MountNoMount, MountNotRunning, MountNotRunning), "4 of 8 services verified; 4 not checked", false},
		{"all stopped", mk(MountNotRunning, MountNotRunning), "0 of 2 services verified; 2 not checked", false},
		{"drift is checked, not unchecked", mk(MountVerified, MountDrift), "1 of 2 services verified; 1 drifted", false},
		{"drift and unchecked", mk(MountVerified, MountDrift, MountError), "1 of 3 services verified; 1 drifted; 1 not checked", false},
		{"docker error", mk(MountError), "0 of 1 service verified; 1 not checked", false},
		{"no reports", nil, "0 of 0 services verified", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := SummarizeMountReports(tt.reports)
			if got := s.Line(); got != tt.wantLine {
				t.Errorf("Line() = %q, want %q", got, tt.wantLine)
			}
			if s.AllVerified() != tt.wantAllClear {
				t.Errorf("AllVerified() = %v, want %v", s.AllVerified(), tt.wantAllClear)
			}
		})
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
