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
			if !have[k] {
				t.Errorf("binding %s key %q has no registry action", name, k)
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

// b on the main worktree opens nothing, and the key stops there.
func TestDispatch_DisabledActionIsConsumed(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.list.Select(0)
	next, cmd := m.Update(makeKeyMsg("b"))
	if next.(Model).activeView() != ViewDashboard || cmd != nil {
		t.Errorf("b on main: view %v cmd %v, want nothing", next.(Model).activeView(), cmd != nil)
	}
}

func TestFooter_DashboardHintsFromRegistry(t *testing.T) {
	var want []Hint
	for _, a := range dashboardActions() {
		if a.Footer > 0 {
			want = append(want, Hint{a.display(), a.Label})
		}
	}
	got := NewHelpFooter().CompactHints(ViewDashboard)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("footer = %v, want %v", got, want)
	}
	found := false
	for _, h := range got {
		found = found || (h.Key == "g" && h.Description == "goto")
	}
	if !found {
		t.Error("footer lacks g goto")
	}
}

// The dashboard handler is a table lookup: every action with a Run is
// reachable by its key.
func TestDispatch_EveryBareActionIsReachable(t *testing.T) {
	for _, a := range dashboardActions() {
		if a.Layer != "" || a.Run == nil {
			continue
		}
		for _, k := range a.Keys {
			if got, ok := findAction("", k); !ok || got.ID != a.ID {
				t.Errorf("key %q finds %q, want %s", k, got.ID, a.ID)
			}
		}
	}
}
