package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/worktree"
)

// selectWorktree presents an interactive worktree chooser when no name is given.
// In interactive mode, shows a formatted numbered list and prompts for selection.
// In non-interactive mode, returns an error listing available worktrees.
func selectWorktree(ctx *GroveContext, prompt string) (string, error) {
	mgr, err := ctx.WorktreeManager()
	if err != nil {
		return "", err
	}

	trees, err := mgr.List()
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %w", err)
	}

	if len(trees) == 0 {
		return "", fmt.Errorf("no worktrees found")
	}

	if !cli.IsInteractive() {
		var names []string
		for _, t := range trees {
			names = append(names, t.DisplayName())
		}
		return "", fmt.Errorf("no worktree name provided\n\nAvailable worktrees: %s", strings.Join(names, ", "))
	}

	return chooseWorktree(mgr, trees, prompt)
}

// chooseWorktree renders a numbered worktree list and prompts for selection.
// Supports selection by number or by name. Escape and Ctrl+C cancel cleanly.
func chooseWorktree(mgr *worktree.Manager, trees []*worktree.Worktree, prompt string) (string, error) {
	w := cli.NewStderr()
	// Just need the path to mark the current row — CurrentPath skips
	// GetCurrent's commit-info enrichment + dirty check.
	currentPath, _ := mgr.CurrentPath()

	cli.Bold(w, "%s", prompt)
	_, _ = fmt.Fprintln(w)

	for i, tree := range trees {
		indicator := "  "
		if currentPath != "" && tree.Path == currentPath {
			indicator = cli.Accent(w, "● ") //nolint:gosmopolitan
		}

		status := "clean"
		if tree.IsPrunable {
			status = "stale"
		} else if tree.IsDirty {
			status = "dirty"
		}
		statusStr := cli.StatusText(w, cli.StatusLevel(status), status)

		_, _ = fmt.Fprintf(w, "  %s%d) %-20s %-25s %s\n", indicator, i+1, tree.DisplayName(), tree.Branch, statusStr)
	}

	_, _ = fmt.Fprintln(w)

	input, err := cli.ReadLine(fmt.Sprintf("Choice [1-%d]: ", len(trees)))
	if err != nil {
		return "", err
	}

	if input == "" {
		return "", fmt.Errorf("no selection made")
	}

	// Try selection by number. strconv.Atoi (unlike Sscanf "%d") rejects a
	// trailing non-numeric suffix, so a name like "2024-fixes" or "2abc" falls
	// through to name matching instead of being misread as a position (B33).
	if choice, convErr := strconv.Atoi(input); convErr == nil {
		if choice < 1 || choice > len(trees) {
			return "", fmt.Errorf("invalid choice %d: expected 1-%d", choice, len(trees))
		}
		return trees[choice-1].DisplayName(), nil
	}

	// Try matching by name
	for _, tree := range trees {
		if tree.DisplayName() == input || tree.ShortName == input || tree.Name == input {
			return tree.DisplayName(), nil
		}
	}

	return "", fmt.Errorf("worktree %q not found", input)
}
