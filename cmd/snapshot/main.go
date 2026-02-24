// cmd/snapshot renders the TUI list with mock data for visual inspection.
// Usage: go run ./cmd/snapshot [width]
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/bubbles/list"

	"github.com/LeahArmstrong/grove-cli/internal/plugins"
	"github.com/LeahArmstrong/grove-cli/internal/tui"
)

func main() {
	width := 160
	if len(os.Args) > 1 {
		if w, err := strconv.Atoi(os.Args[1]); err == nil && w > 40 {
			width = w
		}
	}

	items := mockItems()

	// Convert to list.Item slice
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	// Create a bubbles/list with the delegate
	delegate := tui.NewWorktreeDelegate()
	l := list.New(listItems, delegate, width, len(items)+2)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowFilter(false)
	l.SetShowPagination(false)

	// Compute content-adaptive column widths
	delegate = tui.ComputeDelegateWidths(listItems, width)
	l.SetDelegate(delegate)

	// Print header + rendered list
	fmt.Printf("=== Grove TUI Snapshot (width=%d) ===\n\n", width)

	header := tui.RenderListHeader(delegate, width)
	fmt.Println(header)
	fmt.Println(l.View())
}

func mockItems() []tui.WorktreeItem {
	return []tui.WorktreeItem{
		{
			ShortName:     "root",
			FullName:      "admin-root",
			Path:          "/home/dev/work/app",
			Branch:        "main",
			Commit:        "9b2ede86",
			CommitMessage: "Merge pull request #1234",
			CommitAge:     "3 days ago",
			IsDirty:       true,
			IsMain:        true,
			IsCurrent:     true,
			TmuxStatus:    "attached",
			HasRemote:     true,
			AheadCount:    0,
			BehindCount:   0,
			DirtyFiles:    []string{"M .gitignore", "?? .grove/", "?? bin/appsignal-mcp", "?? spec/models/serialized_attribute_configuration_spec.rb"},
		},
		{
			ShortName:  "qa-blueprint",
			FullName:   "admin-qa-blueprint",
			Path:       "/home/dev/work/app-qa-blueprint",
			Branch:     "codex/qa-journey-blueprint",
			IsPrunable: true,
			TmuxStatus: "none",
		},
		{
			ShortName:     "agent-slot-db",
			FullName:      "admin-agent-slot-db",
			Path:          "/home/dev/work/app-agent-slot-db",
			Branch:        "feat/agent-slot-db",
			Commit:        "a1b2c3d4",
			CommitMessage: "Add agent slot migration",
			CommitAge:     "2 days ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			AheadCount:    0,
			BehindCount:   0,
			DirtyFiles:    []string{"M db/migrate/001_create_agent_slots.rb"},
		},
		{
			ShortName:     "fix-theme-nil-guard",
			FullName:      "admin-fix-theme-nil-guard",
			Path:          "/home/dev/work/app-fix-theme-nil-guard",
			Branch:        "fix-theme-nil-guard",
			Commit:        "e5f6a7b8",
			CommitMessage: "Guard against nil theme",
			CommitAge:     "5 days ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			BehindCount:   3,
			DirtyFiles:    []string{"M app/models/theme.rb"},
		},
		{
			ShortName:     "modal-refactor",
			FullName:      "admin-modal-refactor",
			Path:          "/home/dev/work/app-modal-refactor",
			Branch:        "modal-refactor",
			Commit:        "c9d0e1f2",
			CommitMessage: "Extract modal component",
			CommitAge:     "5 days ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			DirtyFiles:    []string{"M app/javascript/components/modal.js"},
			PluginStatuses: []plugins.StatusEntry{
				{ProviderName: "docker", Level: plugins.StatusActive, Short: "up (17)", Detail: "17 containers running"},
			},
		},
		{
			ShortName:     "pr-13013-rails-71-pre-bump",
			FullName:      "admin-pr-13013-rails-71-pre-bump",
			Path:          "/home/dev/work/app-pr-13013-rails-71-pre-bump",
			Branch:        "la-rails7.1-prebump",
			Commit:        "1a2b3c4d",
			CommitMessage: "Prebump Rails to 7.1",
			CommitAge:     "11 days ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			DirtyFiles:    []string{"M Gemfile", "M Gemfile.lock"},
		},
		{
			ShortName:     "pr-13084-rails-71-bump-config",
			FullName:      "admin-pr-13084-rails-71-bump-config",
			Path:          "/home/dev/work/app-pr-13084-rails-71-bump-config",
			Branch:        "la-rails7.1-bump",
			Commit:        "5e6f7a8b",
			CommitMessage: "Bump Rails config for 7.1",
			CommitAge:     "8 hours ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			DirtyFiles:    []string{"M config/application.rb"},
		},
		{
			ShortName:     "pr-13093-fix-disable-subscription",
			FullName:      "admin-pr-13093-fix-disable-subscription",
			Path:          "/home/dev/work/app-pr-13093-fix-disable-subscription",
			Branch:        "sl/wait-for-stripe-elements",
			Commit:        "9c0d1e2f",
			CommitMessage: "Fix Stripe subscription disable",
			CommitAge:     "4 days ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			DirtyFiles:    []string{"M app/services/stripe_service.rb"},
			PluginStatuses: []plugins.StatusEntry{
				{ProviderName: "docker", Level: plugins.StatusActive, Short: "up (3)", Detail: "3 containers running"},
				{ProviderName: "redis", Level: plugins.StatusInfo, Short: "idle", Detail: "redis standby"},
			},
		},
		{
			ShortName:     "pr-13095-a-couple-fixes-for-gal",
			FullName:      "admin-pr-13095-a-couple-fixes-for-gal",
			Path:          "/home/dev/work/app-pr-13095-a-couple-fixes-for-gal",
			Branch:        "fix/gal-1387",
			Commit:        "3a4b5c6d",
			CommitMessage: "Fix gallery edge cases",
			CommitAge:     "4 days ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			AheadCount:    9,
			BehindCount:   0,
			DirtyFiles:    []string{"M app/models/gallery.rb", "M spec/models/gallery_spec.rb"},
		},
	}
}
