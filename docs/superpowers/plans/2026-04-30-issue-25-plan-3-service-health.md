# Issue 25 — Plan 3: Service-Health Discrimination

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Distinguish one-shot init services (asset precompile, DB seed) from long-running services in the external-mode stack, so a finished-or-failed init service doesn't make the stack appear "unhealthy" or block operations like `grove up`.

**Architecture:** Add `non_blocking_services []string` to `ExternalComposeConfig` to let users mark services that may legitimately exit. Replace the current crude `composeRunningCount` heuristic (which uses `docker compose ps -q` and only sees running containers) with a structured probe `serviceHealth()` that parses `docker compose ps --all --format json` and classifies services as `Running`, `Exited`, or `Failed`. Status reporting and `Up()` consult the classification, treating exited/failed `non_blocking_services` as "not a problem." The probe is reused; no new failure paths are introduced.

**Tech Stack:** Go 1.24, `docker compose ps --all --format json`, `encoding/json`, the existing `composeCommand` helper.

**Read this first:** Plan 1 (`grove test --no-deps` default) eliminates the *acute* user pain. Plan 3 polishes the broader stack-health UX. **Validate Plan 1's outcome with the user before starting Plan 3** — if Plan 1 sufficiently solves the problem, Plan 3 may be deferred indefinitely.

**Compatibility with Plans 1 & 2:**
- Plan 1 adds `IncludeDeps` and `BindMount` to `TestConfig`. This plan adds `NonBlockingServices` to `ExternalComposeConfig` (different struct). No collisions.
- Plan 2 adds `BootstrapWorktree` and `grove adopt`. No overlap.
- This plan modifies `plugins/docker/status.go` — Plan 1 doesn't, Plan 2 doesn't.

---

### Task 1: Add `NonBlockingServices` to `ExternalComposeConfig`

**Files:**
- Modify: `internal/config/config.go:96-105`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test** — append to `internal/config/config_test.go`:

```go
func TestLoadConfig_ExternalNonBlockingServices(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.toml")
	body := `
[plugins.docker]
mode = "external"

[plugins.docker.external]
path = "/tmp/compose"
env_var = "APP_DIR"
services = ["app", "asset_precompile", "db_seed"]
non_blocking_services = ["asset_precompile", "db_seed"]
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadConfigFromPath(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ext := cfg.Plugins.Docker.External
	if ext == nil {
		t.Fatal("External config nil")
	}
	want := []string{"asset_precompile", "db_seed"}
	if len(ext.NonBlockingServices) != len(want) {
		t.Fatalf("NonBlockingServices length: got %d want %d", len(ext.NonBlockingServices), len(want))
	}
	for i, w := range want {
		if ext.NonBlockingServices[i] != w {
			t.Errorf("NonBlockingServices[%d]: got %q want %q", i, ext.NonBlockingServices[i], w)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoadConfig_ExternalNonBlockingServices -v`
Expected: FAIL with `unknown field "non_blocking_services"` (TOML rejects unknown keys) or `NonBlockingServices undefined`.

- [ ] **Step 3: Add the field** — modify `internal/config/config.go` (lines 94-105). Replace the `ExternalComposeConfig` struct with:

```go
// ExternalComposeConfig configures external Docker Compose mode where services
// are defined in a shared compose setup outside the project directory.
type ExternalComposeConfig struct {
	Path                string            `toml:"path"`                  // Path to external compose directory
	EnvVar              string            `toml:"env_var"`               // Environment variable name (e.g., "APP_DIR")
	EnvFile             string            `toml:"env_file"`              // File to write env vars to (default: ".env")
	Services            []string          `toml:"services"`              // Service names to manage
	NonBlockingServices []string          `toml:"non_blocking_services"` // Services allowed to exit (one-shot init, etc.) without marking stack unhealthy
	CopyFiles           []string          `toml:"copy_files"`            // Files to copy from main on worktree create
	SymlinkFiles        []string          `toml:"symlink_files"`         // Files to symlink from main on create
	SymlinkDirs         []string          `toml:"symlink_dirs"`          // Directories to symlink from main on create
	Agent               *AgentStackConfig `toml:"agent"`                 // Optional agent stack configuration
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestLoadConfig_ExternalNonBlockingServices -v`
Expected: PASS.

- [ ] **Step 5: Run the full config suite**

Run: `go test ./internal/config/ -v`
Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add external.non_blocking_services"
```

---

### Task 2: Add a `ServiceHealth` probe parsing `docker compose ps --all --format json`

**Why this matters (Learning Mode design note):**
- `docker compose ps -q` (current approach) only returns running containers — it can't tell you whether a service "exited successfully" or "exited with error." `--all --format json` gives per-service `State`, `ExitCode`, and `Health` fields.
- We classify with an explicit enum (`ServiceRunning`, `ServiceExitedClean`, `ServiceExitedError`, `ServiceUnknown`) rather than passing raw strings around. The classification is the contract; the JSON shape is an implementation detail.

**Files:**
- Create: `plugins/docker/service_health.go`
- Test: `plugins/docker/service_health_test.go`

- [ ] **Step 1: Write the failing test** — create `plugins/docker/service_health_test.go`:

```go
package docker

import (
	"testing"
)

func TestParseServiceHealth_Running(t *testing.T) {
	jsonOut := `[{"Service":"app","State":"running","ExitCode":0,"Health":""}]`
	got, err := parseServiceHealth([]byte(jsonOut))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].Name != "app" || got[0].Status != ServiceRunning {
		t.Errorf("got %#v", got)
	}
}

func TestParseServiceHealth_ExitedSuccess(t *testing.T) {
	jsonOut := `[{"Service":"asset_precompile","State":"exited","ExitCode":0,"Health":""}]`
	got, err := parseServiceHealth([]byte(jsonOut))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got[0].Status != ServiceExitedClean {
		t.Errorf("expected ServiceExitedClean, got %v", got[0].Status)
	}
}

func TestParseServiceHealth_ExitedFailed(t *testing.T) {
	jsonOut := `[{"Service":"db_seed","State":"exited","ExitCode":1,"Health":""}]`
	got, err := parseServiceHealth([]byte(jsonOut))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got[0].Status != ServiceExitedError {
		t.Errorf("expected ServiceExitedError, got %v", got[0].Status)
	}
}

func TestParseServiceHealth_NDJSON(t *testing.T) {
	// Some compose versions emit one JSON object per line rather than an array
	jsonOut := `{"Service":"app","State":"running","ExitCode":0}
{"Service":"db","State":"running","ExitCode":0}`
	got, err := parseServiceHealth([]byte(jsonOut))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 services, got %d: %#v", len(got), got)
	}
}

func TestClassifyHealth_NonBlockingExitedTreatedAsOK(t *testing.T) {
	statuses := []ServiceStatus{
		{Name: "app", Status: ServiceRunning},
		{Name: "asset_precompile", Status: ServiceExitedError},
	}
	healthy, blockers := classifyHealth(statuses, []string{"asset_precompile"})
	if !healthy {
		t.Errorf("expected healthy=true (asset_precompile is non-blocking), got false. blockers: %v", blockers)
	}
	if len(blockers) != 0 {
		t.Errorf("expected no blockers, got %v", blockers)
	}
}

func TestClassifyHealth_BlockingFailedReportsBlocker(t *testing.T) {
	statuses := []ServiceStatus{
		{Name: "app", Status: ServiceExitedError},
		{Name: "asset_precompile", Status: ServiceExitedError},
	}
	healthy, blockers := classifyHealth(statuses, []string{"asset_precompile"})
	if healthy {
		t.Errorf("expected unhealthy (app is blocking), got healthy")
	}
	if len(blockers) != 1 || blockers[0] != "app" {
		t.Errorf("expected blockers=[app], got %v", blockers)
	}
}

func TestClassifyHealth_NonBlockingRunningIsNormal(t *testing.T) {
	statuses := []ServiceStatus{
		{Name: "app", Status: ServiceRunning},
	}
	healthy, _ := classifyHealth(statuses, nil)
	if !healthy {
		t.Errorf("expected healthy")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./plugins/docker/ -run TestParseServiceHealth -v && go test ./plugins/docker/ -run TestClassifyHealth -v`
Expected: FAIL with `parseServiceHealth undefined` and `classifyHealth undefined`.

- [ ] **Step 3: Implement parser and classifier** — create `plugins/docker/service_health.go`:

```go
package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ServiceHealthStatus enumerates the possible classified states for a compose service.
type ServiceHealthStatus int

const (
	ServiceUnknown      ServiceHealthStatus = iota
	ServiceRunning                          // container is up
	ServiceExitedClean                      // container exited with code 0 (one-shot completed)
	ServiceExitedError                      // container exited with non-zero code
)

// ServiceStatus is the classified state of a single compose service.
type ServiceStatus struct {
	Name   string
	Status ServiceHealthStatus
}

// composePsEntry is the subset of `docker compose ps --format json` we care about.
type composePsEntry struct {
	Service  string `json:"Service"`
	State    string `json:"State"`
	ExitCode int    `json:"ExitCode"`
	Health   string `json:"Health"`
}

// parseServiceHealth parses compose's `--format json` output into ServiceStatus.
// Handles both array form (some versions emit `[...]`) and NDJSON (one object per line).
func parseServiceHealth(out []byte) ([]ServiceStatus, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var entries []composePsEntry
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			return nil, fmt.Errorf("parse compose ps array: %w", err)
		}
	} else {
		// NDJSON: one object per line
		for _, line := range strings.Split(string(trimmed), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var e composePsEntry
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				return nil, fmt.Errorf("parse compose ps line %q: %w", line, err)
			}
			entries = append(entries, e)
		}
	}

	result := make([]ServiceStatus, 0, len(entries))
	for _, e := range entries {
		result = append(result, ServiceStatus{
			Name:   e.Service,
			Status: classifyEntry(e),
		})
	}
	return result, nil
}

func classifyEntry(e composePsEntry) ServiceHealthStatus {
	switch strings.ToLower(e.State) {
	case "running":
		return ServiceRunning
	case "exited":
		if e.ExitCode == 0 {
			return ServiceExitedClean
		}
		return ServiceExitedError
	}
	return ServiceUnknown
}

// classifyHealth returns (stackHealthy, blockingServiceNames). A stack is healthy
// when every service that is NOT in nonBlocking is in {Running, ExitedClean}.
// Services in nonBlocking are ignored for the healthy verdict but still tracked
// in returned status if relevant elsewhere.
func classifyHealth(statuses []ServiceStatus, nonBlocking []string) (bool, []string) {
	skip := make(map[string]struct{}, len(nonBlocking))
	for _, n := range nonBlocking {
		skip[n] = struct{}{}
	}

	var blockers []string
	for _, s := range statuses {
		if _, isNonBlocking := skip[s.Name]; isNonBlocking {
			continue
		}
		if s.Status == ServiceExitedError || s.Status == ServiceUnknown {
			blockers = append(blockers, s.Name)
		}
	}
	return len(blockers) == 0, blockers
}

// probeServiceHealth runs `docker compose ps --all --format json` and returns parsed statuses.
// composePath is the directory containing the compose file; envFile is the env file name
// (e.g., ".env" or ".env.local") to pass via --env-file.
func probeServiceHealth(composePath, envFile string, env []string) ([]ServiceStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	args := []string{"compose"}
	if envFile != "" && envFile != ".env" {
		args = append(args, "--env-file", envFile)
	}
	args = append(args, "ps", "--all", "--format", "json")

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = composePath
	if len(env) > 0 {
		// Prepend OS env so docker / PATH are available
		cmd.Env = append(cmd.Env, env...)
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("compose ps: %w", err)
	}
	return parseServiceHealth(out)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./plugins/docker/ -run TestParseServiceHealth && go test ./plugins/docker/ -run TestClassifyHealth -v`
Expected: PASS for all sub-tests.

- [ ] **Step 5: Commit**

```bash
git add plugins/docker/service_health.go plugins/docker/service_health_test.go
git commit -m "feat(docker): add service-health probe and non-blocking classification"
```

---

### Task 3: Use `classifyHealth` in `externalStatuses`

**Files:**
- Modify: `plugins/docker/status.go:67-97`
- Test: `plugins/docker/status_test.go` (extend)

- [ ] **Step 1: Write the failing test** — append to `plugins/docker/status_test.go`:

```go
func TestExternalStatuses_NonBlockingExitedDoesNotDowngrade(t *testing.T) {
	// Simulate: app running, asset_precompile exited(0), and asset_precompile is non-blocking.
	// classifyExternalStatusFromHealth should return StatusActive, not StatusWarning.
	statuses := []ServiceStatus{
		{Name: "app", Status: ServiceRunning},
		{Name: "asset_precompile", Status: ServiceExitedClean},
	}
	level, _ := classifyExternalStatusFromHealth(statuses, []string{"asset_precompile"}, true /* matchesActive */)
	if level != "active" {
		t.Errorf("expected active, got %s", level)
	}
}

func TestExternalStatuses_BlockingFailedDowngrades(t *testing.T) {
	statuses := []ServiceStatus{
		{Name: "app", Status: ServiceExitedError},
	}
	level, detail := classifyExternalStatusFromHealth(statuses, nil, true)
	if level != "warning" {
		t.Errorf("expected warning, got %s", level)
	}
	if !strings.Contains(detail, "app") {
		t.Errorf("expected detail to name 'app', got %q", detail)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./plugins/docker/ -run TestExternalStatuses -v`
Expected: FAIL with `classifyExternalStatusFromHealth undefined`.

- [ ] **Step 3: Implement the helper and rewire `externalStatuses`** — at the bottom of `plugins/docker/status.go` add:

```go
// classifyExternalStatusFromHealth maps a service-health snapshot to the
// status entry level + detail surfaced via WorktreeStatuses.
//
// Returns ("active", detail) when all non-skipped services are healthy,
// ("warning", detail) when blockers exist, ("info", detail) when nothing
// is running but no failures either.
func classifyExternalStatusFromHealth(statuses []ServiceStatus, nonBlocking []string, matchesActive bool) (string, string) {
	if !matchesActive {
		return "info", "Configured but not the active worktree"
	}
	if len(statuses) == 0 {
		return "info", "Configured as active worktree but no services reported"
	}

	healthy, blockers := classifyHealth(statuses, nonBlocking)
	runningCount := 0
	for _, s := range statuses {
		if s.Status == ServiceRunning {
			runningCount++
		}
	}

	if healthy {
		if runningCount == 0 {
			return "info", "Configured as active worktree but no services running"
		}
		return "active", fmt.Sprintf("%d service(s) running, pointed to this worktree", runningCount)
	}
	return "warning", fmt.Sprintf("blocking service(s) failed: %s", strings.Join(blockers, ", "))
}
```

Then **modify** the existing `externalStatuses` (lines 67-97 of `status.go`) to use the probe and helper. Replace its body with:

```go
func externalStatuses(s *externalStrategy, paths []string) map[string]plugins.StatusEntry {
	result := make(map[string]plugins.StatusEntry)

	composePath := s.composePath()
	activeWorktree := readEnvVar(composePath, s.ext.EnvVar)

	// Probe service health once for the whole stack.
	statuses, _ := probeServiceHealth(composePath, s.ext.EnvFileName(), nil)

	for _, path := range paths {
		matches := pathMatchesEnv(path, activeWorktree, composePath)
		if !matches {
			continue
		}

		level, detail := classifyExternalStatusFromHealth(statuses, s.ext.NonBlockingServices, true)

		var statusLevel plugins.StatusLevel
		var short string
		switch level {
		case "active":
			statusLevel = plugins.StatusActive
			short = "up"
		case "warning":
			statusLevel = plugins.StatusWarning
			short = "degraded"
		default:
			statusLevel = plugins.StatusInfo
			short = "configured"
		}

		result[path] = plugins.StatusEntry{
			ProviderName: "docker",
			Level:        statusLevel,
			Short:        short,
			Detail:       detail,
		}
	}

	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./plugins/docker/ -run TestExternalStatuses -v`
Expected: PASS.

- [ ] **Step 5: Run the full plugin suite**

Run: `go test ./plugins/docker/ -v`
Expected: all tests pass. Some existing tests may need adjustment if they asserted on the old hard-coded "Configured as active worktree but services not running" string — update those expectations to match the new precise messages.

- [ ] **Step 6: Commit**

```bash
git add plugins/docker/status.go plugins/docker/status_test.go
git commit -m "feat(docker): classify status using service health + non-blocking list"
```

---

### Task 4: Make `externalStrategy.Up()` ignore non-blocking service exit codes

**Why this matters (Learning Mode design note):**
- `compose up` returns non-zero if any service in the up set fails. Today, that means a flaky one-shot init service makes `grove new` / `grove up` exit non-zero even though the long-running services are fine. This task post-checks service health before deciding whether to surface the failure.

**Files:**
- Modify: `plugins/docker/external.go:97-114` (the `Up` method)
- Test: `plugins/docker/external_up_test.go` (NEW)

- [ ] **Step 1: Write the failing test** — create `plugins/docker/external_up_test.go`:

```go
package docker

import (
	"errors"
	"testing"
)

func TestUpResult_FailureBecomesSuccessIfBlockersHealthy(t *testing.T) {
	statuses := []ServiceStatus{
		{Name: "app", Status: ServiceRunning},
		{Name: "asset_precompile", Status: ServiceExitedError},
	}
	cmdErr := errors.New("exit status 1")

	got := finalizeUpResult(cmdErr, statuses, []string{"asset_precompile"})

	if got != nil {
		t.Errorf("expected nil error (only non-blocking service failed), got: %v", got)
	}
}

func TestUpResult_FailurePreservedIfBlockerFailed(t *testing.T) {
	statuses := []ServiceStatus{
		{Name: "app", Status: ServiceExitedError},
	}
	cmdErr := errors.New("exit status 1")

	got := finalizeUpResult(cmdErr, statuses, nil)
	if got == nil {
		t.Errorf("expected non-nil error (app is blocking)")
	}
}

func TestUpResult_NilOnNoCmdError(t *testing.T) {
	statuses := []ServiceStatus{{Name: "app", Status: ServiceRunning}}
	if got := finalizeUpResult(nil, statuses, nil); got != nil {
		t.Errorf("expected nil, got: %v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./plugins/docker/ -run TestUpResult -v`
Expected: FAIL with `finalizeUpResult undefined`.

- [ ] **Step 3: Implement `finalizeUpResult` and wire into `Up`** — add to `plugins/docker/service_health.go`:

```go
// finalizeUpResult inspects post-up service health and decides whether the
// reported cmdErr (from `compose up`) should propagate. If only non-blocking
// services failed, returns nil. Otherwise wraps cmdErr with blocker context.
func finalizeUpResult(cmdErr error, statuses []ServiceStatus, nonBlocking []string) error {
	if cmdErr == nil {
		return nil
	}
	healthy, blockers := classifyHealth(statuses, nonBlocking)
	if healthy {
		return nil
	}
	return fmt.Errorf("up failed; blocking service(s) not healthy: %s (underlying: %w)", strings.Join(blockers, ", "), cmdErr)
}
```

Then modify `externalStrategy.Up` (lines 97-114 of `plugins/docker/external.go`):

```go
func (s *externalStrategy) Up(worktreePath string, detach bool) error {
	if err := s.persistEnvVar(worktreePath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to persist %s to %s: %v\n", s.ext.EnvVar, s.ext.EnvFileName(), err)
	}
	s.emitEnvDirective(worktreePath)

	args := []string{"up"}
	if detach {
		args = append(args, "-d")
	}
	args = append(args, s.ext.Services...)

	cmd := composeCommand(s.composePath(), s.ext.EnvFileName(), s.envForWorktree(worktreePath), args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmdErr := cmd.Run()

	// Even if up returned non-zero, check whether only non-blocking services failed.
	statuses, _ := probeServiceHealth(s.composePath(), s.ext.EnvFileName(), s.envForWorktree(worktreePath))
	return finalizeUpResult(cmdErr, statuses, s.ext.NonBlockingServices)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./plugins/docker/ -run TestUpResult -v`
Expected: PASS for all three sub-tests.

- [ ] **Step 5: Run the full plugin suite**

Run: `go test ./plugins/docker/ -v`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add plugins/docker/service_health.go plugins/docker/external.go plugins/docker/external_up_test.go
git commit -m "feat(docker): grove up tolerates non-blocking service failures"
```

---

### Task 5: Surface non-blocking-service info in `grove doctor`

**Files:**
- Modify: `cmd/grove/commands/doctor.go` (extend `runExternalModeChecks`)

- [ ] **Step 1: Add a non-blocking-services info line** — locate `runExternalModeChecks` in `doctor.go` (around line 244). Inside the `if ext.Agent == nil || ...` block where existing agent info is reported, add (before that block) a new info line:

```go
if len(ext.NonBlockingServices) > 0 {
    runInfo(w, "Non-blocking services", strings.Join(ext.NonBlockingServices, ", "))
}
```

- [ ] **Step 2: Manual smoke test**

```bash
go run ./cmd/grove doctor
```

Expected: when `non_blocking_services` is set in config, doctor prints them; otherwise the line is absent.

- [ ] **Step 3: Commit**

```bash
git add cmd/grove/commands/doctor.go
git commit -m "feat(doctor): show configured non-blocking services"
```

---

### Task 6: Document `non_blocking_services`

**Files:**
- Modify: `docs/CONFIGURATION_REFERENCE.md`

- [ ] **Step 1: Add to the `[plugins.docker.external]` section** — locate the table in `CONFIGURATION_REFERENCE.md` and append:

```markdown
| `non_blocking_services` | `[]string` | `[]` | Services allowed to exit (one-shot init: asset precompile, DB seed) without marking the stack unhealthy. Failures in these services do not block `grove up` or downgrade `grove ps` status. |
```

Add an explanatory paragraph below the table:

```markdown
**Why mark a service non-blocking:** in shared external-mode stacks, one-shot
init services frequently exit (cleanly or with errors) once their work is done.
Without this list, grove treats every exited service as a stack-health problem.
List services here when their lifecycle is "run once, stop"; long-running
services (web, db, redis, etc.) should NOT be in this list.
```

- [ ] **Step 2: Commit**

```bash
git add docs/CONFIGURATION_REFERENCE.md
git commit -m "docs: document non_blocking_services config option"
```

---

### Task 7: Final verification

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: all pass.

- [ ] **Step 2: Run linter**

Run: `make lint`
Expected: clean.

- [ ] **Step 3: End-to-end manual scenario** — in an external-mode project where one of the configured `services` is a known one-shot init that may exit non-zero:

```toml
# .grove/config.toml
[plugins.docker.external]
services = ["app", "asset_precompile"]
non_blocking_services = ["asset_precompile"]
```

```bash
# Force asset_precompile to fail (e.g., introduce a typo in a Rakefile in the worktree),
# then:
grove up <worktree>     # should succeed with a notice that asset_precompile failed
grove ps <worktree>     # should show "up" not "degraded"
grove doctor            # should list asset_precompile under "Non-blocking services"
```

- [ ] **Step 4: Verify clean tree**

Run: `git status`
Expected: clean.

---

## Self-review summary

- **Spec coverage:** Issue #2 (distinguish service health) addressed via `non_blocking_services` config (Task 1), `classifyHealth` (Task 2), status reporting (Task 3), `Up` tolerance (Task 4), doctor surface (Task 5).
- **No placeholders:** every step gives concrete code, exact commands, expected output.
- **Type consistency:** `ServiceStatus`, `ServiceHealthStatus`, `parseServiceHealth`, `classifyHealth`, `probeServiceHealth`, `finalizeUpResult`, `classifyExternalStatusFromHealth` are all introduced in this plan, used consistently across tasks.
- **No drift between plans:** `ExternalComposeConfig.NonBlockingServices` is additive — Plan 1 doesn't read or write it. Plan 2 doesn't touch any plugin code.
- **Defer trigger:** if Plan 1 fully resolves the user's pain (validate by re-running the original repro after Plan 1 ships), Plan 3 may stay unimplemented.
