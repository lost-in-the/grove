package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/plugins"
)

// statusTimeout is the maximum time allowed for docker compose ps calls.
const statusTimeout = 300 * time.Millisecond

// WorktreeStatuses implements plugins.StatusProvider.
func (p *Plugin) WorktreeStatuses(worktreePaths []string) map[string]plugins.StatusEntry {
	if p.strategy == nil {
		return nil
	}

	switch s := p.strategy.(type) {
	case *localStrategy:
		return localStatuses(s, worktreePaths)
	case *externalStrategy:
		return externalStatuses(s, worktreePaths)
	case *agentExternalStrategy:
		return agentStatuses(s, worktreePaths)
	default:
		return nil
	}
}

// localStatuses checks each worktree for a compose file and running containers.
// The per-worktree docker compose ps calls are independent, so they run
// concurrently — a serial loop would cost N x statusTimeout in the worst case,
// blowing the <500ms budget for grove ls as soon as 2+ worktrees have compose files.
func localStatuses(_ *localStrategy, paths []string) map[string]plugins.StatusEntry {
	result := make(map[string]plugins.StatusEntry)

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, path := range paths {
		if !hasDockerCompose(path) {
			continue
		}

		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			// Has compose file — at minimum "configured"
			entry := plugins.StatusEntry{
				ProviderName: "docker",
				Level:        plugins.StatusInfo,
				Short:        "compose",
				Detail:       "Compose file found",
			}

			count, err := localPsCount(path)
			switch {
			case err != nil:
				entry.Detail = "Compose file found; state unknown (docker did not answer)"
			case count > 0:
				entry.Level = plugins.StatusActive
				entry.Short = fmt.Sprintf("up (%d)", count)
				entry.Detail = fmt.Sprintf("%d container(s) running", count)
			}

			mu.Lock()
			result[path] = entry
			mu.Unlock()
		}(path)
	}
	wg.Wait()

	return result
}

// probeHealth is the docker probe used by the display paths; tests replace it.
var probeHealth = probeServiceHealth

// externalStatuses reads the .env to find which worktree the shared stack is
// configured for and reports its state. A worktree that owns an agent slot
// reports its agent stack instead.
func externalStatuses(s *externalStrategy, paths []string) map[string]plugins.StatusEntry {
	result := make(map[string]plugins.StatusEntry)

	composePath := s.composePath()
	activeWorktree := readEnvVar(composePath, s.ext.EnvFileName(), s.ext.EnvVar)

	var verdict stackVerdict
	probed := false

	for _, path := range paths {
		if slot := agentSlotFor(s.cfg, path); slot > 0 {
			result[path] = agentEntry(slot, s.ext.Agent)
			continue
		}
		if !pathMatchesEnv(path, activeWorktree, composePath) {
			continue
		}

		// Probe once for the whole stack. statusTimeout (not probeTimeout)
		// keeps grove ls within the <500ms budget.
		if !probed {
			statuses, err := probeHealth(composePath, s.ext.EnvFileName(), nil, statusTimeout)
			verdict = classifyStack(statuses, err, s.ext.Services, s.ext.NonBlockingServices)
			probed = true
		}

		result[path] = sharedStackEntry(verdict)
	}

	return result
}

// agentSlotFor returns the agent slot a worktree owns, or 0 when agent stacks
// are not enabled or the worktree has none.
func agentSlotFor(cfg *config.Config, path string) int {
	if cfg == nil {
		return 0
	}
	return FindWorktreeSlot(cfg, path)
}

func agentEntry(slot int, agent *config.AgentStackConfig) plugins.StatusEntry {
	detail := fmt.Sprintf("Stack #%d", slot)
	if agent != nil {
		if url := formatAgentURL(agent.URLPattern, slot); url != "" {
			detail += " at " + url
		}
	}
	return plugins.StatusEntry{
		ProviderName: "docker",
		Level:        plugins.StatusActive,
		Short:        fmt.Sprintf("#%d", slot),
		Detail:       detail,
	}
}

func sharedStackEntry(v stackVerdict) plugins.StatusEntry {
	entry := plugins.StatusEntry{ProviderName: "docker", Level: plugins.StatusInfo, Short: "configured"}
	switch v.State {
	case StackRunning:
		entry.Level = plugins.StatusActive
		entry.Short = "up"
		entry.Detail = fmt.Sprintf("%d service(s) running, the stack is on this worktree", v.Running)
	case StackDegraded:
		entry.Level = plugins.StatusWarning
		entry.Short = "degraded"
		entry.Detail = fmt.Sprintf("blocking service(s) failed: %s", strings.Join(v.Blockers, ", "))
	case StackUnknown:
		entry.Detail = "The stack is on this worktree; state unknown (docker did not answer)"
	default:
		entry.Detail = "The stack is on this worktree but no services are running"
	}
	return entry
}

// agentStatuses checks slot allocations (file read, no docker calls).
func agentStatuses(s *agentExternalStrategy, paths []string) map[string]plugins.StatusEntry {
	result := make(map[string]plugins.StatusEntry)

	activeSlots, err := s.slots.ListActive()
	if err != nil {
		return result
	}

	slotsByWorktree := make(map[string]int, len(activeSlots))
	for _, slot := range activeSlots {
		slotsByWorktree[slot.Worktree] = slot.Slot
	}

	for _, path := range paths {
		wtName := filepath.Base(path)
		slot, ok := slotsByWorktree[wtName]
		if !ok {
			continue
		}

		result[path] = agentEntry(slot, s.agent)
	}

	return result
}

// readEnvVar reads a specific variable from the env file in composePath.
// envFileName is the file name (e.g., ".env" or ".env.local"); if empty, ".env" is used.
func readEnvVar(composePath, envFileName, key string) string {
	if envFileName == "" {
		envFileName = ".env"
	}
	p := filepath.Join(composePath, envFileName)
	content, err := os.ReadFile(p)
	if err != nil {
		return ""
	}

	prefix := key + "="
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	return ""
}

// pathMatchesEnv checks whether a worktree path matches the env var value.
// The env value may be a relative path from composePath (e.g., "./myapp-feature-x")
// or an absolute path.
func pathMatchesEnv(worktreePath, envValue, composePath string) bool {
	if envValue == "" {
		return false
	}

	// Try absolute match
	if worktreePath == envValue {
		return true
	}

	// Resolve relative env value against composePath
	resolved := envValue
	if !filepath.IsAbs(envValue) {
		resolved = filepath.Join(composePath, envValue)
	}
	resolved = filepath.Clean(resolved)

	return filepath.Clean(worktreePath) == resolved
}

// StackState is the run state of the configured services.
type StackState int

const (
	// StackUnknown means docker did not answer; it is not the same as stopped.
	StackUnknown StackState = iota
	StackStopped
	StackRunning
	StackDegraded
)

// ServiceInfo describes the current Docker service state for a single worktree.
type ServiceInfo struct {
	// ConfiguredPath is the worktree the stack is configured for (what the env
	// file says, not what the container has mounted). Empty when the shared
	// stack is not configured for any worktree.
	ConfiguredPath string
	State          StackState
	Blockers       []string // configured services that failed (StackDegraded)
	Slot           int      // agent slot owned by the queried worktree, 0 if none
	MatchesCurrent bool     // whether ConfiguredPath is the queried worktree
	IsRunning      bool
	Shared         bool // reported from the shared external stack, not a local or agent stack
}

// stackVerdict is the run state judged from a service-health snapshot.
type stackVerdict struct {
	State    StackState
	Blockers []string
	Running  int
}

// classifyStack is the single source of the run-state verdict for grove ls and
// grove which. Only the configured services count (all services when none are
// configured), so shared infrastructure cannot make the stack look up and an
// unrelated failing container cannot make it look down. A probe error yields
// StackUnknown rather than StackStopped.
func classifyStack(statuses []ServiceStatus, probeErr error, services, nonBlocking []string) stackVerdict {
	if probeErr != nil {
		return stackVerdict{State: StackUnknown}
	}

	if len(services) > 0 {
		wanted := make(map[string]struct{}, len(services))
		for _, name := range services {
			wanted[name] = struct{}{}
		}
		filtered := make([]ServiceStatus, 0, len(statuses))
		for _, st := range statuses {
			if _, ok := wanted[st.Name]; ok {
				filtered = append(filtered, st)
			}
		}
		statuses = filtered
	}

	running := 0
	for _, st := range statuses {
		if st.Status == ServiceRunning {
			running++
		}
	}
	healthy, blockers := classifyHealth(statuses, nonBlocking)
	switch {
	case !healthy:
		return stackVerdict{State: StackDegraded, Blockers: blockers, Running: running}
	case running == 0:
		return stackVerdict{State: StackStopped}
	default:
		return stackVerdict{State: StackRunning, Running: running}
	}
}

// CurrentServiceInfo returns Docker service status for the given worktree path.
// Returns nil if Docker is not configured or not applicable.
func CurrentServiceInfo(cfg *config.Config, currentPath string) *ServiceInfo {
	if cfg == nil {
		return nil
	}

	dockerCfg := cfg.Plugins.Docker
	if dockerCfg.Enabled != nil && !*dockerCfg.Enabled {
		return nil
	}

	mode := dockerCfg.Mode
	if mode == "" {
		mode = "local"
	}

	switch mode {
	case "external":
		return externalServiceInfo(cfg, currentPath)
	case "local":
		return localServiceInfo(currentPath)
	default:
		return nil
	}
}

// localServiceInfo checks docker compose status for a worktree with a local compose file.
func localServiceInfo(currentPath string) *ServiceInfo {
	if !hasDockerCompose(currentPath) {
		return nil
	}

	info := &ServiceInfo{
		ConfiguredPath: currentPath,
		State:          StackStopped,
		MatchesCurrent: true, // local always matches
	}
	count, err := localPsCount(currentPath)
	switch {
	case err != nil:
		info.State = StackUnknown
	case count > 0:
		info.State = StackRunning
		info.IsRunning = true
	}
	return info
}

// externalServiceInfo reports the stack for an external compose setup: the
// agent stack when the worktree owns a slot, otherwise the shared stack.
func externalServiceInfo(cfg *config.Config, currentPath string) *ServiceInfo {
	ext := cfg.Plugins.Docker.External
	if ext == nil {
		return nil
	}

	if slot := agentSlotFor(cfg, currentPath); slot > 0 {
		return &ServiceInfo{
			ConfiguredPath: currentPath,
			State:          StackRunning,
			Slot:           slot,
			MatchesCurrent: true,
			IsRunning:      true,
		}
	}

	configured, ok := ConfiguredWorktreePath(cfg)
	if !ok {
		return &ServiceInfo{Shared: true}
	}

	composePath := resolveComposePath(ext.Path)
	// statusTimeout (not probeTimeout) keeps grove which within the <500ms budget.
	statuses, err := probeHealth(composePath, ext.EnvFileName(), nil, statusTimeout)
	v := classifyStack(statuses, err, ext.Services, ext.NonBlockingServices)

	return &ServiceInfo{
		ConfiguredPath: configured,
		State:          v.State,
		Blockers:       v.Blockers,
		MatchesCurrent: filepath.Clean(currentPath) == configured,
		IsRunning:      v.State == StackRunning,
		Shared:         true,
	}
}

// localPsCount is the local-mode docker probe; tests replace it.
var localPsCount = composePsCount

// composePsCount runs docker compose ps -q with a timeout and counts containers.
// An error means docker did not answer, which is not the same as zero containers.
func composePsCount(composePath string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "compose", "ps", "-q")
	cmd.Dir = composePath
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			count++
		}
	}
	return count, nil
}

// composeRunningCount reports whether any container runs, treating a docker
// error as none.
func composeRunningCount(composePath string) (bool, int) {
	count, err := composePsCount(composePath)
	if err != nil {
		return false, 0
	}
	return count > 0, count
}

// classifyExternalStatusFromHealth maps a service-health snapshot to the
// status entry level + detail surfaced via WorktreeStatuses.
//
// Returns ("active", detail) when the configured services are running,
// ("warning", detail) when blockers exist, ("info", detail) otherwise.
func classifyExternalStatusFromHealth(statuses []ServiceStatus, probeErr error, services, nonBlocking []string) (string, string) {
	entry := sharedStackEntry(classifyStack(statuses, probeErr, services, nonBlocking))
	switch entry.Level {
	case plugins.StatusActive:
		return "active", entry.Detail
	case plugins.StatusWarning:
		return "warning", entry.Detail
	default:
		return "info", entry.Detail
	}
}
