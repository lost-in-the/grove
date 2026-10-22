package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/git"
)

// editorClosedMsg arrives when the editor opened from the review exits.
type editorClosedMsg struct{ err error }

// execEditor hands the terminal to the editor and resumes the frame when it
// exits (spec 13.4). Tests replace it to see the command.
var execEditor = func(c *exec.Cmd) tea.Cmd {
	return tea.ExecProcess(c, func(err error) tea.Msg { return editorClosedMsg{err} })
}

// editorArgs is the command that opens file at line in editor (plan 8d
// decision 1): $VISUAL or $EDITOR as given, split on spaces so "code -w"
// keeps its flag (a quoted path inside it would split wrongly; rare), with
// the line flag its family takes. An empty editor is vi.
func editorArgs(editor, file string, line int) (string, []string) {
	fields := strings.Fields(editor)
	if len(fields) == 0 {
		fields = []string{"vi"}
	}
	n := strconv.Itoa(line)
	args := fields[1:]
	switch filepath.Base(fields[0]) {
	case "code", "code-insiders", "cursor", "codium":
		args = append(args, "-g", file+":"+n)
	case "subl", "zed", "mate":
		args = append(args, file+":"+n)
	case "idea", "goland", "webstorm":
		args = append(args, "--line", n, file)
	default:
		args = append(args, "+"+n, file)
	}
	return fields[0], args
}

// editorLine is the new-side number of the first line in view from row
// scroll down (plan 8d decision 3), laid out as renderFileDiff lays the
// file out; 1 when no line in view has one.
func editorLine(f git.FileDiff, scroll int, folded bool) int {
	var rows []int // the new-side number on each row, 0 for none
	for _, h := range f.Hunks {
		rows = append(rows, 0)
		for i := 0; i < len(h.Lines); i++ {
			if folded && h.Lines[i].Kind == git.LineRemoved {
				run := 0
				for i+run < len(h.Lines) && h.Lines[i+run].Kind == git.LineRemoved {
					run++
				}
				if run >= foldRun {
					rows = append(rows, 0)
					i += run - 1
					continue
				}
			}
			rows = append(rows, h.Lines[i].NewNo)
		}
	}
	for i := max(0, scroll); i < len(rows); i++ {
		if rows[i] > 0 {
			return rows[i]
		}
	}
	return 1
}

// openInEditor opens the files tab's current file in the editor at the top
// line in view, in the worktree checked out on the PR's branch.
func (m Model) openInEditor() (Model, tea.Cmd) {
	s := m.prReview
	f := s.Files[s.FileIdx]
	if f.Status == git.FileDeleted {
		m.toast.Show(NewToast("file was deleted in this change", ToastInfo))
		return m, nil
	}
	root := ""
	if s.Local != nil {
		root = s.Local.Path // the reviewed worktree, whatever its branch
	} else {
		for _, w := range m.existingWorktreeItems() {
			if w.Branch == s.PR.Branch {
				root = w.Path
				break
			}
		}
	}
	if root == "" {
		m.toast.Show(NewToast("no worktree on "+safeText(s.PR.Branch)+" · n in the PR browser creates one", ToastInfo))
		return m, nil
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	file := filepath.Join(root, f.Path())
	if rel, err := filepath.Rel(root, file); err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		m.toast.Show(NewToast("the file's path leads outside the worktree", ToastWarning))
		return m, nil
	}
	name, args := editorArgs(editor, file, editorLine(f, s.Scroll, s.Folded))
	return m, execEditor(exec.Command(name, args...))
}

func (m Model) handleEditorClosed(msg editorClosedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.toast.Show(NewToast("editor: "+msg.err.Error(), ToastWarning))
	}
	return m, nil
}
