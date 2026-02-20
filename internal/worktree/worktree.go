package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// gitLogFormatParts is the expected number of parts in git log output
	gitLogFormatParts = 4
)

// Worktree represents a git worktree
type Worktree struct {
	Name          string // Short name (derived from path)
	Path          string // Absolute path to worktree
	Branch        string // Branch name or "detached"
	Commit        string // Commit hash (full)
	ShortCommit   string // Short commit hash (7 chars)
	CommitMessage string // Commit message subject
	CommitAge     string // Relative commit time
	IsDirty       bool   // Whether there are uncommitted changes
	DirtyFiles    string // List of dirty files (from git status --porcelain)
	IsMain        bool   // Whether this is the main worktree
	ShortName     string // Short name without project prefix
	IsPrunable    bool   // Whether the worktree directory is missing (stale)
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
		cmd := exec.Command("git", "rev-parse", "--show-toplevel")
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("not in a git repository: %w", err)
		}
		repoRoot = strings.TrimSpace(string(output))
	}

	return &Manager{
		repoRoot: repoRoot,
	}, nil
}

// Create creates a new worktree
// The name parameter is the short name (e.g., "testing")
// The directory will be created with the full name including project prefix
func (m *Manager) Create(name, branch string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	// Get full name with project prefix
	fullName := m.FullName(name)

	// Worktree path is relative to repo root's parent
	wtPath := filepath.Join(filepath.Dir(m.repoRoot), fullName)

	// Check if worktree already exists
	if _, err := os.Stat(wtPath); err == nil {
		return fmt.Errorf("worktree already exists at %s", wtPath)
	}

	// Create worktree with new branch
	args := []string{"worktree", "add", "-b", branch, wtPath}
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create worktree: %s: %w", string(output), err)
	}

	return nil
}

// CreateFromExisting creates a worktree from an existing branch
// The name parameter is the short name (e.g., "testing")
// The directory will be created with the full name including project prefix
func (m *Manager) CreateFromExisting(name, branch string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	// Get full name with project prefix
	fullName := m.FullName(name)

	wtPath := filepath.Join(filepath.Dir(m.repoRoot), fullName)

	// Check if worktree already exists
	if _, err := os.Stat(wtPath); err == nil {
		return fmt.Errorf("worktree already exists at %s", wtPath)
	}

	// Create worktree from existing branch (no -b flag)
	args := []string{"worktree", "add", wtPath, branch}
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create worktree: %s: %w", string(output), err)
	}

	return nil
}

// CreateFromBranch creates a worktree from a branch (local or remote).
// For remote branches (e.g., PR branches), it fetches and checks out the branch.
// The name parameter is the short name (e.g., "pr-123-fix-bug")
func (m *Manager) CreateFromBranch(name, branch string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}
	if branch == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// Get full name with project prefix
	fullName := m.FullName(name)
	wtPath := filepath.Join(filepath.Dir(m.repoRoot), fullName)

	// Check if worktree already exists
	if _, err := os.Stat(wtPath); err == nil {
		return fmt.Errorf("worktree already exists at %s", wtPath)
	}

	// First, try to fetch the branch if it doesn't exist locally
	// This is important for PR branches that only exist remotely
	fetchCmd := exec.Command("git", "fetch", "origin", branch+":"+branch)
	fetchCmd.Dir = m.repoRoot
	_ = fetchCmd.Run() // Ignore errors - branch might already exist locally

	// Create worktree from the branch
	args := []string{"worktree", "add", wtPath, branch}
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create worktree from branch %q: %s: %w", branch, string(output), err)
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
		// Match by short name, display name, full name, or path basename
		baseName := filepath.Base(tree.Path)
		if tree.ShortName == name || tree.DisplayName() == name || baseName == name || baseName == fullName {
			return tree, nil
		}
	}

	return nil, nil
}

// List returns all worktrees in the repository
func (m *Manager) List() ([]*Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	projectName := m.GetProjectName()
	mainPath := m.getMainWorktreePath()
	trees := parseWorktreeList(string(output), mainPath, projectName)

	// Check dirty status for each worktree
	for _, tree := range trees {
		dirty, err := m.isDirty(tree.Path)
		if err == nil {
			tree.IsDirty = dirty
		}
	}

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
		cmd := exec.Command("git", "worktree", "prune")
		cmd.Dir = m.repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to prune stale worktree: %s: %w", string(output), err)
		}
		return nil
	}

	// Remove the worktree normally
	cmd := exec.Command("git", "worktree", "remove", targetTree.Path)
	cmd.Dir = m.repoRoot

	_, err = cmd.CombinedOutput()
	if err != nil {
		// Try force remove if regular remove fails
		cmd = exec.Command("git", "worktree", "remove", "--force", targetTree.Path)
		cmd.Dir = m.repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to remove worktree: %s: %w", string(output), err)
		}
	}

	return nil
}

// GetCurrent returns the current worktree
func (m *Manager) GetCurrent() (*Worktree, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
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
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = path

	output, err := cmd.Output()
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
	cmd := exec.Command("git", "log", "-1", "--format=%H%x1E%h%x1E%s%x1E%cr")
	cmd.Dir = path

	output, err := cmd.Output()
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

// parseWorktreeList parses the output of 'git worktree list --porcelain'
func parseWorktreeList(output, mainPath, projectName string) []*Worktree {
	var trees []*Worktree
	var current *Worktree

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				trees = append(trees, current)
				current = nil
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			name := filepath.Base(path)
			isMain := (path == mainPath)

			// Extract short name by removing project prefix
			shortName := name
			if !isMain {
				prefix := projectName + "-"
				if strings.HasPrefix(name, prefix) {
					shortName = strings.TrimPrefix(name, prefix)
				}
			}

			current = &Worktree{
				Path:      path,
				Name:      name,
				IsMain:    isMain,
				ShortName: shortName,
			}
		} else if strings.HasPrefix(line, "HEAD ") {
			if current != nil {
				current.Commit = strings.TrimPrefix(line, "HEAD ")
			}
		} else if strings.HasPrefix(line, "branch ") {
			if current != nil {
				branch := strings.TrimPrefix(line, "branch ")
				// Remove refs/heads/ prefix
				branch = strings.TrimPrefix(branch, "refs/heads/")
				current.Branch = branch
			}
		} else if strings.HasPrefix(line, "detached") {
			if current != nil {
				current.Branch = "detached"
			}
		} else if strings.HasPrefix(line, "prunable") {
			if current != nil {
				current.IsPrunable = true
			}
		}
	}

	// Don't forget the last worktree
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

// TmuxSessionName returns the tmux session name for a worktree
// Format: {project}-{worktree-name}
func TmuxSessionName(project, worktreeName string) string {
	return fmt.Sprintf("%s-%s", project, worktreeName)
}

// GetRepoRoot returns the repository root path
func (m *Manager) GetRepoRoot() string {
	return m.repoRoot
}
