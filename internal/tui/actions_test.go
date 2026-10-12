package tui

import (
	"reflect"
	"testing"

	"charm.land/bubbles/v2/key"
)

// Overlay-only bindings: they belong to overlays, not the dashboard.
var overlayOnlyBindings = map[string]bool{
	"Confirm": true, "Deny": true, "Toggle": true, "Back": true, "ShiftTab": true,
}

func registryKeys() map[string]bool {
	keys := map[string]bool{}
	for _, a := range dashboardActions() {
		for _, k := range a.Keys {
			keys[a.Layer+" "+k] = true
			keys[k] = true
		}
	}
	return keys
}

// Every key.go binding has a registry action, so the footer, strips, and
// dispatch cannot drift from the key map.
func TestRegistry_EveryKeyMapBindingHasAction(t *testing.T) {
	have := registryKeys()
	km := reflect.ValueOf(DefaultKeyMap())
	for i := 0; i < km.NumField(); i++ {
		name := km.Type().Field(i).Name
		b, ok := km.Field(i).Interface().(key.Binding)
		if !ok || overlayOnlyBindings[name] {
			continue
		}
		for _, k := range b.Keys() {
			if !have[" "+k] {
				t.Errorf("binding %s key %q has no bare-key registry action", name, k)
			}
		}
	}
}

func TestRegistry_EveryActionKeyHasBinding(t *testing.T) {
	bound := map[string]bool{}
	km := reflect.ValueOf(DefaultKeyMap())
	for i := 0; i < km.NumField(); i++ {
		if b, ok := km.Field(i).Interface().(key.Binding); ok {
			for _, k := range b.Keys() {
				bound[k] = true
			}
		}
	}
	for _, a := range dashboardActions() {
		if a.Layer != "" {
			continue // strip keys follow a prefix; they need no binding of their own
		}
		for _, k := range a.Keys {
			if !bound[k] {
				t.Errorf("action %s key %q has no keys.go binding", a.ID, k)
			}
		}
	}
}

func TestRegistry_NoDuplicateKeysPerLayer(t *testing.T) {
	seen := map[string]string{}
	for _, a := range dashboardActions() {
		for _, k := range a.Keys {
			id := a.Layer + " " + k
			if prev, ok := seen[id]; ok {
				t.Errorf("key %q in layer %q is on both %s and %s", k, a.Layer, prev, a.ID)
			}
			seen[id] = a.ID
		}
	}
}

func TestRegistry_LabelsAndSynonyms(t *testing.T) {
	want := map[string][]string{
		"worktree.copy":          {"fork", "duplicate", "clone"},
		"worktree.switch_branch": {"checkout", "change branch"},
		"worktree.sync":          {"pull changes", "bring over"},
		"worktree.delete":        {"remove", "trash"},
	}
	byID := map[string]Action{}
	for _, a := range dashboardActions() {
		if a.Label == "" {
			t.Errorf("action %s has no label", a.ID)
		}
		byID[a.ID] = a
	}
	for id, syns := range want {
		a, ok := byID[id]
		if !ok {
			t.Errorf("no action %s", id)
			continue
		}
		have := map[string]bool{}
		for _, s := range a.Synonyms {
			have[s] = true
		}
		for _, s := range syns {
			if !have[s] {
				t.Errorf("%s lacks synonym %q", id, s)
			}
		}
	}
}

// Delete, rename, and switch branch are disabled on the main worktree, and
// a disabled action still consumes its key.
func TestDispatch_DisabledActionIsConsumed(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.list.Select(0)
	for _, k := range []string{"b", "d", "R"} {
		a, ok := findAction("", k)
		if !ok || a.Enabled == nil || a.Enabled(m) {
			t.Errorf("%s should be disabled on the main worktree", k)
		}
		next, cmd := m.Update(makeKeyMsg(k))
		if next.(Model).activeView() != ViewDashboard || cmd != nil {
			t.Errorf("%s on main: view %v cmd %v, want nothing", k, next.(Model).activeView(), cmd != nil)
		}
	}
	m.list.Select(1)
	if a, _ := findAction("", "b"); !a.Enabled(m) {
		t.Error("b should be enabled on a regular worktree")
	}
}

func TestFooter_DashboardHintsFromRegistry(t *testing.T) {
	want := []Hint{
		{"↑↓", "navigate"}, {"enter", "switch"}, {"U", "up"}, {"n", "new"}, {"/", "filter"},
		{"tab", "detail"}, {"o", "sort"}, {"g", "goto"}, {"?", "help"}, {"q", "quit"},
	}
	if got := NewHelpFooter().CompactHints(ViewDashboard); !reflect.DeepEqual(got, want) {
		t.Errorf("footer = %v, want %v", got, want)
	}
}
