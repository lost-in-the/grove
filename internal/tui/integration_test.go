//go:build integration

package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/worktree"
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

// drainCreationStream invokes a streaming creation command, walks all
// intermediate creationLogMsg events, and returns the final creationDoneMsg.
// Used by tests that previously expected a single worktreeCreatedMsg before
// the streaming refactor in commands.go.
func drainCreationStream(t *testing.T, cmd tea.Cmd) creationDoneMsg {
	t.Helper()
	for {
		switch m := cmd().(type) {
		case creationLogMsg:
			cmd = readCreationLog(m.ch, m.source)
		case creationDoneMsg:
			return m
		default:
			t.Fatalf("unexpected message type %T", m)
			return creationDoneMsg{}
		}
	}
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

	cmd := createWorktreeCmd(mgr, stateMgr, nil, repo, "new-feature", "", "", "")
	created := drainCreationStream(t, cmd)
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

	// When baseBranch is non-empty, createWorktreeCmd uses CreateFromBranch
	// which checks the worktree out at an existing branch. The branch must
	// already exist and not be checked out elsewhere — main fails because
	// the fixture repo itself is checked out at main.
	runGit(t, repo, "branch", "feature-base")
	cmd := createWorktreeCmd(mgr, stateMgr, nil, repo, "auth-work", "feature-base", "", "")
	created := drainCreationStream(t, cmd)
	if created.err != nil {
		t.Fatalf("unexpected error: %v", created.err)
	}

	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-auth-work")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree directory does not exist at %s", wtPath)
	}
}

// TestCreateWorktreeCmd_NewBranchNameThreaded verifies the typed new-branch
// name is used for the git branch even when it differs from the worktree name
// (regression for the create wizard discarding NewBranchName).
func TestCreateWorktreeCmd_NewBranchNameThreaded(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	// Worktree name "login" differs from the typed branch "feature/login-fix".
	// Before the fix, the branch was named after the worktree and the typed
	// name was silently lost.
	cmd := createWorktreeCmd(mgr, stateMgr, nil, repo, "login", "", "feature/login-fix", "")
	created := drainCreationStream(t, cmd)
	if created.err != nil {
		t.Fatalf("unexpected error: %v", created.err)
	}

	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-login")
	branch := strings.TrimSpace(runGit(t, wtPath, "rev-parse", "--abbrev-ref", "HEAD"))
	if branch != "feature/login-fix" {
		t.Errorf("expected worktree on branch %q, got %q", "feature/login-fix", branch)
	}
}

// TestCreateWorktreeCmd_ForkFromBase verifies a fork creates a new branch based
// on the selected base ref rather than HEAD (regression for the fork action
// dropping the base branch).
func TestCreateWorktreeCmd_ForkFromBase(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	// Point "develop" at the current commit, then advance main so HEAD differs
	// from develop. A fork from develop must land on develop's commit, not HEAD.
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	runGit(t, repo, "branch", "develop", base)
	if err := os.WriteFile(filepath.Join(repo, "advance.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "advance.txt")
	runGit(t, repo, "commit", "-m", "advance main")
	head := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	if head == base {
		t.Fatal("expected main HEAD to advance past develop")
	}

	// Fork: no explicit new-branch name (named after the worktree), fromRef=develop.
	cmd := createWorktreeCmd(mgr, stateMgr, nil, repo, "forked", "", "", "develop")
	created := drainCreationStream(t, cmd)
	if created.err != nil {
		t.Fatalf("unexpected error: %v", created.err)
	}

	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-forked")
	got := strings.TrimSpace(runGit(t, wtPath, "rev-parse", "HEAD"))
	if got != base {
		t.Errorf("expected forked worktree at develop's commit %q, got %q", base, got)
	}
	branch := strings.TrimSpace(runGit(t, wtPath, "rev-parse", "--abbrev-ref", "HEAD"))
	if branch != "forked" {
		t.Errorf("expected new branch %q named after worktree, got %q", "forked", branch)
	}
}

func TestCreateWorktreeCmd_InvalidName(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	// Create a worktree, then try to create it again (duplicate)
	cmd := createWorktreeCmd(mgr, stateMgr, nil, repo, "dup-test", "", "", "")
	drainCreationStream(t, cmd) // first creation

	cmd2 := createWorktreeCmd(mgr, stateMgr, nil, repo, "dup-test", "", "", "")
	created := drainCreationStream(t, cmd2)
	if created.err == nil {
		t.Error("expected error for duplicate worktree creation")
	}
}

func TestCreateWorktreeCmd_BranchInState(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	cmd := createWorktreeCmd(mgr, stateMgr, nil, repo, "my-feature", "", "", "")
	created := drainCreationStream(t, cmd)
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

	cmd := deleteWorktreeCmd(mgr, stateMgr, nil, repo, "to-delete", false)
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

	cmd := deleteWorktreeCmd(mgr, stateMgr, nil, repo, "del-branch", true)
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

	cmd := deleteWorktreeCmd(mgr, stateMgr, nil, repo, "ghost", false)
	msg := cmd()

	deleted, ok := msg.(worktreeDeletedMsg)
	if !ok {
		t.Fatalf("expected worktreeDeletedMsg, got %T", msg)
	}
	if deleted.err == nil {
		t.Error("expected error for non-existent worktree")
	}
}

// TestDeleteWorktreeCmd_RequiredPreRemoveHookAborts mirrors the CLI's B7
// guarantee in the dashboard: a required (on_failure="fail") pre-remove hook
// that fails must abort the delete and leave the worktree in place — `grove
// rm` already refused, but the TUI logged the failure and deleted anyway.
func TestDeleteWorktreeCmd_RequiredPreRemoveHookAborts(t *testing.T) {
	repo := setupRailsFixtureWithWorktrees(t, "guarded")
	mgr, stateMgr := newTestManagers(t, repo)

	hooksToml := `[hooks]
[[hooks.pre_remove]]
type = "command"
command = "exit 1"
on_failure = "fail"
working_dir = "main"
`
	if err := os.WriteFile(filepath.Join(repo, ".grove", "hooks.toml"), []byte(hooksToml), 0o644); err != nil {
		t.Fatalf("write hooks.toml: %v", err)
	}

	cmd := deleteWorktreeCmd(mgr, stateMgr, nil, repo, "guarded", false)
	msg := cmd()

	deleted, ok := msg.(worktreeDeletedMsg)
	if !ok {
		t.Fatalf("expected worktreeDeletedMsg, got %T", msg)
	}
	if deleted.err == nil {
		t.Fatal("expected required pre-remove hook failure to abort the delete")
	}

	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-guarded")
	if _, err := os.Stat(wtPath); err != nil {
		t.Errorf("worktree was deleted despite required hook failure: %v", err)
	}
}

// TestDeleteWorktreeCmd_RunsMainDirHooks pins two dashboard-delete fixes at
// once: a pre_remove hook with working_dir="main" must run in the project root
// (proving MainPath is set — an empty one ran it in the dashboard's cwd and
// expanded {{.main_path}} to nothing), and hooks.toml post_remove actions must
// run at all (the TUI used to skip them, unlike `grove rm`).
func TestDeleteWorktreeCmd_RunsMainDirHooks(t *testing.T) {
	repo := setupRailsFixtureWithWorktrees(t, "cleanup")
	mgr, stateMgr := newTestManagers(t, repo)

	hooksToml := `[hooks]
[[hooks.pre_remove]]
type = "command"
command = "touch pre-remove-ran"
working_dir = "main"
[[hooks.post_remove]]
type = "command"
command = "touch post-remove-ran"
working_dir = "main"
`
	if err := os.WriteFile(filepath.Join(repo, ".grove", "hooks.toml"), []byte(hooksToml), 0o644); err != nil {
		t.Fatalf("write hooks.toml: %v", err)
	}

	cmd := deleteWorktreeCmd(mgr, stateMgr, nil, repo, "cleanup", false)
	msg := cmd()
	deleted, ok := msg.(worktreeDeletedMsg)
	if !ok {
		t.Fatalf("expected worktreeDeletedMsg, got %T", msg)
	}
	if deleted.err != nil {
		t.Fatalf("delete failed: %v", deleted.err)
	}

	if _, err := os.Stat(filepath.Join(repo, "pre-remove-ran")); err != nil {
		t.Errorf("pre_remove working_dir=main did not run in the main worktree (MainPath unset?): %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "post-remove-ran")); err != nil {
		t.Errorf("post_remove hook did not run on TUI delete: %v", err)
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

	cmd := createWorktreeCmd(mgr, stateMgr, nil, repo, "docker-test", "", "", "")
	created := drainCreationStream(t, cmd)
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
	if m.toast.Message() == "" {
		t.Error("expected non-empty status message after successful delete")
	}
}

func TestUpdate_WorktreeCreatedMsg(t *testing.T) {
	repo := setupRailsFixture(t)
	mgr, stateMgr := newTestManagers(t, repo)

	m := NewModel(mgr, stateMgr, repo)
	m.activeView = ViewCreate
	m.createState = &CreateState{}

	msg := worktreeCreatedMsg{name: "new-wt"}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after create, got %d", m.activeView)
	}
	if m.toast.Message() == "" {
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
	if m.toast.Message() == "" {
		t.Error("expected non-empty status message after bulk delete")
	}
}
