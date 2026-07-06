package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lost-in-the/grove/internal/config"
)

// MountDriftReport is one drift verdict for one service. Drift = true means
// the running container's bind-mount source at MountDest does NOT match the
// worktree directory grove's env file currently points at. That state happens
// when grove switched worktrees but containers weren't restarted, so dev work
// hits the wrong source tree silently.
type MountDriftReport struct {
	Service          string // configured service name
	Container        string // container ID (12-char short) or "" if not running
	MountDest        string // container path probed (e.g. "/app")
	ConfiguredSource string // host-side path the env file currently points at
	ActualSource     string // host-side path the container is actually mounting
	Drift            bool
	Reason           string // non-empty when status can't be determined cleanly
}

// MountDriftConfig is the subset of grove config required to detect drift.
// Extracted so callers can construct it from the resolved external config
// without re-importing the plugin types they don't need.
type MountDriftConfig struct {
	ComposePath string   // resolved compose directory
	EnvFileName string   // env file in compose dir holding the worktree pointer
	EnvVar      string   // env-var key inside that file (e.g. "PROJECT_DIR")
	Services    []string // services to inspect
	MountDest   string   // container path probed (default "/app")
}

// CheckMountDrift queries each configured service via `docker compose ps -q`
// and `docker inspect`, then compares the actual bind-mount source against
// the worktree pointer currently in the env file. Returns one report per
// service; never returns a partial set with an error — collection failures
// are surfaced via the per-report Reason field so the caller can still show
// what's known.
func CheckMountDrift(cfg MountDriftConfig) ([]MountDriftReport, error) {
	if len(cfg.Services) == 0 {
		return nil, errors.New("no services configured for drift check")
	}
	if cfg.MountDest == "" {
		cfg.MountDest = "/app"
	}

	envValue := readEnvVar(cfg.ComposePath, cfg.EnvFileName, cfg.EnvVar)
	configuredSource := resolveEnvPath(cfg.ComposePath, envValue)

	reports := make([]MountDriftReport, 0, len(cfg.Services))
	for _, svc := range cfg.Services {
		report := MountDriftReport{
			Service:          svc,
			MountDest:        cfg.MountDest,
			ConfiguredSource: configuredSource,
		}

		containerID, err := containerIDForService(cfg.ComposePath, cfg.EnvFileName, svc)
		if err != nil || containerID == "" {
			report.Reason = "container not running"
			reports = append(reports, report)
			continue
		}
		report.Container = shortID(containerID)

		actual, err := inspectMountSource(containerID, cfg.MountDest)
		if err != nil {
			report.Reason = err.Error()
			reports = append(reports, report)
			continue
		}
		report.ActualSource = actual
		report.Drift = !samePath(actual, configuredSource)
		reports = append(reports, report)
	}
	return reports, nil
}

// resolveEnvPath turns an env-file value (relative or absolute) into an
// absolute path against composePath. Empty envValue returns empty string.
func resolveEnvPath(composePath, envValue string) string {
	if envValue == "" {
		return ""
	}
	if filepath.IsAbs(envValue) {
		return filepath.Clean(envValue)
	}
	return filepath.Clean(filepath.Join(composePath, envValue))
}

// samePath compares two host paths after Clean. Symlink resolution is left
// to the caller's environment — both docker inspect and env file values
// typically come pre-resolved, and EvalSymlinks here would require the host
// path to exist, which may not be true for a destination dir grove only knows
// about by reference.
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// composePsQArgs builds the `docker` argument list for `compose ps -q <service>`,
// layering env files via composeEnvFileArgs so a non-default env_file doesn't
// make compose ignore .env (issue #98). Extracted for unit testing.
func composePsQArgs(composePath, envFile, service string) []string {
	args := []string{"compose"}
	args = append(args, composeEnvFileArgs(composePath, envFile)...)
	return append(args, "ps", "-q", service)
}

// containerIDForService runs `docker compose ps -q <service>` and returns the
// (single) container ID. Multi-replica compose configs return multiple IDs;
// we take the first since drift detection only needs one sample of the mount.
func containerIDForService(composePath, envFile, service string) (string, error) {
	args := composePsQArgs(composePath, envFile, service)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = composePath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker compose ps: %w", err)
	}
	ids := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(ids) == 0 || ids[0] == "" {
		return "", nil
	}
	return ids[0], nil
}

// dockerMount mirrors the relevant fields of an entry in docker inspect's
// Mounts array. Other fields (Mode, Propagation, RW, Driver) are ignored.
type dockerMount struct {
	Type        string `json:"Type"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
}

// inspectMountSource returns the host-side Source for the first mount whose
// Destination matches mountDest. Returns an error explaining the failure mode
// when no such mount exists.
func inspectMountSource(containerID, mountDest string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .Mounts}}", containerID)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker inspect: %w", err)
	}

	return parseInspectMounts(out, mountDest)
}

// parseInspectMounts is the pure-function half of inspectMountSource —
// extracted so the JSON parsing + Destination matching logic can be unit
// tested without invoking docker. cleanedMountDest tolerates trailing
// slashes from either side of the comparison.
func parseInspectMounts(jsonBlob []byte, mountDest string) (string, error) {
	var mounts []dockerMount
	if err := json.Unmarshal(jsonBlob, &mounts); err != nil {
		return "", fmt.Errorf("parse docker inspect mounts: %w", err)
	}
	cleanedDest := filepath.Clean(mountDest)
	for _, m := range mounts {
		if filepath.Clean(m.Destination) == cleanedDest {
			return m.Source, nil
		}
	}
	return "", fmt.Errorf("no bind-mount at %q in container", mountDest)
}

// FromExternalConfig builds a MountDriftConfig from a resolved
// ExternalComposeConfig and the project root. Returns nil if external mode
// isn't configured or no services are listed.
func MountDriftConfigFromExternal(ext *config.ExternalComposeConfig, composePath string) *MountDriftConfig {
	if ext == nil || len(ext.Services) == 0 {
		return nil
	}
	return &MountDriftConfig{
		ComposePath: composePath,
		EnvFileName: ext.EnvFileName(),
		EnvVar:      ext.EnvVar,
		Services:    ext.Services,
		MountDest:   ext.MountDestPath(),
	}
}
