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
	cli.Label(w, "Stack on:        ", displayPath(reports))
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

	for _, r := range reports {
		level, text := mountReportLine(r)
		switch level {
		case mountLineOK:
			cli.Success(w, "  %s", text)
		case mountLineWarn:
			cli.Warning(w, "  %s", text)
		case mountLineError:
			cli.Error(w, "  %s", text)
		default:
			cli.Faint(w, "  %s", text)
		}
	}

	summary := docker.SummarizeMountReports(reports)
	_, _ = fmt.Fprintln(w)
	code, exit := mountCheckExitCode(note != "", configuredName == "", requireCurrent, summary)
	if summary.AllVerified() {
		cli.Success(w, "%s", summary.Line())
	} else {
		cli.Warning(w, "%s", summary.Line())
	}
	if summary.Drift > 0 {
		cli.Warning(w, "Restart needed — run `grove up` to apply.")
	}
	if exit {
		os.Exit(code)
	}
	return nil
}

type mountLineLevel int

const (
	mountLineOK mountLineLevel = iota
	mountLineNote
	mountLineWarn
	mountLineError
)

// mountReportLine renders one service's verdict. A stopped container is a
// quiet note; a running container with no mount at the probed path is a
// warning, since grove cannot vouch for what that service serves.
func mountReportLine(r docker.MountDriftReport) (mountLineLevel, string) {
	switch r.Status {
	case docker.MountVerified:
		return mountLineOK, fmt.Sprintf("%s ok: %s", r.Service, r.ActualSource)
	case docker.MountDrift:
		return mountLineWarn, fmt.Sprintf("%s drift: actual %s ≠ configured %s", r.Service, r.ActualSource, r.ConfiguredSource)
	case docker.MountNoMount:
		return mountLineWarn, fmt.Sprintf("%s: no bind-mount at %s; this service mounts the worktree at a different path or not at all (set mount_dests.%s if it uses another path)", r.Service, r.MountDest, r.Service)
	case docker.MountError:
		return mountLineError, fmt.Sprintf("%s: %s", r.Service, r.Detail)
	default:
		return mountLineNote, fmt.Sprintf("%s: %s", r.Service, r.Detail)
	}
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
	return fmt.Sprintf("note: you are in '%s'; the stack is on '%s'", currentName, configuredName)
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
	return "could not tell which worktree the stack is on, failing --require-current"
}

// mountCheckExitCode decides the process exit code for `grove here
// --check-mount`, given the cwd-vs-configured mismatch verdict, whether the
// configured worktree could be resolved at all, whether --require-current
// was passed, and the per-service summary. A requireCurrent
// mismatch — or an unresolved configured worktree, which --require-current
// treats the same way since it can't verify anything without it — takes
// priority over drift: if the user isn't even standing in the configured
// worktree (or grove can't tell), the per-service drift verdict beneath it
// isn't the actionable signal — the wrong-worktree guard is. Then drift,
// then a docker failure (ExternalCommandFailed). Stopped or unmounted
// services are informational unless requireCurrent, which fails closed on
// any service it could not verify. exit=false
// tells the caller to fall through to the normal exit-0 success path.
func mountCheckExitCode(mismatch, unresolved, requireCurrent bool, summary docker.MountSummary) (code int, exit bool) {
	switch {
	case requireCurrent && (mismatch || unresolved):
		return exitcode.MountCheckMismatch, true
	case summary.Drift > 0:
		return exitcode.MountDrift, true
	case summary.Error > 0:
		return exitcode.ExternalCommandFailed, true
	case requireCurrent && summary.Unchecked() > 0:
		return exitcode.MountCheckMismatch, true
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
