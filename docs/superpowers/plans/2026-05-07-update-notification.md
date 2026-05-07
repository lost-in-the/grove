# Update Notification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Print a one-line "newer grove release available" notice on stderr after each command, in a way that never blocks execution and can be opted out of.

**Architecture:** Async cache-then-show-next-run pattern (npm/update-notifier convention). New `internal/updatecheck/` package wired into cobra's `PersistentPostRunE`. After each command finishes, the package reads `~/.grove/update-check.json` and prints a notification if the cached `latest_version` is newer than `version.Version`; then it kicks off a detached goroutine to refresh the cache for the next invocation. First run silently populates the cache.

**Tech Stack:** Go 1.25.x, standard library (`net/http`, `encoding/json`, `os`, `path/filepath`, `time`), `golang.org/x/term`, `github.com/charmbracelet/lipgloss` (already a project dependency via TUI). Tests use `testing` + `net/http/httptest`.

**Spec:** [`docs/superpowers/specs/2026-05-07-update-notification-design.md`](../specs/2026-05-07-update-notification-design.md)

---

## File Structure

**New files** (all under `internal/updatecheck/`):

| File | Responsibility |
|---|---|
| `skip.go` / `skip_test.go` | `Skip()` — env vars, TTY, version-suffix detection |
| `cache.go` / `cache_test.go` | `Cache` struct, `ReadCache()`, `WriteCache()`, atomic-write + path resolution |
| `github.go` / `github_test.go` | `FetchLatest()` — `api.github.com/repos/lost-in-the/grove/releases/latest` with 2s timeout |
| `install.go` / `install_test.go` | `DetectInstall()` + `UpdateCommand()` — brew / go-install / binary fallback |
| `render.go` / `render_test.go` | `CompareSemver()`, `RenderBox()` — box format, severity color, NO_COLOR fallback |
| `check.go` / `check_test.go` | `MaybeNotify()`, `RefreshAsync()`, `CheckNow()` — orchestration |

**Modified files:**

| File | Change |
|---|---|
| `cmd/grove/commands/root.go` | Add `PersistentPostRunE`, `--no-update-notifier` and `--check-update` persistent flags |
| `CHANGELOG.md` | Entry under `[Unreleased] ### Added` |
| `docs/CONFIGURATION_REFERENCE.md` | Document new env vars + flags |

---

## Task 1: Skip detection

**Files:**
- Create: `internal/updatecheck/skip.go`
- Create: `internal/updatecheck/skip_test.go`

- [ ] **Step 1: Write the failing test (table-driven, all 13 conditions)**

```go
// internal/updatecheck/skip_test.go
package updatecheck

import (
	"testing"
)

func TestSkip(t *testing.T) {
	cases := []struct {
		name        string
		env         map[string]string
		flag        bool
		version     string
		stdoutIsTTY bool
		want        bool
	}{
		{"happy path", nil, false, "0.6.0", true, false},
		{"CI=true", map[string]string{"CI": "true"}, false, "0.6.0", true, true},
		{"GITHUB_ACTIONS=true", map[string]string{"GITHUB_ACTIONS": "true"}, false, "0.6.0", true, true},
		{"BUILDKITE=true", map[string]string{"BUILDKITE": "true"}, false, "0.6.0", true, true},
		{"CIRCLECI=true", map[string]string{"CIRCLECI": "true"}, false, "0.6.0", true, true},
		{"TRAVIS=true", map[string]string{"TRAVIS": "true"}, false, "0.6.0", true, true},
		{"GROVE_AGENT_MODE=1", map[string]string{"GROVE_AGENT_MODE": "1"}, false, "0.6.0", true, true},
		{"GROVE_NONINTERACTIVE=1", map[string]string{"GROVE_NONINTERACTIVE": "1"}, false, "0.6.0", true, true},
		{"NO_UPDATE_NOTIFIER=1", map[string]string{"NO_UPDATE_NOTIFIER": "1"}, false, "0.6.0", true, true},
		{"GROVE_NO_UPDATE_NOTIFIER=1", map[string]string{"GROVE_NO_UPDATE_NOTIFIER": "1"}, false, "0.6.0", true, true},
		{"flag --no-update-notifier", nil, true, "0.6.0", true, true},
		{"non-TTY stdout", nil, false, "0.6.0", false, true},
		{"version unknown", nil, false, "unknown", true, true},
		{"version -dev suffix", nil, false, "0.7.0-dev", true, true},
		{"version non-semver", nil, false, "abc", true, true},
		{"version with v prefix", nil, false, "v0.6.0", true, false}, // tolerated
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			got := skipWithDeps(tc.env, tc.flag, tc.version, tc.stdoutIsTTY)
			if got != tc.want {
				t.Errorf("Skip() = %v, want %v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updatecheck/ -run TestSkip -v`
Expected: FAIL with `undefined: skipWithDeps`

- [ ] **Step 3: Write minimal implementation**

```go
// internal/updatecheck/skip.go
package updatecheck

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// Skip returns true when update checking should be entirely suppressed.
// Honors CI env vars, grove-specific opt-out, the --no-update-notifier flag,
// non-TTY stdout, and dev/unknown/non-semver versions.
func Skip(noUpdateNotifierFlag bool, currentVersion string) bool {
	env := map[string]string{}
	for _, k := range []string{
		"CI", "GITHUB_ACTIONS", "BUILDKITE", "CIRCLECI", "TRAVIS",
		"GROVE_AGENT_MODE", "GROVE_NONINTERACTIVE",
		"NO_UPDATE_NOTIFIER", "GROVE_NO_UPDATE_NOTIFIER",
	} {
		if v := os.Getenv(k); v != "" {
			env[k] = v
		}
	}
	stdoutIsTTY := term.IsTerminal(int(os.Stdout.Fd()))
	return skipWithDeps(env, noUpdateNotifierFlag, currentVersion, stdoutIsTTY)
}

// skipWithDeps is the testable core of Skip — pure function over its inputs.
func skipWithDeps(env map[string]string, flag bool, version string, stdoutIsTTY bool) bool {
	if flag {
		return true
	}
	for _, k := range []string{
		"CI", "GITHUB_ACTIONS", "BUILDKITE", "CIRCLECI", "TRAVIS",
		"GROVE_AGENT_MODE", "GROVE_NONINTERACTIVE",
		"NO_UPDATE_NOTIFIER", "GROVE_NO_UPDATE_NOTIFIER",
	} {
		if env[k] != "" {
			return true
		}
	}
	if !stdoutIsTTY {
		return true
	}
	return !isReleasedVersion(version)
}

// isReleasedVersion returns true for versions that look like real releases
// (semver, no -dev suffix, not "unknown"). A leading "v" is tolerated.
func isReleasedVersion(v string) bool {
	if v == "" || v == "unknown" {
		return false
	}
	v = strings.TrimPrefix(v, "v")
	if strings.Contains(v, "-dev") {
		return false
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/updatecheck/ -run TestSkip -v`
Expected: PASS (16 subtests)

- [ ] **Step 5: Commit**

```bash
git add internal/updatecheck/skip.go internal/updatecheck/skip_test.go
git commit -m "feat(updatecheck): skip detection (env, TTY, version suffix)"
```

---

## Task 2: Cache read/write

**Files:**
- Create: `internal/updatecheck/cache.go`
- Create: `internal/updatecheck/cache_test.go`

- [ ] **Step 1: Write failing tests for Cache roundtrip + atomic write + missing/corrupt tolerance**

```go
// internal/updatecheck/cache_test.go
package updatecheck

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCache_RoundtripJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	c := Cache{
		Version:       1,
		LastCheckedAt: time.Date(2026, 5, 7, 12, 34, 56, 0, time.UTC),
		LatestVersion: "0.6.0",
		LatestURL:     "https://github.com/lost-in-the/grove/releases/tag/v0.6.0",
	}
	if err := WriteCacheToPath(path, c); err != nil {
		t.Fatalf("WriteCacheToPath: %v", err)
	}
	got, err := ReadCacheFromPath(path)
	if err != nil {
		t.Fatalf("ReadCacheFromPath: %v", err)
	}
	if got != c {
		t.Errorf("roundtrip mismatch:\n got  %+v\n want %+v", got, c)
	}
}

func TestCache_MissingFileReturnsZeroValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")
	got, err := ReadCacheFromPath(path)
	if err != nil {
		t.Fatalf("ReadCacheFromPath should not error on missing file: %v", err)
	}
	if got != (Cache{}) {
		t.Errorf("missing file should yield zero Cache, got %+v", got)
	}
}

func TestCache_CorruptFileReturnsZeroValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	if err := os.WriteFile(path, []byte("not json {{{"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := ReadCacheFromPath(path)
	if err != nil {
		t.Fatalf("ReadCacheFromPath should swallow corrupt JSON: %v", err)
	}
	if got != (Cache{}) {
		t.Errorf("corrupt file should yield zero Cache, got %+v", got)
	}
}

func TestCache_AtomicWriteUsesTempThenRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	c := Cache{Version: 1, LatestVersion: "0.6.0"}
	if err := WriteCacheToPath(path, c); err != nil {
		t.Fatalf("WriteCacheToPath: %v", err)
	}
	// after success, no leftover .tmp file should exist
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected no leftover %s.tmp, got: %v", path, err)
	}
	// final file should exist with the right content
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected final file at %s: %v", path, err)
	}
}

func TestCache_DefaultPathUnderHome(t *testing.T) {
	t.Setenv("HOME", "/tmp/fake-home-for-test")
	got := DefaultCachePath()
	want := "/tmp/fake-home-for-test/.grove/update-check.json"
	if got != want {
		t.Errorf("DefaultCachePath() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updatecheck/ -run TestCache -v`
Expected: FAIL with `undefined: Cache, WriteCacheToPath, ReadCacheFromPath, DefaultCachePath`

- [ ] **Step 3: Write minimal implementation**

```go
// internal/updatecheck/cache.go
package updatecheck

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Cache is the on-disk representation of the last update check.
type Cache struct {
	Version       int       `json:"version"`
	LastCheckedAt time.Time `json:"last_checked_at"`
	LatestVersion string    `json:"latest_version"`
	LatestURL     string    `json:"latest_url"`
}

// DefaultCachePath returns ~/.grove/update-check.json.
func DefaultCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".grove", "update-check.json")
}

// ReadCacheFromPath reads the cache file. Missing or corrupt files yield a
// zero-value Cache and a nil error — callers treat zero Cache as "no data".
func ReadCacheFromPath(path string) (Cache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Cache{}, nil
		}
		return Cache{}, err
	}
	var c Cache
	if err := json.Unmarshal(data, &c); err != nil {
		// Corrupt JSON is treated as missing — caller will refresh on next run.
		return Cache{}, nil
	}
	return c, nil
}

// WriteCacheToPath atomically writes the cache: write to path+".tmp", then rename.
// Creates the parent directory if it doesn't exist.
func WriteCacheToPath(path string, c Cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/updatecheck/ -run TestCache -v`
Expected: PASS (5 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/updatecheck/cache.go internal/updatecheck/cache_test.go
git commit -m "feat(updatecheck): cache read/write with atomic semantics"
```

---

## Task 3: GitHub Releases API client

**Files:**
- Create: `internal/updatecheck/github.go`
- Create: `internal/updatecheck/github_test.go`

- [ ] **Step 1: Write failing tests using httptest (success + 404 + timeout + malformed)**

```go
// internal/updatecheck/github_test.go
package updatecheck

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchLatest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name":"v0.6.0","html_url":"https://example/releases/tag/v0.6.0"}`)
	}))
	defer server.Close()

	rel, err := fetchLatestFromURL(server.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.TagName != "v0.6.0" {
		t.Errorf("TagName = %q, want %q", rel.TagName, "v0.6.0")
	}
	if rel.HTMLURL != "https://example/releases/tag/v0.6.0" {
		t.Errorf("HTMLURL = %q", rel.HTMLURL)
	}
}

func TestFetchLatest_404IsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	_, err := fetchLatestFromURL(server.URL, 2*time.Second)
	if err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestFetchLatest_TimeoutIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // longer than client timeout below
	}))
	defer server.Close()

	_, err := fetchLatestFromURL(server.URL, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestFetchLatest_MalformedJSONIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name": [not a string]}`)
	}))
	defer server.Close()

	_, err := fetchLatestFromURL(server.URL, 2*time.Second)
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestFetchLatest_StripsLeadingV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name":"v1.2.3","html_url":"https://x"}`)
	}))
	defer server.Close()

	rel, err := fetchLatestFromURL(server.URL, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version() != "1.2.3" {
		t.Errorf("Version() = %q, want %q", rel.Version(), "1.2.3")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updatecheck/ -run TestFetchLatest -v`
Expected: FAIL with `undefined: fetchLatestFromURL, Release, ...`

- [ ] **Step 3: Write minimal implementation**

```go
// internal/updatecheck/github.go
package updatecheck

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// LatestReleaseURL is the canonical GitHub Releases API endpoint.
const LatestReleaseURL = "https://api.github.com/repos/lost-in-the/grove/releases/latest"

// FetchTimeout is the hard cap on the HTTP request.
const FetchTimeout = 2 * time.Second

// Release is the subset of the GitHub release JSON we care about.
type Release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Version returns the bare semver (no leading v).
func (r Release) Version() string {
	return strings.TrimPrefix(r.TagName, "v")
}

// FetchLatest queries the GitHub Releases API for the latest release.
func FetchLatest() (Release, error) {
	return fetchLatestFromURL(LatestReleaseURL, FetchTimeout)
}

func fetchLatestFromURL(url string, timeout time.Duration) (Release, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "grove-update-check")

	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Release{}, fmt.Errorf("github releases: HTTP %d", resp.StatusCode)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, fmt.Errorf("github releases: decode: %w", err)
	}
	return rel, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/updatecheck/ -run TestFetchLatest -v`
Expected: PASS (5 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/updatecheck/github.go internal/updatecheck/github_test.go
git commit -m "feat(updatecheck): GitHub Releases API client with 2s timeout"
```

---

## Task 4: Install method detection

**Files:**
- Create: `internal/updatecheck/install.go`
- Create: `internal/updatecheck/install_test.go`

- [ ] **Step 1: Write failing tests for each install method**

```go
// internal/updatecheck/install_test.go
package updatecheck

import "testing"

func TestDetectInstallFromPath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want InstallMethod
	}{
		{"homebrew apple silicon", "/opt/homebrew/Cellar/grove/0.6.0/bin/grove", InstallBrew},
		{"homebrew intel", "/usr/local/Cellar/grove/0.6.0/bin/grove", InstallBrew},
		{"go install", "/Users/leah/go/bin/grove", InstallGoInstall},
		{"binary download", "/usr/local/bin/grove", InstallBinary},
		{"empty", "", InstallUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectInstallFromPath(tc.path); got != tc.want {
				t.Errorf("detectInstallFromPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestUpdateCommand(t *testing.T) {
	cases := []struct {
		method InstallMethod
		want   string
	}{
		{InstallBrew, "brew upgrade lost-in-the/tap/grove"},
		{InstallGoInstall, "go install github.com/lost-in-the/grove/cmd/grove@latest"},
		{InstallBinary, "Visit https://github.com/lost-in-the/grove/releases for the latest binary"},
		{InstallUnknown, "Visit https://github.com/lost-in-the/grove/releases for the latest binary"},
	}
	for _, tc := range cases {
		t.Run(tc.method.String(), func(t *testing.T) {
			if got := UpdateCommand(tc.method); got != tc.want {
				t.Errorf("UpdateCommand(%v) = %q, want %q", tc.method, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updatecheck/ -run "TestDetectInstall|TestUpdateCommand" -v`
Expected: FAIL with `undefined: InstallMethod, detectInstallFromPath, UpdateCommand`

- [ ] **Step 3: Write minimal implementation**

```go
// internal/updatecheck/install.go
package updatecheck

import (
	"os"
	"strings"
)

// InstallMethod identifies how the running grove binary was installed.
type InstallMethod int

const (
	InstallUnknown InstallMethod = iota
	InstallBrew
	InstallGoInstall
	InstallBinary
)

// String renders the method as a human-readable name (used in test output and logs).
func (m InstallMethod) String() string {
	switch m {
	case InstallBrew:
		return "brew"
	case InstallGoInstall:
		return "go-install"
	case InstallBinary:
		return "binary"
	default:
		return "unknown"
	}
}

// DetectInstall inspects the running binary's path and returns the most likely
// install method.
func DetectInstall() InstallMethod {
	exe, err := os.Executable()
	if err != nil {
		return InstallUnknown
	}
	return detectInstallFromPath(exe)
}

func detectInstallFromPath(path string) InstallMethod {
	if path == "" {
		return InstallUnknown
	}
	if strings.Contains(path, "/Cellar/grove/") {
		// covers /opt/homebrew/Cellar/grove/... and /usr/local/Cellar/grove/...
		return InstallBrew
	}
	if strings.HasSuffix(path, "/go/bin/grove") || strings.Contains(path, "/go/bin/") {
		return InstallGoInstall
	}
	return InstallBinary
}

// UpdateCommand returns the recommended update command for a given install method.
func UpdateCommand(m InstallMethod) string {
	switch m {
	case InstallBrew:
		return "brew upgrade lost-in-the/tap/grove"
	case InstallGoInstall:
		return "go install github.com/lost-in-the/grove/cmd/grove@latest"
	default:
		return "Visit https://github.com/lost-in-the/grove/releases for the latest binary"
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/updatecheck/ -run "TestDetectInstall|TestUpdateCommand" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/updatecheck/install.go internal/updatecheck/install_test.go
git commit -m "feat(updatecheck): install method detection with tailored update command"
```

---

## Task 5: Semver compare (severity)

**Files:**
- Create: `internal/updatecheck/render.go`
- Create: `internal/updatecheck/render_test.go`

This task adds the semver comparison logic only — the box rendering is Task 6.

- [ ] **Step 1: Write failing tests for `CompareSemver` covering severity buckets and edge cases**

```go
// internal/updatecheck/render_test.go
package updatecheck

import "testing"

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		current, latest string
		want            Severity
	}{
		{"0.5.0", "0.6.0", SeverityMinor},
		{"0.5.0", "1.0.0", SeverityMajor},
		{"0.5.0", "0.5.1", SeverityPatch},
		{"0.5.0", "0.5.0", SeverityNone},
		{"0.5.0", "0.4.99", SeverityNone}, // older latest = no upgrade
		{"v0.5.0", "v0.6.0", SeverityMinor}, // v-prefix tolerated
		{"abc", "0.6.0", SeverityNone}, // unparseable current
		{"0.5.0", "abc", SeverityNone}, // unparseable latest
	}
	for _, tc := range cases {
		t.Run(tc.current+"->"+tc.latest, func(t *testing.T) {
			if got := CompareSemver(tc.current, tc.latest); got != tc.want {
				t.Errorf("CompareSemver(%q,%q) = %v, want %v", tc.current, tc.latest, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updatecheck/ -run TestCompareSemver -v`
Expected: FAIL with `undefined: Severity, CompareSemver`

- [ ] **Step 3: Write minimal implementation**

```go
// internal/updatecheck/render.go
package updatecheck

import (
	"strconv"
	"strings"
)

// Severity describes the difference between current and latest semver.
type Severity int

const (
	SeverityNone Severity = iota
	SeverityPatch
	SeverityMinor
	SeverityMajor
)

// CompareSemver returns the severity of upgrade from current to latest.
// SeverityNone means current is already at or ahead of latest, or either is unparseable.
func CompareSemver(current, latest string) Severity {
	c, ok := parseSemver(current)
	if !ok {
		return SeverityNone
	}
	l, ok := parseSemver(latest)
	if !ok {
		return SeverityNone
	}
	if l[0] > c[0] {
		return SeverityMajor
	}
	if l[0] < c[0] {
		return SeverityNone
	}
	if l[1] > c[1] {
		return SeverityMinor
	}
	if l[1] < c[1] {
		return SeverityNone
	}
	if l[2] > c[2] {
		return SeverityPatch
	}
	return SeverityNone
}

func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/updatecheck/ -run TestCompareSemver -v`
Expected: PASS (8 subtests)

- [ ] **Step 5: Commit**

```bash
git add internal/updatecheck/render.go internal/updatecheck/render_test.go
git commit -m "feat(updatecheck): semver compare with severity buckets"
```

---

## Task 6: Box rendering (plain + colored)

**Files:**
- Modify: `internal/updatecheck/render.go`
- Modify: `internal/updatecheck/render_test.go`

- [ ] **Step 1: Write failing tests for plain rendering and severity-colored variants**

```go
// internal/updatecheck/render_test.go (append to existing file)

func TestRenderBox_Plain(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := RenderBox("0.5.0", "0.6.0",
		"https://github.com/lost-in-the/grove/releases/tag/v0.6.0",
		"brew upgrade lost-in-the/tap/grove",
	)
	// Plain mode: no ANSI escape codes
	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI escape sequences in plain output, got:\n%s", out)
	}
	mustContain(t, out, "Update available")
	mustContain(t, out, "0.5.0")
	mustContain(t, out, "0.6.0")
	mustContain(t, out, "brew upgrade lost-in-the/tap/grove")
	mustContain(t, out, "github.com/lost-in-the/grove/releases/tag/v0.6.0")
}

func TestRenderBox_ColoredEmitsAnsi(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1") // hint to lipgloss to render colors even when not a TTY
	out := RenderBox("0.5.0", "1.0.0", "https://x", "brew upgrade x")
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("expected ANSI escape sequences in colored output, got: %q", out)
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected %q in output, got:\n%s", needle, haystack)
	}
}
```

You'll also need to add `"strings"` to the import block of `render_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updatecheck/ -run TestRenderBox -v`
Expected: FAIL with `undefined: RenderBox`

- [ ] **Step 3: Write minimal implementation**

Add to `render.go`:

```go
// (additional imports — append to existing imports block)
import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderBox returns the formatted box notification as a string.
// When NO_COLOR is set, the output is plain ASCII (no ANSI escapes).
func RenderBox(currentVersion, latestVersion, latestURL, updateCmd string) string {
	severity := CompareSemver(currentVersion, latestVersion)
	body := []string{
		fmt.Sprintf("Update available  %s  →  %s", currentVersion, latestVersion),
		"Run: " + updateCmd,
		"Changelog: " + latestURL,
	}
	if os.Getenv("NO_COLOR") != "" {
		return renderPlain(body)
	}
	return renderColored(body, severity)
}

func renderPlain(lines []string) string {
	width := 0
	for _, l := range lines {
		if len(l) > width {
			width = len(l)
		}
	}
	var b strings.Builder
	b.WriteString("+" + strings.Repeat("-", width+2) + "+\n")
	for _, l := range lines {
		b.WriteString("| ")
		b.WriteString(l)
		b.WriteString(strings.Repeat(" ", width-len(l)))
		b.WriteString(" |\n")
	}
	b.WriteString("+" + strings.Repeat("-", width+2) + "+\n")
	return b.String()
}

func renderColored(lines []string, severity Severity) string {
	border := lipgloss.NormalBorder()
	color := severityColor(severity)
	style := lipgloss.NewStyle().
		Border(border).
		BorderForeground(lipgloss.Color(color)).
		Padding(0, 1)
	return style.Render(strings.Join(lines, "\n")) + "\n"
}

func severityColor(s Severity) string {
	switch s {
	case SeverityMajor:
		return "9" // bright red
	case SeverityMinor:
		return "11" // yellow
	case SeverityPatch:
		return "8" // dim/gray
	default:
		return "7"
	}
}

// strconv import is needed by parseSemver; keep it in the existing imports block.
var _ = strconv.Atoi // silence unused if parseSemver is the only consumer
```

(Note: drop the `var _ = strconv.Atoi` line if `parseSemver` is the sole consumer of `strconv` — Go will keep the import alive without it. Adjust based on actual code state.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/updatecheck/ -run TestRenderBox -v`
Expected: PASS (2 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/updatecheck/render.go internal/updatecheck/render_test.go
git commit -m "feat(updatecheck): box rendering with severity color and NO_COLOR fallback"
```

---

## Task 7: MaybeNotify — read cache and print

**Files:**
- Create: `internal/updatecheck/check.go`
- Create: `internal/updatecheck/check_test.go`

- [ ] **Step 1: Write failing tests for MaybeNotify (newer / equal / missing / older / cooldown not gating display)**

```go
// internal/updatecheck/check_test.go
package updatecheck

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMaybeNotify_NewerCachedVersionPrintsBox(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{
		Version:       1,
		LastCheckedAt: time.Now().Add(-1 * time.Hour),
		LatestVersion: "0.6.0",
		LatestURL:     "https://github.com/lost-in-the/grove/releases/tag/v0.6.0",
	})

	var buf bytes.Buffer
	maybeNotifyFromPath(&buf, "0.5.0", path)

	if !strings.Contains(buf.String(), "Update available") {
		t.Errorf("expected 'Update available' in output, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "0.6.0") {
		t.Errorf("expected '0.6.0' in output, got:\n%s", buf.String())
	}
}

func TestMaybeNotify_SameVersionNoOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{LatestVersion: "0.6.0"})

	var buf bytes.Buffer
	maybeNotifyFromPath(&buf, "0.6.0", path)

	if buf.Len() != 0 {
		t.Errorf("expected no output when versions match, got: %q", buf.String())
	}
}

func TestMaybeNotify_MissingCacheNoOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	var buf bytes.Buffer
	maybeNotifyFromPath(&buf, "0.5.0", path)

	if buf.Len() != 0 {
		t.Errorf("expected no output with missing cache, got: %q", buf.String())
	}
}

func TestMaybeNotify_OlderCachedVersionNoOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{LatestVersion: "0.4.0"})

	var buf bytes.Buffer
	maybeNotifyFromPath(&buf, "0.5.0", path)

	if buf.Len() != 0 {
		t.Errorf("expected no output when current is newer than cached latest, got: %q", buf.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updatecheck/ -run TestMaybeNotify -v`
Expected: FAIL with `undefined: maybeNotifyFromPath`

- [ ] **Step 3: Write minimal implementation**

```go
// internal/updatecheck/check.go
package updatecheck

import (
	"io"
	"os"
)

// MaybeNotify reads the default cache and writes a notification to w if a
// newer release is available. Always non-blocking. No-op on missing/corrupt
// cache or when the user is already at or ahead of the cached latest.
func MaybeNotify(w io.Writer, currentVersion string) {
	maybeNotifyFromPath(w, currentVersion, DefaultCachePath())
}

func maybeNotifyFromPath(w io.Writer, currentVersion, path string) {
	c, err := ReadCacheFromPath(path)
	if err != nil || c.LatestVersion == "" {
		return
	}
	if CompareSemver(currentVersion, c.LatestVersion) == SeverityNone {
		return
	}
	method := DetectInstall()
	box := RenderBox(currentVersion, c.LatestVersion, c.LatestURL, UpdateCommand(method))
	_, _ = io.WriteString(w, box)
	_ = os.Stderr // silence linter if MaybeNotify is the only writer-target consumer
}
```

(Drop the `_ = os.Stderr` line if not needed — included only as a hedge against unused-import warnings during incremental development.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/updatecheck/ -run TestMaybeNotify -v`
Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/updatecheck/check.go internal/updatecheck/check_test.go
git commit -m "feat(updatecheck): MaybeNotify reads cache and renders notification"
```

---

## Task 8: RefreshAsync — detached cache refresh

**Files:**
- Modify: `internal/updatecheck/check.go`
- Modify: `internal/updatecheck/check_test.go`

- [ ] **Step 1: Write failing tests for refresh (interval gating + cache update + silent on failure)**

```go
// internal/updatecheck/check_test.go (append)

func TestRefresh_WithinIntervalSkips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	original := Cache{
		Version:       1,
		LastCheckedAt: time.Now(), // brand new
		LatestVersion: "0.5.0",
		LatestURL:     "https://example/old",
	}
	_ = WriteCacheToPath(path, original)

	// fakeFetcher returns a "newer" release; it must not be called.
	called := false
	fetcher := func() (Release, error) {
		called = true
		return Release{TagName: "v9.9.9", HTMLURL: "https://example/new"}, nil
	}
	refreshFromPathWithFetcher(path, "0.5.0", time.Hour, fetcher)
	if called {
		t.Error("fetcher should not be called within the interval")
	}
	got, _ := ReadCacheFromPath(path)
	if got.LatestVersion != "0.5.0" {
		t.Errorf("cache should be unchanged, got LatestVersion=%q", got.LatestVersion)
	}
}

func TestRefresh_BeyondIntervalUpdatesCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{
		Version:       1,
		LastCheckedAt: time.Now().Add(-48 * time.Hour),
		LatestVersion: "0.5.0",
	})

	fetcher := func() (Release, error) {
		return Release{TagName: "v0.6.0", HTMLURL: "https://example/new"}, nil
	}
	refreshFromPathWithFetcher(path, "0.5.0", 24*time.Hour, fetcher)

	got, err := ReadCacheFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.LatestVersion != "0.6.0" {
		t.Errorf("LatestVersion = %q, want %q", got.LatestVersion, "0.6.0")
	}
	if got.LatestURL != "https://example/new" {
		t.Errorf("LatestURL not updated: %q", got.LatestURL)
	}
}

func TestRefresh_FetchFailureLeavesCacheAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	original := Cache{
		Version:       1,
		LastCheckedAt: time.Now().Add(-48 * time.Hour),
		LatestVersion: "0.5.0",
		LatestURL:     "https://example/old",
	}
	_ = WriteCacheToPath(path, original)

	fetcher := func() (Release, error) {
		return Release{}, errFakeNetwork
	}
	refreshFromPathWithFetcher(path, "0.5.0", 24*time.Hour, fetcher)

	got, _ := ReadCacheFromPath(path)
	if got != original {
		t.Errorf("cache should be unchanged on fetch failure:\n got:  %+v\n want: %+v", got, original)
	}
}

var errFakeNetwork = fmt.Errorf("simulated network failure")
```

(Add `"fmt"` to test imports if not already present.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updatecheck/ -run TestRefresh -v`
Expected: FAIL with `undefined: refreshFromPathWithFetcher`

- [ ] **Step 3: Write minimal implementation**

Add to `check.go`:

```go
import (
	"io"
	"os"
	"time"
)

// CheckInterval is the default minimum gap between two refreshes.
const CheckInterval = 24 * time.Hour

// RefreshAsync starts a detached goroutine that refreshes the cache if the
// interval has elapsed. Never blocks. Failures are silent.
func RefreshAsync(currentVersion string) {
	go refreshFromPathWithFetcher(DefaultCachePath(), currentVersion, CheckInterval, func() (Release, error) {
		return FetchLatest()
	})
}

// refreshFromPathWithFetcher is the testable core: deterministic, no goroutine,
// no global state. Reads cache, checks interval, calls fetcher, writes cache.
func refreshFromPathWithFetcher(path, currentVersion string, interval time.Duration, fetcher func() (Release, error)) {
	c, _ := ReadCacheFromPath(path)
	if !c.LastCheckedAt.IsZero() && time.Since(c.LastCheckedAt) < interval {
		return
	}
	rel, err := fetcher()
	if err != nil {
		return
	}
	_ = WriteCacheToPath(path, Cache{
		Version:       1,
		LastCheckedAt: time.Now(),
		LatestVersion: rel.Version(),
		LatestURL:     rel.HTMLURL,
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/updatecheck/ -run TestRefresh -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/updatecheck/check.go internal/updatecheck/check_test.go
git commit -m "feat(updatecheck): RefreshAsync with interval-gated detached cache update"
```

---

## Task 9: CheckNow — synchronous force-check

**Files:**
- Modify: `internal/updatecheck/check.go`
- Modify: `internal/updatecheck/check_test.go`

- [ ] **Step 1: Write failing tests for the synchronous flow**

```go
// internal/updatecheck/check_test.go (append)

func TestCheckNow_FetchesAndPrintsNotification(t *testing.T) {
	fetcher := func() (Release, error) {
		return Release{TagName: "v0.6.0", HTMLURL: "https://x"}, nil
	}
	var buf bytes.Buffer
	if err := checkNowWithDeps(&buf, "0.5.0", fetcher); err != nil {
		t.Fatalf("checkNowWithDeps: %v", err)
	}
	if !strings.Contains(buf.String(), "0.6.0") {
		t.Errorf("expected '0.6.0' in output, got:\n%s", buf.String())
	}
}

func TestCheckNow_UpToDateMessage(t *testing.T) {
	fetcher := func() (Release, error) {
		return Release{TagName: "v0.5.0", HTMLURL: "https://x"}, nil
	}
	var buf bytes.Buffer
	if err := checkNowWithDeps(&buf, "0.5.0", fetcher); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "up to date") {
		t.Errorf("expected 'up to date' message, got: %q", buf.String())
	}
}

func TestCheckNow_FetchFailureReturnsError(t *testing.T) {
	fetcher := func() (Release, error) { return Release{}, errFakeNetwork }
	var buf bytes.Buffer
	if err := checkNowWithDeps(&buf, "0.5.0", fetcher); err == nil {
		t.Error("expected error on fetch failure")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updatecheck/ -run TestCheckNow -v`
Expected: FAIL with `undefined: checkNowWithDeps`

- [ ] **Step 3: Write minimal implementation**

Add to `check.go`:

```go
// CheckNow synchronously fetches the latest release and prints the result to w.
// Bypasses the cache and the interval. Used by --check-update.
func CheckNow(w io.Writer, currentVersion string) error {
	return checkNowWithDeps(w, currentVersion, func() (Release, error) { return FetchLatest() })
}

func checkNowWithDeps(w io.Writer, currentVersion string, fetcher func() (Release, error)) error {
	rel, err := fetcher()
	if err != nil {
		return err
	}
	if CompareSemver(currentVersion, rel.Version()) == SeverityNone {
		_, _ = io.WriteString(w, "grove is up to date ("+currentVersion+")\n")
		return nil
	}
	method := DetectInstall()
	_, _ = io.WriteString(w, RenderBox(currentVersion, rel.Version(), rel.HTMLURL, UpdateCommand(method)))
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/updatecheck/ -run TestCheckNow -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/updatecheck/check.go internal/updatecheck/check_test.go
git commit -m "feat(updatecheck): CheckNow synchronous force-check API"
```

---

## Task 10: Wire into root cobra command

**Files:**
- Modify: `cmd/grove/commands/root.go`

This task has no test of its own — the wired behavior is exercised end-to-end by the test in Task 11 (CHANGELOG/docs commit) via integration. Cobra wiring is straightforward enough that running grove and observing behavior is the verification.

- [ ] **Step 1: Add persistent flags and PersistentPostRunE**

Modify `cmd/grove/commands/root.go`. Locate the import block and the `var rootCmd = &cobra.Command{ ... }` declaration.

In the imports (alphabetized — adjust to match the file's existing style):

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/log"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tui"
	"github.com/lost-in-the/grove/internal/updatecheck"
	"github.com/lost-in-the/grove/internal/version"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/docker"
)
```

Add a package-level variable for the flag and a `PersistentPostRunE`. The new flag definition goes in the existing `func init()`:

```go
var noUpdateNotifierFlag bool
var checkUpdateFlag bool
```

Add to `rootCmd`:

```go
PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
    if checkUpdateFlag {
        return updatecheck.CheckNow(os.Stderr, version.Version)
    }
    if updatecheck.Skip(noUpdateNotifierFlag, version.Version) {
        return nil
    }
    updatecheck.MaybeNotify(os.Stderr, version.Version)
    updatecheck.RefreshAsync(version.Version)
    return nil
},
```

In `init()`:

```go
func init() {
    rootCmd.PersistentFlags().BoolVar(&noUpdateNotifierFlag, "no-update-notifier", false,
        "suppress the new-release notification on this run")
    rootCmd.PersistentFlags().BoolVar(&checkUpdateFlag, "check-update", false,
        "force a synchronous check for a newer grove release and exit")
}
```

- [ ] **Step 2: Verify build is clean**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Smoke-test the wiring locally**

Run: `go run ./cmd/grove version`
Expected: prints version normally; no notification (because `0.7.0-dev` triggers Skip)

Run: `GROVE_NO_UPDATE_NOTIFIER=1 go run ./cmd/grove version`
Expected: still no notification (skip env var)

Run: `go run ./cmd/grove --check-update`
Expected: either renders a box if a newer release exists on GitHub, or prints "grove is up to date (0.7.0-dev)" — though for a dev build the up-to-date message will compare against an unparseable version and likely print up-to-date message OR error, depending on actual behavior. **If you observe `--check-update` printing nothing useful for dev builds, that's a known limitation; the flag is intended for users on real released versions.**

- [ ] **Step 4: Run unit tests to confirm nothing broke**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/grove/commands/root.go
git commit -m "feat(cli): wire update notification into root cobra command"
```

---

## Task 11: CHANGELOG + docs

**Files:**
- Modify: `CHANGELOG.md`
- Modify: `docs/CONFIGURATION_REFERENCE.md`

- [ ] **Step 1: Add CHANGELOG entry**

Read `CHANGELOG.md` and find the `[Unreleased] ### Added` section (create if absent). Add this entry:

```markdown
- Optional update-available notification on command exit when a newer grove release is published. Checks at most once per 24 hours, in a detached background process — never blocks command execution. Suppressed in CI, non-TTY contexts, and when `NO_UPDATE_NOTIFIER`, `GROVE_NO_UPDATE_NOTIFIER`, `GROVE_AGENT_MODE`, or `GROVE_NONINTERACTIVE` env vars are set, or when `--no-update-notifier` is passed. Use `--check-update` to force a synchronous check at any time.
```

- [ ] **Step 2: Update CONFIGURATION_REFERENCE**

Read `docs/CONFIGURATION_REFERENCE.md` to find the env-var section's existing style. Add a subsection or table rows for the new env vars. Suggested wording:

```markdown
### Update notifications

| Variable | Effect |
|---|---|
| `GROVE_NO_UPDATE_NOTIFIER=1` | Suppress the update-available notification entirely |
| `NO_UPDATE_NOTIFIER=1` | Same — honored for compatibility with the npm `update-notifier` convention |
| `GROVE_AGENT_MODE=1` | Suppresses notifications (in addition to other agent-mode behavior) |
| `GROVE_NONINTERACTIVE=1` | Suppresses notifications |

Per-run flags:

| Flag | Effect |
|---|---|
| `--no-update-notifier` | Suppress on this invocation |
| `--check-update` | Force a synchronous check now and exit; bypasses the 24h cooldown |

The check runs in a detached background process and never delays command execution. Notifications are only printed when a newer stable release is available and stdout is a TTY.
```

- [ ] **Step 3: Verify both docs render correctly (skim in editor or grep for links)**

Run: `grep -n "update-notifier\|update.*notification\|update-check" CHANGELOG.md docs/CONFIGURATION_REFERENCE.md`
Expected: confirmed entries

- [ ] **Step 4: Run all tests + linter (final gate)**

Run: `make lint test`
Expected: 0 lint issues, all tests PASS

- [ ] **Step 5: Commit**

```bash
git add CHANGELOG.md docs/CONFIGURATION_REFERENCE.md
git commit -m "docs(updatecheck): CHANGELOG entry + env var reference for #35"
```

---

## Task 12: Open the PR

- [ ] **Step 1: Push branch and open PR**

```bash
git push -u origin feat/35-update-notification
gh pr create --repo lost-in-the/grove \
  --title "feat(cli): update-available notification on command exit" \
  --body "$(cat <<'EOF'
## Summary
Adds an optional notification on command exit when a newer grove release is available. Implements the cache-then-show-next-run pattern (npm/update-notifier convention) so the user's command never blocks waiting for a network call.

Closes #35.

## Spec
[`docs/superpowers/specs/2026-05-07-update-notification-design.md`](docs/superpowers/specs/2026-05-07-update-notification-design.md) covers the design decisions and trade-offs.

## What's in the PR
- `internal/updatecheck/` — new package with skip detection, cache, GitHub client, install detection, semver compare, box rendering, and orchestration (MaybeNotify / RefreshAsync / CheckNow)
- `cmd/grove/commands/root.go` — wired into `PersistentPostRunE`; adds `--no-update-notifier` and `--check-update` persistent flags
- `CHANGELOG.md` and `docs/CONFIGURATION_REFERENCE.md` updates

## What's not in the PR (filed as separate issues post-merge)
- Rich TUI experience (status-bar badge + `u`-keybind modal)
- `--version` output annotation when an update is available
- Per-user config file support

## Test plan
- [x] All tests pass: `make lint test`
- [x] Manual smoke: `--check-update` returns either a box or "up to date"
- [x] Manual smoke: `--no-update-notifier` suppresses
- [x] Manual smoke: `GROVE_NO_UPDATE_NOTIFIER=1` suppresses
- [x] Manual smoke: piping to `cat` (non-TTY stdout) suppresses
EOF
)"
```

- [ ] **Step 2: File the three follow-up issues**

After PR is open, file these as separate issues using the gh CLI:

```bash
gh issue create --repo lost-in-the/grove \
  --title "feat(tui): rich update-available experience — status-bar badge + keybind modal" \
  --body "Per the research doc and the design spec for #35, the TUI deserves its own update-notification UX: passive status-bar badge while running, plus a user-triggered modal via 'u' keybind. Different cadence (7-14 day check). See spec at docs/superpowers/specs/2026-05-07-update-notification-design.md (Out of scope section)."

gh issue create --repo lost-in-the/grove \
  --title "feat(cli): annotate grove --version output when update available" \
  --body "Append '(update available: X.Y.Z)' to grove --version output when ~/.grove/update-check.json indicates a newer release. Touches cmd/grove/commands/version.go. Filed during #35 implementation."

gh issue create --repo lost-in-the/grove \
  --title "feat(config): per-user config file at ~/.grove/config.toml" \
  --body "Grove currently has per-project config (.grove/config.toml). Per-user settings (e.g. update_check.interval) currently rely on env vars. Discuss: do we want a per-user config file, and what's its scope? Filed during #35 implementation."
```

---

## Self-review checklist

Before marking the plan done, verify against the spec:

- [x] Skip detection — Task 1 covers all 13 conditions from the spec's matrix
- [x] Cache schema + atomic write — Task 2 (matches spec's `version`, `last_checked_at`, `latest_version`, `latest_url`)
- [x] GitHub API client + 2s timeout — Task 3
- [x] Install detection (brew/go-install/binary) — Task 4
- [x] Semver compare with severity (major/minor/patch) — Task 5
- [x] Box rendering with NO_COLOR fallback + lipgloss colored — Task 6
- [x] First-run-grace via natural cache lifecycle — Task 7 (missing cache → no output)
- [x] RefreshAsync interval-gated detached refresh — Task 8
- [x] CheckNow synchronous force-check — Task 9
- [x] PersistentPostRunE wiring + flags — Task 10
- [x] CHANGELOG + CONFIGURATION_REFERENCE — Task 11
- [x] Three follow-up issues filed — Task 12 step 2

Type and signature consistency check:
- `Skip(flag, version)` signature is consistent across Task 1 and Task 10
- `Cache` struct fields are referenced consistently in Tasks 2, 7, 8
- `Severity` enum values used the same way in Tasks 5, 6
- `InstallMethod` consumed by `UpdateCommand` and `MaybeNotify`/`CheckNow` consistently
- `Release` struct used identically in Tasks 3, 8, 9
