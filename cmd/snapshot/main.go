// cmd/snapshot renders the TUI list with mock data for visual inspection.
// Usage: go run ./cmd/snapshot [width]
package main

import (
	"fmt"
	"os"
	"strconv"

	"charm.land/bubbles/v2/list"

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
			FullName:      "myapp-root",
			Path:          "/home/dev/projects/myapp",
			Branch:        "main",
			Commit:        "9b2ede86",
			CommitMessage: "Merge pull request #42",
			CommitAge:     "3 days ago",
			IsDirty:       true,
			IsMain:        true,
			IsCurrent:     true,
			TmuxStatus:    "attached",
			HasRemote:     true,
			AheadCount:    0,
			BehindCount:   0,
			DirtyFiles:    []string{"M .gitignore", "?? .grove/", "?? bin/dev-tool", "?? test/models/user_test.go"},
		},
		{
			ShortName:  "qa-blueprint",
			FullName:   "myapp-qa-blueprint",
			Path:       "/home/dev/projects/myapp-qa-blueprint",
			Branch:     "feat/qa-journey-blueprint",
			IsPrunable: true,
			TmuxStatus: "none",
		},
		{
			ShortName:     "feature-auth",
			FullName:      "myapp-feature-auth",
			Path:          "/home/dev/projects/myapp-feature-auth",
			Branch:        "feat/auth-tokens",
			Commit:        "a1b2c3d4",
			CommitMessage: "Add token-based auth",
			CommitAge:     "2 days ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			AheadCount:    0,
			BehindCount:   0,
			DirtyFiles:    []string{"M db/migrate/001_create_auth_tokens.go"},
		},
		{
			ShortName:     "fix-theme-nil",
			FullName:      "myapp-fix-theme-nil",
			Path:          "/home/dev/projects/myapp-fix-theme-nil",
			Branch:        "fix/theme-nil-guard",
			Commit:        "e5f6a7b8",
			CommitMessage: "Guard against nil theme",
			CommitAge:     "5 days ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			BehindCount:   3,
			DirtyFiles:    []string{"M internal/theme/theme.go"},
		},
		{
			ShortName:     "modal-refactor",
			FullName:      "myapp-modal-refactor",
			Path:          "/home/dev/projects/myapp-modal-refactor",
			Branch:        "refactor/modal-component",
			Commit:        "c9d0e1f2",
			CommitMessage: "Extract modal component",
			CommitAge:     "5 days ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			DirtyFiles:    []string{"M web/components/modal.js"},
			PluginStatuses: []plugins.StatusEntry{
				{ProviderName: "docker", Level: plugins.StatusActive, Short: "up (17)", Detail: "17 containers running"},
			},
		},
		{
			ShortName:     "upgrade-framework",
			FullName:      "myapp-upgrade-framework",
			Path:          "/home/dev/projects/myapp-upgrade-framework",
			Branch:        "feat/upgrade-framework",
			Commit:        "1a2b3c4d",
			CommitMessage: "Pre-bump framework to v2",
			CommitAge:     "11 days ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			DirtyFiles:    []string{"M go.mod", "M go.sum"},
		},
		{
			ShortName:     "bump-config",
			FullName:      "myapp-bump-config",
			Path:          "/home/dev/projects/myapp-bump-config",
			Branch:        "feat/bump-config",
			Commit:        "5e6f7a8b",
			CommitMessage: "Update config for framework v2",
			CommitAge:     "8 hours ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			DirtyFiles:    []string{"M config/app.toml"},
		},
		{
			ShortName:     "fix-payment-elements",
			FullName:      "myapp-fix-payment-elements",
			Path:          "/home/dev/projects/myapp-fix-payment-elements",
			Branch:        "fix/payment-elements",
			Commit:        "9c0d1e2f",
			CommitMessage: "Fix payment form rendering",
			CommitAge:     "4 days ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			DirtyFiles:    []string{"M internal/payments/handler.go"},
			PluginStatuses: []plugins.StatusEntry{
				{ProviderName: "docker", Level: plugins.StatusActive, Short: "up (3)", Detail: "3 containers running"},
				{ProviderName: "redis", Level: plugins.StatusInfo, Short: "idle", Detail: "redis standby"},
			},
		},
		{
			ShortName:     "fix-gallery",
			FullName:      "myapp-fix-gallery",
			Path:          "/home/dev/projects/myapp-fix-gallery",
			Branch:        "fix/gallery-edge-cases",
			Commit:        "3a4b5c6d",
			CommitMessage: "Fix gallery edge cases",
			CommitAge:     "4 days ago",
			IsDirty:       true,
			TmuxStatus:    "detached",
			HasRemote:     true,
			AheadCount:    9,
			BehindCount:   0,
			DirtyFiles:    []string{"M internal/gallery/gallery.go", "M internal/gallery/gallery_test.go"},
		},
	}
}
