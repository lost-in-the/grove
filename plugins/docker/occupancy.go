package docker

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lost-in-the/grove/internal/cmdexec"
)

// occupancyFormat asks docker for each container's compose project name and
// the working directory compose was invoked from. The working directory is
// the cross-namespace correlation key: docker compose sets
// com.docker.compose.project.working_dir to the resolved compose directory
// regardless of which grove project's COMPOSE_PROJECT_NAME started the
// container, so two grove projects pointed at the same [plugins.docker.external]
// path always agree on it even though their compose project names differ
// (#147). project.working_dir is the label docker compose actually sets;
// there is no dedicated "stack path" label to key on instead.
const occupancyFormat = `{{.Label "com.docker.compose.project"}}|{{.Label "com.docker.compose.project.working_dir"}}`

// stackOccupants returns, for the shared compose stack rooted at composePath,
// which compose project currently holds each numbered agent slot — across
// ALL grove projects that mount that same stack, not just the caller's own
// namespace. Discovery is via `docker ps -a` label inspection rather than
// `docker compose ps` because there's no single compose project to scope the
// query to; the whole point is that more than one project may be involved.
//
// Best-effort: returns an error if docker can't be queried, but callers
// should treat that as "occupancy unknown" and fall back to their own
// records rather than blocking on it.
func stackOccupants(composePath string) (map[int]string, error) {
	out, err := cmdexec.Output(context.Background(), "docker",
		[]string{"ps", "-a", "--format", occupancyFormat}, "", cmdexec.Docker)
	if err != nil {
		return nil, err
	}
	return parseStackOccupants(string(out), composePath), nil
}

// parseStackOccupants is the pure half of stackOccupants, extracted for unit
// testing without invoking docker. Each line of out is expected to be
// "<compose-project>|<working-dir>" as produced by occupancyFormat. Lines
// whose working dir doesn't match composePath (cleaned, exact match) belong
// to an unrelated compose stack and are ignored. Project names that don't
// end in "-agent-<N>" (e.g. the main stack, or the "-agent-ephemeral"
// project used for one-off Run/Exec) aren't slot occupants and are ignored.
func parseStackOccupants(out, composePath string) map[int]string {
	result := make(map[int]string)
	cleanPath := filepath.Clean(composePath)

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		project, workingDir := parts[0], parts[1]
		if project == "" || workingDir == "" {
			continue
		}
		if filepath.Clean(workingDir) != cleanPath {
			continue
		}
		slot, ok := parseAgentSlot(project)
		if !ok {
			continue
		}
		// Multiple containers from the same compose project all resolve to
		// the same slot; first-write-wins is fine since they agree.
		result[slot] = project
	}
	return result
}

// parseAgentSlot extracts the slot number from a compose project name of the
// form "<project>-agent-<N>" (see agentComposeProjectName). Returns
// ok=false for names that aren't numbered slot projects, such as
// "<project>-agent-ephemeral" (grove run/exec) or compose projects unrelated
// to agent stacks entirely.
func parseAgentSlot(projectName string) (int, bool) {
	idx := strings.LastIndex(projectName, "-agent-")
	if idx == -1 {
		return 0, false
	}
	suffix := projectName[idx+len("-agent-"):]
	if suffix == "" {
		return 0, false
	}
	slot, err := strconv.Atoi(suffix)
	if err != nil || slot <= 0 {
		return 0, false
	}
	return slot, true
}

// foreignOccupants filters an occupancy map down to slots held by a compose
// project OTHER than the one the given grove project would itself use for
// that slot — i.e. genuine cross-project conflicts (#147), not a project
// simply seeing its own already-running containers.
func foreignOccupants(occupants map[int]string, projectName string) map[int]string {
	if len(occupants) == 0 {
		return nil
	}
	foreign := make(map[int]string, len(occupants))
	for slot, project := range occupants {
		if project == agentComposeProjectName(projectName, slot) {
			continue
		}
		foreign[slot] = project
	}
	return foreign
}
