// Package worktree provides WIP (Work-In-Progress) handling utilities.
package worktree

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// WIPHandler handles work-in-progress detection and manipulation.
type WIPHandler struct {
	repoPath string
}

// NewWIPHandler creates a new WIP handler for the given repository path.
func NewWIPHandler(repoPath string) *WIPHandler {
	return &WIPHandler{repoPath: repoPath}
}

// HasWIP checks if there are any uncommitted changes (staged or unstaged).
func (h *WIPHandler) HasWIP() (bool, error) {
	cmd := exec.Command("git", "-C", h.repoPath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}
	return len(bytes.TrimSpace(output)) > 0, nil
}

// ListWIPFiles returns a list of files with uncommitted changes.
func (h *WIPHandler) ListWIPFiles() ([]string, error) {
	cmd := exec.Command("git", "-C", h.repoPath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list WIP files: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		if len(line) >= 3 {
			// Status format: XY filename (where XY is 2 chars + space)
			files = append(files, strings.TrimSpace(line[3:]))
		}
	}
	return files, nil
}

// Stash saves uncommitted changes to the stash with the given message.
func (h *WIPHandler) Stash(message string) error {
	args := []string{"-C", h.repoPath, "stash", "push", "--include-untracked"}
	if message != "" {
		args = append(args, "-m", message)
	}
	cmd := exec.Command("git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stash changes: %w\n%s", err, output)
	}
	return nil
}

// PopStash applies and removes the most recent stash entry.
func (h *WIPHandler) PopStash() error {
	cmd := exec.Command("git", "-C", h.repoPath, "stash", "pop")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pop stash: %w\n%s", err, output)
	}
	return nil
}

// CreatePatch creates a patch file from all uncommitted changes (staged and unstaged).
func (h *WIPHandler) CreatePatch() ([]byte, error) {
	// First, add all untracked files to index temporarily
	addCmd := exec.Command("git", "-C", h.repoPath, "add", "--all")
	if output, err := addCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to stage files: %w\n%s", err, output)
	}

	// Create patch from staged changes
	diffCmd := exec.Command("git", "-C", h.repoPath, "diff", "--cached", "--binary")
	patch, err := diffCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to create patch: %w", err)
	}

	// Reset staging area (keep working tree changes)
	resetCmd := exec.Command("git", "-C", h.repoPath, "reset", "HEAD")
	if output, err := resetCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to reset staging: %w\n%s", err, output)
	}

	return patch, nil
}

// ApplyPatch applies a patch to the working tree.
func (h *WIPHandler) ApplyPatch(patch []byte) error {
	if len(patch) == 0 {
		return nil
	}

	cmd := exec.Command("git", "-C", h.repoPath, "apply", "--3way")
	cmd.Stdin = bytes.NewReader(patch)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to apply patch: %w\n%s", err, output)
	}
	return nil
}

// HasStagedChanges checks if there are any staged changes.
func (h *WIPHandler) HasStagedChanges() (bool, error) {
	cmd := exec.Command("git", "-C", h.repoPath, "diff", "--cached", "--quiet")
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 means there are differences (staged changes exist)
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, fmt.Errorf("failed to check staged changes: %w", err)
	}
	return false, nil
}

// HasUnstagedChanges checks if there are any unstaged changes in tracked files.
func (h *WIPHandler) HasUnstagedChanges() (bool, error) {
	cmd := exec.Command("git", "-C", h.repoPath, "diff", "--quiet")
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 means there are differences (unstaged changes exist)
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, fmt.Errorf("failed to check unstaged changes: %w", err)
	}
	return false, nil
}

// HasUntrackedFiles checks if there are any untracked files.
func (h *WIPHandler) HasUntrackedFiles() (bool, error) {
	cmd := exec.Command("git", "-C", h.repoPath, "ls-files", "--others", "--exclude-standard")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check untracked files: %w", err)
	}
	return len(bytes.TrimSpace(output)) > 0, nil
}
