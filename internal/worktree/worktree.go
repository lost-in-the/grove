package worktree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/log"
)

const (
	// gitLogFormatParts is the expected number of parts in git log output
	gitLogFormatParts = 4
)

// Worktree represents a git worktree
type Worktree struct {
	Name             string // Short name (derived from path)
	Path             string // Absolute path to worktree
	Branch           string // Branch name or "detached"
	Commit           string // Commit hash (full)
	ShortCommit      string // Short commit hash (7 chars)
	CommitMessage    string // Commit message subject
	CommitAge        string // Relative commit time
	IsDirty          bool   // Whether there are uncommitted changes
	DirtyCheckFailed bool   // Whether the dirty check errored (status unknown)
	DirtyFiles       string // List of dirty files (from git status --porcelain)
	IsMain           bool   // Whether this is the main worktree
	ShortName        string // Short name without project prefix
	IsPrunable       bool   // Whether the worktree directory is missing (stale)
}

// Manager handles git worktree operations
type Manager struct {
	repoRoot    string // Root of the git repository
	projectName string // Cached project name
	namePattern string // Cached worktree naming pattern (see getNamePattern)
}

// NewManager creates a new worktree manager
// If repoRoot is empty, it will detect from current directory
func NewManager(repoRoot string) (*Manager, error) {
	if repoRoot == "" {
		// Try to detect repo root from current directory
		output, err := cmdexec.Output(context.TODO(), "git", []string{"rev-parse", "--show-toplevel"}, "", cmdexec.GitLocal)
		if err != nil {
			return nil, fmt.Errorf("not in a git repository: %w", err)
		}
		repoRoot = strings.TrimSpace(string(output))
	}

	return &Manager{
		repoRoot: repoRoot,
	}, nil
}

// Create creates a new worktree with a new branch from HEAD.
// The name parameter is the short name (e.g., "testing")
// The directory will be created with the full name including project prefix
func (m *Manager) Create(name, branch string) error {
	return m.CreateFromRef(name, branch, "")
}

// prepareWorktreePath validates the name and returns the full worktree path,
// returning an error if the name is empty or the path already exists.
func (m *Manager) prepareWorktreePath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("worktree name cannot be empty")
	}
	wtPath := filepath.Join(filepath.Dir(m.repoRoot), m.FullName(name))
	if _, err := os.Stat(wtPath); err == nil {
		return "", fmt.Errorf("worktree already exists at %s", wtPath)
	}
	return wtPath, nil
}

// CreateFromRef creates a new worktree with a new branch starting from a specific ref.
// The name parameter is the short name (e.g., "testing")
// The branch parameter is the new branch name to create.
// The fromRef parameter is the starting point (e.g., "develop", "origin/main", a commit SHA).
// If fromRef is empty, the worktree is created from HEAD.
func (m *Manager) CreateFromRef(name, branch, fromRef string) error {
	wtPath, err := m.prepareWorktreePath(name)
	if err != nil {
		return err
	}

	// Validate the ref exists before attempting worktree creation
	if fromRef != "" {
		verifyArgs := []string{"rev-parse", "--verify", fromRef}
		if _, err := cmdexec.CombinedOutput(context.TODO(), "git", verifyArgs, m.repoRoot, cmdexec.GitLocal); err != nil {
			return fmt.Errorf("ref '%s' does not exist", fromRef)
		}
	}

	// Create worktree with new branch, optionally from a specific ref
	args := []string{"worktree", "add", "-b", branch, wtPath}
	if fromRef != "" {
		args = append(args, fromRef)
	}
	output, err := cmdexec.CombinedOutput(context.TODO(), "git", args, m.repoRoot, cmdexec.GitLocal)
	if err != nil {
		return fmt.Errorf("failed to create worktree: %s: %w", string(output), err)
	}

	return nil
}

// CreateFromExisting creates a worktree from an existing branch
// The name parameter is the short name (e.g., "testing")
// The directory will be created with the full name including project prefix
func (m *Manager) CreateFromExisting(name, branch string) error {
	wtPath, err := m.prepareWorktreePath(name)
	if err != nil {
		return err
	}

	args := []string{"worktree", "add", wtPath, branch}
	output, err := cmdexec.CombinedOutput(context.TODO(), "git", args, m.repoRoot, cmdexec.GitLocal)
	if err != nil {
		return fmt.Errorf("failed to create worktree: %s: %w", string(output), err)
	}

	return nil
}

// CreateFromBranch creates a worktree from a branch (local or remote).
// For remote branches (e.g., PR branches), it fetches and checks out the branch.
// The name parameter is the short name (e.g., "pr-123-fix-bug")
func (m *Manager) CreateFromBranch(name, branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	wtPath, err := m.prepareWorktreePath(name)
	if err != nil {
		return err
	}

	// Fetch the branch if it doesn't exist locally (important for PR branches)
	if err := cmdexec.Run(context.TODO(), "git", []string{"fetch", "origin", branch + ":" + branch}, m.repoRoot, cmdexec.GitRemote); err != nil {
		log.Printf("fetch origin %s failed (may already exist locally): %v", branch, err)
	}

	// Create worktree from the branch
	args := []string{"worktree", "add", wtPath, branch}
	output, err := cmdexec.CombinedOutput(context.TODO(), "git", args, m.repoRoot, cmdexec.GitLocal)
	if err != nil {
		return fmt.Errorf("failed to create worktree from branch %q: %s: %w", branch, string(output), err)
	}

	return nil
}

// Move renames a worktree directory using git worktree move.
// Both oldName and newName are short names (without project prefix).
func (m *Manager) Move(oldName, newName string) error {
	if oldName == "" {
		return fmt.Errorf("old worktree name cannot be empty")
	}
	if newName == "" {
		return fmt.Errorf("new worktree name cannot be empty")
	}

	// Look up the actual worktree path instead of constructing it.
	// Worktrees may live in non-standard locations (e.g. .claude/worktrees/).
	wt, err := m.Find(oldName)
	if err != nil {
		return fmt.Errorf("failed to find worktree %q: %w", oldName, err)
	}
	if wt == nil {
		return fmt.Errorf("worktree %q not found", oldName)
	}
	oldPath := wt.Path

	// Place the renamed worktree as a sibling of the original, preserving
	// the naming convention: directories that match the naming pattern get
	// the new name interpolated into the same pattern, others are renamed
	// bare (e.g. worktrees created by raw `git worktree add`).
	oldBase := filepath.Base(oldPath)
	var newBase string
	if _, matched := shortNameFromFull(oldBase, m.GetProjectName(), m.getNamePattern()); matched {
		newBase = m.FullName(newName)
	} else {
		newBase = newName
	}
	newPath := filepath.Join(filepath.Dir(oldPath), newBase)

	output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"worktree", "move", oldPath, newPath}, m.repoRoot, cmdexec.GitLocal)
	if err != nil {
		return fmt.Errorf("failed to move worktree: %s: %w", string(output), err)
	}

	return nil
}

// Find searches for a worktree by short name, display name, branch, full
// name, or path basename. Returns the worktree if found, nil if not found.
//
// Returned worktrees do not have IsDirty/DirtyFiles populated — callers that
// need dirty status should call GetDirtyFiles on the returned tree's Path.
// Skipping dirty checks here keeps Find fast in repos with many worktrees.
func (m *Manager) Find(name string) (*Worktree, error) {
	trees, err := m.listLight()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	fullName := m.FullName(name)

	for _, tree := range trees {
		// Match by short name, display name, branch, full name, or path basename
		baseName := filepath.Base(tree.Path)
		if tree.ShortName == name || tree.DisplayName() == name || tree.Branch == name || baseName == name || baseName == fullName {
			return tree, nil
		}
	}

	return nil, nil
}

// List returns all worktrees in the repository.
// Each tree's IsDirty + DirtyFiles are populated from a single git status call —
// callers that need the dirty file list should read tree.DirtyFiles rather than
// calling GetDirtyFiles a second time.
//
// In repos with many worktrees, the per-worktree dirty checks dominate cost.
// Callers that only need the porcelain metadata (path, branch, prunable) can
// use listLight or higher-level helpers (Find, ListNames) to skip them.
func (m *Manager) List() ([]*Worktree, error) {
	trees, err := m.listLight()
	if err != nil {
		return nil, err
	}

	// Check dirty status in parallel — each goroutine writes to its own struct.
	// Cache the file list too so callers don't re-run git status to get it.
	var wg sync.WaitGroup
	for _, tree := range trees {
		if tree.IsPrunable {
			continue
		}
		wg.Add(1)
		go func(t *Worktree) {
			defer wg.Done()
			files, err := m.getDirtyFiles(t.Path)
			if err != nil {
				log.Printf("warning: dirty check failed for %q: %v", t.Path, err)
				t.DirtyCheckFailed = true
				return
			}
			t.DirtyFiles = files
			t.IsDirty = files != ""
		}(tree)
	}
	wg.Wait()

	return trees, nil
}

// listLight returns worktrees parsed from `git worktree list --porcelain`
// without the per-worktree dirty status check. Returned worktrees have all
// porcelain-derived fields (Path, Name, ShortName, Branch, Commit, IsMain,
// IsPrunable) but IsDirty/DirtyFiles/DirtyCheckFailed remain at zero values.
//
// Use for callers that need the worktree set but not dirty state — much
// faster in repos with many worktrees because it skips N parallel git status
// invocations.
func (m *Manager) listLight() ([]*Worktree, error) {
	output, err := cmdexec.Output(context.TODO(), "git", []string{"worktree", "list", "--porcelain"}, m.repoRoot, cmdexec.GitLocal)
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	// The main worktree is always the first `worktree <path>` line —
	// extract it from the output we already have rather than running
	// `git worktree list --porcelain` a second time via getMainWorktreePath.
	raw := string(output)
	mainPath := mainWorktreePathFromPorcelain(raw)
	if mainPath == "" {
		mainPath = m.repoRoot
	}
	return parseWorktreeList(raw, mainPath, m.GetProjectName(), m.getNamePattern()), nil
}

// mainWorktreePathFromPorcelain returns the first worktree path in the output
// of `git worktree list --porcelain`, which is always the main worktree.
// Returns the empty string when no worktree line is found.
func mainWorktreePathFromPorcelain(porcelain string) string {
	for _, line := range strings.Split(porcelain, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			return strings.TrimPrefix(line, "worktree ")
		}
	}
	return ""
}

// CurrentPath returns the absolute path of the worktree containing the current
// working directory. Unlike GetCurrent, it does NOT call List() — use this when
// you only need the path and the trees list either isn't needed or is already
// in hand.
func (m *Manager) CurrentPath() (string, error) {
	output, err := cmdexec.Output(context.TODO(), "git", []string{"rev-parse", "--show-toplevel"}, "", cmdexec.GitLocal)
	if err != nil {
		return "", fmt.Errorf("failed to get current worktree path: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// PathForName returns the canonical filesystem path for a worktree of the
// given short name, whether it exists or not. Used by callers that have
// already created (or are about to create) a worktree at the standard
// location and want to avoid re-running List().
func (m *Manager) PathForName(name string) string {
	return filepath.Join(filepath.Dir(m.repoRoot), m.FullName(name))
}

// PathExists reports whether a worktree directory already exists at the
// canonical path for `name`. Cheap (single os.Stat) — prefer this over Find
// for "does this name collide?" checks where the standard location is enough.
//
// Doesn't catch worktrees registered with git but missing from disk
// (prunable) or worktrees living at non-standard paths; Create() will surface
// those as a git-level error if encountered.
func (m *Manager) PathExists(name string) bool {
	_, err := os.Stat(m.PathForName(name))
	return err == nil
}

// DisplayNameForPath returns the display name a worktree at `path` would have
// (e.g. "root" for the main worktree, or the short name with project prefix
// stripped for everything else). No git calls — pure string manipulation
// against the configured project name and repo root.
func (m *Manager) DisplayNameForPath(path string) string {
	if path == "" {
		return ""
	}
	if path == m.repoRoot {
		return "root"
	}
	return m.ShortName(filepath.Base(path))
}

// ListNames returns display names for all worktrees without running dirty checks.
// This is significantly faster than List() and suitable for tab completion.
func (m *Manager) ListNames() ([]string, error) {
	trees, err := m.listLight()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(trees))
	for _, t := range trees {
		names = append(names, t.DisplayName())
	}
	return names, nil
}

// Remove removes a worktree
func (m *Manager) Remove(name string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	// Find the worktree by name. listLight skips per-worktree dirty checks
	// — Remove only needs the target's path/IsMain/IsPrunable to decide what
	// to do.
	trees, err := m.listLight()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	var targetTree *Worktree
	for _, tree := range trees {
		if tree.Name == name || tree.ShortName == name || tree.DisplayName() == name {
			targetTree = tree
			break
		}
	}

	if targetTree == nil {
		return fmt.Errorf("worktree '%s' not found", name)
	}

	// Defense in depth: refuse to remove the main worktree. Callers (rm/clean/TUI)
	// already filter this out, but our os.RemoveAll fallback below would happily
	// wipe the main checkout if a future caller forgot the guard.
	if targetTree.IsMain {
		return fmt.Errorf("cannot remove main worktree '%s'", name)
	}

	// If the worktree is prunable (directory missing), use git worktree prune
	if targetTree.IsPrunable {
		output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"worktree", "prune"}, m.repoRoot, cmdexec.GitLocal)
		if err != nil {
			return fmt.Errorf("failed to prune stale worktree: %s: %w", string(output), err)
		}
		return nil
	}

	// Remove the worktree normally
	if _, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"worktree", "remove", targetTree.Path}, m.repoRoot, cmdexec.GitLocal); err == nil {
		return nil
	}

	// Try git's own --force first
	forceOutput, forceErr := cmdexec.CombinedOutput(context.TODO(), "git", []string{"worktree", "remove", "--force", targetTree.Path}, m.repoRoot, cmdexec.GitLocal)
	if forceErr == nil {
		return nil
	}

	// Final fallback: nuke the directory ourselves and prune git's metadata.
	// `git worktree remove --force` refuses to delete non-empty untracked
	// directories (e.g. node_modules left behind by a post-create hook),
	// so when git has already given up we tear the directory down directly.
	// targetTree.Path comes from `git worktree list`, so it is bounded to a
	// known worktree path rather than user input.
	if rmErr := os.RemoveAll(targetTree.Path); rmErr != nil {
		return fmt.Errorf("failed to remove worktree: %s: %w", string(forceOutput), forceErr)
	}
	if pruneOutput, pruneErr := cmdexec.CombinedOutput(context.TODO(), "git", []string{"worktree", "prune"}, m.repoRoot, cmdexec.GitLocal); pruneErr != nil {
		return fmt.Errorf("removed worktree directory but failed to prune git metadata: %s: %w", string(pruneOutput), pruneErr)
	}

	return nil
}

// GetCurrent returns the current worktree, enriched with commit info and
// the dirty file list for just that worktree.
//
// Performs O(1) dirty checks instead of O(N) — only the current worktree is
// inspected, not every worktree in the repo. Significantly faster than the
// pre-refactor implementation in repos with many worktrees.
func (m *Manager) GetCurrent() (*Worktree, error) {
	currentPath, err := m.CurrentPath()
	if err != nil {
		return nil, err
	}

	trees, err := m.listLight()
	if err != nil {
		return nil, err
	}

	for _, tree := range trees {
		if tree.Path != currentPath {
			continue
		}
		shortHash, message, age, err := m.getCommitInfo(tree.Path)
		if err == nil {
			tree.ShortCommit = shortHash
			tree.CommitMessage = message
			tree.CommitAge = age
		}
		files, dirtyErr := m.getDirtyFiles(tree.Path)
		if dirtyErr != nil {
			tree.DirtyCheckFailed = true
		} else {
			tree.DirtyFiles = files
			tree.IsDirty = files != ""
		}
		return tree, nil
	}

	return nil, fmt.Errorf("current worktree not found")
}

// GetDirtyFiles returns the list of dirty files from git status.
func (m *Manager) GetDirtyFiles(path string) (string, error) {
	return m.getDirtyFiles(path)
}

// getDirtyFiles returns the list of dirty files from git status
func (m *Manager) getDirtyFiles(path string) (string, error) {
	output, err := cmdexec.Output(context.TODO(), "git", []string{"status", "--porcelain"}, path, cmdexec.GitLocal)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// GetCommitInfo retrieves detailed commit information for a worktree.
func (m *Manager) GetCommitInfo(path string) (shortHash, message, age string, err error) {
	return m.getCommitInfo(path)
}

// getCommitInfo retrieves detailed commit information for a worktree
func (m *Manager) getCommitInfo(path string) (shortHash, message, age string, err error) {
	// Use a delimiter that's unlikely to appear in commit messages
	// Format: full_hash<delim>short_hash<delim>subject<delim>relative_date
	output, err := cmdexec.Output(context.TODO(), "git", []string{"log", "-1", "--format=%H%x1E%h%x1E%s%x1E%cr"}, path, cmdexec.GitLocal)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get commit info: %w", err)
	}

	// Split by ASCII Record Separator (0x1E) which is safe for commit messages
	parts := strings.Split(strings.TrimSpace(string(output)), "\x1E")
	if len(parts) < gitLogFormatParts {
		return "", "", "", fmt.Errorf("unexpected git log output format")
	}

	return parts[1], parts[2], parts[3], nil
}

// newWorktreeEntry creates a Worktree from a "worktree <path>" porcelain line.
func newWorktreeEntry(path, mainPath, projectName, namePattern string) *Worktree {
	name := filepath.Base(path)
	isMain := (path == mainPath)

	shortName := name
	if !isMain {
		shortName, _ = shortNameFromFull(name, projectName, namePattern)
	}

	return &Worktree{
		Path:      path,
		Name:      name,
		IsMain:    isMain,
		ShortName: shortName,
	}
}

// applyWorktreeAttribute sets a field on wt from a porcelain attribute line.
func applyWorktreeAttribute(wt *Worktree, line string) {
	switch {
	case strings.HasPrefix(line, "HEAD "):
		wt.Commit = strings.TrimPrefix(line, "HEAD ")
	case strings.HasPrefix(line, "branch "):
		branch := strings.TrimPrefix(line, "branch ")
		wt.Branch = strings.TrimPrefix(branch, "refs/heads/")
	case strings.HasPrefix(line, "detached"):
		wt.Branch = "detached"
	case strings.HasPrefix(line, "prunable"):
		wt.IsPrunable = true
	}
}

// parseWorktreeList parses the output of 'git worktree list --porcelain'
func parseWorktreeList(output, mainPath, projectName, namePattern string) []*Worktree {
	var trees []*Worktree
	var current *Worktree

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		if line == "" {
			if current != nil {
				trees = append(trees, current)
				current = nil
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current = newWorktreeEntry(strings.TrimPrefix(line, "worktree "), mainPath, projectName, namePattern)
		} else if current != nil {
			applyWorktreeAttribute(current, line)
		}
	}

	if current != nil {
		trees = append(trees, current)
	}

	return trees
}

// DisplayName returns the display name for a worktree
// Root worktree returns "root", others return short name without project prefix
func (w *Worktree) DisplayName() string {
	if w.IsMain {
		return "root"
	}
	return w.ShortName
}

// GetProjectName returns the project name for the repository
func (m *Manager) GetProjectName() string {
	if m.projectName != "" {
		return m.projectName
	}

	m.projectName = m.detectProjectName()
	return m.projectName
}

// TmuxSessionName returns the tmux session name for a worktree.
// The main/root worktree uses just the project name (e.g. "myapp"),
// while branch worktrees use project-name (e.g. "myapp-testing").
func TmuxSessionName(project, worktreeName string) string {
	if worktreeName == "root" {
		return project
	}
	return fmt.Sprintf("%s-%s", project, worktreeName)
}

// GetRepoRoot returns the repository root path
func (m *Manager) GetRepoRoot() string {
	return m.repoRoot
}
