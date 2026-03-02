package tui

import (
	"sync"
	"testing"
	"time"

	"github.com/LeahArmstrong/grove-cli/plugins/tracker"
)

// viewString extracts the View() output as a string.
// In v2, View() returns tea.View, so we extract the content string.
func (m Model) viewString() string { return m.viewContent() }

// goldenMu serializes golden tests that mutate the global Colors/Styles vars.
// Golden tests must NOT use t.Parallel().
var goldenMu sync.Mutex

// termSize defines a terminal size preset for golden tests.
type termSize struct {
	name          string
	width, height int
}

var (
	sizeNarrow    = termSize{"narrow_60x24", 60, 24}
	sizeStandard  = termSize{"standard_80x24", 80, 24}
	sizeWide      = termSize{"wide_120x40", 120, 40}
	sizeUltraWide = termSize{"ultrawide_160x40", 160, 40}
	allSizes      = []termSize{sizeNarrow, sizeStandard, sizeWide, sizeUltraWide}
)

// goldenModel creates a test model with NO_COLOR mode and the given size.
// It locks goldenMu and restores Colors/Styles via t.Cleanup.
//
// Spinner determinism: the spinner is created but never ticked, so it always
// renders its first frame ("⠋"). Never send spinner.TickMsg before capturing
// View() in golden tests.
func goldenModel(t *testing.T, size termSize, opts ...testOpt) Model {
	t.Helper()

	goldenMu.Lock()

	// Force NO_COLOR mode for structural golden tests
	t.Setenv("NO_COLOR", "1")
	Colors = noColorScheme()
	Styles = NewStyleSet(Colors)

	t.Cleanup(func() {
		Colors = NewColorScheme()
		Styles = NewStyleSet(Colors)
		goldenMu.Unlock()
	})

	allOpts := append([]testOpt{withSize(size.width, size.height)}, opts...)
	return newTestModel(allOpts...)
}

// goldenModelThemed creates a test model with full color output for themed golden tests.
// Uses defaultColorScheme() directly for deterministic color output.
func goldenModelThemed(t *testing.T, size termSize, opts ...testOpt) Model {
	t.Helper()

	goldenMu.Lock()

	Colors = defaultColorScheme()
	Styles = NewStyleSet(Colors)

	t.Cleanup(func() {
		Colors = NewColorScheme()
		Styles = NewStyleSet(Colors)
		goldenMu.Unlock()
	})

	allOpts := append([]testOpt{withSize(size.width, size.height)}, opts...)
	return newTestModel(allOpts...)
}

// --- State builder opts for overlay golden tests ---

// withDeleteOverlay sets up the delete overlay with a test item.
func withDeleteOverlay(warnings ...string) testOpt {
	return func(m *Model) {
		item := makeTestItems(3)[1] // non-main item
		m.activeView = ViewDelete
		m.deleteState = &DeleteState{
			Item:     &item,
			Warnings: warnings,
		}
	}
}

// withCreateStep sets up the create overlay at a specific step.
func withCreateStep(step CreateStep) testOpt {
	return func(m *Model) {
		m.activeView = ViewCreate
		m.createState = &CreateState{
			Step:              step,
			Branches:          []string{"main", "develop", "feature/auth", "fix/login-bug", "release/v2"},
			ProjectName:       m.projectName,
			BranchFilterInput: newBranchFilterInput(),
		}
		switch step {
		case CreateStepName:
			m.createState.BaseBranch = "feature/auth"
			m.createState.NameSuggestion = "auth"
			m.createState.NameInput = newNameInput("auth")
		case CreateStepBranchAction:
			m.createState.BaseBranch = "feature/auth"
		case CreateStepConfirm:
			m.createState.BaseBranch = "feature/auth"
			m.createState.Name = "auth-work"
			ni := newNameInput("")
			ni.SetValue("auth-work")
			m.createState.NameInput = ni
		}
	}
}

// withBulkOverlay sets up the bulk delete overlay with n items.
func withBulkOverlay(n int) testOpt {
	return func(m *Model) {
		items := makeTestItems(n)
		// Skip main item
		var bulkItems []WorktreeItem
		for _, item := range items {
			if !item.IsMain {
				bulkItems = append(bulkItems, item)
			}
		}
		m.activeView = ViewBulk
		m.bulkState = &BulkState{
			Items:    bulkItems,
			Selected: make([]bool, len(bulkItems)),
		}
		// Select a couple for visual interest
		if len(m.bulkState.Selected) > 0 {
			m.bulkState.Selected[0] = true
		}
		if len(m.bulkState.Selected) > 2 {
			m.bulkState.Selected[2] = true
		}
	}
}

// withPRData sets up the PR browser overlay with mock data.
func withPRData() testOpt {
	return func(m *Model) {
		m.activeView = ViewPRs
		fi := newPRFilterInput()
		m.prState = &PRViewState{
			FilterInput: fi,
			PRs: []*tracker.PullRequest{
				{Number: 42, Title: "Add user authentication flow", Author: "alice", Branch: "feature/auth", BaseBranch: "main", CommitCount: 3, Additions: 245, Deletions: 12},
				{Number: 38, Title: "Fix login redirect loop", Author: "bob", Branch: "fix/login", BaseBranch: "main", IsDraft: true, CommitCount: 1, Additions: 8, Deletions: 3},
				{Number: 35, Title: "Refactor database connection pooling", Author: "carol", Branch: "refactor/db-pool", BaseBranch: "main", CommitCount: 7, Additions: 1203, Deletions: 456},
			},
			WorktreeBranches: map[string]bool{
				"feature/auth": true,
			},
		}
	}
}

// withIssueData sets up the issue browser overlay with mock data.
func withIssueData() testOpt {
	return func(m *Model) {
		m.activeView = ViewIssues
		ifi := newIssueFilterInput()
		m.issueState = &IssueViewState{
			FilterInput: ifi,
			Issues: []*tracker.Issue{
				{Number: 101, Title: "Login page crashes on mobile", Author: "alice", Labels: []string{"bug", "high-priority"}, CreatedAt: time.Now().Add(-48 * time.Hour)},
				{Number: 95, Title: "Add dark mode support", Author: "bob", Labels: []string{"enhancement"}, CreatedAt: time.Now().Add(-7 * 24 * time.Hour)},
				{Number: 88, Title: "Update README with setup instructions", Author: "carol", Labels: []string{"docs"}, CreatedAt: time.Now().Add(-14 * 24 * time.Hour)},
			},
		}
	}
}

// withForkOverlay sets up the fork overlay at the confirm step.
func withForkOverlay() testOpt {
	return func(m *Model) {
		source := makeTestItems(3)[1]
		ni := newForkNameInput()
		ni.SetValue("experiment")
		m.activeView = ViewFork
		m.forkState = &ForkState{
			Step:      ForkStepConfirm,
			Source:    source,
			Name:      "experiment",
			NameInput: ni,
			Stepper:   NewStepper("Name", "WIP", "Confirm"),
		}
		m.forkState.Stepper.Current = 2
	}
}

// withSyncOverlay sets up the sync overlay at the source selection step.
func withSyncOverlay() testOpt {
	return func(m *Model) {
		items := makeTestItems(5)
		m.activeView = ViewSync
		m.syncState = &SyncState{
			Step:    SyncStepSource,
			Target:  items[0],
			Stepper: NewStepper("Source", "Preview", "Confirm"),
			Sources: []WorktreeWIPInfo{
				{Item: items[1], HasWIP: true, Files: []string{"main.go", "config.go"}},
				{Item: items[2], HasWIP: false},
				{Item: items[3], HasWIP: true, Files: []string{"handler.go"}},
			},
		}
	}
}

// withConfigOverlay sets up the config overlay with deterministic fields.
func withConfigOverlay() testOpt {
	return func(m *Model) {
		m.activeView = ViewConfig
		cs := NewConfigState()
		cs.Fields[ConfigTabGeneral] = []ConfigField{
			{Key: "project_name", Label: "project_name", Value: "grove-cli", Default: "grove-cli", Type: ConfigString, Description: "Project name"},
			{Key: "alias", Label: "alias", Value: "", Default: "", Type: ConfigString, Description: "Short name for display"},
			{Key: "projects_dir", Label: "projects_dir", Value: "/home/dev/projects", Default: "/home/dev/projects", Type: ConfigString, Description: "Where worktrees are created"},
			{Key: "default_base_branch", Label: "default_branch", Value: "main", Default: "main", Type: ConfigString, Description: "Base branch for new worktrees"},
		}
		cs.Fields[ConfigTabBehavior] = []ConfigField{
			{Key: "switch.dirty_handling", Label: "dirty_handling", Value: "auto-stash", Default: "auto-stash", Type: ConfigEnum, Options: []string{"auto-stash", "prompt", "refuse"}, Description: "How to handle dirty worktree on switch"},
			{Key: "tmux.mode", Label: "tmux_mode", Value: "auto", Default: "auto", Type: ConfigEnum, Options: []string{"auto", "manual", "off"}, Description: "Tmux session behavior"},
		}
		cs.Fields[ConfigTabPlugins] = []ConfigField{
			{Key: "plugins.docker.enabled", Label: "docker_enabled", Value: "true", Default: "true", Type: ConfigBool, Description: "Enable Docker plugin"},
		}
		cs.Fields[ConfigTabProtection] = []ConfigField{
			{Key: "protection.protected", Label: "protected", Value: "main, staging", Default: "main, staging", Type: ConfigList, Description: "Protected worktrees (comma-separated)"},
		}
		m.configState = cs
	}
}

// withToastVisible shows a toast with a far-future expiry so it never expires during the test.
func withToastVisible(msg string, level ToastLevel) testOpt {
	return func(m *Model) {
		m.toast.Current = &Toast{
			Message:   msg,
			Level:     level,
			Duration:  24 * time.Hour,
			CreatedAt: time.Now(),
		}
	}
}

// withHelpExpanded expands the help footer overlay.
func withHelpExpanded() testOpt {
	return func(m *Model) {
		m.helpFooter.Expanded = true
	}
}

// withSortMode sets the sort mode.
func withSortMode(mode SortMode) testOpt {
	return func(m *Model) {
		m.sortMode = mode
	}
}

// withCompactMode switches the model to compact (V1) delegate.
func withCompactMode() testOpt {
	return func(m *Model) {
		m.compactMode = true
		d := ComputeDelegateWidths(m.list.Items(), m.list.Width())
		m.listDelegate = d
		m.list.SetDelegate(d)
		m.updateLayout()
	}
}
