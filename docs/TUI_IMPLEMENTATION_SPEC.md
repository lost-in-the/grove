# Grove TUI Implementation Specification

**Version:** 2.0
**Status:** Consolidated Source of Truth
**Created:** January 30, 2026
**Replaces:** TUI_REDESIGN_PROPOSAL.md, TUI_IMPLEMENTATION_GUIDE.md, TUI_VISUAL_MOCKUPS.md, TUI_DESIGN_ADDENDUM.md

---

## Table of Contents

1. [Overview](#overview)
2. [Current State Analysis](#current-state-analysis)
3. [Design Philosophy](#design-philosophy)
4. [Phase Breakdown](#phase-breakdown)
5. [Task Specifications](#task-specifications)
6. [Visual Mockups](#visual-mockups)
7. [Code Examples](#code-examples)
8. [Testing Requirements](#testing-requirements)
9. [Agent Checklist (JSON)](#agent-checklist-json)

---

## Overview

This document specifies the comprehensive TUI redesign for Grove CLI. It consolidates all design decisions, implementation details, and visual mockups into a single source of truth for agent-driven development.

### Goals

1. **Improve usability** - Better visual hierarchy, progressive disclosure
2. **Enhance functionality** - Add PR context, issue tracking, sync status
3. **Polish aesthetics** - Consistent theming, rounded borders, semantic colors
4. **Maintain performance** - All operations under 500ms
5. **Support all skill levels** - Beginner hints to expert shortcuts

### Key Libraries

| Library | Purpose | Priority |
|---------|---------|----------|
| Lipgloss | Styling with AdaptiveColor | Core |
| Bubbles | List, Table, Viewport, Spinner | Core |
| Huh | Forms for wizards | High |
| Glamour | Markdown rendering (PR/Issue) | High |

---

## Current State Analysis

### What Works Well

- Core Elm Architecture (Model-View-Update) is solid
- Responsive layout (side-by-side at 120 chars, stacked below)
- Vim-style navigation (j/k, arrow keys)
- Quick-switch (1-9 keys)
- Filtering with "/"
- Centered overlay modals

### Pain Points to Address

| Issue | Impact | Solution |
|-------|--------|----------|
| Flat visual hierarchy | Hard to scan | Panel titles, visual indicators |
| Minimal color usage | Lacks personality | Semantic AdaptiveColor theme |
| Wizard context lost | Confusing multi-step | Show summary in each step |
| No duplicate validation | User only sees error on create | Real-time validation |
| Transient status | Easy to miss | Toast notifications |
| Full-screen help | Disrupts workflow | Inline expandable help |
| No sync status | Unknown push/pull state | Show ahead/behind counts |
| Missing PR context | Limited info for decisions | Draft, commits, description |
| No issue tracking | Must leave TUI | Add issue browser |

---

## Design Philosophy

### Guiding Principles

1. **Speed First** - TUI users value efficiency; minimize keystrokes
2. **Progressive Disclosure** - Essential info by default, details on demand
3. **Consistent Mental Model** - Panel-based approach like lazygit
4. **Visual Clarity** - Color conveys state, not decoration
5. **Keyboard-Centric** - Every action reachable without mouse
6. **Accessibility** - High contrast, screen reader support via Huh

### Inspiration Sources

- **lazygit** - Panel organization, keybinding hints
- **gitui** - Clean status display, context-sensitive help
- **Charm apps** - Lipgloss styling, gradient usage, borders

---

## Phase Breakdown

### Phase TUI-1: Foundation (2-3 days)
**Goal:** Enhanced theme system and core components

- TUI-1.1: Semantic color theme with AdaptiveColor
- TUI-1.2: StyleSet for consistent styling
- TUI-1.3: Toast notification system
- TUI-1.4: Improved list item delegate with indicators

### Phase TUI-2: Core UX (3-4 days)
**Goal:** Dashboard and navigation improvements

- TUI-2.1: Dashboard header with context
- TUI-2.2: Contextual help footer (compact + expanded)
- TUI-2.3: Detail panel enhancements
- TUI-2.4: Remote sync status display

### Phase TUI-3: Wizards (3-4 days)
**Goal:** Improved create/delete workflows

- TUI-3.1: Stepper component for multi-step flows
- TUI-3.2: Create wizard with context preservation
- TUI-3.3: Real-time duplicate validation
- TUI-3.4: Enhanced delete confirmation

### Phase TUI-4: PR/Issue Integration (4-5 days)
**Goal:** Rich PR and issue browsing

- TUI-4.1: Enhanced PR data fields (draft, commits, review)
- TUI-4.2: PR browser with table layout
- TUI-4.3: PR preview panel with Glamour
- TUI-4.4: Issue browser implementation
- TUI-4.5: Unified CLI/TUI entry points

### Phase TUI-5: Polish (2-3 days)
**Goal:** Final refinements and Huh integration

- TUI-5.1: Huh forms for create wizard
- TUI-5.2: Animation refinements
- TUI-5.3: Narrow terminal responsive layout
- TUI-5.4: Accessibility review

---

## Task Specifications

### TUI-1.1: Semantic Color Theme

**Description:** Implement AdaptiveColor-based theme with semantic tokens.

**Acceptance Criteria:**
- [ ] ColorScheme struct with all semantic colors defined
- [ ] AdaptiveColor for automatic dark/light mode
- [ ] Colors organized by purpose (brand, status, surface, text)
- [ ] NO_COLOR environment variable respected
- [ ] Tests verify color contrast ratios

**Test Cases:**
```go
func TestColorScheme(t *testing.T) {
    tests := []struct {
        name     string
        color    lipgloss.AdaptiveColor
        wantDark string
        wantLight string
    }{
        {"Primary", Colors.Primary, "#A78BFA", "#7C3AED"},
        {"Success", Colors.Success, "#34D399", "#059669"},
        {"Warning", Colors.Warning, "#FBBF24", "#D97706"},
        {"Danger", Colors.Danger, "#F87171", "#DC2626"},
    }
    // ...
}
```

**Files to Create/Modify:**
- `internal/tui/theme_v2.go` - New theme system
- `internal/tui/theme_v2_test.go` - Theme tests

**Definition of Done:**
- [ ] Tests written first (TDD)
- [ ] All color tokens defined
- [ ] NO_COLOR support works
- [ ] Code reviewed

---

### TUI-1.2: StyleSet Implementation

**Description:** Create pre-composed styles using the ColorScheme.

**Acceptance Criteria:**
- [ ] StyleSet struct with all style categories
- [ ] Styles for: borders, text, status, list items, inputs, overlays, footer
- [ ] NewStyleSet(ColorScheme) factory function
- [ ] Global Styles variable initialized

**Test Cases:**
```go
func TestStyleSet(t *testing.T) {
    tests := []struct {
        name  string
        style lipgloss.Style
        check func(lipgloss.Style) bool
    }{
        {"RoundedBorder has border", Styles.RoundedBorder,
            func(s lipgloss.Style) bool { return s.GetBorder() != lipgloss.Border{} }},
        {"Title is bold", Styles.Title,
            func(s lipgloss.Style) bool { return s.GetBold() }},
    }
    // ...
}
```

**Files to Create/Modify:**
- `internal/tui/theme_v2.go` - Add StyleSet
- `internal/tui/theme_v2_test.go` - Add StyleSet tests

**Definition of Done:**
- [ ] Tests written first (TDD)
- [ ] All style categories implemented
- [ ] Styles compose correctly

---

### TUI-1.3: Toast Notification System

**Description:** Implement non-intrusive toast notifications for feedback.

**Acceptance Criteria:**
- [ ] Toast struct with Message, Level, Duration, CreatedAt
- [ ] ToastModel for managing current toast
- [ ] Levels: Success, Warning, Error, Info
- [ ] Auto-expiry after duration
- [ ] Positioned in top-right corner

**Test Cases:**
```go
func TestToast(t *testing.T) {
    tests := []struct {
        name      string
        toast     *Toast
        afterMs   int
        wantExpired bool
    }{
        {"Fresh toast not expired", NewToast("msg", ToastSuccess), 0, false},
        {"Toast expires after duration", NewToast("msg", ToastSuccess), 3100, true},
    }
    // ...
}
```

**Files to Create:**
- `internal/tui/toast.go`
- `internal/tui/toast_test.go`

**Definition of Done:**
- [ ] Tests written first (TDD)
- [ ] All toast levels render correctly
- [ ] Expiry logic works
- [ ] Positioning works at various widths

---

### TUI-1.4: List Item Delegate V2

**Description:** Improved list delegate with visual indicators.

**Acceptance Criteria:**
- [ ] WorktreeDelegateV2 with responsive columns
- [ ] Visual indicators: ● green (current), ● yellow (dirty), ❯ purple (selected)
- [ ] Column widths adapt to terminal width
- [ ] Status text: clean/dirty/stale
- [ ] Tmux badge when session exists

**Test Cases:**
```go
func TestListDelegate(t *testing.T) {
    tests := []struct {
        name     string
        item     WorktreeItem
        selected bool
        wantIndicator string
    }{
        {"Current worktree shows green dot", WorktreeItem{IsCurrent: true}, false, "●"},
        {"Dirty worktree shows yellow dot", WorktreeItem{IsDirty: true}, false, "●"},
        {"Selected shows cursor", WorktreeItem{}, true, "❯"},
        {"Normal shows space", WorktreeItem{}, false, " "},
    }
    // ...
}
```

**Files to Create:**
- `internal/tui/list_v2.go`
- `internal/tui/list_v2_test.go`

**Definition of Done:**
- [ ] Tests written first (TDD)
- [ ] All indicators render correctly
- [ ] Responsive column sizing works
- [ ] Works at 80, 100, 120+ char widths

---

### TUI-2.1: Dashboard Header

**Description:** Context-rich header showing project and current worktree.

**Acceptance Criteria:**
- [ ] Header struct with ProjectName, WorktreeCount, CurrentBranch, CurrentName
- [ ] View renders: project · count · branch · current worktree indicator
- [ ] Left-aligned project info, right-aligned current indicator
- [ ] Responsive spacing

**Test Cases:**
```go
func TestHeader(t *testing.T) {
    h := &Header{
        ProjectName: "acupoll",
        WorktreeCount: 5,
        CurrentBranch: "main",
        CurrentName: "fix/diff-review",
    }
    view := h.View(80)
    // Assert contains "acupoll", "5 worktrees", "main", "fix/diff-review"
}
```

**Files to Create:**
- `internal/tui/header.go`
- `internal/tui/header_test.go`

---

### TUI-2.2: Contextual Help Footer

**Description:** Two-level help system - compact hints and expanded panel.

**Acceptance Criteria:**
- [ ] HelpFooter with expanded toggle state
- [ ] CompactHints returns context-aware hints per view
- [ ] RenderCompact for single-line footer
- [ ] RenderExpanded for three-column help panel
- [ ] Toggle with "?" key

**Test Cases:**
```go
func TestHelpFooter(t *testing.T) {
    tests := []struct {
        name     string
        view     ActiveView
        wantKeys []string
    }{
        {"Dashboard hints", ViewDashboard, []string{"↑↓", "enter", "n", "d", "?"}},
        {"Create hints", ViewCreate, []string{"enter", "esc"}},
        {"Delete hints", ViewDelete, []string{"y", "n", "space"}},
    }
    // ...
}
```

**Files to Create:**
- `internal/tui/help_v2.go`
- `internal/tui/help_v2_test.go`

---

### TUI-2.3: Detail Panel Enhancements

**Description:** Rich detail panel with metadata grid and file changes.

**Acceptance Criteria:**
- [ ] Bordered card with worktree name as title
- [ ] Metadata: Branch, Commit hash, Age, Status
- [ ] Sync status visualization (arrows for ahead/behind)
- [ ] Changed files list with modification type indicators
- [ ] Tmux session indicator

**Files to Create/Modify:**
- `internal/tui/detail_v2.go`
- `internal/tui/detail_v2_test.go`

---

### TUI-2.4: Remote Sync Status

**Description:** Show ahead/behind counts for remote tracking.

**Acceptance Criteria:**
- [ ] WorktreeItem fields: AheadCount, BehindCount, HasRemote
- [ ] Fetch sync status from git rev-list
- [ ] Display in list: ↑2 ↓3 or "no remote" or "✓ synced"
- [ ] Visual in detail: ←←← behind | ahead →→→

**Test Cases:**
```go
func TestSyncStatus(t *testing.T) {
    tests := []struct {
        name   string
        item   WorktreeItem
        want   string
    }{
        {"Synced", WorktreeItem{HasRemote: true, AheadCount: 0, BehindCount: 0}, "✓ synced"},
        {"Ahead only", WorktreeItem{HasRemote: true, AheadCount: 3}, "↑3"},
        {"Behind only", WorktreeItem{HasRemote: true, BehindCount: 2}, "↓2"},
        {"Diverged", WorktreeItem{HasRemote: true, AheadCount: 2, BehindCount: 1}, "↑2 ↓1"},
        {"No remote", WorktreeItem{HasRemote: false}, "⚠ no remote"},
    }
}
```

**Files to Modify:**
- `internal/tui/data.go` - Add sync status fetching
- `internal/tui/types.go` - Add fields to WorktreeItem

---

### TUI-3.1: Stepper Component

**Description:** Visual progress indicator for multi-step wizards.

**Acceptance Criteria:**
- [ ] Stepper struct with Steps []string and Current int
- [ ] Advance() and Back() methods
- [ ] View renders: ●━━━━●━━━━○ with labels
- [ ] Completed steps green, current purple, future gray

**Test Cases:**
```go
func TestStepper(t *testing.T) {
    s := NewStepper("Name", "Branch", "Create")
    assert.Equal(t, 0, s.Current)
    s.Advance()
    assert.Equal(t, 1, s.Current)
    s.Back()
    assert.Equal(t, 0, s.Current)
}
```

**Files to Create:**
- `internal/tui/stepper.go`
- `internal/tui/stepper_test.go`

---

### TUI-3.2: Create Wizard Context Preservation

**Description:** Show summary of previous steps in multi-step wizard.

**Acceptance Criteria:**
- [ ] CreateState preserves Name, BaseBranch, ProjectName
- [ ] Step 2+ shows context summary box with previous choices
- [ ] Summary includes: Name, Full name, Path, Branch (when set)
- [ ] Backspace returns to previous step with values preserved

**Test Cases:**
```go
func TestCreateContext(t *testing.T) {
    s := &CreateState{
        Step: CreateStepBranch,
        Name: "feature-auth",
        ProjectName: "acupoll",
    }
    view := renderCreateBranchV2(s, 80)
    assert.Contains(t, view, "feature-auth")
    assert.Contains(t, view, "acupoll-feature-auth")
}
```

**Files to Modify:**
- `internal/tui/overlay_create_v2.go`
- `internal/tui/overlay_create_v2_test.go`

---

### TUI-3.3: Real-time Duplicate Validation

**Description:** Validate worktree name uniqueness as user types.

**Acceptance Criteria:**
- [ ] CreateState.ExistingWorktree populated if name conflicts
- [ ] Validation runs on each keypress (debounced)
- [ ] Shows existing worktree details: path, branch, status
- [ ] Footer changes to offer switching to existing
- [ ] Error styling on input field when invalid

**Test Cases:**
```go
func TestDuplicateValidation(t *testing.T) {
    mgr := mockWorktreeManager([]string{"feature-auth"})
    existing, err := validateWorktreeName(mgr, "feature-auth", "acupoll")
    assert.NoError(t, err)
    assert.NotNil(t, existing)
    assert.Equal(t, "feature-auth", existing.ShortName)
}
```

---

### TUI-3.4: Enhanced Delete Confirmation

**Description:** Rich delete confirmation with worktree details.

**Acceptance Criteria:**
- [ ] Warning box for dirty worktrees with file count
- [ ] Details section: Path, Branch, Last commit
- [ ] Visual checkbox for "Also delete branch"
- [ ] Clear footer with action consequences

---

### TUI-4.1: Enhanced PR Data Fields

**Description:** Extend PullRequest struct with additional context.

**Acceptance Criteria:**
- [ ] Add to PullRequest: IsDraft, CommitCount, Additions, Deletions, ReviewDecision
- [ ] Update GitHub adapter to request new fields
- [ ] Parse and populate new fields from gh CLI output

**Files to Modify:**
- `plugins/tracker/tracker.go` - Add fields
- `plugins/tracker/github.go` - Request and parse new fields

---

### TUI-4.2: PR Browser Table Layout

**Description:** Redesigned PR browser with rich information.

**Acceptance Criteria:**
- [ ] Table layout with columns: #, Title, Branch, Author, Status
- [ ] Two-line items: title + metadata (author, time, commits, +/-)
- [ ] Filter input at top with count
- [ ] Draft PRs labeled with [DRAFT]
- [ ] Existing worktree badge ✓

---

### TUI-4.3: PR Preview Panel

**Description:** Markdown-rendered PR description preview.

**Acceptance Criteria:**
- [ ] Tab key toggles preview panel
- [ ] Glamour renders PR body markdown
- [ ] Shows title, branch info, review status
- [ ] Footer actions: Create worktree, Back, Open in browser

**Files to Create:**
- `internal/tui/pr_preview.go`

---

### TUI-4.4: Issue Browser

**Description:** Browse and create worktrees from issues.

**Acceptance Criteria:**
- [ ] New ViewIssues mode accessible via 'i' key
- [ ] Issue list with: number, title, author, age, labels
- [ ] Filter input
- [ ] Preview panel with issue description
- [ ] Create worktree with suggested name from issue

---

### TUI-4.5: Unified CLI/TUI Entry Points

**Description:** Make grove prs/issues open TUI by default.

**Acceptance Criteria:**
- [ ] `grove prs` opens TUI PR browser
- [ ] `grove prs --fzf` uses fzf (backward compatible)
- [ ] `grove issues` opens TUI issue browser
- [ ] TUI shows project context in header

**Files to Modify:**
- `cmd/grove/commands/browse.go`

---

### TUI-5.1: Huh Forms Integration

**Description:** Replace custom form handling with Huh library.

**Acceptance Criteria:**
- [ ] Create wizard uses Huh forms
- [ ] Built-in validation with error display
- [ ] Accessible mode for screen readers
- [ ] Consistent theming with huh.ThemeCharm()

---

## Visual Mockups

### Main Dashboard

```
╭─ grove ─────────────────────────────────────────────────────────────────╮
│                                                                          │
│  acupoll  ·  5 worktrees  ·  on main                    ● fix/diff-rev…  │
│                                                                          │
├─ Worktrees ─────────────────────────┬─ Details ─────────────────────────┤
│                                     │                                    │
│  1 ● main              main  20h    │  ╭─ fix/diff-review-cleanup ──────╮│
│  2   new-tree          new…  20h    │  │                                ││
│  3   pr-29-bump-rail   dep…  1y     │  │  Branch   fix/diff-review-cl…  ││
│ ❯4 ● fix/diff-review   fix…  20h    │  │  Commit   87f14f7c · 20 hours  ││
│  5   test              test  20h    │  │  Status   ● dirty (4 files)    ││
│                                     │  │  Tmux     ● active session     ││
│                                     │  │                                ││
│                                     │  │  ┌─ Changes ────────────────┐  ││
│                                     │  │  │ M .gitignore             │  ││
│                                     │  │  │ M lib/powerpoint/pptx_…  │  ││
│                                     │  │  │ + plan-breezy-plotting…  │  ││
│                                     │  │  └──────────────────────────┘  ││
│                                     │  │                                ││
│                                     │  ╰────────────────────────────────╯│
│                                     │                                    │
├─────────────────────────────────────┴────────────────────────────────────┤
│  ↑↓ navigate  enter switch  n new  d delete  p PRs  ? more               │
╰──────────────────────────────────────────────────────────────────────────╯
```

### Create Wizard Step 1 (Name)

```
╭─ New Worktree ──────────────────────────────────────────────────────────╮
│                                                                          │
│         ●━━━━━━━━━━━━━━━━━━━━━━━━━━○                                     │
│        Name                     Branch                                   │
│                                                                          │
│  ╭────────────────────────────────────────────────────────────────────╮  │
│  │ new-feature█                                                       │  │
│  ╰────────────────────────────────────────────────────────────────────╯  │
│                                                                          │
│  Will create:   acupoll-new-feature                                      │
│  Location:      ~/Work/acupoll-new-feature                               │
│                                                                          │
│  ✓ Valid name                                                            │
│                                                                          │
│  ╭────────────────────────────────────────────────────────────────────╮  │
│  │ ℹ Tip: Worktree names are automatically prefixed with the         │  │
│  │   project name for organization.                                   │  │
│  ╰────────────────────────────────────────────────────────────────────╯  │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  Enter Continue    Esc Cancel                                            │
╰──────────────────────────────────────────────────────────────────────────╯
```

### Create Wizard Step 2 (Branch) - With Context

```
╭─ New Worktree ──────────────────────────────────────────────────────────╮
│                                                                          │
│  ●━━━━━━━━━━━━━━━━━━━━●━━━━━━━━━━━━━━━━━━━━━○                            │
│       Name ✓           Branch              Create                        │
│                                                                          │
│  ┌─ Summary ────────────────────────────────────────────────────────┐   │
│  │  Name:     feature-auth                                          │   │
│  │  Full:     acupoll-feature-auth                                  │   │
│  │  Path:     ~/Work/acupoll-feature-auth                           │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  Select branch strategy:                                                 │
│                                                                          │
│  ❯ Create new branch                                                     │
│    From existing branch...                                               │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  Enter Continue    Backspace Back    Esc Cancel                          │
╰──────────────────────────────────────────────────────────────────────────╯
```

### Duplicate Validation Error

```
╭─ New Worktree ──────────────────────────────────────────────────────────╮
│                                                                          │
│  ╭────────────────────────────────────────────────────────────────────╮  │
│  │ feature-auth█                                                      │  │
│  ╰────────────────────────────────────────────────────────────────────╯  │
│                                                                          │
│  ✗ Worktree "feature-auth" already exists                                │
│                                                                          │
│  Existing worktree:                                                      │
│    Path:     ~/Work/acupoll-feature-auth                                 │
│    Branch:   feature/authentication                                      │
│    Status:   ● dirty (2 files)                                           │
│                                                                          │
│  ╭────────────────────────────────────────────────────────────────────╮  │
│  │ ℹ Try a different name, or switch to the existing worktree        │  │
│  ╰────────────────────────────────────────────────────────────────────╯  │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  Enter Switch to existing    Tab Edit name    Esc Cancel                 │
╰──────────────────────────────────────────────────────────────────────────╯
```

### PR Browser

```
╭─ Pull Requests ─────────────────────────────────────────────────────────╮
│                                                                          │
│  Filter: █                                                      16 open  │
│                                                                          │
│  ╭────────────────────────────────────────────────────────────────────╮  │
│  │                                                                    │  │
│  │  ❯ #116   Fix/diff review cleanup                    fix/diff-r…  │  │
│  │           @LeahArmstrong · 5 commits · +234 -89      ✓ worktree   │  │
│  │  ──────────────────────────────────────────────────────────────── │  │
│  │    #106   [DRAFT] Staging                            staging      │  │
│  │           @LeahArmstrong · 12 commits · +1,203 -445               │  │
│  │  ──────────────────────────────────────────────────────────────── │  │
│  │    #89    Bump @playwright/test from 1.48 to 1.50    dependabot…  │  │
│  │           @app/dependabot · 1 commit · +5 -5                      │  │
│  │                                                                    │  │
│  ╰────────────────────────────────────────────────────────────────────╯  │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  Enter Create worktree    Tab Preview    Esc Close    ↑↓ Navigate        │
╰──────────────────────────────────────────────────────────────────────────╯
```

### Issue Browser

```
╭─ Issues ────────────────────────────────────────────────────────────────╮
│                                                                          │
│  Filter: █                                                    6 open     │
│                                                                          │
│  ╭────────────────────────────────────────────────────────────────────╮  │
│  │                                                                    │  │
│  │❯ #33    grove last should be project-scoped                        │  │
│  │         @LeahArmstrong · opened 2 days ago                         │  │
│  │         Labels: enhancement, CLI                                    │  │
│  │  ───────────────────────────────────────────────────────────────── │  │
│  │  #32    grove new missing --branch and --from flags                │  │
│  │         @LeahArmstrong · opened 3 days ago                         │  │
│  │         Labels: enhancement                                         │  │
│  │  ───────────────────────────────────────────────────────────────── │  │
│  │  #31    grove to should handle dirty worktrees                     │  │
│  │         @LeahArmstrong · opened 4 days ago                         │  │
│  │         Labels: bug                                                 │  │
│  │                                                                    │  │
│  ╰────────────────────────────────────────────────────────────────────╯  │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  Enter Create worktree    Tab Preview    Esc Close                       │
╰──────────────────────────────────────────────────────────────────────────╯
```

### Toast Notifications

```
Success:  ╭─────────────────────────╮
          │ ✓ Created "feature-x"  │
          ╰─────────────────────────╯

Warning:  ╭─────────────────────────╮
          │ ⚠ Worktree has changes │
          ╰─────────────────────────╯

Error:    ╭─────────────────────────╮
          │ ✗ Failed to delete     │
          ╰─────────────────────────╯
```

### Expanded Help Panel

```
╭─ Quick Reference ───────────────────────────────────────────────────────╮
│                                                                          │
│  Navigation              Actions                Views                    │
│                                                                          │
│  ↑/k      move up        n   new worktree       1-9  quick-switch       │
│  ↓/j      move down      d   delete             /    filter             │
│  enter    switch         p   browse PRs         ?    toggle help        │
│  esc      back/close     i   browse Issues      q    quit               │
│                          r   refresh                                     │
│                          o   cycle sort                                  │
│                                                                          │
╰─ Press ? again to close ────────────────────────────────────────────────╯
```

---

## Code Examples

See the Task Specifications section for code patterns. Key implementation patterns:

### Theme Usage

```go
// Use semantic colors
Styles.StatusSuccess.Render("✓ Created")
Styles.StatusWarning.Render("⚠ Dirty")
Styles.StatusDanger.Render("✗ Failed")

// Use style composition
Styles.RoundedBorder.Width(40).Render(content)
```

### Toast Usage

```go
// Show success toast
m.toast.Show(NewToast("Created worktree", ToastSuccess))

// In Update, tick toasts
m.toast.Tick()
```

### Stepper Usage

```go
stepper := NewStepper("Name", "Branch", "Create")
stepper.Current = 1  // On step 2
view := stepper.View(80)
```

---

## Testing Requirements

### TDD Workflow

For every task:
1. Write failing test first
2. Implement minimum code to pass
3. Refactor while keeping tests green
4. Commit test + implementation together

### Test Patterns

```go
// Table-driven tests
func TestFeature(t *testing.T) {
    tests := []struct {
        name     string
        input    InputType
        want     OutputType
        wantErr  bool
    }{
        {"valid case", validInput, expectedOutput, false},
        {"error case", invalidInput, nil, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Function(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            assert.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Coverage Targets

- `internal/tui/` packages: 80% minimum
- New components: 90% minimum
- Integration tests for full workflows

---

## Agent Checklist (JSON)

This JSON checklist is for agent progress tracking. Agents should update the status field as they work.

```json
{
  "version": "2.0",
  "lastUpdated": "2026-01-30",
  "phases": [
    {
      "id": "TUI-1",
      "name": "Foundation",
      "status": "pending",
      "tasks": [
        {
          "id": "TUI-1.1",
          "name": "Semantic Color Theme",
          "status": "complete",
          "files": ["internal/tui/theme_v2.go", "internal/tui/theme_v2_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "ColorScheme struct with all semantic colors",
            "AdaptiveColor for dark/light mode",
            "NO_COLOR environment variable respected",
            "Tests verify color definitions"
          ]
        },
        {
          "id": "TUI-1.2",
          "name": "StyleSet Implementation",
          "status": "pending",
          "dependsOn": ["TUI-1.1"],
          "files": ["internal/tui/theme_v2.go", "internal/tui/theme_v2_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "StyleSet struct with all categories",
            "NewStyleSet factory function",
            "Styles compose correctly"
          ]
        },
        {
          "id": "TUI-1.3",
          "name": "Toast Notification System",
          "status": "pending",
          "files": ["internal/tui/toast.go", "internal/tui/toast_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "Toast struct with Message, Level, Duration",
            "ToastModel for managing current toast",
            "Auto-expiry after duration",
            "Top-right positioning"
          ]
        },
        {
          "id": "TUI-1.4",
          "name": "List Item Delegate V2",
          "status": "pending",
          "dependsOn": ["TUI-1.2"],
          "files": ["internal/tui/list_v2.go", "internal/tui/list_v2_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "Visual indicators for current/dirty/selected",
            "Responsive column widths",
            "Status text clean/dirty/stale",
            "Tmux badge"
          ]
        }
      ]
    },
    {
      "id": "TUI-2",
      "name": "Core UX",
      "status": "pending",
      "dependsOn": ["TUI-1"],
      "tasks": [
        {
          "id": "TUI-2.1",
          "name": "Dashboard Header",
          "status": "pending",
          "files": ["internal/tui/header.go", "internal/tui/header_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "Header struct with project context",
            "Left-aligned info, right-aligned indicator",
            "Responsive spacing"
          ]
        },
        {
          "id": "TUI-2.2",
          "name": "Contextual Help Footer",
          "status": "pending",
          "files": ["internal/tui/help_v2.go", "internal/tui/help_v2_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "CompactHints per view",
            "RenderCompact single-line",
            "RenderExpanded three-column",
            "Toggle with ? key"
          ]
        },
        {
          "id": "TUI-2.3",
          "name": "Detail Panel Enhancements",
          "status": "pending",
          "files": ["internal/tui/detail_v2.go", "internal/tui/detail_v2_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "Bordered card with title",
            "Metadata grid",
            "Changed files list",
            "Tmux indicator"
          ]
        },
        {
          "id": "TUI-2.4",
          "name": "Remote Sync Status",
          "status": "pending",
          "files": ["internal/tui/data.go", "internal/tui/types.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "AheadCount, BehindCount, HasRemote fields",
            "Git rev-list fetch",
            "Visual display in list and detail"
          ]
        }
      ]
    },
    {
      "id": "TUI-3",
      "name": "Wizards",
      "status": "pending",
      "dependsOn": ["TUI-2"],
      "tasks": [
        {
          "id": "TUI-3.1",
          "name": "Stepper Component",
          "status": "pending",
          "files": ["internal/tui/stepper.go", "internal/tui/stepper_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "Stepper struct with Steps and Current",
            "Advance and Back methods",
            "Visual rendering with colors"
          ]
        },
        {
          "id": "TUI-3.2",
          "name": "Create Wizard Context",
          "status": "pending",
          "dependsOn": ["TUI-3.1"],
          "files": ["internal/tui/overlay_create_v2.go", "internal/tui/overlay_create_v2_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "Context summary in step 2+",
            "Shows Name, Full name, Path",
            "Backspace preserves values"
          ]
        },
        {
          "id": "TUI-3.3",
          "name": "Duplicate Validation",
          "status": "pending",
          "files": ["internal/tui/overlay_create_v2.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "Real-time validation on keypress",
            "Shows existing worktree details",
            "Offers switch to existing"
          ]
        },
        {
          "id": "TUI-3.4",
          "name": "Enhanced Delete Confirmation",
          "status": "pending",
          "files": ["internal/tui/overlay_delete_v2.go", "internal/tui/overlay_delete_v2_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "Warning box for dirty worktrees",
            "Details section",
            "Visual checkbox"
          ]
        }
      ]
    },
    {
      "id": "TUI-4",
      "name": "PR/Issue Integration",
      "status": "pending",
      "dependsOn": ["TUI-3"],
      "tasks": [
        {
          "id": "TUI-4.1",
          "name": "Enhanced PR Data Fields",
          "status": "pending",
          "files": ["plugins/tracker/tracker.go", "plugins/tracker/github.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "IsDraft, CommitCount, Additions, Deletions fields",
            "ReviewDecision field",
            "GitHub adapter requests and parses new fields"
          ]
        },
        {
          "id": "TUI-4.2",
          "name": "PR Browser Table Layout",
          "status": "pending",
          "dependsOn": ["TUI-4.1"],
          "files": ["internal/tui/view_prs_v2.go", "internal/tui/view_prs_v2_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "Table layout with columns",
            "Two-line items",
            "Draft and worktree badges"
          ]
        },
        {
          "id": "TUI-4.3",
          "name": "PR Preview Panel",
          "status": "pending",
          "files": ["internal/tui/pr_preview.go", "internal/tui/pr_preview_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "Tab toggles preview",
            "Glamour renders markdown",
            "Shows review status"
          ]
        },
        {
          "id": "TUI-4.4",
          "name": "Issue Browser",
          "status": "pending",
          "files": ["internal/tui/view_issues.go", "internal/tui/view_issues_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "ViewIssues mode via 'i' key",
            "Issue list with labels",
            "Preview panel",
            "Create worktree from issue"
          ]
        },
        {
          "id": "TUI-4.5",
          "name": "Unified CLI/TUI Entry",
          "status": "pending",
          "files": ["cmd/grove/commands/browse.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "grove prs opens TUI",
            "--fzf flag for backward compat",
            "grove issues opens TUI"
          ]
        }
      ]
    },
    {
      "id": "TUI-5",
      "name": "Polish",
      "status": "pending",
      "dependsOn": ["TUI-4"],
      "tasks": [
        {
          "id": "TUI-5.1",
          "name": "Huh Forms Integration",
          "status": "pending",
          "files": ["internal/tui/forms.go", "internal/tui/forms_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "Create wizard uses Huh",
            "Built-in validation",
            "Accessible mode"
          ]
        },
        {
          "id": "TUI-5.2",
          "name": "Animation Refinements",
          "status": "pending",
          "files": ["internal/tui/animation.go"],
          "testFirst": false,
          "acceptanceCriteria": [
            "Smooth spinner",
            "Toast fade-out",
            "Panel transitions"
          ]
        },
        {
          "id": "TUI-5.3",
          "name": "Narrow Terminal Layout",
          "status": "pending",
          "files": ["internal/tui/responsive.go", "internal/tui/responsive_test.go"],
          "testFirst": true,
          "acceptanceCriteria": [
            "Stacked layout under 80 chars",
            "Truncated columns",
            "No horizontal overflow"
          ]
        },
        {
          "id": "TUI-5.4",
          "name": "Accessibility Review",
          "status": "pending",
          "files": [],
          "testFirst": false,
          "acceptanceCriteria": [
            "Color contrast WCAG AA",
            "Screen reader with Huh",
            "High contrast mode option"
          ]
        }
      ]
    }
  ],
  "completionCriteria": {
    "allTasksComplete": false,
    "testsPass": false,
    "coverageAbove80": false,
    "manualTestingComplete": false,
    "documentationUpdated": false
  }
}
```

---

## Agent Instructions

1. **Start each session** by reading this document and the JSON checklist
2. **Pick ONE pending task** - start with first pending in current phase
3. **Follow TDD** - write tests before implementation
4. **Update checklist** - mark tasks complete as you finish
5. **Commit atomically** - test + implementation together
6. **Run validations** - `make test lint` before commit
7. **Signal completion** - when phase done, update phase status

### Commit Message Format

```
type(tui): description

- Detail 1
- Detail 2

Task: TUI-X.Y
```

### Validation Commands

```bash
make test          # All tests pass
make lint          # No linting errors
make build         # Builds successfully
./grove            # TUI launches correctly
```

---

*This document is the single source of truth for the TUI redesign.*
*All previous TUI design documents are superseded by this specification.*
