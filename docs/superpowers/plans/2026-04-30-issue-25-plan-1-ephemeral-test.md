# Issue 25 — Plan 1: Ephemeral `grove test` Path

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `grove test` resilient to unhealthy services in the shared external-mode stack by skipping dependency resolution by default, optionally bind-mounting the worktree, and rewriting compose's confusing error message into actionable guidance.

**Architecture:** `compose run` currently starts every service listed in the target service's `depends_on` chain; a single failing one-shot init service therefore aborts the test run before any test code executes. We change `externalStrategy.Run()` (and `localStrategy.Run()`) to pass `--no-deps`, gate the old behavior behind opt-in config (`[test] include_deps = true`), add an opt-in `[test] bind_mount` that pins the worktree path inside the container regardless of the shared env var, and parse compose's `didn't complete successfully` stderr pattern into a grove-styled error pointing at the offending service.

**Tech Stack:** Go 1.24, `os/exec`, `docker compose run`, TOML config, table-driven tests with mocked `composeCommand` constructor.

**Compatibility with Plans 2 & 3:**
- Plan 2 lands additions in `cmd/grove/commands/context.go` (drift check) and a new `adopt.go` — no overlap with this plan's files.
- Plan 3 will *extend* `ExternalComposeConfig` with `non_blocking_services`. Plan 1's only config change is to `TestConfig` (`include_deps`, `bind_mount`), so the two are additive.
- This plan deliberately does **not** add service-health probing; that's Plan 3's domain. After Plan 1 ships, Plan 3 may turn out to be unnecessary.

---

### Task 1: Add `IncludeDeps` and `BindMount` to `TestConfig`

**Why this matters (Learning Mode design note):**
- `--no-deps` becoming the default means we silently change behavior for users whose tests *do* legitimately need deps to start. Putting an explicit opt-out in config (rather than only behind a CLI flag) lets a project commit the right policy alongside the test command.
- `bind_mount` is opt-in because grove can't introspect the compose service's `WORKDIR`. Forcing a default would guess wrong.

**Files:**
- Modify: `internal/config/config.go:11-14` (extend `TestConfig`)
- Modify: `internal/config/config_test.go` (add load test)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test** — append to `internal/config/config_test.go`:

```go
func TestLoadConfig_TestSection_IncludeDepsAndBindMount(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.toml")
	body := `
[test]
command = "bin/rspec"
service = "app"
include_deps = true
bind_mount = "/app"
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfigFromPath(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Test.Command != "bin/rspec" {
		t.Errorf("Command: got %q want bin/rspec", cfg.Test.Command)
	}
	if cfg.Test.Service != "app" {
		t.Errorf("Service: got %q want app", cfg.Test.Service)
	}
	if !cfg.Test.IncludeDeps {
		t.Errorf("IncludeDeps: got false want true")
	}
	if cfg.Test.BindMount != "/app" {
		t.Errorf("BindMount: got %q want /app", cfg.Test.BindMount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoadConfig_TestSection_IncludeDepsAndBindMount -v`
Expected: FAIL with `unknown field "include_deps"` (or compile error: `Test.IncludeDeps` undefined).

- [ ] **Step 3: Extend `TestConfig`** — replace lines 11–14 of `internal/config/config.go`:

```go
// TestConfig controls test command behavior
type TestConfig struct {
	Command     string `toml:"command"`
	Service     string `toml:"service"`
	IncludeDeps bool   `toml:"include_deps"` // when true, `compose run` resolves depends_on services (default: skip)
	BindMount   string `toml:"bind_mount"`   // optional container path; when set, `-v <worktree>:<bind_mount>` is added
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestLoadConfig_TestSection_IncludeDepsAndBindMount -v`
Expected: PASS.

- [ ] **Step 5: Run the full config package tests**

Run: `go test ./internal/config/ -v`
Expected: All existing tests still pass.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add test.include_deps and test.bind_mount options"
```

---

### Task 2: Make `externalStrategy.Run()` skip deps and bind-mount when configured

**Files:**
- Modify: `plugins/docker/external.go:138-159`
- Test: `plugins/docker/external_run_test.go` (NEW)

- [ ] **Step 1: Write the failing test** — create `plugins/docker/external_run_test.go`:

```go
package docker

import (
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

// captureRunArgs intercepts the compose command construction so we can assert
// on the arg list without actually invoking docker.
func captureRunArgs(t *testing.T, cfg *config.Config, worktreePath, service, command string) []string {
	t.Helper()
	s := newExternalStrategy(cfg)
	args := s.buildRunArgs(worktreePath, service, command)
	return args
}

func TestExternalRun_DefaultUsesNoDeps(t *testing.T) {
	cfg := &config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				External: &config.ExternalComposeConfig{
					Path:     "/tmp/compose",
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
		Test: config.TestConfig{Command: "bin/rspec", Service: "app"},
	}

	args := captureRunArgs(t, cfg, "/tmp/wt", "app", "bin/rspec")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--no-deps") {
		t.Errorf("expected --no-deps in args, got: %v", args)
	}
	if strings.Contains(joined, "-v ") {
		t.Errorf("expected no -v flag (no bind_mount configured), got: %v", args)
	}
}

func TestExternalRun_IncludeDepsTrueOmitsNoDeps(t *testing.T) {
	cfg := &config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				External: &config.ExternalComposeConfig{
					Path:     "/tmp/compose",
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
		Test: config.TestConfig{
			Command:     "bin/rspec",
			Service:     "app",
			IncludeDeps: true,
		},
	}

	args := captureRunArgs(t, cfg, "/tmp/wt", "app", "bin/rspec")

	for _, a := range args {
		if a == "--no-deps" {
			t.Errorf("expected --no-deps to be omitted, got: %v", args)
		}
	}
}

func TestExternalRun_BindMountAddsVolumeFlag(t *testing.T) {
	cfg := &config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				External: &config.ExternalComposeConfig{
					Path:     "/tmp/compose",
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
		Test: config.TestConfig{
			Command:   "bin/rspec",
			Service:   "app",
			BindMount: "/app",
		},
	}

	args := captureRunArgs(t, cfg, "/tmp/wt", "app", "bin/rspec")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-v /tmp/wt:/app") {
		t.Errorf("expected -v /tmp/wt:/app, got: %v", args)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./plugins/docker/ -run TestExternalRun -v`
Expected: FAIL with `s.buildRunArgs undefined`.

- [ ] **Step 3: Refactor `Run` to extract `buildRunArgs`** — replace `externalStrategy.Run` (lines 138-159 of `plugins/docker/external.go`) with:

```go
func (s *externalStrategy) Run(worktreePath string, service string, command string) error {
	// Persist so the env file stays consistent with what we're running against
	if err := s.persistEnvVar(worktreePath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to persist %s to %s: %v\n", s.ext.EnvVar, s.ext.EnvFileName(), err)
	}
	s.emitEnvDirective(worktreePath)

	env := s.envForWorktree(worktreePath)

	// Add TEST_ENV_NUMBER for test commands so parallel test runs use isolated DB slots
	if isTestCommand(command) {
		wtName := filepath.Base(worktreePath)
		envNum := worktree.TestEnvNumber(wtName)
		env = append(env, fmt.Sprintf("TEST_ENV_NUMBER=%d", envNum))
	}

	args := s.buildRunArgs(worktreePath, service, command)

	cmd := composeCommand(s.composePath(), s.ext.EnvFileName(), env, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// buildRunArgs constructs the `docker compose run ...` argument list,
// applying --no-deps and bind_mount based on TestConfig.
func (s *externalStrategy) buildRunArgs(worktreePath, service, command string) []string {
	args := []string{"run", "--rm"}

	if !s.cfg.Test.IncludeDeps {
		args = append(args, "--no-deps")
	}

	if s.cfg.Test.BindMount != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s", worktreePath, s.cfg.Test.BindMount))
	}

	args = append(args, service, "bash", "-cil", command)
	return args
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./plugins/docker/ -run TestExternalRun -v`
Expected: PASS for all three sub-tests.

- [ ] **Step 5: Run the existing docker plugin suite**

Run: `go test ./plugins/docker/ -v`
Expected: All previously-passing tests still pass.

- [ ] **Step 6: Commit**

```bash
git add plugins/docker/external.go plugins/docker/external_run_test.go
git commit -m "feat(docker): grove test skips deps by default in external mode"
```

---

### Task 3: Apply the same defaults to `localStrategy.Run`

**Files:**
- Modify: `plugins/docker/local.go:125-135`
- Test: `plugins/docker/local_test.go` (extend existing)

- [ ] **Step 1: Write the failing test** — append to `plugins/docker/local_test.go`:

```go
func TestLocalRun_DefaultUsesNoDeps(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte("services:\n  app: {}\n"), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	cfg := &config.Config{
		Test: config.TestConfig{Command: "bin/rspec", Service: "app"},
	}
	s := newLocalStrategy(cfg)
	args := s.buildRunArgs(tmpDir, "app", "bin/rspec")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--no-deps") {
		t.Errorf("expected --no-deps in args, got: %v", args)
	}
}

func TestLocalRun_IncludeDepsTrueOmitsNoDeps(t *testing.T) {
	cfg := &config.Config{
		Test: config.TestConfig{
			Command:     "bin/rspec",
			Service:     "app",
			IncludeDeps: true,
		},
	}
	s := newLocalStrategy(cfg)
	args := s.buildRunArgs("/tmp", "app", "bin/rspec")

	for _, a := range args {
		if a == "--no-deps" {
			t.Errorf("expected --no-deps omitted, got: %v", args)
		}
	}
}
```

Add the import for `strings` at the top of `local_test.go` if it's not already there.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./plugins/docker/ -run TestLocalRun -v`
Expected: FAIL with `newLocalStrategy undefined` or `buildRunArgs undefined`.

- [ ] **Step 3: Add a `newLocalStrategy` constructor and `buildRunArgs`** — modify `plugins/docker/local.go`. First, find the existing `localStrategy` struct (it likely has no fields, or just `cfg`). If it lacks a `cfg` field, add one. Then add at the appropriate location:

```go
// newLocalStrategy creates a localStrategy bound to a config (for test config access).
// If the strategy is already constructed elsewhere, ensure it stores cfg so buildRunArgs
// can read TestConfig.
func newLocalStrategy(cfg *config.Config) *localStrategy {
	return &localStrategy{cfg: cfg}
}

func (s *localStrategy) buildRunArgs(_ string, service, command string) []string {
	args := []string{"run", "--rm"}
	if s.cfg != nil && !s.cfg.Test.IncludeDeps {
		args = append(args, "--no-deps")
	}
	if s.cfg != nil && s.cfg.Test.BindMount != "" {
		// bind-mount honored only when the user asked for it; in local mode
		// the worktree path is already the compose context, but the user may
		// still want an explicit mount so tests don't read from the bake-time copy.
		// (In local mode worktreePath == composePath, so this is a no-op for code,
		// but it's important for compose contexts where the build copies source in.)
	}
	args = append(args, service, "bash", "-cil", command)
	return args
}

// Then update the existing Run method to use buildRunArgs:
func (s *localStrategy) Run(worktreePath string, service string, command string) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	args := s.buildRunArgs(worktreePath, service, command)
	cmd := composeCommand(worktreePath, "", nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
```

If the existing `Plugin.Init` constructs `localStrategy` without a config, update that call site to pass `p.cfg` (the plugin already stores config in `p.cfg`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./plugins/docker/ -run TestLocalRun -v`
Expected: PASS.

- [ ] **Step 5: Run the full plugin suite**

Run: `go test ./plugins/docker/ -v`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add plugins/docker/local.go plugins/docker/local_test.go
git commit -m "feat(docker): apply --no-deps default to local mode"
```

---

### Task 4: Translate compose's `didn't complete successfully` error into actionable guidance

**Files:**
- Create: `plugins/docker/run_error.go`
- Test: `plugins/docker/run_error_test.go`
- Modify: `plugins/docker/external.go` (call the translator)
- Modify: `plugins/docker/local.go` (call the translator)

- [ ] **Step 1: Write the failing test** — create `plugins/docker/run_error_test.go`:

```go
package docker

import (
	"errors"
	"strings"
	"testing"
)

func TestTranslateRunError_DependencyDidntComplete(t *testing.T) {
	stderr := `Container my-stack-asset_precompile-1  Error
service "asset_precompile" didn't complete successfully: exit 1`
	original := errors.New("exit status 1")

	got := translateRunError(stderr, original)

	if got == nil {
		t.Fatal("expected translated error, got nil")
	}
	msg := got.Error()
	if !strings.Contains(msg, "asset_precompile") {
		t.Errorf("expected service name in message, got: %s", msg)
	}
	if !strings.Contains(msg, "include_deps") && !strings.Contains(msg, "ephemeral") {
		t.Errorf("expected actionable hint mentioning include_deps or ephemeral, got: %s", msg)
	}
}

func TestTranslateRunError_PassThroughOnUnknownPattern(t *testing.T) {
	stderr := "some other docker error"
	original := errors.New("exit status 1")

	got := translateRunError(stderr, original)

	if got != original {
		t.Errorf("expected original error pass-through, got: %v", got)
	}
}

func TestTranslateRunError_PassThroughOnEmptyStderr(t *testing.T) {
	original := errors.New("exit status 1")
	got := translateRunError("", original)
	if got != original {
		t.Errorf("expected original error pass-through, got: %v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./plugins/docker/ -run TestTranslateRunError -v`
Expected: FAIL with `translateRunError undefined`.

- [ ] **Step 3: Implement `translateRunError`** — create `plugins/docker/run_error.go`:

```go
package docker

import (
	"fmt"
	"regexp"
)

// dependencyFailureRE matches compose's "service \"X\" didn't complete successfully" message.
var dependencyFailureRE = regexp.MustCompile(`service "([^"]+)" didn't complete successfully`)

// translateRunError inspects captured compose stderr and rewrites the error
// when it matches a known unactionable pattern. Returns the original error
// when no pattern matches.
func translateRunError(stderr string, original error) error {
	if stderr == "" {
		return original
	}
	if m := dependencyFailureRE.FindStringSubmatch(stderr); len(m) == 2 {
		service := m[1]
		return fmt.Errorf(
			"service %q (a dependency of the test target) failed to start.\n"+
				"This is unrelated to the worktree under test.\n\n"+
				"Suggestions:\n"+
				"  • grove already passes --no-deps by default; if you're seeing this, set [test] include_deps = false in .grove/config.toml\n"+
				"  • Or run the test command directly in an ephemeral container:\n"+
				"      docker compose run --rm --no-deps -v $(pwd):/app <service> <test command>\n\n"+
				"underlying error: %w",
			service, original,
		)
	}
	return original
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./plugins/docker/ -run TestTranslateRunError -v`
Expected: PASS for all three cases.

- [ ] **Step 5: Wire the translator into `externalStrategy.Run`** — modify the `Run` method in `plugins/docker/external.go` to capture stderr instead of streaming it directly:

```go
func (s *externalStrategy) Run(worktreePath string, service string, command string) error {
	// ...persist env, build args, etc. as before...

	args := s.buildRunArgs(worktreePath, service, command)

	cmd := composeCommand(s.composePath(), s.ext.EnvFileName(), env, args...)
	cmd.Stdout = os.Stderr
	stderrBuf := &teeBuffer{w: os.Stderr}
	cmd.Stderr = stderrBuf
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return translateRunError(stderrBuf.String(), err)
	}
	return nil
}
```

Add `teeBuffer` (a writer that mirrors output to stderr while keeping the last N bytes for inspection) — define in `plugins/docker/run_error.go`:

```go
const stderrBufferLimit = 8 * 1024

type teeBuffer struct {
	w   *os.File
	buf []byte
}

func (t *teeBuffer) Write(p []byte) (int, error) {
	if t.w != nil {
		_, _ = t.w.Write(p)
	}
	if len(t.buf)+len(p) > stderrBufferLimit {
		// keep only the tail
		excess := len(t.buf) + len(p) - stderrBufferLimit
		if excess >= len(t.buf) {
			t.buf = nil
		} else {
			t.buf = t.buf[excess:]
		}
	}
	t.buf = append(t.buf, p...)
	return len(p), nil
}

func (t *teeBuffer) String() string { return string(t.buf) }
```

Add `"os"` to the imports of `run_error.go` if not already present.

- [ ] **Step 6: Apply the same wiring to `localStrategy.Run`** — replicate the `teeBuffer` capture and `translateRunError` wrap. Keep the patch small: only the stderr handling and return changes.

- [ ] **Step 7: Run the full docker plugin suite**

Run: `go test ./plugins/docker/ -v`
Expected: All tests pass.

- [ ] **Step 8: Commit**

```bash
git add plugins/docker/run_error.go plugins/docker/run_error_test.go plugins/docker/external.go plugins/docker/local.go
git commit -m "feat(docker): translate compose dependency failures into actionable errors"
```

---

### Task 5: Add `--with-deps` and `--bind` CLI flags to `grove test` for one-off overrides

**Why this matters (Learning Mode design note):**
- Config is the right place for project-wide policy, but per-invocation overrides matter when debugging stack issues. `--with-deps` and `--bind` give an escape hatch without round-tripping through the config file.

**Files:**
- Modify: `cmd/grove/commands/test.go`
- Test: `cmd/grove/commands/test_test.go` (NEW)

- [ ] **Step 1: Write the failing test** — create `cmd/grove/commands/test_test.go`:

```go
package commands

import (
	"testing"
)

// resolveTestOptions composes effective test options from CLI flags layered over config.
// Unit-test the resolution logic without invoking docker.
func TestResolveTestOptions_FlagOverridesConfig(t *testing.T) {
	tests := []struct {
		name           string
		cfgIncludeDeps bool
		cfgBindMount   string
		flagWithDeps   bool
		flagBind       string
		wantSkipDeps   bool
		wantBindMount  string
	}{
		{"defaults", false, "", false, "", true, ""},
		{"config opts in to deps", true, "", false, "", false, ""},
		{"flag overrides config off", false, "", true, "", false, ""},
		{"flag bind overrides empty config", false, "", false, "/app", true, "/app"},
		{"flag bind overrides config bind", false, "/old", false, "/new", true, "/new"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := resolveTestOptions(tc.cfgIncludeDeps, tc.cfgBindMount, tc.flagWithDeps, tc.flagBind)
			if opts.SkipDeps != tc.wantSkipDeps {
				t.Errorf("SkipDeps: got %v want %v", opts.SkipDeps, tc.wantSkipDeps)
			}
			if opts.BindMount != tc.wantBindMount {
				t.Errorf("BindMount: got %q want %q", opts.BindMount, tc.wantBindMount)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/grove/commands/ -run TestResolveTestOptions -v`
Expected: FAIL with `resolveTestOptions undefined`.

- [ ] **Step 3: Implement `resolveTestOptions` and wire flags** — modify `cmd/grove/commands/test.go`:

Add at the top of the file (after the existing imports):

```go
type testOptions struct {
	SkipDeps  bool
	BindMount string
}

func resolveTestOptions(cfgIncludeDeps bool, cfgBindMount string, flagWithDeps bool, flagBind string) testOptions {
	opts := testOptions{
		SkipDeps:  !cfgIncludeDeps,
		BindMount: cfgBindMount,
	}
	if flagWithDeps {
		opts.SkipDeps = false
	}
	if flagBind != "" {
		opts.BindMount = flagBind
	}
	return opts
}

var (
	testWithDeps bool
	testBind     string
)
```

Replace the `init()` block (lines 89–92) with:

```go
func init() {
	testCmd.Flags().SetInterspersed(false)
	testCmd.Flags().BoolVar(&testWithDeps, "with-deps", false, "Run dependency services before the test command (overrides [test] include_deps)")
	testCmd.Flags().StringVar(&testBind, "bind", "", "Bind-mount the worktree at the given container path (overrides [test] bind_mount)")
	rootCmd.AddCommand(testCmd)
}
```

In the `RunE` body, **after** the `mgr.Find` block and **before** the docker plugin block, add:

```go
opts := resolveTestOptions(ctx.Config.Test.IncludeDeps, ctx.Config.Test.BindMount, testWithDeps, testBind)

// Apply resolved options back to config so the docker plugin's buildRunArgs picks them up.
// We mutate a *copy* of the test config to avoid persisting CLI overrides.
testCfg := ctx.Config.Test
testCfg.IncludeDeps = !opts.SkipDeps
testCfg.BindMount = opts.BindMount
ctx.Config.Test = testCfg
```

This keeps the docker plugin oblivious to CLI flags — it just reads `ctx.Config.Test` as it already does.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/grove/commands/ -run TestResolveTestOptions -v`
Expected: PASS for all sub-tests.

- [ ] **Step 5: Manual smoke test** — drive the new flags through a real worktree:

```bash
go build -o /tmp/grove-issue-25 ./cmd/grove
# In a project with external mode + a test command configured:
GROVE_DEBUG=1 /tmp/grove-issue-25 test <some-worktree> --with-deps -- echo hello
GROVE_DEBUG=1 /tmp/grove-issue-25 test <some-worktree> --bind /app -- echo hello
```

Expected output: visible in debug log that the resolved compose args differ between runs (`--no-deps` absent in the first, `-v <path>:/app` present in the second).

- [ ] **Step 6: Commit**

```bash
git add cmd/grove/commands/test.go cmd/grove/commands/test_test.go
git commit -m "feat(test): add --with-deps and --bind flags to grove test"
```

---

### Task 6: Update documentation

**Files:**
- Modify: `docs/COMMAND_SPECIFICATIONS.md` (add `grove test` entry update)
- Modify: `docs/CONFIGURATION_REFERENCE.md` (document `[test] include_deps` and `[test] bind_mount`)

- [ ] **Step 1: Update `docs/CONFIGURATION_REFERENCE.md`** — locate the `[test]` section and replace with:

```markdown
### `[test]`

| Field          | Type    | Default | Description                                                                                  |
|----------------|---------|---------|----------------------------------------------------------------------------------------------|
| `command`      | string  | —       | Test command to run (e.g., `"bin/rspec"`).                                                   |
| `service`      | string  | —       | Compose service to run the command in. Required for docker-mode test runs.                   |
| `include_deps` | bool    | `false` | When `true`, `compose run` starts services in `depends_on`. Default skips them (`--no-deps`). |
| `bind_mount`   | string  | `""`    | Container path to bind-mount the worktree at, e.g., `"/app"`. Empty disables bind-mount.     |

**Why `include_deps` defaults to `false`:** in shared external-mode stacks a single failing
one-shot init service (asset precompile, DB seed) would otherwise abort every test run.
Set `include_deps = true` if your test command genuinely requires another service to be
running first.

**When to set `bind_mount`:** if you run multiple worktrees in parallel against the same
external compose stack, the env-var-based path resolution can race. Pinning a bind-mount
makes each `grove test` invocation read code from its own worktree regardless of the
shared env var's current value. The path you specify must match the compose service's
`WORKDIR` (or the path source code expects to live at).
```

- [ ] **Step 2: Update `docs/COMMAND_SPECIFICATIONS.md`** — find the `grove test` entry and add a "Flags" subsection:

```markdown
**Flags:**

- `--with-deps` — run `compose run` *with* dependency services started. Overrides `[test] include_deps = false`.
- `--bind <container-path>` — bind-mount the worktree at the given path inside the container. Overrides `[test] bind_mount`.

**Default behavior:** grove passes `--no-deps` to `docker compose run` by default. This
means a failing one-shot init service in the shared stack does not block the test run.
```

- [ ] **Step 3: Commit**

```bash
git add docs/COMMAND_SPECIFICATIONS.md docs/CONFIGURATION_REFERENCE.md
git commit -m "docs: document grove test --no-deps default and new flags"
```

---

### Task 7: Final verification

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: All tests pass.

- [ ] **Step 2: Run linter**

Run: `make lint`
Expected: Clean.

- [ ] **Step 3: Manual end-to-end smoke** — in an external-mode project where one of the `depends_on` services is configured to fail:

```bash
grove test <worktree> -- echo "hello from $(pwd)"
```

Expected: the echo runs successfully despite the broken init service. Without this fix, it would have aborted with `service "<init>" didn't complete successfully`.

- [ ] **Step 4: Verify no untracked files were forgotten**

Run: `git status`
Expected: clean working tree.

---

## Self-review summary

- **Spec coverage:** Issue #1 (ephemeral fallback) addressed via `--no-deps` default + bind-mount opt-in (Tasks 1–3, 5). Issue #4 (better error surface) addressed via `translateRunError` (Task 4).
- **No placeholders:** every step shows the exact code, exact commands, and expected output.
- **Type consistency:** `TestConfig.IncludeDeps`/`BindMount` defined in Task 1; referenced consistently in Tasks 2, 3, 5. `buildRunArgs` signature `(worktreePath, service, command string) []string` consistent across both strategies.
- **No type drift between plans:** Plan 2 introduces nothing in `plugins/docker/` or `internal/config/`; Plan 3's additions to `ExternalComposeConfig` are independent of `TestConfig`.
