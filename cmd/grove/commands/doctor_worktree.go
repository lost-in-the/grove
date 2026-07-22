package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/fsutil"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/worktree"
)

// provisioningStatus classifies the on-disk state of a single provisioning
// entry against the project config's intent.
type provisioningStatus int

const (
	provisioningOK provisioningStatus = iota
	provisioningMissing
	provisioningOverride
)

func (s provisioningStatus) String() string {
	switch s {
	case provisioningOK:
		return "ok"
	case provisioningMissing:
		return "missing"
	case provisioningOverride:
		return "override"
	}
	return "unknown"
}

// provisioningResult is one audit verdict for one entry in one worktree.
type provisioningResult struct {
	Field   string // "copy_files" | "symlink_files" | "symlink_dirs"
	Path    string // relative path inside the worktree
	Status  provisioningStatus
	Detail  string // human-readable explanation
	Sources string // absolute source path (for --fix)
}

// classifyEntry inspects one entry and returns its audit verdict. Pure
// filesystem inspection — no mutation. Symlinks are inspected with Lstat so
// they're not silently followed.
func classifyEntry(field, projectRoot, worktreePath, rel string) provisioningResult {
	res := provisioningResult{Field: field, Path: rel, Sources: filepath.Join(projectRoot, rel)}

	target := filepath.Join(worktreePath, rel)
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			res.Status = provisioningMissing
			res.Detail = "not present in worktree"
			return res
		}
		res.Status = provisioningOverride
		res.Detail = fmt.Sprintf("stat error: %v", err)
		return res
	}

	isSymlink := info.Mode()&os.ModeSymlink != 0

	switch field {
	case "copy_files":
		if isSymlink {
			res.Status = provisioningOverride
			res.Detail = "expected regular file, found symlink"
			return res
		}
		if info.IsDir() {
			res.Status = provisioningOverride
			res.Detail = "expected regular file, found directory"
			return res
		}
	case "symlink_files", "symlink_dirs":
		if !isSymlink {
			res.Status = provisioningOverride
			if info.IsDir() {
				res.Detail = "expected symlink, found regular directory"
			} else {
				res.Detail = "expected symlink, found regular file"
			}
			return res
		}
	}

	res.Status = provisioningOK
	return res
}

// auditWorktreeProvisioning walks every copy_files / symlink_files /
// symlink_dirs entry in the project config and returns one classification per
// entry. Read-only.
func auditWorktreeProvisioning(ext *config.ExternalComposeConfig, projectRoot, worktreePath string) []provisioningResult {
	if ext == nil {
		return nil
	}
	results := make([]provisioningResult, 0, len(ext.CopyFiles)+len(ext.SymlinkFiles)+len(ext.SymlinkDirs))
	for _, p := range ext.CopyFiles {
		results = append(results, classifyEntry("copy_files", projectRoot, worktreePath, p))
	}
	for _, p := range ext.SymlinkFiles {
		results = append(results, classifyEntry("symlink_files", projectRoot, worktreePath, p))
	}
	for _, p := range ext.SymlinkDirs {
		results = append(results, classifyEntry("symlink_dirs", projectRoot, worktreePath, p))
	}
	return results
}

// repairMissing acts only on `missing` entries — restores them from the main
// worktree using the same copy/symlink semantics as initial worktree setup.
// Returns the number of entries fixed and the first error encountered (if any).
// Override entries are left alone by design: the user changed something on
// purpose, and silently reverting that would be hostile.
func repairMissing(w *cli.Writer, results []provisioningResult, worktreePath string) (int, error) {
	fixed := 0
	var firstErr error
	for _, r := range results {
		if r.Status != provisioningMissing {
			continue
		}
		dst, err := fsutil.SafeJoin(worktreePath, r.Path)
		if err != nil {
			cli.Warning(w, "  skip %s: %v", r.Path, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			cli.Warning(w, "  failed to ensure parent dir for %s: %v", r.Path, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		switch r.Field {
		case "copy_files":
			if err := fsutil.CopyFile(r.Sources, dst); err != nil {
				cli.Warning(w, "  copy failed for %s: %v", r.Path, err)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			cli.Success(w, "  copied %s", r.Path)
		case "symlink_files", "symlink_dirs":
			if err := os.Symlink(r.Sources, dst); err != nil {
				cli.Warning(w, "  symlink failed for %s: %v", r.Path, err)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			cli.Success(w, "  symlinked %s", r.Path)
		}
		fixed++
	}
	return fixed, firstErr
}

// runWorktreeAudit drives the per-worktree audit flow: resolves the project
// root and the target worktree path(s), runs the audit, prints results, and
// repairs missing entries when fix=true.
func runWorktreeAudit(w *cli.Writer, args []string, all, fix bool) error {
	groveDir, err := grove.FindRoot("")
	if err != nil || groveDir == "" {
		return fmt.Errorf("not in a grove project — `grove doctor <worktree>` and `--all` only make sense inside one")
	}
	projectRoot := filepath.Dir(groveDir)

	cfg, err := config.LoadFromGroveDir(groveDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ext := cfg.Plugins.Docker.External
	if ext == nil || (len(ext.CopyFiles) == 0 && len(ext.SymlinkFiles) == 0 && len(ext.SymlinkDirs) == 0) {
		cli.Info(w, "No copy_files / symlink_files / symlink_dirs configured — nothing to audit.")
		return nil
	}

	mgr, err := worktree.NewManager(projectRoot)
	if err != nil {
		return fmt.Errorf("init worktree manager: %w", err)
	}

	var targets []*worktree.Worktree
	if all {
		list, err := mgr.List()
		if err != nil {
			return fmt.Errorf("list worktrees: %w", err)
		}
		// The main worktree is the source of copy_files/symlink_dirs entries;
		// auditing it against itself reports every source as "missing" and, with
		// --fix, creates self-referential (ELOOP) symlinks that every future
		// `grove new` then propagates (B25). Skip it.
		for _, wt := range list {
			if wt.IsMain {
				continue
			}
			targets = append(targets, wt)
		}
	} else {
		name := args[0]
		wt, err := mgr.Find(name)
		if err != nil {
			return fmt.Errorf("find worktree %q: %w", name, err)
		}
		if wt == nil {
			return fmt.Errorf("worktree %q not found", name)
		}
		targets = []*worktree.Worktree{wt}
	}

	totalIssues := 0
	for _, wt := range targets {
		cli.Header(w, "Auditing worktree '%s' at %s", wt.Name, wt.Path)
		results := auditWorktreeProvisioning(ext, projectRoot, wt.Path)

		issues := 0
		for _, r := range results {
			switch r.Status {
			case provisioningOK:
				cli.Faint(w, "  ok:       %s", r.Path)
			case provisioningMissing:
				cli.Warning(w, "  missing:  %s — %s", r.Path, r.Detail)
				issues++
			case provisioningOverride:
				cli.Info(w, "  override: %s — %s", r.Path, r.Detail)
			}
		}

		if issues == 0 {
			cli.Success(w, "all %d entries present", len(results))
		} else if fix {
			fixed, fixErr := repairMissing(w, results, wt.Path)
			if fixed > 0 {
				cli.Success(w, "Repaired %d missing %s", fixed, pluralize(fixed, "entry", "entries"))
			}
			if fixErr != nil {
				cli.Warning(w, "Some repairs failed (first error: %v)", fixErr)
			}
		} else {
			cli.Warning(w, "%d %s. Run with --fix to remediate, or accept overrides as-is.", issues, pluralize(issues, "issue", "issues"))
		}

		totalIssues += issues
		_, _ = fmt.Fprintln(w)
	}

	if totalIssues > 0 && !fix {
		// Mirror project-level doctor's behavior of finishing on a clear note.
		cli.Faint(w, "Run `grove doctor %s --fix` to restore missing entries.", strings.Join(argsOrAll(args, all), " "))
	}
	return nil
}

func argsOrAll(args []string, all bool) []string {
	if all {
		return []string{"--all"}
	}
	return args
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
