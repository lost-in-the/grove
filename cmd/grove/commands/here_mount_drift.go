package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/docker"
)

// runMountDriftCheck implements `grove here --check-mount`. Compares each
// configured service's bind-mount Source against the worktree currently
// pointed at by the env file, and exits with code MountDrift when they
// disagree on any service.
//
// Reporter shape is deliberately scriptable: human-readable when running
// interactively, exit code carries the parity verdict for automation.
func runMountDriftCheck(ctx *GroveContext, tree *worktree.Worktree) error {
	w := cli.NewStdout()
	stderr := cli.NewStderr()

	ext := ctx.Config.Plugins.Docker.External
	if ext == nil {
		cli.Faint(w, "Mount drift check skipped — external Docker mode not configured.")
		return nil
	}

	composePath := ext.Path
	if !filepath.IsAbs(composePath) {
		composePath = filepath.Clean(filepath.Join(ctx.ProjectRoot, composePath))
	}

	driftCfg := docker.MountDriftConfigFromExternal(ext, composePath)
	if driftCfg == nil {
		cli.Faint(w, "Mount drift check skipped — no services configured.")
		return nil
	}

	reports, err := docker.CheckMountDrift(*driftCfg)
	if err != nil {
		cli.Warning(stderr, "mount drift check failed: %v", err)
		os.Exit(exitcode.ExternalCommandFailed)
	}

	cli.Header(w, "Mount drift — '%s'", tree.DisplayName())
	cli.Label(w, "Env file:        ", filepath.Join(composePath, driftCfg.EnvFileName))
	cli.Label(w, "Configured src:  ", displayPath(reports))
	cli.Label(w, "Mount dest:      ", driftCfg.MountDest)
	_, _ = fmt.Fprintln(w)

	anyDrift := false
	for _, r := range reports {
		switch {
		case r.Reason != "":
			cli.Faint(w, "  %s: %s", r.Service, r.Reason)
		case r.Drift:
			anyDrift = true
			cli.Warning(w, "  %s drift: actual %s ≠ configured %s", r.Service, r.ActualSource, r.ConfiguredSource)
		default:
			cli.Success(w, "  %s ok: %s", r.Service, r.ActualSource)
		}
	}

	_, _ = fmt.Fprintln(w)
	if anyDrift {
		cli.Warning(w, "Restart needed — run `grove up` to apply.")
		os.Exit(exitcode.MountDrift)
	}
	cli.Success(w, "All services mounted from the configured worktree.")
	return nil
}

// displayPath returns the first non-empty ConfiguredSource across all reports,
// since every report computes the same value. Falls back to "(unset)".
func displayPath(reports []docker.MountDriftReport) string {
	for _, r := range reports {
		if r.ConfiguredSource != "" {
			return r.ConfiguredSource
		}
	}
	return "(unset)"
}
