package tui

import (
	"reflect"
	"testing"
)

func TestPinSet_Rules(t *testing.T) {
	for _, c := range []struct {
		name        string
		start       pinSet
		slot, alias string
		want        pinSet
	}{
		{"pin to an empty slot", pinSet{}, "3", "auth", pinSet{"3": "auth"}},
		{"same slot again unpins", pinSet{"3": "auth"}, "3", "auth", pinSet{}},
		{"slot held by another moves it", pinSet{"3": "login"}, "3", "auth", pinSet{"3": "auth"}},
		{"a worktree holds one slot", pinSet{"2": "auth"}, "5", "auth", pinSet{"5": "auth"}},
		{"0 unpins from any slot", pinSet{"7": "auth", "1": "root"}, "0", "auth", pinSet{"1": "root"}},
		{"0 on an unpinned worktree changes nothing", pinSet{"1": "root"}, "0", "auth", pinSet{"1": "root"}},
	} {
		before := clonePins(c.start)
		if got := c.start.pin(c.slot, c.alias); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
		if !reflect.DeepEqual(c.start, before) {
			t.Errorf("%s: input changed to %v", c.name, c.start)
		}
	}
}

func TestPinSet_SlotOf(t *testing.T) {
	p := pinSet{"4": "auth"}
	if p.slotOf("auth") != "4" || p.slotOf("root") != "" {
		t.Errorf("slotOf: %q %q", p.slotOf("auth"), p.slotOf("root"))
	}
}

func TestPinSet_Upkeep(t *testing.T) {
	p := pinSet{"1": "root", "2": "auth", "3": "gone"}
	if got := p.rename("auth", "oauth"); !reflect.DeepEqual(got, pinSet{"1": "root", "2": "oauth", "3": "gone"}) {
		t.Errorf("rename: %v", got)
	}
	if got := p.drop("auth"); !reflect.DeepEqual(got, pinSet{"1": "root", "3": "gone"}) {
		t.Errorf("drop: %v", got)
	}
	if got := p.prune(map[string]bool{"root": true, "auth": true}); !reflect.DeepEqual(got, pinSet{"1": "root", "2": "auth"}) {
		t.Errorf("prune: %v", got)
	}
	if len(p) != 3 {
		t.Errorf("input changed: %v", p)
	}
}

func clonePins(p pinSet) pinSet {
	out := pinSet{}
	for k, v := range p {
		out[k] = v
	}
	return out
}
