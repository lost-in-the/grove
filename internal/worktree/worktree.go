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

	oldPath := filepath.Join(filepath.Dir(m.repoRoot), m.FullName(oldName))
	newPath := filepath.Join(filepath.Dir(m.repoRoot), m.FullName(newName))

	output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"worktree", "move", oldPath, newPath}, m.repoRoot, cmdexec.GitLocal)
	if err != nil {
		return fmt.Errorf("failed to move worktree: %s: %w", string(output), err)
	}

	return nil
}

// Find searches for a worktree by short name or full name
// Returns the worktree if found, nil if not found
func (m *Manager) Find(name string) (*Worktree, error) {
	trees, err := m.List()
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

// List returns all worktrees in the repository
func (m *Manager) List() ([]*Worktree, error) {
	output, err := cmdexec.Output(context.TODO(), "git", []string{"worktree", "list", "--porcelain"}, m.repoRoot, cmdexec.GitLocal)
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	projectName := m.GetProjectName()
	mainPath := m.getMainWorktreePath()
	trees := parseWorktreeList(string(output), mainPath, projectName)

	// Check dirty status in parallel — each goroutine writes to its own struct
	var wg sync.WaitGroup
	for _, tree := range trees {
		wg.Add(1)
		go func(t *Worktree) {
			defer wg.Done()
			dirty, err := m.isDirty(t.Path)
			if err != nil {
				log.Printf("warning: dirty check failed for %q: %v", t.Path, err)
				t.DirtyCheckFailed = true
				return
			}
			t.IsDirty = dirty
		}(tree)
	}
	wg.Wait()

	return trees, nil
}

// Remove removes a worktree
func (m *Manager) Remove(name string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	// Find the worktree by name
	trees, err := m.List()
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

	// If the worktree is prunable (directory missing), use git worktree prune
	if targetTree.IsPrunable {
		output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"worktree", "prune"}, m.repoRoot, cmdexec.GitLocal)
		if err != nil {
			return fmt.Errorf("failed to prune stale worktree: %s: %w", string(output), err)
		}
		return nil
	}

	// Remove the worktree normally
	_, err = cmdexec.CombinedOutput(context.TODO(), "git", []string{"worktree", "remove", targetTree.Path}, m.repoRoot, cmdexec.GitLocal)
	if err != nil {
		// Try force remove if regular remove fails
		output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"worktree", "remove", "--force", targetTree.Path}, m.repoRoot, cmdexec.GitLocal)
		if err != nil {
			return fmt.Errorf("failed to remove worktree: %s: %w", string(output), err)
		}
	}

	return nil
}

// GetCurrent returns the current worktree
func (m *Manager) GetCurrent() (*Worktree, error) {
	output, err := cmdexec.Output(context.TODO(), "git", []string{"rev-parse", "--show-toplevel"}, "", cmdexec.GitLocal)
	if err != nil {
		return nil, fmt.Errorf("failed to get current worktree: %w", err)
	}

	currentPath := strings.TrimSpace(string(output))

	trees, err := m.List()
	if err != nil {
		return nil, err
	}

	for _, tree := range trees {
		if tree.Path == currentPath {
			// Enrich with commit information
			shortHash, message, age, err := m.getCommitInfo(tree.Path)
			if err == nil {
				tree.ShortCommit = shortHash
				tree.CommitMessage = message
				tree.CommitAge = age
			}

			// Get dirty files if the worktree is dirty
			if tree.IsDirty {
				dirtyFiles, err := m.getDirtyFiles(tree.Path)
				if err == nil {
					tree.DirtyFiles = dirtyFiles
				}
			}

			return tree, nil
		}
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

// isDirty checks if a worktree has uncommitted changes
func (m *Manager) isDirty(path string) (bool, error) {
	dirtyFiles, err := m.getDirtyFiles(path)
	if err != nil {
		return false, err
	}

	return len(dirtyFiles) > 0, nil
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
func newWorktreeEntry(path, mainPath, projectName string) *Worktree {
	name := filepath.Base(path)
	isMain := (path == mainPath)

	shortName := name
	if !isMain {
		prefix := projectName + "-"
		if strings.HasPrefix(name, prefix) {
			shortName = strings.TrimPrefix(name, prefix)
		}
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
func parseWorktreeList(output, mainPath, projectName string) []*Worktree {
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
			current = newWorktreeEntry(strings.TrimPrefix(line, "worktree "), mainPath, projectName)
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
