package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree represents a git worktree
type Worktree struct {
	Name    string // Short name (derived from path)
	Path    string // Absolute path to worktree
	Branch  string // Branch name or "detached"
	Commit  string // Commit hash
	IsDirty bool   // Whether there are uncommitted changes
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
func (m *Manager) Create(name, branch string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	// Worktree path is relative to repo root's parent
	wtPath := filepath.Join(filepath.Dir(m.repoRoot), name)

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
func (m *Manager) CreateFromExisting(name, branch string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	wtPath := filepath.Join(filepath.Dir(m.repoRoot), name)

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

// List returns all worktrees in the repository
func (m *Manager) List() ([]*Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	trees := parseWorktreeList(string(output))

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

	var targetPath string
	for _, tree := range trees {
		if tree.Name == name {
			targetPath = tree.Path
			break
		}
	}

	if targetPath == "" {
		return fmt.Errorf("worktree '%s' not found", name)
	}

	// Remove the worktree
	cmd := exec.Command("git", "worktree", "remove", targetPath)
	cmd.Dir = m.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try force remove if regular remove fails
		cmd = exec.Command("git", "worktree", "remove", "--force", targetPath)
		cmd.Dir = m.repoRoot
		output, err = cmd.CombinedOutput()
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
			return tree, nil
		}
	}

	return nil, fmt.Errorf("current worktree not found")
}

// isDirty checks if a worktree has uncommitted changes
func (m *Manager) isDirty(path string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = path

	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

// parseWorktreeList parses the output of 'git worktree list --porcelain'
func parseWorktreeList(output string) []*Worktree {
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
			current = &Worktree{
				Path: path,
				Name: filepath.Base(path),
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
		}
	}

	// Don't forget the last worktree
	if current != nil {
		trees = append(trees, current)
	}

	return trees
}

// GetProjectName returns the project name for the repository
func (m *Manager) GetProjectName() string {
	if m.projectName != "" {
		return m.projectName
	}

	// Try to get project name from git remote
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = m.repoRoot
	output, err := cmd.Output()
	
	remoteURL := ""
	if err == nil {
		remoteURL = strings.TrimSpace(string(output))
	}

	// Fallback to directory name
	dirName := filepath.Base(m.repoRoot)
	
	m.projectName = getProjectName(remoteURL, dirName)
	return m.projectName
}

// getProjectName extracts project name from git remote URL or falls back to directory name
func getProjectName(remoteURL, dirName string) string {
	if remoteURL == "" {
		return dirName
	}

	// Remove .git suffix if present
	remoteURL = strings.TrimSuffix(remoteURL, ".git")
	
	// Extract repo name from URL
	// Handles: https://github.com/owner/repo, git@github.com:owner/repo.git
	var repoName string
	
	// Try SSH format first: git@github.com:owner/repo
	if strings.Contains(remoteURL, ":") && !strings.HasPrefix(remoteURL, "http") {
		parts := strings.SplitN(remoteURL, ":", 2)
		if len(parts) == 2 && parts[1] != "" {
			// Extract last path component
			pathParts := strings.Split(parts[1], "/")
			if len(pathParts) > 0 {
				repoName = pathParts[len(pathParts)-1]
			}
		}
	} else if strings.Contains(remoteURL, "/") {
		// Handle HTTP(S) format: https://github.com/owner/repo
		parts := strings.Split(remoteURL, "/")
		if len(parts) > 0 {
			repoName = parts[len(parts)-1]
		}
	}
	
	// Fallback to directory name if extraction failed
	if repoName == "" {
		return dirName
	}
	
	return repoName
}

// TmuxSessionName returns the tmux session name for a worktree
// Format: {project}-{worktree-name}
func TmuxSessionName(project, worktreeName string) string {
	return fmt.Sprintf("%s-%s", project, worktreeName)
}
