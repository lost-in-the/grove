package tui

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/plugins/tracker"
)

func omniTestSource() omniSource {
	items := makeTestItems(5) // root, feature-auth, fix-bug, testing, refactor
	items[1].AgentStatus = mux.AgentBlocked
	items[3].AgentStatus = mux.AgentDone
	return omniSource{
		Worktrees:  items,
		Actions:    omniActionsFrom(dashboardActions()),
		PRs:        []*tracker.PullRequest{{Number: 42, Title: "Rename the read path", Branch: "feat/rename-read"}},
		Issues:     []*tracker.Issue{{Number: 7, Title: "Login page crashes"}},
		Pins:       pinSet{"1": "root", "2": "fix-bug"},
		AgentAware: true,
	}
}

func groupNames(gs []omniGroup) []string {
	var out []string
	for _, g := range gs {
		out = append(out, g.Name)
	}
	return out
}

func labels(g omniGroup) []string {
	var out []string
	for _, it := range g.Items {
		out = append(out, it.Label)
	}
	return out
}

func TestOmniResults_GroupsAndOrder(t *testing.T) {
	gs := omniResults("re", omniScopeAll, omniTestSource())
	if got := groupNames(gs); !reflect.DeepEqual(got, []string{"actions", "worktrees", "pull requests", "issues"}) {
		t.Fatalf("groups = %v", got)
	}
	if l := labels(gs[0]); !slices.Contains(l, "rename worktree") {
		t.Errorf("actions = %v, want rename worktree among them", l)
	}
	if l := labels(gs[1]); len(l) == 0 || l[0] != "refactor" {
		t.Errorf("worktrees = %v, want refactor", l)
	}
	for _, g := range gs {
		if len(g.Items) > 5 {
			t.Errorf("group %s has %d items, want at most 5", g.Name, len(g.Items))
		}
	}
}

func TestOmniResults_Synonyms(t *testing.T) {
	for q, want := range map[string]string{"fork": "copy worktree", "checkout": "switch branch", "trash": "delete worktree"} {
		gs := omniResults(q, omniScopeActions, omniTestSource())
		if len(gs) == 0 || len(gs[0].Items) == 0 || gs[0].Items[0].Label != want {
			t.Errorf("%q: got %v, want %q first", q, gs, want)
		}
	}
}

// A synonym hit shows the synonym, and the highlight falls inside it.
func TestOmniResults_SynonymHitShowsSynonym(t *testing.T) {
	gs := omniResults("fork", omniScopeActions, omniTestSource())
	it := gs[0].Items[0]
	if it.Matched != "fork" {
		t.Fatalf("matched = %q, want fork", it.Matched)
	}
	for _, h := range it.Hits {
		if h < 0 || h >= len([]rune(it.Matched)) {
			t.Errorf("hit %d outside %q", h, it.Matched)
		}
	}
}

func TestOmniResults_EmptyQuery(t *testing.T) {
	src := omniTestSource()
	now := testNow
	for i := range src.Worktrees {
		src.Worktrees[i].LastAccessed = now.Add(-time.Duration(len(src.Worktrees)-i) * time.Hour)
	}
	gs := omniResults("", omniScopeAll, src)
	if got := groupNames(gs); !reflect.DeepEqual(got, []string{"needs you", "pinned", "recent"}) {
		t.Fatalf("groups = %v", got)
	}
	if l := labels(gs[0]); !reflect.DeepEqual(l, []string{"feature-auth", "testing"}) {
		t.Errorf("needs you = %v (blocked first, then done)", l)
	}
	if l := labels(gs[1]); !reflect.DeepEqual(l, []string{"root", "fix-bug"}) {
		t.Errorf("pinned = %v (slot order)", l)
	}
	if l := labels(gs[2]); !reflect.DeepEqual(l, []string{"refactor", "testing", "feature-auth"}) {
		t.Errorf("recent = %v (newest first, without pinned or current)", l)
	}
	src.AgentAware = false
	if got := groupNames(omniResults("", omniScopeAll, src)); got[0] == "needs you" {
		t.Error("needs you shown without agent awareness")
	}
}

func TestOmniResults_ActionsScope(t *testing.T) {
	src := omniTestSource()
	gs := omniResults("", omniScopeActions, src)
	if got := groupNames(gs); !reflect.DeepEqual(got, []string{"actions"}) {
		t.Fatalf("groups = %v", got)
	}
	if len(gs[0].Items) != len(src.Actions) {
		t.Errorf("empty actions scope lists %d of %d actions", len(gs[0].Items), len(src.Actions))
	}
	if got := groupNames(omniResults("ren", omniScopeActions, src)); !reflect.DeepEqual(got, []string{"actions"}) {
		t.Errorf("actions scope with a query: groups %v", got)
	}
}

// Omni lists only runnable actions: no list-owned keys, strip openers, or
// digit slots.
func TestOmniActionsFrom_RunnableOnly(t *testing.T) {
	for _, a := range omniActionsFrom(dashboardActions()) {
		switch a.ID {
		case "list.navigate", "list.filter", "list.page", "goto.open", "pin.open", "pin.switch", "detail.close", "app.back", "app.help", "app.update":
			t.Errorf("omni offers %s", a.ID)
		}
		if a.Run == nil {
			t.Errorf("omni offers %s, which has nothing to run", a.ID)
		}
	}
}

func TestHighlightRunes(t *testing.T) {
	// ć and é take two bytes each; é is byte 4 / rune 3, b is byte 7 / rune 5.
	if got := highlightRunes("ćafé-bar", []int{4, 7}); !reflect.DeepEqual(got, []int{3, 5}) {
		t.Errorf("got %v, want [3 5]", got)
	}
}

func TestOmniResults_200ItemsFast(t *testing.T) {
	src := omniTestSource()
	src.Worktrees = nil
	for i := 0; i < 200; i++ {
		src.Worktrees = append(src.Worktrees, WorktreeItem{ShortName: fmt.Sprintf("feature-%03d-thing", i), Branch: fmt.Sprintf("feat/thing-%03d", i)})
	}
	start := time.Now()
	for i := 0; i < 50; i++ {
		omniResults("thg9", omniScopeAll, src)
	}
	if d := time.Since(start); d > 250*time.Millisecond {
		t.Errorf("50 rounds over 200 worktrees took %v", d)
	}
}

// Omni does not offer itself, and action labels read as full phrases.
func TestOmniActionsFrom_DescriptiveAndNotItself(t *testing.T) {
	byID := map[string]Action{}
	for _, a := range omniActionsFrom(dashboardActions()) {
		byID[a.ID] = a
	}
	if _, ok := byID["omni.open"]; ok {
		t.Error("omni offers find, which opens omni")
	}
	for id, want := range map[string]string{
		"worktree.switch": "switch to worktree", "worktree.switch_up": "switch and start containers",
		"worktree.new": "new worktree", "detail.open": "show detail", "list.sort": "change sort",
	} {
		if got := byID[id].Label; got != want {
			t.Errorf("%s label = %q, want %q", id, got, want)
		}
	}
}
