package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ServiceHealthStatus enumerates the possible classified states for a compose service.
type ServiceHealthStatus int

const (
	ServiceUnknown     ServiceHealthStatus = iota
	ServiceRunning                         // container is up
	ServiceExitedClean                     // container exited with code 0 (one-shot completed)
	ServiceExitedError                     // container exited with non-zero code
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

// finalizeUpResult inspects post-up service health and decides whether the
// reported cmdErr (from `compose up`) should propagate. If only non-blocking
// services failed, returns nil. Otherwise wraps cmdErr with blocker context.
func finalizeUpResult(cmdErr error, statuses []ServiceStatus, nonBlocking []string) error {
	if cmdErr == nil {
		return nil
	}
	if len(statuses) == 0 {
		// No probe data — can't decide whether non-blocking services masked the failure.
		// Propagate the original error.
		return cmdErr
	}
	healthy, blockers := classifyHealth(statuses, nonBlocking)
	if healthy {
		return nil
	}
	return fmt.Errorf("up failed; blocking service(s) not healthy: %s (underlying: %w)", strings.Join(blockers, ", "), cmdErr)
}

// probeTimeout is longer than statusTimeout (300ms) because Up() can tolerate a
// slower probe — docker compose ps can be slow on busy systems (NFS, heavy disk I/O)
// and 1s caused false-empty probes that fed the silent-failure bug in finalizeUpResult.
// Display paths (grove ls, grove which) must pass statusTimeout instead so a slow
// daemon can't blow the <500ms command budget; a timed-out probe degrades to
// "configured" via classifyExternalStatusFromHealth.
const probeTimeout = 3 * time.Second

// probeServiceHealth runs `docker compose ps --all --format json` and returns parsed statuses.
// composePath is the directory containing the compose file; envFile is the env file name
// (e.g., ".env" or ".env.local") to pass via --env-file. timeout bounds the compose call:
// probeTimeout for correctness paths (Up), statusTimeout for display paths.
func probeServiceHealth(composePath, envFile string, env []string, timeout time.Duration) ([]ServiceStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{"compose"}
	args = append(args, composeEnvFileArgs(composePath, envFile)...)
	args = append(args, "ps", "--all", "--format", "json")

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = composePath
	// Always start from the parent process env so PATH/HOME/etc. are available,
	// then layer caller-supplied vars on top.
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("compose ps: %w", err)
	}
	return parseServiceHealth(out)
}
