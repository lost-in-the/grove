package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
func localStatuses(_ *localStrategy, paths []string) map[string]plugins.StatusEntry {
	result := make(map[string]plugins.StatusEntry)

	for _, path := range paths {
		if !hasDockerCompose(path) {
			continue
		}

		// Has compose file — at minimum "configured"
		entry := plugins.StatusEntry{
			ProviderName: "docker",
			Level:        plugins.StatusInfo,
			Short:        "compose",
			Detail:       "Compose file found",
		}

		if running, count := composeRunningCount(path); running {
			entry.Level = plugins.StatusActive
			entry.Short = fmt.Sprintf("up (%d)", count)
			entry.Detail = fmt.Sprintf("%d container(s) running", count)
		}

		result[path] = entry
	}

	return result
}

// externalStatuses reads the .env to find which worktree is pointed and checks if running.
func externalStatuses(s *externalStrategy, paths []string) map[string]plugins.StatusEntry {
	result := make(map[string]plugins.StatusEntry)

	composePath := s.composePath()
	activeWorktree := readEnvVar(composePath, s.ext.EnvFileName(), s.ext.EnvVar)

	// Probe service health once for the whole stack. Probe errors are intentionally
	// ignored here: status display is informational and a nil result downgrades to
	// "configured" via classifyExternalStatusFromHealth, which is correct for
	// "stack not running yet" — surfacing the error would just be noise on the dashboard.
	statuses, _ := probeServiceHealth(composePath, s.ext.EnvFileName(), nil)

	for _, path := range paths {
		matches := pathMatchesEnv(path, activeWorktree, composePath)
		if !matches {
			continue
		}

		level, detail := classifyExternalStatusFromHealth(statuses, s.ext.NonBlockingServices)

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

		detail := fmt.Sprintf("Stack #%d", slot)
		if url := formatAgentURL(s.agent.URLPattern, slot); url != "" {
			detail += " at " + url
		}

		result[path] = plugins.StatusEntry{
			ProviderName: "docker",
			Level:        plugins.StatusActive,
			Short:        fmt.Sprintf("#%d", slot),
			Detail:       detail,
		}
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

// ServiceInfo describes the current Docker service state for a single worktree.
type ServiceInfo struct {
	RunningFor     string // worktree name services are running for
	IsRunning      bool
	MatchesCurrent bool // whether services are running for the queried worktree
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

	running, _ := composeRunningCount(currentPath)
	return &ServiceInfo{
		RunningFor:     filepath.Base(currentPath),
		IsRunning:      running,
		MatchesCurrent: true, // local always matches
	}
}

// externalServiceInfo checks docker compose status for an external compose setup.
func externalServiceInfo(cfg *config.Config, currentPath string) *ServiceInfo {
	ext := cfg.Plugins.Docker.External
	if ext == nil {
		return nil
	}

	// Check agent mode first
	if ext.Agent != nil && ext.Agent.Enabled != nil && *ext.Agent.Enabled {
		return agentServiceInfo(cfg, currentPath)
	}

	composePath := resolveComposePath(ext.Path)
	activeWorktree := readEnvVar(composePath, ext.EnvFileName(), ext.EnvVar)
	matches := pathMatchesEnv(currentPath, activeWorktree, composePath)

	// Use the same probeServiceHealth + classifyHealth path as externalStatuses so
	// that non_blocking_services is honored and both code paths agree on verdict.
	statuses, _ := probeServiceHealth(composePath, ext.EnvFileName(), nil)
	healthy, _ := classifyHealth(statuses, ext.NonBlockingServices)
	running := healthy && len(statuses) > 0

	runningFor := filepath.Base(activeWorktree)
	if activeWorktree == "" {
		runningFor = ""
	}

	return &ServiceInfo{
		RunningFor:     runningFor,
		IsRunning:      running,
		MatchesCurrent: matches,
	}
}

// agentServiceInfo checks agent slot allocation for the given worktree path.
func agentServiceInfo(cfg *config.Config, currentPath string) *ServiceInfo {
	slot := FindWorktreeSlot(cfg, currentPath)
	if slot == 0 {
		return &ServiceInfo{
			IsRunning:      false,
			MatchesCurrent: false,
		}
	}

	return &ServiceInfo{
		RunningFor:     filepath.Base(currentPath),
		IsRunning:      true,
		MatchesCurrent: true,
	}
}

// composeRunningCount runs docker compose ps -q with a timeout to count running containers.
func composeRunningCount(composePath string) (bool, int) {
	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "compose", "ps", "-q")
	cmd.Dir = composePath
	out, err := cmd.Output()
	if err != nil {
		return false, 0
	}

	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			count++
		}
	}
	return count > 0, count
}

// classifyExternalStatusFromHealth maps a service-health snapshot to the
// status entry level + detail surfaced via WorktreeStatuses.
//
// Returns ("active", detail) when all non-skipped services are healthy,
// ("warning", detail) when blockers exist, ("info", detail) when nothing
// is running but no failures either.
func classifyExternalStatusFromHealth(statuses []ServiceStatus, nonBlocking []string) (string, string) {
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
