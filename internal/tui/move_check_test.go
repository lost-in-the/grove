package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/mux"
)

// usableMux is a session backend that is installed and running.
type usableMux struct{ *mux.OffBackend }

func (usableMux) Available() bool { return true }

// noShellIntegration clears every variable the grove shell function sets.
func noShellIntegration(t *testing.T) {
	t.Helper()
	t.Setenv("GROVE_SHELL", "")
	t.Setenv("GROVE_CD_FILE", "")
}

func existingCdFile(t *testing.T) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "cd")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestCanMoveUser(t *testing.T) {
	cdFile := existingCdFile(t)
	stale := filepath.Join(t.TempDir(), "gone")
	here := config.LoadDefaults()
	here.Session.OpenIn = config.OpenInHere
	tmuxOff := config.LoadDefaults()
	tmuxOff.Tmux.Mode = "off"

	tests := []struct {
		name   string
		cfg    *config.Config
		mx     mux.Multiplexer
		shell  string
		cdFile string
		want   bool
	}{
		{"session backend, no shell function", config.LoadDefaults(), usableMux{mux.NewOff()}, "", "", true},
		{"session backend and shell function", config.LoadDefaults(), usableMux{mux.NewOff()}, "1", cdFile, true},
		{"no backend, shell function with cd file", config.LoadDefaults(), mux.NewOff(), "1", cdFile, true},
		{"no backend, cd file alone", config.LoadDefaults(), mux.NewOff(), "", cdFile, true},
		{"no backend, shell function alone", config.LoadDefaults(), mux.NewOff(), "1", "", true},
		{"no backend, no shell function", config.LoadDefaults(), mux.NewOff(), "", "", false},
		{"no backend, stale cd file only", config.LoadDefaults(), mux.NewOff(), "", stale, false},
		{"backend, but open_in here, no shell function", here, usableMux{mux.NewOff()}, "", "", false},
		{"backend, but legacy tmux mode off, no shell function", tmuxOff, usableMux{mux.NewOff()}, "", "", false},
		{"backend, but open_in here, shell function", here, usableMux{mux.NewOff()}, "1", "", true},
		{"nil config, backend", nil, usableMux{mux.NewOff()}, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GROVE_SHELL", tt.shell)
			t.Setenv("GROVE_CD_FILE", tt.cdFile)
			if got := canMoveUser(tt.cfg, tt.mx); got != tt.want {
				t.Errorf("canMoveUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

// offModel is a dashboard whose project turned sessions off, so the exit
// can move the user only through the shell function.
func offModel() Model {
	m := newTestModel(withItems(3))
	m.cfg = config.LoadDefaults()
	m.cfg.Mux.Backend = "off"
	m.switchToDisplayName = "feature"
	return m
}

func TestMoveAfterExit_ExplainsWhenItCannotMove(t *testing.T) {
	noShellIntegration(t)
	m := offModel()
	var stderr bytes.Buffer

	out, err := captureStdio(func() error {
		m.moveAfterExit("/src/it's-app/app-feature", cli.NewWriter(&stderr, false))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	got := stderr.String()
	for _, want := range []string{
		"⚠ grove could not move you: no session could be switched, and shell integration is not loaded.",
		"Running the binary by its path skips that function",
		`eval "$(grove install zsh)"`,
		`cd '/src/it'\''s-app/app-feature'`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr is missing %q:\n%s", want, got)
		}
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("stdout = %q, want nothing: no wrapper reads it", out)
	}
}

func TestMoveAfterExit_SaysNothingWhenTheWrapperMovesYou(t *testing.T) {
	t.Run("cd file", func(t *testing.T) {
		cdFile := existingCdFile(t)
		t.Setenv("GROVE_SHELL", "1")
		t.Setenv("GROVE_CD_FILE", cdFile)
		m := offModel()
		var stderr bytes.Buffer

		out, _ := captureStdio(func() error {
			m.moveAfterExit("/src/app-feature", cli.NewWriter(&stderr, false))
			return nil
		})

		if stderr.Len() != 0 || out != "" {
			t.Errorf("stderr = %q, stdout = %q, want both empty", stderr.String(), out)
		}
		if got, _ := os.ReadFile(cdFile); string(got) != "/src/app-feature" {
			t.Errorf("cd file = %q, want the worktree path", got)
		}
	})

	t.Run("captured cd line", func(t *testing.T) {
		t.Setenv("GROVE_SHELL", "1")
		t.Setenv("GROVE_CD_FILE", "")
		m := offModel()
		var stderr bytes.Buffer

		out, _ := captureStdio(func() error {
			m.moveAfterExit("/src/app-feature", cli.NewWriter(&stderr, false))
			return nil
		})

		if stderr.Len() != 0 {
			t.Errorf("stderr = %q, want empty", stderr.String())
		}
		if !strings.Contains(out, "cd:/src/app-feature\n") {
			t.Errorf("stdout = %q, want the cd: directive", out)
		}
	})
}

func TestDashboard_CannotMoveNoticeShowsOnlyWhenItCannotMove(t *testing.T) {
	for _, size := range []termSize{sizeStandard, sizeWide} {
		can := newTestModel(withItems(5), withSize(size.width, size.height))
		cannot := newTestModel(withItems(5), withSize(size.width, size.height), withCannotMove())

		if strings.Contains(ansi.Strip(can.viewString()), "cannot move you") {
			t.Errorf("%s: notice shown though grove can move you", size.name)
		}
		lines := screenLines(t, cannot)
		if i := lineIndex(lines, moveNoticeText); i != 1 {
			t.Errorf("%s: notice on line %d, want line 1 (the filter row):\n%s", size.name, i, strings.Join(lines, "\n"))
		}
	}
}

func TestDashboard_CannotMoveNoticeDoesNotShiftRows(t *testing.T) {
	for _, size := range []termSize{sizeNarrow, sizeStandard, sizeWide} {
		can := screenLines(t, newTestModel(withItems(5), withSize(size.width, size.height)))
		cannot := screenLines(t, newTestModel(withItems(5), withSize(size.width, size.height), withCannotMove()))
		for _, name := range []string{"root", "feature-auth", "refactor"} {
			if a, b := lineIndex(can, name), lineIndex(cannot, name); a != b || a < 0 {
				t.Errorf("%s: row %s on line %d without the notice, %d with it", size.name, name, a, b)
			}
		}
		if len(can) != len(cannot) {
			t.Errorf("%s: %d lines without the notice, %d with it", size.name, len(can), len(cannot))
		}
	}
}

func TestDashboard_CannotMoveNoticeTruncatesOnOneLine(t *testing.T) {
	m := newTestModel(withItems(5), withSize(50, 24), withCannotMove())
	lines := screenLines(t, m)
	row := lines[1]
	if !strings.Contains(row, "⚠ grove cannot move you") || !strings.HasSuffix(strings.TrimRight(row, " "), "…") {
		t.Errorf("filter row = %q, want the notice cut with an ellipsis", row)
	}
	if lineIndex(lines, "root") != 2 {
		t.Errorf("rows moved at 50 columns:\n%s", strings.Join(lines, "\n"))
	}
}

func TestDashboard_CannotMoveNoticeStaysBesideAFilter(t *testing.T) {
	m := newTestModel(withItems(5), withSize(120, 40), withCannotMove())
	m.list.SetFilterText("feat")
	lines := screenLines(t, m)
	if !strings.Contains(lines[1], "feat") || !strings.Contains(lines[1], "⚠ grove cannot move you") {
		t.Errorf("filter row = %q, want the filter and the notice", lines[1])
	}
}

func TestDashboard_CannotMoveNoticeUsesTheWarningToken(t *testing.T) {
	notice := moveNotice()
	if want := Styles.WarningText.Render(moveNoticeText); notice != want {
		t.Errorf("notice = %q, want the warning token around %q", notice, moveNoticeText)
	}
}

func TestHelp_CannotMoveExplainsTheFix(t *testing.T) {
	for _, cannot := range []bool{false, true} {
		m := newTestModel(withItems(3), withSize(120, 40))
		if cannot {
			m = newTestModel(withItems(3), withSize(120, 40), withCannotMove())
		}
		m.helpOverlay.Open(ViewDashboard, m.width, m.height)
		text := ansi.Strip(m.helpOverlay.viewport.View())
		has := strings.Contains(text, "grove cannot move you from here")
		if has != cannot {
			t.Errorf("cannotMove=%v: help explains the fix = %v", cannot, has)
		}
		if cannot {
			for _, want := range []string{"shell function", `eval "$(grove install zsh)"`, "by its path"} {
				if !strings.Contains(text, want) {
					t.Errorf("help is missing %q:\n%s", want, text)
				}
			}
		}
	}
}

// withCannotMove starts the dashboard where grove cannot move the user.
func withCannotMove() testOpt {
	return func(m *Model) { m.setCannotMove(true) }
}

func TestBrowsers_CannotMoveNotice(t *testing.T) {
	for _, size := range []termSize{sizeStandard, sizeWide} {
		for _, browser := range browserFilters() {
			t.Run(size.name+"/"+browser.name, func(t *testing.T) {
				m := newTestModel(withItems(3), withSize(size.width, size.height), browser.open)
				before := screenLines(t, m)
				m.setCannotMove(true)
				after := screenLines(t, m)
				if !strings.Contains(strings.Join(after, "\n"), "⚠ grove cannot move you") {
					t.Fatalf("missing move notice:\n%s", strings.Join(after, "\n"))
				}
				if len(before) != len(after) || lineIndex(before, "❯") != lineIndex(after, "❯") {
					t.Fatal("move notice shifted the list or changed screen height")
				}
				m = typeText(sendKey(m, "/"), "a")
				view := stripAnsi(m.viewString())
				if !strings.Contains(view, "Filter: a") || !strings.Contains(view, "⚠ grove cannot move you") {
					t.Fatalf("filter hid move notice:\n%s", view)
				}
			})
		}
	}
}
