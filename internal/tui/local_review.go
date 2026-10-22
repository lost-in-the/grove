package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/plugins/tracker"
)

// localTarget is the worktree a local review shows (spec 13.1 g d).
type localTarget struct {
	Name, Path, Base string
}

// openLocalReview reviews item's changes against its merge-base with the
// default branch: committed and uncommitted edits to tracked files, no gh.
// The review's PR is a placeholder (number 0, the worktree's name and
// branch) so nothing that reads it can fail (plan 8d decision 7).
func (m Model) openLocalReview(item WorktreeItem) (tea.Model, tea.Cmd) {
	m.prReviewGen++
	m.pushView(ViewPRReview)
	base := ResolveDefaultBranch(m.cfg)
	s := &PRReviewState{
		PR:          &tracker.PullRequest{Title: item.displayName(), Branch: item.Branch},
		Tab:         4,
		Gen:         m.prReviewGen,
		Local:       &localTarget{Name: item.displayName(), Path: item.Path, Base: base},
		LoadingDiff: true,
	}
	m.prReview = s
	return m, tea.Batch(m.spinner.Tick, localDiffCmd(s.Gen, item.Path, base))
}

// localDiffCmd diffs the worktree at path, working tree included, against
// its merge-base with base, or with origin/base when there is no local
// base. The prefixes are set, since diff.noprefix or
// diff.mnemonicPrefix would change what ParseDiff strips; -M keeps rename
// detection on whatever diff.renames says.
func localDiffCmd(gen int, path, base string) tea.Cmd {
	return func() tea.Msg {
		mb, err := cmdexec.Output(context.TODO(), "git", []string{"-C", path, "merge-base", "HEAD", base}, "", cmdexec.GitLocal)
		if err != nil {
			// A checkout without a local default branch (a fresh clone of a
			// feature branch) still has origin's.
			mb, err = cmdexec.Output(context.TODO(), "git", []string{"-C", path, "merge-base", "HEAD", "origin/" + base}, "", cmdexec.GitLocal)
		}
		if err != nil {
			return prDiffMsg{gen: gen, err: fmt.Errorf("no merge-base with %s or origin/%s", base, base)}
		}
		out, err := cmdexec.Output(context.TODO(), "git", []string{"-C", path, "diff", "--no-color", "--no-ext-diff", "-M",
			"--src-prefix=a/", "--dst-prefix=b/", strings.TrimSpace(string(mb))}, "", cmdexec.GitLocal)
		if err != nil {
			return prDiffMsg{gen: gen, err: fmt.Errorf("git diff: %w", err)}
		}
		files, err := git.ParseDiff(string(out))
		return prDiffMsg{gen: gen, files: files, err: err}
	}
}

// openSelectedLocalReview is g d and d in the detail pane or sheet.
func (m Model) openSelectedLocalReview() (tea.Model, tea.Cmd) {
	item, ok := m.selectedItem()
	if !ok {
		return m, nil
	}
	return m.openLocalReview(item)
}
