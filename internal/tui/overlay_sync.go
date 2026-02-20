package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

// SyncStep represents the current step in the sync wizard.
type SyncStep int

const (
	SyncStepSource  SyncStep = iota // select source worktree
	SyncStepPreview                 // preview changes
	SyncStepConfirm                 // confirm sync
)

// WorktreeWIPInfo pairs a worktree item with its WIP status.
type WorktreeWIPInfo struct {
	Item     WorktreeItem
	HasWIP   bool
	Files    []string
	CheckErr error // non-nil if WIP check failed
}

// SyncState holds the state for the sync overlay.
type SyncState struct {
	Step     SyncStep
	Target   WorktreeItem      // current worktree (receiving changes)
	Sources  []WorktreeWIPInfo // other worktrees with WIP info
	Selected int               // cursor in source list
	Err      error
	Syncing  bool
	Stepper  *Stepper
}

// NewSyncState creates a new SyncState. Target is the current worktree.
func NewSyncState(items []WorktreeItem) *SyncState {
	var target WorktreeItem
	for _, item := range items {
		if item.IsCurrent {
			target = item
			break
		}
	}

	return &SyncState{
		Step:    SyncStepSource,
		Target:  target,
		Stepper: NewStepper("Source", "Preview", "Confirm"),
	}
}

// syncWIPInfoMsg is sent after gathering WIP info for all worktrees.
type syncWIPInfoMsg struct {
	sources []WorktreeWIPInfo
	err     error
}

// syncCompleteMsg is sent after sync completes.
type syncCompleteMsg struct {
	filesApplied int
	err          error
}

// gatherWIPInfoCmd checks WIP status for all worktrees.
func gatherWIPInfoCmd(items []WorktreeItem) tea.Cmd {
	return func() tea.Msg {
		var sources []WorktreeWIPInfo
		for _, item := range items {
			if item.IsCurrent {
				continue // skip target
			}
			wip := worktree.NewWIPHandler(item.Path)
			hasWIP, err := wip.HasWIP()
			if err != nil {
				sources = append(sources, WorktreeWIPInfo{Item: item, CheckErr: err})
				continue
			}
			var files []string
			if hasWIP {
				files, err = wip.ListWIPFiles()
				if err != nil {
					sources = append(sources, WorktreeWIPInfo{Item: item, HasWIP: hasWIP, CheckErr: fmt.Errorf("failed to list files: %w", err)})
					continue
				}
			}
			sources = append(sources, WorktreeWIPInfo{
				Item:   item,
				HasWIP: hasWIP,
				Files:  files,
			})
		}
		return syncWIPInfoMsg{sources: sources}
	}
}

// syncWorktreeCmd copies WIP from source to target.
func syncWorktreeCmd(source WorktreeWIPInfo, target WorktreeItem) tea.Cmd {
	return func() tea.Msg {
		srcWIP := worktree.NewWIPHandler(source.Item.Path)
		patch, err := srcWIP.CreatePatch()
		if err != nil {
			return syncCompleteMsg{err: fmt.Errorf("failed to create patch from source: %w", err)}
		}

		// Re-apply the patch to source (CreatePatch stages then resets, but doesn't lose changes)
		// The source keeps its changes since CreatePatch does: add --all, diff --cached, reset HEAD
		// After reset HEAD, the working tree still has changes.

		if len(patch) == 0 {
			return syncCompleteMsg{filesApplied: 0}
		}

		tgtWIP := worktree.NewWIPHandler(target.Path)
		if err := tgtWIP.ApplyPatch(patch); err != nil {
			return syncCompleteMsg{err: fmt.Errorf("failed to apply patch to target: %w", err)}
		}

		return syncCompleteMsg{filesApplied: len(source.Files)}
	}
}

// selectedSource returns the currently selected source, if any valid one is selected.
func (s *SyncState) selectedSource() *WorktreeWIPInfo {
	if s.Selected >= 0 && s.Selected < len(s.Sources) {
		return &s.Sources[s.Selected]
	}
	return nil
}

// handleSyncKey handles key input for the sync overlay.
func (m Model) handleSyncKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.syncState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	s := m.syncState

	if s.Syncing {
		return m, nil
	}

	switch s.Step {
	case SyncStepSource:
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.activeView = ViewDashboard
			m.syncState = nil
			return m, nil

		case key.Matches(msg, m.keys.Up):
			if s.Selected > 0 {
				s.Selected--
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if len(s.Sources) > 0 && s.Selected < len(s.Sources)-1 {
				s.Selected++
			}
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			src := s.selectedSource()
			if src == nil {
				return m, nil
			}
			if src.CheckErr != nil {
				s.Err = fmt.Errorf("cannot sync: WIP check failed for %s", src.Item.ShortName)
				return m, nil
			}
			if !src.HasWIP {
				s.Err = fmt.Errorf("no uncommitted changes in %s", src.Item.ShortName)
				return m, nil
			}
			s.Err = nil
			s.Step = SyncStepPreview
			s.Stepper.Current = 1
			return m, nil
		}

	case SyncStepPreview:
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.activeView = ViewDashboard
			m.syncState = nil
			return m, nil

		case key.Matches(msg, m.keys.Back):
			s.Step = SyncStepSource
			s.Stepper.Current = 0
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			s.Step = SyncStepConfirm
			s.Stepper.Current = 2
			return m, nil
		}

	case SyncStepConfirm:
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.activeView = ViewDashboard
			m.syncState = nil
			return m, nil

		case key.Matches(msg, m.keys.Back):
			s.Step = SyncStepPreview
			s.Stepper.Current = 1
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			src := s.selectedSource()
			if src == nil {
				return m, nil
			}
			s.Syncing = true
			return m, tea.Batch(m.spinner.Tick, syncWorktreeCmd(*src, s.Target))
		}
	}

	return m, nil
}

// renderSync renders the sync overlay.
func renderSync(s *SyncState, width int) string {
	overlayWidth := width * 50 / 100
	if overlayWidth < 50 {
		overlayWidth = 50
	}
	if overlayWidth > 70 {
		overlayWidth = 70
	}
	contentWidth := overlayWidth - 6
	indent := huhOverlayIndent
	innerWidth := contentWidth - len(indent)*2

	var b strings.Builder

	// Stepper
	b.WriteString(indentBlock(s.Stepper.View(innerWidth), indent) + "\n\n")

	if s.Syncing {
		b.WriteString(indent + "⏳ Syncing changes...\n")
		if s.Err != nil {
			b.WriteString("\n" + indent + Styles.ErrorText.Render(s.Err.Error()) + "\n")
		}
		b.WriteString("\n" + Styles.Footer.Render(indent+"Please wait..."))
		return Styles.OverlayBorderInfo.Width(overlayWidth).Render(
			Styles.OverlayTitle.Render("Sync Changes") + "\n\n" + b.String(),
		)
	}

	if s.Err != nil {
		b.WriteString(indent + Styles.ErrorText.Render("Error: "+s.Err.Error()) + "\n\n")
	}

	switch s.Step {
	case SyncStepSource:
		b.WriteString(indent + "Pull uncommitted changes from another worktree\n")
		b.WriteString(indent + "into " + Styles.DetailValue.Render(s.Target.ShortName) + ".\n\n")

		if len(s.Sources) == 0 {
			b.WriteString(indent + Styles.DetailDim.Render("No other worktrees found.") + "\n")
		} else {
			for i, src := range s.Sources {
				cursor := "  "
				if i == s.Selected {
					cursor = Styles.ListCursor.String()
				}

				name := src.Item.ShortName
				var status string
				if src.CheckErr != nil {
					status = Styles.ErrorText.Render("error")
				} else if src.HasWIP {
					status = Styles.WarningText.Render(fmt.Sprintf("%d files changed", len(src.Files)))
				} else {
					status = Styles.DetailDim.Render("clean")
				}

				b.WriteString(indent + cursor + name + "    " + status + "\n")
			}
		}

		b.WriteString("\n" + Styles.Footer.Render(indent+"↑↓ navigate  enter select  esc cancel"))

	case SyncStepPreview:
		src := s.selectedSource()
		if src == nil {
			break
		}
		b.WriteString(indent + Styles.DetailLabel.Render("From: ") + Styles.DetailValue.Render(src.Item.ShortName) + " → " + Styles.DetailValue.Render(s.Target.ShortName) + "\n\n")
		b.WriteString(indent + "Modified:\n")
		maxShow := 12
		for i, f := range src.Files {
			if i >= maxShow {
				b.WriteString(indent + Styles.DetailDim.Render(fmt.Sprintf("  … and %d more", len(src.Files)-maxShow)) + "\n")
				break
			}
			b.WriteString(indent + "  " + f + "\n")
		}

		b.WriteString("\n" + Styles.Footer.Render(indent+"enter confirm  backspace back  esc cancel"))

	case SyncStepConfirm:
		src := s.selectedSource()
		if src == nil {
			break
		}
		b.WriteString(indent + Styles.DetailLabel.Render("From:   ") + Styles.DetailValue.Render(src.Item.ShortName) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("To:     ") + Styles.DetailValue.Render(s.Target.ShortName) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("Files:  ") + Styles.DetailValue.Render(fmt.Sprintf("%d", len(src.Files))) + "\n")

		b.WriteString("\n" + Styles.SuccessText.Render(indent+"Ready to sync.") + "\n")
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] sync  [backspace] back  [esc] cancel"))
	}

	return Styles.OverlayBorderInfo.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("Sync Changes") + "\n\n" + b.String(),
	)
}
