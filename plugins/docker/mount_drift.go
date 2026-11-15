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

// MountStatus is the outcome of checking one service's bind-mount.
type MountStatus string

const (
	// MountVerified: the container mounts the configured worktree.
	MountVerified MountStatus = "verified"
	// MountDrift: the container mounts a different directory than the env file
	// points at. Happens when grove switched worktrees but containers weren't
	// restarted, so dev work hits the wrong source tree silently.
	MountDrift MountStatus = "drift"
	// MountNotRunning: compose reports no container for the service.
	MountNotRunning MountStatus = "not_running"
	// MountNoMount: the container runs but has no bind-mount at the probed path.
	MountNoMount MountStatus = "no_mount"
	// MountError: docker itself failed, so the state is unknown.
	MountError MountStatus = "error"
)

// MountDriftReport is one verdict for one service.
type MountDriftReport struct {
	Service          string // configured service name
	Container        string // container ID (12-char short) or "" if not running
	MountDest        string // container path probed (e.g. "/app")
	ConfiguredSource string // host-side path the env file currently points at
	ActualSource     string // host-side path the container is actually mounting
	Status           MountStatus
	Detail           string // human-readable explanation for non-verified statuses
}

// MountDriftConfig is the subset of grove config required to detect drift.
// Extracted so callers can construct it from the resolved external config
// without re-importing the plugin types they don't need.
type MountDriftConfig struct {
	ComposePath string            // resolved compose directory
	EnvFileName string            // env file in compose dir holding the worktree pointer
	EnvVar      string            // env-var key inside that file (e.g. "PROJECT_DIR")
	Services    []string          // services to inspect
	MountDest   string            // container path probed (default "/app")
	MountDests  map[string]string // per-service override of MountDest
}

// mountInspector is the docker-facing seam of the drift check.
type mountInspector interface {
	// ContainerID returns "" with a nil error when the service has no container.
	ContainerID(composePath, envFile, service string) (string, error)
	// MountSource returns an error wrapping errNoMount when nothing is mounted at dest.
	MountSource(containerID, dest string) (string, error)
}

type dockerMountInspector struct{}

func (dockerMountInspector) ContainerID(composePath, envFile, service string) (string, error) {
	return containerIDForService(composePath, envFile, service)
}

func (dockerMountInspector) MountSource(containerID, dest string) (string, error) {
	return inspectMountSource(containerID, dest)
}

var errNoMount = errors.New("no bind-mount")

// CheckMountDrift queries each configured service via `docker compose ps -q`
// and `docker inspect`, then compares the actual bind-mount source against
// the worktree pointer currently in the env file. Returns one report per
// service; collection failures are surfaced per report as MountError so the
// caller can still show what's known.
func CheckMountDrift(cfg MountDriftConfig) ([]MountDriftReport, error) {
	return checkMountDrift(cfg, dockerMountInspector{})
}

func checkMountDrift(cfg MountDriftConfig, insp mountInspector) ([]MountDriftReport, error) {
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
		dest := cfg.MountDest
		if d := cfg.MountDests[svc]; d != "" {
			dest = d
		}
		report := MountDriftReport{
			Service:          svc,
			MountDest:        dest,
			ConfiguredSource: configuredSource,
		}

		containerID, err := insp.ContainerID(cfg.ComposePath, cfg.EnvFileName, svc)
		switch {
		case err != nil:
			report.Status = MountError
			report.Detail = err.Error()
		case containerID == "":
			report.Status = MountNotRunning
			report.Detail = "container not running"
		default:
			report.Container = shortID(containerID)
			actual, err := insp.MountSource(containerID, dest)
			switch {
			case errors.Is(err, errNoMount):
				report.Status = MountNoMount
				report.Detail = err.Error()
			case err != nil:
				report.Status = MountError
				report.Detail = err.Error()
			case mountWithinWorktree(actual, configuredSource):
				report.ActualSource = actual
				report.Status = MountVerified
			default:
				report.ActualSource = actual
				report.Status = MountDrift
			}
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// MountSummary counts reports by status.
type MountSummary struct {
	Total, Verified, Drift, NotRunning, NoMount, Error int
}

// SummarizeMountReports tallies reports by status.
func SummarizeMountReports(reports []MountDriftReport) MountSummary {
	s := MountSummary{Total: len(reports)}
	for _, r := range reports {
		switch r.Status {
		case MountVerified:
			s.Verified++
		case MountDrift:
			s.Drift++
		case MountNotRunning:
			s.NotRunning++
		case MountNoMount:
			s.NoMount++
		case MountError:
			s.Error++
		}
	}
	return s
}

// Unchecked counts services whose mount could not be compared at all.
func (s MountSummary) Unchecked() int { return s.NotRunning + s.NoMount + s.Error }

// AllVerified is true only when at least one service exists and every one verified.
func (s MountSummary) AllVerified() bool { return s.Total > 0 && s.Verified == s.Total }

// Line is the one-line verdict. Zero verified services never reads as success.
func (s MountSummary) Line() string {
	noun := "services"
	if s.Total == 1 {
		noun = "service"
	}
	if s.AllVerified() {
		return fmt.Sprintf("All %d %s mounted from the configured worktree.", s.Total, noun)
	}
	line := fmt.Sprintf("%d of %d %s verified", s.Verified, s.Total, noun)
	if s.Drift > 0 {
		line += fmt.Sprintf("; %d drifted", s.Drift)
	}
	if n := s.Unchecked(); n > 0 {
		line += fmt.Sprintf("; %d not checked", n)
	}
	return line
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

// mountWithinWorktree reports whether a container's mount source is the
// worktree root or a directory inside it. A service may mount only a
// subdirectory (e.g. <worktree>/app) and still serve the right worktree.
// The boundary check is on whole path elements, so a sibling worktree whose
// name merely starts with the same characters does not match. Symlinks are
// not resolved, same as samePath.
func mountWithinWorktree(source, root string) bool {
	if source == "" || root == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(source))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
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
	return "", fmt.Errorf("%w at %q in container", errNoMount, mountDest)
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
		MountDests:  ext.MountDests,
	}
}
