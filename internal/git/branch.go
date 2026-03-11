package git

import (
	"context"
	"fmt"
	"strings"

	"github.com/lost-in-the/grove/internal/cmdexec"
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

	output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", b.repoPath, "branch", flag, branch}, "", cmdexec.GitLocal)
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

	output, err := cmdexec.Output(context.TODO(), "git", args, "", cmdexec.GitLocal)
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
	output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", repoPath, "branch", "--format=%(refname:short)"}, "", cmdexec.GitLocal)
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
	output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", b.repoPath, "branch", "--merged", b.defaultBranch}, "", cmdexec.GitLocal)
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
	output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", b.repoPath, "rev-parse", "--abbrev-ref",
		fmt.Sprintf("%s@{upstream}", branch)}, "", cmdexec.GitLocal)
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
	output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", b.repoPath, "rev-list", "--count",
		fmt.Sprintf("%s..%s", upstream, branch)}, "", cmdexec.GitLocal)
	if err != nil {
		return 0, err
	}

	count := 0
	_, _ = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count, nil
}

// branchUsedByWorktree returns the worktree path using this branch, or empty string
func (b *BranchManager) branchUsedByWorktree(branch string, excludeWorktree string) (string, error) {
	output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", b.repoPath, "worktree", "list", "--porcelain"}, "", cmdexec.GitLocal)
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
	output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", repoPath, "symbolic-ref", "refs/remotes/origin/HEAD"}, "", cmdexec.GitLocal)
	if err == nil {
		ref := strings.TrimSpace(string(output))
		// Extract branch name from refs/remotes/origin/main
		if branchName, found := strings.CutPrefix(ref, "refs/remotes/origin/"); found {
			return branchName, nil
		}
	}

	// Try git config init.defaultBranch
	output, err = cmdexec.Output(context.TODO(), "git", []string{"-C", repoPath, "config", "init.defaultBranch"}, "", cmdexec.GitLocal)
	if err == nil {
		candidate := strings.TrimSpace(string(output))
		if candidate != "" {
			if err := cmdexec.Run(context.TODO(), "git", []string{"-C", repoPath, "rev-parse", "--verify", candidate}, "", cmdexec.GitLocal); err == nil {
				return candidate, nil
			}
		}
	}

	// Fall back to checking common default branch names
	for _, candidate := range []string{"main", "master"} {
		if err := cmdexec.Run(context.TODO(), "git", []string{"-C", repoPath, "rev-parse", "--verify", candidate}, "", cmdexec.GitLocal); err == nil {
			return candidate, nil
		}
	}

	// Last resort: use the current branch (skip if detached HEAD)
	output, err = cmdexec.Output(context.TODO(), "git", []string{"-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD"}, "", cmdexec.GitLocal)
	if err != nil {
		return "", fmt.Errorf("could not determine default branch")
	}
	branch := strings.TrimSpace(string(output))
	if branch != "" && branch != "HEAD" {
		return branch, nil
	}
	return "", fmt.Errorf("could not determine default branch")
}
