package docker

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lost-in-the/grove/internal/cmdexec"
)

// occupancyFormat asks docker for each container's compose project name, the
// working directory compose was invoked from, and the container's current
// state. The working directory is the cross-namespace correlation key:
// docker compose sets com.docker.compose.project.working_dir to the resolved
// compose directory regardless of which grove project's COMPOSE_PROJECT_NAME
// started the container, so two grove projects pointed at the same
// [plugins.docker.external] path always agree on it even though their
// compose project names differ (#147). project.working_dir is the label
// docker compose actually sets; there is no dedicated "stack path" label to
// key on instead. State (e.g. "running", "exited") lets callers distinguish
// a live stack from one whose containers have merely never been removed.
const occupancyFormat = `{{.Label "com.docker.compose.project"}}|{{.Label "com.docker.compose.project.working_dir"}}|{{.State}}`

// stackOccupant is one compose project's claim on an agent slot, along with
// whether that project currently has any container running for it.
type stackOccupant struct {
	Project string
	Running bool
}

// stackOccupants returns, for the shared compose stack rooted at composePath,
// which compose project currently holds each numbered agent slot — across
// ALL grove projects that mount that same stack, not just the caller's own
// namespace. Discovery is via `docker ps -a` label inspection rather than
// `docker compose ps` because there's no single compose project to scope the
// query to; the whole point is that more than one project may be involved.
// `-a` (not just running containers) is deliberate: a stopped container with
// a pinned container_name still causes the docker daemon name-conflict this
// occupancy check exists to prevent, so slots held by exited-but-not-removed
// containers must keep counting as occupied for allocation purposes. Callers
// that need to know whether a slot is actually live (e.g. `grove ps`) should
// consult the per-occupant Running flag rather than filtering occupancy
// itself.
//
// Best-effort: returns an error if docker can't be queried, but callers
// should treat that as "occupancy unknown" and fall back to their own
// records rather than blocking on it.
func stackOccupants(composePath string) (map[int]stackOccupant, error) {
	out, err := cmdexec.Output(context.Background(), "docker",
		[]string{"ps", "-a", "--format", occupancyFormat}, "", cmdexec.Docker)
	if err != nil {
		return nil, err
	}
	return parseStackOccupants(string(out), composePath), nil
}

// parseStackOccupants is the pure half of stackOccupants, extracted for unit
// testing without invoking docker. Each line of out is expected to be
// "<compose-project>|<working-dir>|<state>" as produced by occupancyFormat.
// Lines whose working dir doesn't match composePath (cleaned, exact match)
// belong to an unrelated compose stack and are ignored. Project names that
// don't end in "-agent-<N>" (e.g. the main stack, or the "-agent-ephemeral"
// project used for one-off Run/Exec) aren't slot occupants and are ignored.
//
// A slot's Running is true if ANY container belonging to its compose project
// is in the "running" state — a project with one running and one exited
// container (e.g. mid-restart) still counts as running.
func parseStackOccupants(out, composePath string) map[int]stackOccupant {
	result := make(map[int]stackOccupant)
	cleanPath := filepath.Clean(composePath)

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		project, workingDir, state := parts[0], parts[1], parts[2]
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
		occ := result[slot]
		occ.Project = project
		if state == "running" {
			occ.Running = true
		}
		result[slot] = occ
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
// simply seeing its own already-running containers. Foreign occupants are
// excluded from allocation regardless of Running: a stopped container still
// owns its pinned container_name until the holder's `grove down` or an
// explicit container removal frees it, so treating a stopped foreign slot as
// available would reintroduce the name-conflict this check exists to
// prevent.
func foreignOccupants(occupants map[int]stackOccupant, projectName string) map[int]stackOccupant {
	if len(occupants) == 0 {
		return nil
	}
	foreign := make(map[int]stackOccupant, len(occupants))
	for slot, occ := range occupants {
		if occ.Project == agentComposeProjectName(projectName, slot) {
			continue
		}
		foreign[slot] = occ
	}
	return foreign
}
