package tui

// pinSet maps a slot digit ("1".."9") to the alias pinned there (spec 8).
// Every method returns a new set; the receiver is never changed.
type pinSet map[string]string

func (p pinSet) clone() pinSet {
	out := make(pinSet, len(p))
	for k, v := range p {
		out[k] = v
	}
	return out
}

// slotOf returns the slot alias is pinned to, or "".
func (p pinSet) slotOf(alias string) string {
	for slot, a := range p {
		if a == alias {
			return slot
		}
	}
	return ""
}

// pin applies "m <slot>" to alias: pin it there, unpin it if it already
// holds that slot, or take the slot from another worktree. A worktree holds
// at most one slot. Slot "0" unpins alias from any slot.
func (p pinSet) pin(slot, alias string) pinSet {
	out := p.drop(alias)
	if slot == "0" || p[slot] == alias {
		return out
	}
	out[slot] = alias
	return out
}

// rename moves a pin from old to new, keeping its slot.
func (p pinSet) rename(old, new string) pinSet {
	out := p.clone()
	if slot := p.slotOf(old); slot != "" {
		out[slot] = new
	}
	return out
}

// drop removes alias's pin.
func (p pinSet) drop(alias string) pinSet {
	out := p.clone()
	for slot, a := range p {
		if a == alias {
			delete(out, slot)
		}
	}
	return out
}

// prune keeps only pins whose alias still exists.
func (p pinSet) prune(aliases map[string]bool) pinSet {
	out := pinSet{}
	for slot, a := range p {
		if aliases[a] {
			out[slot] = a
		}
	}
	return out
}
