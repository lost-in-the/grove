package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/plugins/docker"
)

// mountCheckHeaderTemplate is deliberately outcome-neutral: it prints before
// any comparison has run, so it must not read as a verdict. "Mount drift —
// 'x'" followed by a clean report several lines later is a false alarm
// (issue #148).
const mountCheckHeaderTemplate = "Mount check — '%s'"

// runMountDriftCheck implements `grove here --check-mount`. Compares each
// configured service's bind-mount Source against the worktree currently
// pointed at by the env file, and exits with code MountDrift when they
// disagree on any service. Separately, it compares the worktree the user is
// currently standing in (displayName) against the worktree the env file is
// configured for, and notes any mismatch — that comparison doesn't affect
// the default exit code, but requireCurrent opts a caller into gating on it
// too, via the distinct MountCheckMismatch exit code.
//
// Reporter shape is deliberately scriptable: human-readable when running
// interactively, exit code carries the parity verdict for automation.
func runMountDriftCheck(ctx *GroveContext, displayName string, requireCurrent bool) error {
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

	cli.Header(w, mountCheckHeaderTemplate, displayName)
	cli.Label(w, "Env file:        ", filepath.Join(composePath, driftCfg.EnvFileName))
	cli.Label(w, "Configured src:  ", displayPath(reports))
	cli.Label(w, "Mount dest:      ", driftCfg.MountDest)
	_, _ = fmt.Fprintln(w)

	configuredName := ""
	if mgr, mgrErr := ctx.WorktreeManager(); mgrErr == nil {
		configuredName = mgr.DisplayNameForPath(configuredSourcePath(reports))
	}
	note := mountCheckMismatchNote(displayName, configuredName)
	if note != "" {
		cli.Warning(w, "%s", note)
		_, _ = fmt.Fprintln(w)
	}
	unresolvedNote := mountCheckUnresolvedNote(requireCurrent, configuredName)
	if unresolvedNote != "" {
		cli.Warning(w, "%s", unresolvedNote)
		_, _ = fmt.Fprintln(w)
	}

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
	if code, exit := mountCheckExitCode(note != "", configuredName == "", requireCurrent, anyDrift); exit {
		if anyDrift {
			cli.Warning(w, "Restart needed — run `grove up` to apply.")
		}
		os.Exit(code)
	}
	cli.Success(w, "All services mounted from the configured worktree.")
	return nil
}

// mountCheckMismatchNote flags when the worktree the user is standing in
// (currentName) differs from the one the stack's env file is configured for
// (configuredName). Returns "" when there's nothing to flag — either no
// configured worktree could be resolved (env unset) or it already matches
// currentName. This note is informational by default: exit code stays 0,
// since env-vs-container parity (MountDrift) is the documented contract for
// --check-mount's exit code. --require-current opts a caller into gating on
// this comparison too (issue #148).
func mountCheckMismatchNote(currentName, configuredName string) string {
	if configuredName == "" || configuredName == currentName {
		return ""
	}
	return fmt.Sprintf("note: you are in '%s'; the stack is configured for '%s'", currentName, configuredName)
}

// mountCheckUnresolvedNote flags when --require-current can't do its job
// because the configured worktree couldn't be resolved at all — the env var
// is unset, or ctx.WorktreeManager() errored. mountCheckMismatchNote stays
// silent in that state (there's nothing to compare currentName against), but
// a guard that can't establish which worktree the stack serves must not
// silently pass; it must fail closed. Without --require-current this stays
// informational-only and prints nothing, matching the pre-existing
// env-unset behavior.
func mountCheckUnresolvedNote(requireCurrent bool, configuredName string) string {
	if configuredName != "" || !requireCurrent {
		return ""
	}
	return "could not determine the configured worktree — failing --require-current"
}

// mountCheckExitCode decides the process exit code for `grove here
// --check-mount`, given the cwd-vs-configured mismatch verdict, whether the
// configured worktree could be resolved at all, whether --require-current
// was passed, and the env-vs-container drift verdict. A requireCurrent
// mismatch — or an unresolved configured worktree, which --require-current
// treats the same way since it can't verify anything without it — takes
// priority over drift: if the user isn't even standing in the configured
// worktree (or grove can't tell), the per-service drift verdict beneath it
// isn't the actionable signal — the wrong-worktree guard is. exit=false
// tells the caller to fall through to the normal exit-0 success path.
func mountCheckExitCode(mismatch, unresolved, requireCurrent, anyDrift bool) (code int, exit bool) {
	if requireCurrent && (mismatch || unresolved) {
		return exitcode.MountCheckMismatch, true
	}
	if anyDrift {
		return exitcode.MountDrift, true
	}
	return 0, false
}

// configuredSourcePath returns the first non-empty ConfiguredSource across
// all reports, since every report computes the same value from the same env
// read. Returns "" when no report carries one (env var unset).
func configuredSourcePath(reports []docker.MountDriftReport) string {
	for _, r := range reports {
		if r.ConfiguredSource != "" {
			return r.ConfiguredSource
		}
	}
	return ""
}

// displayPath formats configuredSourcePath(reports) for the "Configured
// src:" label, falling back to "(unset)" when no report carries a value.
func displayPath(reports []docker.MountDriftReport) string {
	if src := configuredSourcePath(reports); src != "" {
		return src
	}
	return "(unset)"
}
