package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LeahArmstrong/grove-cli/internal/plugins"
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

		if running, count := composeRunningCount(path, nil); running {
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
	activeWorktree := readEnvVar(composePath, s.ext.EnvVar)
	running, count := composeRunningCount(composePath, nil)

	for _, path := range paths {
		if !pathMatchesEnv(path, activeWorktree, composePath) {
			continue
		}

		if running {
			result[path] = plugins.StatusEntry{
				ProviderName: "docker",
				Level:        plugins.StatusActive,
				Short:        fmt.Sprintf("up (%d)", count),
				Detail:       fmt.Sprintf("%d service(s) running, pointed to this worktree", count),
			}
		} else {
			result[path] = plugins.StatusEntry{
				ProviderName: "docker",
				Level:        plugins.StatusWarning,
				Short:        "pointed",
				Detail:       "Configured as active worktree but services not running",
			}
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
		if s.agent.URLPattern != "" {
			url := strings.ReplaceAll(s.agent.URLPattern, "{slot}", fmt.Sprintf("%d", slot))
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

// readEnvVar reads a specific variable from the .env file in composePath.
func readEnvVar(composePath, key string) string {
	envFile := filepath.Join(composePath, ".env")
	content, err := os.ReadFile(envFile)
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

// composeRunningCount runs docker compose ps -q with a timeout to count running containers.
func composeRunningCount(composePath string, env []string) (bool, int) {
	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "compose", "ps", "-q")
	cmd.Dir = composePath
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
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
