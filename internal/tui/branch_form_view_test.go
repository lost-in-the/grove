package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
)

type formCase struct {
	name  string
	build func() *BranchFormState
}

func formCases() []formCase {
	return []formCase{
		{"empty", func() *BranchFormState { return newBranchForm(testFormOptions()) }},
		{"new_branch", func() *BranchFormState {
			s := newBranchForm(testFormOptions())
			s.Branch.SetValue("abc-12")
			s.syncAlias()
			return s
		}},
		{"existing_branch", func() *BranchFormState {
			s := newBranchForm(testFormOptions())
			s.Branch.SetValue("fix/login")
			s.syncAlias()
			return s
		}},
		{"in_use", func() *BranchFormState {
			s := newBranchForm(testFormOptions())
			s.Branch.SetValue("feature/auth")
			s.syncAlias()
			return s
		}},
		{"error", func() *BranchFormState {
			s := newBranchForm(testFormOptions())
			s.Branch.SetValue("abc-12")
			s.syncAlias()
			s.Status = "failed to create worktree: something went wrong in git"
			return s
		}},
		{"creating", func() *BranchFormState {
			s := newBranchForm(testFormOptions())
			s.Branch.SetValue("abc-12")
			s.syncAlias()
			s.Busy = true
			s.Log = NewActivityLog(formLogWidth(80), 6)
			s.Log.AddLine("Creating worktree 'abc-12'...")
			return s
		}},
		{"copy_checking", func() *BranchFormState { return newCopyForm(testFormOptions(), testSource()) }},
		{"copy_dirty", func() *BranchFormState {
			s := newCopyForm(testFormOptions(), testSource())
			s.setWIP("/r/auth", true, []string{"a.go", "b.go"}, nil)
			return s
		}},
		{"switch_clean", func() *BranchFormState {
			s := newSwitchForm(testFormOptions(), testSource())
			s.setWIP("/r/auth", false, nil, nil)
			return s
		}},
		{"switch_dirty", func() *BranchFormState {
			s := newSwitchForm(testFormOptions(), testSource())
			s.setWIP("/r/auth", true, []string{"a.go"}, nil)
			return s
		}},
	}
}

var formSizes = []termSize{
	{"short_80x20", 80, 20},
	{"standard_80x24", 80, 24},
	{"narrow_60x24", 60, 24},
	{"wide_120x40", 120, 40},
}

func TestBranchForm_FrameSizeIsFixed(t *testing.T) {
	for _, sz := range formSizes {
		wantW, wantH := branchFormSize(sz.width, sz.height)
		for _, c := range formCases() {
			f := c.build()
			f.setWidth(sz.width, sz.height)
			out := renderBranchForm(f, sz.width, sz.height, "⠋")
			if w, h := lipgloss.Width(out), lipgloss.Height(out); w != wantW || h != wantH {
				t.Errorf("%s/%s: frame %dx%d, want %dx%d", sz.name, c.name, w, h, wantW, wantH)
			}
		}
	}
}

func TestBranchForm_FitsEightyByTwenty(t *testing.T) {
	_, h := branchFormSize(80, 20)
	if formTop+h+1 > 20 {
		t.Errorf("frame of %d rows at top %d leaves no room for the footer in 20 rows", h, formTop)
	}
}

func TestBranchForm_KeyLineVisibleWhenShort(t *testing.T) {
	out := ansi.Strip(renderBranchForm(newBranchForm(testFormOptions()), 80, 12, "⠋"))
	if !strings.Contains(out, "esc back") {
		t.Errorf("key line missing at 80x12:\n%s", out)
	}
}

func TestBranchForm_PreviewShowsFolderAndSession(t *testing.T) {
	s := newBranchForm(testFormOptions())
	s.Branch.SetValue("fix/login")
	s.syncAlias()
	out := ansi.Strip(renderBranchForm(s, 120, 40, "⠋"))
	if !strings.Contains(out, "grove-login") {
		t.Errorf("preview missing folder name:\n%s", out)
	}
}

func TestGolden_BranchForm(t *testing.T) {
	for _, sz := range formSizes {
		for _, c := range formCases() {
			t.Run(sz.name+"/"+c.name, func(t *testing.T) {
				goldenModel(t, sz)
				f := c.build()
				f.setWidth(sz.width, sz.height)
				golden.RequireEqual(t, []byte(renderBranchForm(f, sz.width, sz.height, "⠋")))
			})
		}
	}
}

func TestCopyForm_RendersChangesChoice(t *testing.T) {
	s := newCopyForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", true, []string{"a.go", "b.go"}, nil)
	s.setWidth(120, 40)
	out := ansi.Strip(renderBranchForm(s, 120, 40, "⠋"))
	for _, want := range []string{"Copy auth", "● copy", "○ move", "○ leave", "2 files", "From"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestSwitchForm_RendersStashNote(t *testing.T) {
	s := newSwitchForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", true, []string{"a.go"}, nil)
	s.setWidth(120, 40)
	out := ansi.Strip(renderBranchForm(s, 120, 40, "⠋"))
	for _, want := range []string{"Switch branch of auth", "stash 1 file before switching", "feature/auth"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestCopyForm_ChangesRowFocusVisibleWhenShort(t *testing.T) {
	s := newCopyForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", true, []string{"a.go"}, nil)
	s.setFocus(fieldChanges)
	s.setWidth(80, 12)
	out := ansi.Strip(renderBranchForm(s, 80, 12, "⠋"))
	if !strings.Contains(out, "● copy") {
		t.Errorf("focused Changes row scrolled out of view:\n%s", out)
	}
}
