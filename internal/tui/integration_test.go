//go:build integration

package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

// newTestManagers creates worktree and state managers for a test repo.
func newTestManagers(t *testing.T, repoPath string) (*worktree.Manager, *state.Manager) {
	t.Helper()
	mgr, err := worktree.NewManager(repoPath)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	groveDir := filepath.Join(repoPath, ".grove")
	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		t.Fatalf("NewManager(state): %v", err)
	}
	return mgr, stateMgr
}

// --- FetchWorktrees ---

func TestFetchWorktrees_SingleMainWorktree(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	items, err := FetchWorktrees(mgr, stateMgr)
	if err != nil {
		t.Fatalf("FetchWorktrees: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(items))
	}

	item := items[0]
	if !item.IsMain {
		t.Error("expected main worktree to have IsMain=true")
	}
	if item.Commit == "" {
		t.Error("expected non-empty Commit")
	}
	if item.CommitMessage == "" {
		t.Error("expected non-empty CommitMessage")
	}
	if item.CommitAge == "" {
		t.Error("expected non-empty CommitAge")
	}
	if item.Branch != "main" {
		t.Errorf("expected branch 'main', got %q", item.Branch)
	}
}

func TestFetchWorktrees_MultipleWorktrees(t *testing.T) {
	repo := setupRailsFixtureWithWorktrees(t, "testing", "staging")
	mgr, stateMgr := newTestManagers(t, repo)

	items, err := FetchWorktrees(mgr, stateMgr)
	if err != nil {
		t.Fatalf("FetchWorktrees: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(items))
	}

	shortNames := make(map[string]bool)
	for _, item := range items {
		shortNames[item.ShortName] = true
	}

	for _, expected := range []string{"testing", "staging"} {
		if !shortNames[expected] {
			t.Errorf("missing expected short name %q; have %v", expected, shortNames)
		}
	}
}

func TestFetchWorktrees_DirtyDetection(t *testing.T) {
	repo := setupRailsFixtureWithDirtyWorktree(t)
	mgr, stateMgr := newTestManagers(t, repo)

	items, err := FetchWorktrees(mgr, stateMgr)
	if err != nil {
		t.Fatalf("FetchWorktrees: %v", err)
	}

	var dirtyItem *WorktreeItem
	for i := range items {
		if items[i].ShortName == "dirty-wt" {
			dirtyItem = &items[i]
			break
		}
	}

	if dirtyItem == nil {
		t.Fatal("dirty-wt worktree not found")
	}

	if !dirtyItem.IsDirty {
		t.Error("expected dirty-wt to have IsDirty=true")
	}
	if len(dirtyItem.DirtyFiles) == 0 {
		t.Error("expected dirty-wt to have DirtyFiles populated")
	}
}

func TestFetchWorktrees_CommitInfo(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	items, err := FetchWorktrees(mgr, stateMgr)
	if err != nil {
		t.Fatalf("FetchWorktrees: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("no worktrees returned")
	}

	item := items[0]
	if item.Commit == "" {
		t.Error("expected non-empty Commit")
	}
	if item.CommitMessage == "" {
		t.Error("expected non-empty CommitMessage")
	}
	if item.CommitAge == "" {
		t.Error("expected non-empty CommitAge")
	}
}

// --- getUpstreamCounts ---

func TestGetUpstreamCounts_NoUpstream(t *testing.T) {
	repo := setupRailsFixture(t)

	ahead, behind := getUpstreamCounts(repo)
	if ahead != 0 || behind != 0 {
		t.Errorf("expected (0,0) without upstream, got (%d,%d)", ahead, behind)
	}
}

func TestGetUpstreamCounts_AheadOfUpstream(t *testing.T) {
	repo := setupRailsFixtureWithUpstream(t)

	ahead, behind := getUpstreamCounts(repo)
	if ahead <= 0 {
		t.Errorf("expected ahead > 0, got %d", ahead)
	}
	if behind != 0 {
		t.Errorf("expected behind = 0, got %d", behind)
	}
}

// --- createWorktreeCmd ---

func TestCreateWorktreeCmd_NewBranch(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	cmd := createWorktreeCmd(mgr, stateMgr, repo, "new-feature", "")
	msg := cmd()

	created, ok := msg.(worktreeCreatedMsg)
	if !ok {
		t.Fatalf("expected worktreeCreatedMsg, got %T", msg)
	}
	if created.err != nil {
		t.Fatalf("unexpected error: %v", created.err)
	}
	if created.name != "new-feature" {
		t.Errorf("expected name 'new-feature', got %q", created.name)
	}

	// Verify worktree exists on disk
	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-new-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree directory does not exist at %s", wtPath)
	}
}

func TestCreateWorktreeCmd_WithBaseBranch(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	// When baseBranch is non-empty, createWorktreeCmd passes it as the -b arg.
	// This creates a new branch with that name, so we use a name that doesn't exist yet.
	cmd := createWorktreeCmd(mgr, stateMgr, repo, "auth-work", "auth-work")
	msg := cmd()

	created, ok := msg.(worktreeCreatedMsg)
	if !ok {
		t.Fatalf("expected worktreeCreatedMsg, got %T", msg)
	}
	if created.err != nil {
		t.Fatalf("unexpected error: %v", created.err)
	}

	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-auth-work")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree directory does not exist at %s", wtPath)
	}
}

func TestCreateWorktreeCmd_InvalidName(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	// Create a worktree, then try to create it again (duplicate)
	cmd := createWorktreeCmd(mgr, stateMgr, repo, "dup-test", "")
	cmd() // first creation

	cmd2 := createWorktreeCmd(mgr, stateMgr, repo, "dup-test", "")
	msg := cmd2()

	created, ok := msg.(worktreeCreatedMsg)
	if !ok {
		t.Fatalf("expected worktreeCreatedMsg, got %T", msg)
	}
	if created.err == nil {
		t.Error("expected error for duplicate worktree creation")
	}
}

func TestCreateWorktreeCmd_BranchInState(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	cmd := createWorktreeCmd(mgr, stateMgr, repo, "my-feature", "")
	msg := cmd()

	created, ok := msg.(worktreeCreatedMsg)
	if !ok {
		t.Fatalf("expected worktreeCreatedMsg, got %T", msg)
	}
	if created.err != nil {
		t.Fatalf("unexpected error: %v", created.err)
	}

	// Verify state records the actual git branch, not the worktree name
	ws, err := stateMgr.GetWorktree("my-feature")
	if err != nil {
		t.Fatalf("GetWorktree error: %v", err)
	}
	if ws == nil {
		t.Fatal("worktree not found in state")
	}
	// The branch should be the git branch name (which is "my-feature" when
	// created without baseBranch, but the key point is it comes from wt.Branch)
	if ws.Branch == "" {
		t.Error("expected non-empty branch in state")
	}
}

// --- deleteWorktreeCmd ---

func TestDeleteWorktreeCmd_Basic(t *testing.T) {
	repo := setupRailsFixtureWithWorktrees(t, "to-delete")
	mgr, stateMgr := newTestManagers(t, repo)

	cmd := deleteWorktreeCmd(mgr, stateMgr, repo, "to-delete", false)
	msg := cmd()

	deleted, ok := msg.(worktreeDeletedMsg)
	if !ok {
		t.Fatalf("expected worktreeDeletedMsg, got %T", msg)
	}
	if deleted.err != nil {
		t.Fatalf("unexpected error: %v", deleted.err)
	}

	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-to-delete")
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree directory still exists at %s", wtPath)
	}
}

func TestDeleteWorktreeCmd_WithBranch(t *testing.T) {
	repo := setupRailsFixtureWithWorktrees(t, "del-branch")
	mgr, stateMgr := newTestManagers(t, repo)

	cmd := deleteWorktreeCmd(mgr, stateMgr, repo, "del-branch", true)
	msg := cmd()

	deleted, ok := msg.(worktreeDeletedMsg)
	if !ok {
		t.Fatalf("expected worktreeDeletedMsg, got %T", msg)
	}
	if deleted.err != nil {
		t.Fatalf("unexpected error: %v", deleted.err)
	}
	if !deleted.deleteBranch {
		t.Error("expected deleteBranch=true")
	}

	// Verify worktree gone
	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-del-branch")
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree directory still exists at %s", wtPath)
	}
}

func TestDeleteWorktreeCmd_NonExistent(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	cmd := deleteWorktreeCmd(mgr, stateMgr, repo, "ghost", false)
	msg := cmd()

	deleted, ok := msg.(worktreeDeletedMsg)
	if !ok {
		t.Fatalf("expected worktreeDeletedMsg, got %T", msg)
	}
	if deleted.err == nil {
		t.Error("expected error for non-existent worktree")
	}
}

// --- bulkDeleteCmd ---

func TestBulkDeleteCmd_Multiple(t *testing.T) {
	repo := setupRailsFixtureWithWorktrees(t, "wt1", "wt2", "wt3")
	mgr, stateMgr := newTestManagers(t, repo)

	m := NewModel(mgr, stateMgr, repo)

	toDelete := []WorktreeItem{
		{ShortName: "wt1", Path: filepath.Join(filepath.Dir(repo), "rails-app-wt1")},
		{ShortName: "wt2", Path: filepath.Join(filepath.Dir(repo), "rails-app-wt2")},
	}

	cmd := m.bulkDeleteCmd(toDelete)
	msg := cmd()

	done, ok := msg.(bulkDeleteDoneMsg)
	if !ok {
		t.Fatalf("expected bulkDeleteDoneMsg, got %T", msg)
	}
	if done.count != 2 {
		t.Errorf("expected count=2, got %d", done.count)
	}

	// wt1 and wt2 should be gone
	for _, name := range []string{"wt1", "wt2"} {
		p := filepath.Join(filepath.Dir(repo), "rails-app-"+name)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("worktree %s still exists", name)
		}
	}

	// wt3 should remain
	p := filepath.Join(filepath.Dir(repo), "rails-app-wt3")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Errorf("worktree wt3 should still exist")
	}
}

// --- NewModel + Init ---

func TestNewModel_Integration(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	m := NewModel(mgr, stateMgr, repo)

	if m.worktreeMgr == nil {
		t.Error("expected worktreeMgr != nil")
	}
	if m.projectName != "rails-app" {
		t.Errorf("expected projectName 'rails-app', got %q", m.projectName)
	}
	if !m.loading {
		t.Error("expected loading=true on new model")
	}
}

func TestInit_ReturnsFetchCmd(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	m := NewModel(mgr, stateMgr, repo)
	cmd := m.Init()

	if cmd == nil {
		t.Fatal("expected Init() to return non-nil cmd")
	}

	// Execute the batch — it should contain fetchWorktrees which returns worktreesFetchedMsg
	// We can't easily decompose a tea.Batch, so we verify Init returned something.
	// The real proof is that fetchWorktrees works (tested above).
}

// --- Docker Integration ---

func TestFetchWorktrees_DockerComposePresent(t *testing.T) {
	repo := setupRailsFixture(t)

	// Verify fixture has docker-compose.yml
	composePath := filepath.Join(repo, "docker-compose.yml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		t.Fatal("fixture missing docker-compose.yml")
	}

	// Verify grove config has docker plugin
	configPath := filepath.Join(repo, ".grove", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "plugins.docker") {
		t.Error("grove config missing docker plugin section")
	}
}

func TestCreateWorktreeCmd_DockerComposeInherited(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	cmd := createWorktreeCmd(mgr, stateMgr, repo, "docker-test", "")
	msg := cmd()

	created := msg.(worktreeCreatedMsg)
	if created.err != nil {
		t.Fatalf("unexpected error: %v", created.err)
	}

	// Worktrees share the same git content, so docker-compose.yml is accessible
	// via the worktree's working directory (it's part of the repo)
	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-docker-test")
	composePath := filepath.Join(wtPath, "docker-compose.yml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		t.Error("new worktree should have docker-compose.yml from repo")
	}
}

// --- Update message handling ---

func TestUpdate_WorktreesFetchedMsg(t *testing.T) {
	repo := setupRailsFixtureWithWorktrees(t, "alpha", "beta")
	mgr, stateMgr := newTestManagers(t, repo)

	m := NewModel(mgr, stateMgr, repo)
	// Simulate WindowSizeMsg so ready=true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	// Now fetch worktrees
	msg := m.fetchWorktrees()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.loading {
		t.Error("expected loading=false after fetch")
	}

	items := m.list.Items()
	if len(items) != 3 {
		t.Errorf("expected 3 list items, got %d", len(items))
	}
}

func TestUpdate_WorktreeDeletedMsg(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	m := NewModel(mgr, stateMgr, repo)
	m.activeView = ViewDelete

	msg := worktreeDeletedMsg{name: "test", err: nil}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after delete, got %d", m.activeView)
	}
	if m.statusMsg == "" {
		t.Error("expected non-empty status message after successful delete")
	}
}

func TestUpdate_WorktreeCreatedMsg(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	m := NewModel(mgr, stateMgr, repo)
	m.activeView = ViewCreate
	m.createState = &CreateState{}

	msg := worktreeCreatedMsg{name: "new-wt", path: "/some/path"}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after create, got %d", m.activeView)
	}
	if m.statusMsg == "" {
		t.Error("expected non-empty status message after successful create")
	}
}

func TestUpdate_BulkDeleteDoneMsg(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	m := NewModel(mgr, stateMgr, repo)
	m.activeView = ViewBulk
	m.bulkState = &BulkState{}

	msg := bulkDeleteDoneMsg{count: 3}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after bulk delete, got %d", m.activeView)
	}
	if m.statusMsg == "" {
		t.Error("expected non-empty status message after bulk delete")
	}
}

