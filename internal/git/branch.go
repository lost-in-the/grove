package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// BranchStatus contains information about a git branch's state
type BranchStatus struct {
	Name           string // Branch name
	IsMerged       bool   // Whether branch is merged into default branch
	HasRemote      bool   // Whether branch has a remote tracking branch
	UnpushedCount  int    // Number of commits not pushed to remote
	UsedByWorktree string // Path to worktree using this branch (empty if none)
}

// BranchManager provides operations on git branches
type BranchManager struct {
	repoPath      string
	defaultBranch string
}

// NewBranchManager creates a new branch manager for the given repository
func NewBranchManager(repoPath string) (*BranchManager, error) {
	// Detect default branch
	defaultBranch, err := detectDefaultBranch(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect default branch: %w", err)
	}

	return &BranchManager{
		repoPath:      repoPath,
		defaultBranch: defaultBranch,
	}, nil
}

// GetStatus returns the status of a branch, optionally excluding a worktree from the "used by" check
func (b *BranchManager) GetStatus(branch string, excludeWorktree string) (*BranchStatus, error) {
	status := &BranchStatus{
		Name: branch,
	}

	// Check if branch is merged into default branch
	merged, err := b.isBranchMerged(branch)
	if err != nil {
		return nil, fmt.Errorf("failed to check merge status: %w", err)
	}
	status.IsMerged = merged

	// Check if branch has a remote tracking branch
	hasRemote, err := b.hasRemoteTracking(branch)
	if err != nil {
		return nil, fmt.Errorf("failed to check remote tracking: %w", err)
	}
	status.HasRemote = hasRemote

	// Count unpushed commits if there's a remote
	if hasRemote {
		count, err := b.countUnpushedCommits(branch)
		if err != nil {
			// Non-fatal - might not have upstream set
			count = 0
		}
		status.UnpushedCount = count
	}

	// Check if branch is used by another worktree
	worktreePath, err := b.branchUsedByWorktree(branch, excludeWorktree)
	if err != nil {
		return nil, fmt.Errorf("failed to check worktree usage: %w", err)
	}
	status.UsedByWorktree = worktreePath

	return status, nil
}

// Delete removes a branch, optionally forcing deletion of unmerged branches
func (b *BranchManager) Delete(branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}

	cmd := exec.Command("git", "-C", b.repoPath, "branch", flag, branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete branch: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

// GetUnpushedCommits returns a list of commit summaries not pushed to remote
func (b *BranchManager) GetUnpushedCommits(branch string, limit int) ([]string, error) {
	// Get upstream tracking branch
	upstream, err := b.getUpstream(branch)
	if err != nil || upstream == "" {
		return nil, nil
	}

	// Get commits between upstream and local
	args := []string{"-C", b.repoPath, "log", "--oneline",
		fmt.Sprintf("%s..%s", upstream, branch)}
	if limit > 0 {
		args = append(args, fmt.Sprintf("-%d", limit))
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}

// ListLocalBranches returns the names of all local branches.
func ListLocalBranches(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "branch", "--format=%(refname:short)")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}
	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}

// isBranchMerged checks if a branch is fully merged into the default branch
func (b *BranchManager) isBranchMerged(branch string) (bool, error) {
	// List branches merged into default branch
	cmd := exec.Command("git", "-C", b.repoPath, "branch", "--merged", b.defaultBranch)
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	// Check if our branch is in the list
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		name := strings.TrimSpace(line)
		name, _ = strings.CutPrefix(name, "* ")
		if name == branch {
			return true, nil
		}
	}
	return false, nil
}

// hasRemoteTracking checks if a branch has a remote tracking branch
func (b *BranchManager) hasRemoteTracking(branch string) (bool, error) {
	upstream, err := b.getUpstream(branch)
	return upstream != "", err
}

// getUpstream returns the upstream tracking branch for a local branch
func (b *BranchManager) getUpstream(branch string) (string, error) {
	cmd := exec.Command("git", "-C", b.repoPath, "rev-parse", "--abbrev-ref",
		fmt.Sprintf("%s@{upstream}", branch))
	output, err := cmd.Output()
	if err != nil {
		// No upstream set - not an error
		return "", nil
	}
	return strings.TrimSpace(string(output)), nil
}

// countUnpushedCommits returns the number of commits not pushed to upstream
func (b *BranchManager) countUnpushedCommits(branch string) (int, error) {
	upstream, err := b.getUpstream(branch)
	if err != nil || upstream == "" {
		return 0, nil
	}

	// Count commits ahead of upstream
	cmd := exec.Command("git", "-C", b.repoPath, "rev-list", "--count",
		fmt.Sprintf("%s..%s", upstream, branch))
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	count := 0
	_, _ = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count, nil
}

// branchUsedByWorktree returns the worktree path using this branch, or empty string
func (b *BranchManager) branchUsedByWorktree(branch string, excludeWorktree string) (string, error) {
	cmd := exec.Command("git", "-C", b.repoPath, "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse worktree list output
	var currentWorktree string
	var currentBranch string

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if path, found := strings.CutPrefix(line, "worktree "); found {
			currentWorktree = path
		} else if ref, found := strings.CutPrefix(line, "branch refs/heads/"); found {
			currentBranch = ref
		} else if line == "" {
			// End of a worktree entry
			if currentBranch == branch && currentWorktree != excludeWorktree {
				return currentWorktree, nil
			}
			currentWorktree = ""
			currentBranch = ""
		}
	}

	return "", nil
}

// detectDefaultBranch attempts to determine the default branch for a repository
func detectDefaultBranch(repoPath string) (string, error) {
	// Try to get from remote HEAD reference
	cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "refs/remotes/origin/HEAD")
	output, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(output))
		// Extract branch name from refs/remotes/origin/main
		if branchName, found := strings.CutPrefix(ref, "refs/remotes/origin/"); found {
			return branchName, nil
		}
	}

	// Try git config init.defaultBranch
	cmd = exec.Command("git", "-C", repoPath, "config", "init.defaultBranch")
	if output, err = cmd.Output(); err == nil {
		candidate := strings.TrimSpace(string(output))
		if candidate != "" {
			verify := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", candidate)
			if verify.Run() == nil {
				return candidate, nil
			}
		}
	}

	// Fall back to checking common default branch names
	for _, candidate := range []string{"main", "master"} {
		cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", candidate)
		if err := cmd.Run(); err == nil {
			return candidate, nil
		}
	}

	// Last resort: use the current branch (skip if detached HEAD)
	cmd = exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	output, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("could not determine default branch")
	}
	branch := strings.TrimSpace(string(output))
	if branch != "" && branch != "HEAD" {
		return branch, nil
	}
	return "", fmt.Errorf("could not determine default branch")
}
