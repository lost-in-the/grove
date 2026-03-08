// Package worktree provides WIP (Work-In-Progress) handling utilities.
package worktree

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/LeahArmstrong/grove-cli/internal/cmdexec"
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
	output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", h.repoPath, "status", "--porcelain"}, "", cmdexec.GitLocal)
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}
	return len(bytes.TrimSpace(output)) > 0, nil
}

// ListWIPFiles returns a list of files with uncommitted changes.
func (h *WIPHandler) ListWIPFiles() ([]string, error) {
	output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", h.repoPath, "status", "--porcelain"}, "", cmdexec.GitLocal)
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
	if output, err := cmdexec.CombinedOutput(context.TODO(), "git", args, "", cmdexec.GitLocal); err != nil {
		return fmt.Errorf("failed to stash changes: %w\n%s", err, output)
	}
	return nil
}

// PopStash applies and removes the most recent stash entry.
func (h *WIPHandler) PopStash() error {
	if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", h.repoPath, "stash", "pop"}, "", cmdexec.GitLocal); err != nil {
		return fmt.Errorf("failed to pop stash: %w\n%s", err, output)
	}
	return nil
}

// CreatePatch creates a patch file from all uncommitted changes (staged and unstaged).
func (h *WIPHandler) CreatePatch() ([]byte, error) {
	// First, add all untracked files to index temporarily
	if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", h.repoPath, "add", "--all"}, "", cmdexec.GitLocal); err != nil {
		return nil, fmt.Errorf("failed to stage files: %w\n%s", err, output)
	}

	// Create patch from staged changes
	patch, err := cmdexec.Output(context.TODO(), "git", []string{"-C", h.repoPath, "diff", "--cached", "--binary"}, "", cmdexec.GitLocal)
	if err != nil {
		return nil, fmt.Errorf("failed to create patch: %w", err)
	}

	// Reset staging area (keep working tree changes)
	if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", h.repoPath, "reset", "HEAD"}, "", cmdexec.GitLocal); err != nil {
		return nil, fmt.Errorf("failed to reset staging: %w\n%s", err, output)
	}

	return patch, nil
}

// ApplyPatch applies a patch to the working tree.
func (h *WIPHandler) ApplyPatch(patch []byte) error {
	if len(patch) == 0 {
		return nil
	}

	// Cannot use cmdexec here because cmd.Stdin must be set.
	ctx, cancel := context.WithTimeout(context.TODO(), cmdexec.GitLocal)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", h.repoPath, "apply", "--3way")
	cmd.Stdin = bytes.NewReader(patch)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to apply patch: %w\n%s", err, output)
	}
	return nil
}

// HasStagedChanges checks if there are any staged changes.
func (h *WIPHandler) HasStagedChanges() (bool, error) {
	err := cmdexec.Run(context.TODO(), "git", []string{"-C", h.repoPath, "diff", "--cached", "--quiet"}, "", cmdexec.GitLocal)
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
	err := cmdexec.Run(context.TODO(), "git", []string{"-C", h.repoPath, "diff", "--quiet"}, "", cmdexec.GitLocal)
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
	output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", h.repoPath, "ls-files", "--others", "--exclude-standard"}, "", cmdexec.GitLocal)
	if err != nil {
		return false, fmt.Errorf("failed to check untracked files: %w", err)
	}
	return len(bytes.TrimSpace(output)) > 0, nil
}
