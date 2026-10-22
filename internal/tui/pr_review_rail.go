package tui

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/git"
)

// railMinWidth and railShare size the files tab's rail; railBreakpoint is
// the width below which it starts hidden (plan 8b decision 5).
const (
	railMinWidth   = 24
	railShare      = 30 // percent of the body
	railBreakpoint = 100
)

// railLine is one row of the rail: a folder (file -1) or a file.
type railLine struct {
	text string
	file int
}

type railNode struct {
	name           string
	children       map[string]*railNode
	file           int // index into the files, or -1 for a folder
	added, removed int
}

// railLines lays the files out as a folder tree: folders before files,
// each sorted by name, a folder holding one folder joined with it
// ("cmd/grove/"), and every row ending in its +added -removed. It returns
// the files' order in the tree, which J and K follow, and the rows.
func railLines(files []git.FileDiff, viewed map[string]bool) ([]int, []railLine) {
	root := &railNode{children: map[string]*railNode{}, file: -1}
	for i, f := range files {
		n := root
		parts := strings.Split(f.Path(), "/")
		for j, part := range parts {
			n.added += f.Added
			n.removed += f.Removed
			child := n.children[part]
			if child == nil {
				child = &railNode{name: part, children: map[string]*railNode{}, file: -1}
				n.children[part] = child
			}
			if j == len(parts)-1 {
				child.file, child.added, child.removed = i, f.Added, f.Removed
			}
			n = child
		}
	}
	var order []int
	var lines []railLine
	var walk func(n *railNode, depth int)
	walk = func(n *railNode, depth int) {
		kids := make([]*railNode, 0, len(n.children))
		for _, c := range n.children {
			kids = append(kids, c)
		}
		sort.Slice(kids, func(a, b int) bool {
			if fa, fb := kids[a].file < 0, kids[b].file < 0; fa != fb {
				return fa
			}
			return kids[a].name < kids[b].name
		})
		indent := strings.Repeat("  ", depth)
		for _, c := range kids {
			stats := " " + Styles.SuccessText.Render(fmt.Sprintf("+%d", c.added)) + " " + Styles.ErrorText.Render(fmt.Sprintf("-%d", c.removed))
			if c.file >= 0 {
				mark := ""
				if viewed[files[c.file].Path()] {
					mark = Styles.SuccessText.Render("✓ ")
				}
				order = append(order, c.file)
				lines = append(lines, railLine{text: indent + mark + safeText(c.name) + stats, file: c.file})
				continue
			}
			name, n := c.name+"/", c
			for len(n.children) == 1 {
				only := firstChild(n)
				if only.file >= 0 {
					break
				}
				name, n = name+only.name+"/", only
			}
			lines = append(lines, railLine{text: indent + Styles.TextMuted.Render(safeText(name)) + stats, file: -1})
			walk(n, depth+1)
		}
	}
	walk(root, 0)
	return order, lines
}

func firstChild(n *railNode) *railNode {
	for _, c := range n.children {
		return c
	}
	return nil
}

// renderRail draws the rail rows that fit in height, scrolled so the
// current file shows, with the current file reversed.
func renderRail(lines []railLine, cur, width, height int) string {
	at := 0
	for i, l := range lines {
		if l.file == cur {
			at = i
		}
	}
	start := max(0, min(at-height/2, len(lines)-height))
	rows := make([]string, 0, height)
	for i := start; i < len(lines) && len(rows) < height; i++ {
		text := ansi.Truncate(lines[i].text, width, "…")
		if lines[i].file == cur {
			text = lipgloss.NewStyle().Reverse(true).Render(ansi.Strip(text) + strings.Repeat(" ", max(0, width-lipgloss.Width(text))))
		}
		rows = append(rows, text)
	}
	return strings.Join(rows, "\n")
}

// fileHash identifies a file's diff for its viewed mark: the hunks'
// headers and every line's kind and text. A changed diff changes it.
func fileHash(f git.FileDiff) string {
	h := sha256.New()
	for _, hunk := range f.Hunks {
		_, _ = fmt.Fprintln(h, hunk.Header)
		for _, l := range hunk.Lines {
			_, _ = fmt.Fprintf(h, "%d%s\n", l.Kind, l.Text)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// viewedNow is the set of files whose saved mark still matches their diff.
func (s *PRReviewState) viewedNow() map[string]bool {
	out := map[string]bool{}
	for _, f := range s.Files {
		if s.Viewed[f.Path()] == fileHash(f) {
			out[f.Path()] = true
		}
	}
	return out
}

// toggleViewed marks or unmarks the current file and saves the PR's marks,
// keeping only those that still match their file.
func (m Model) toggleViewed() {
	s := m.prReview
	f := s.Files[s.FileIdx]
	if s.Viewed == nil {
		s.Viewed = map[string]string{}
	}
	if h := fileHash(f); s.Viewed[f.Path()] == h {
		delete(s.Viewed, f.Path())
	} else {
		s.Viewed[f.Path()] = h
	}
	keep := map[string]string{}
	for path := range s.viewedNow() {
		keep[path] = s.Viewed[path]
	}
	s.Viewed = keep
	key := strconv.Itoa(s.PR.Number)
	_ = updateUIPrefs(m.projectRoot, func(p *UIPrefs) {
		if p.Viewed == nil {
			p.Viewed = map[string]map[string]string{}
		}
		if len(keep) == 0 {
			delete(p.Viewed, key)
			return
		}
		p.Viewed[key] = keep
	})
}

// savedViewed is a PR's marks from the UI preferences.
func savedViewed(projectRoot string, number int) map[string]string {
	if p := loadUIPrefs(projectRoot); p != nil {
		return p.Viewed[strconv.Itoa(number)]
	}
	return nil
}
