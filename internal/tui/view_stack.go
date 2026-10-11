package tui

// viewFrame is one level of navigation above the dashboard: the view shown,
// and the dashboard cursor to restore when the frame is popped.
type viewFrame struct {
	view        ActiveView
	returnPath  string // worktree selected when the frame was pushed
	returnIndex int    // fallback row when that worktree is gone
}

// viewStack holds the frames above the dashboard root. Model is copied by
// value on every update, so push always allocates: two copies of a stack
// never share a backing array that one of them could overwrite.
type viewStack []viewFrame

func (s viewStack) top() ActiveView {
	if len(s) == 0 {
		return ViewDashboard
	}
	return s[len(s)-1].view
}

func (s viewStack) push(f viewFrame) viewStack {
	out := make(viewStack, len(s), len(s)+1)
	copy(out, s)
	return append(out, f)
}

// pop removes the top frame. At the root it changes nothing and ok is false.
func (s viewStack) pop() (viewStack, viewFrame, bool) {
	if len(s) == 0 {
		return s, viewFrame{}, false
	}
	return s[: len(s)-1 : len(s)-1], s[len(s)-1], true
}

// toRoot removes every frame and returns the bottom one, which holds the
// cursor from before any frame opened.
func (s viewStack) toRoot() (viewStack, viewFrame, bool) {
	if len(s) == 0 {
		return s, viewFrame{}, false
	}
	return nil, s[0], true
}

// restoreCursor selects the worktree saved in f. A refresh can reorder or
// remove rows while a frame is open, so the path wins over the row index.
// Indexes refer to visible rows, which differ from all rows under a filter.
func (m *Model) restoreCursor(f viewFrame) {
	rows := m.list.VisibleItems()
	if len(rows) == 0 {
		return
	}
	target := min(f.returnIndex, len(rows)-1)
	if f.returnPath != "" {
		for i, row := range rows {
			if item, ok := row.(WorktreeItem); ok && item.Path == f.returnPath {
				target = i
				break
			}
		}
	}
	m.list.Select(target)
	m.updateDetailContent()
}
