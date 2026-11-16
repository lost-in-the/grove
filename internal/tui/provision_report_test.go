package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/worktree"
)

func failed(field worktree.FileField, path string) worktree.FileResult {
	return worktree.FileResult{Field: field, Path: path, Status: worktree.FileFailed, Err: worktree.ErrSourceMissing}
}

func fileErr(results ...worktree.FileResult) error { return worktree.NewFileSetupError(results) }

var (
	errStateFailed = &worktree.StateRegistrationError{Err: errors.New("state.json is read-only")}
	errHookFailed  = fmt.Errorf("%w: %v", worktree.ErrRequiredHookFailed, errors.New("exit status 1"))
)

// remedy is the CLI's remedy line for n missing files in worktree "feature".
func remedy(n int) string {
	noun := "1 file is"
	if n != 1 {
		noun = fmt.Sprintf("%d files are", n)
	}
	return "The worktree was created, but " + noun + " missing. Fix the source, then run `grove doctor feature --fix`."
}

// history returns the message history texts, newest first.
func history(m Model) []string {
	var out []string
	for _, t := range m.toast.History() {
		out = append(out, t.Message)
	}
	return out
}

func TestHandleCreationDone_ReportsProvisioningFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		toast   string
		history []string // after the toast, in display order
	}{
		{"one file", fileErr(failed(worktree.FieldCopyFiles, "config/secret.key")),
			"Created feature. 1 file not copied. See ␣\u00a0messages.",
			[]string{"Not copied: config/secret.key (missing in the main worktree)", remedy(1)}},
		{"several files", fileErr(failed(worktree.FieldCopyFiles, ".env"), failed(worktree.FieldCopyFiles, "config/master.key"), failed(worktree.FieldCopyFiles, "config/secret.key")),
			"Created feature. 3 files not copied. See ␣\u00a0messages.",
			[]string{
				"Not copied: .env (missing in the main worktree)",
				"Not copied: config/master.key (missing in the main worktree)",
				"Not copied: config/secret.key (missing in the main worktree)",
				remedy(3),
			}},
		{"links only", fileErr(failed(worktree.FieldSymlinkFiles, ".tool-versions"), failed(worktree.FieldSymlinkDirs, "node_modules")),
			"Created feature. 2 files not linked. See ␣\u00a0messages.",
			[]string{
				"Not linked: .tool-versions (missing in the main worktree)",
				"Not linked: node_modules (missing in the main worktree)",
				remedy(2),
			}},
		{"copied and linked", fileErr(failed(worktree.FieldCopyFiles, ".env"), failed(worktree.FieldSymlinkDirs, "node_modules")),
			"Created feature. 2 files not set up. See ␣\u00a0messages.",
			[]string{
				"Not copied: .env (missing in the main worktree)",
				"Not linked: node_modules (missing in the main worktree)",
				remedy(2),
			}},
		{"state", errStateFailed,
			"Created feature, but grove could not register it. See ␣\u00a0messages.",
			[]string{
				"Not registered: state.json is read-only",
				"The worktree exists, but grove could not register it. Run `grove adopt /src/myapp-feature`.",
			}},
		{"required hook", errors.Join(fileErr(failed(worktree.FieldCopyFiles, ".env")), errHookFailed),
			"Created feature, but a required hook failed. Setup stopped.",
			[]string{
				"Required post-create hook failed: exit status 1",
				"Not copied: .env (missing in the main worktree)",
				remedy(1),
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(withItems(1), withSize(80, 24))

			m = sendMsg(m, creationDoneMsg{source: "create", name: "feature", path: "/src/myapp-feature", bootErr: tc.err})

			if got := m.toast.Message(); got != tc.toast {
				t.Errorf("toast = %q, want %q", got, tc.toast)
			}
			if m.toast.Current == nil || m.toast.Current.Level != ToastWarning {
				t.Errorf("toast level = %v, want a warning", m.toast.Current)
			}
			want := append([]string{tc.toast}, tc.history...)
			if got := history(m); strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("message history:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

func TestHandleForkComplete_ReportsFileFailuresAsCopy(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))

	m = sendMsg(m, forkCompleteMsg{name: "feature", path: "/src/myapp-feature", err: fileErr(failed(worktree.FieldCopyFiles, "config/secret.key"))})

	if got, want := m.toast.Message(), "Copied to feature. 1 file not copied. See ␣\u00a0messages."; got != want {
		t.Errorf("toast = %q, want %q", got, want)
	}
	if h := history(m); len(h) != 3 || h[1] != "Not copied: config/secret.key (missing in the main worktree)" || h[2] != remedy(1) {
		t.Errorf("message history = %q, want the toast, the file line, then the remedy", h)
	}
}

// stubCreatedSession replaces the session step and counts calls.
func stubCreatedSession(t *testing.T) *int {
	t.Helper()
	calls := 0
	orig := ensureCreatedSession
	ensureCreatedSession = func(chan<- creationEvent, *config.Config, string, string, string, string) { calls++ }
	t.Cleanup(func() { ensureCreatedSession = orig })
	return &calls
}

func TestAfterCreateBootstrap_OnlyARequiredHookStopsSetup(t *testing.T) {
	autoUp := true
	cfg := &config.Config{}
	cfg.Plugins.Docker.AutoUp = &autoUp
	for _, tc := range []struct {
		name string
		err  error
		runs bool
	}{
		{"success", nil, true},
		{"files not copied", fileErr(failed(worktree.FieldCopyFiles, ".env")), true},
		{"not registered", errStateFailed, true},
		{"required hook", errors.Join(fileErr(failed(worktree.FieldCopyFiles, ".env")), errHookFailed), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sessions := stubCreatedSession(t)
			ups := 0
			restore := stubDockerAutoUp(t, func(*config.Config, string) (bool, error) { ups++; return true, nil })
			defer restore()
			ch := make(chan creationEvent, 20)

			afterCreateBootstrap(ch, cfg, "myapp", "/src/myapp", "feature", "/src/myapp-feature", tc.err)

			want := 0
			if tc.runs {
				want = 1
			}
			if ups != want || *sessions != want {
				t.Errorf("auto-up ran %d, session ran %d; want %d each", ups, *sessions, want)
			}
		})
	}
}

func TestAfterForkBootstrap_FileFailureStillCreatesTheSession(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		runs bool
	}{
		{"files not copied", fileErr(failed(worktree.FieldCopyFiles, ".env")), true},
		{"not registered", errStateFailed, true},
		{"required hook", errHookFailed, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			orig := forkSession
			forkSession = func(*worktree.Manager, string, string) { calls++ }
			t.Cleanup(func() { forkSession = orig })

			msg := afterForkBootstrap(nil, "feature", "/src/myapp-feature", tc.err)

			if want := map[bool]int{true: 1, false: 0}[tc.runs]; calls != want {
				t.Errorf("session ran %d times, want %d", calls, want)
			}
			if msg.name != "feature" || !errors.Is(msg.err, tc.err) {
				t.Errorf("msg = %+v, want the worktree kept and the bootstrap error carried", msg)
			}
		})
	}
}

// A long message wraps under its own text instead of being cut, so a
// per-file line is readable in full at 80 columns.
func TestRenderMessages_WrapsLongEntries(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	m.toast.Record(NewToast("Not copied: config/secret.key (missing in the main worktree)", ToastWarning))

	view := ansiStripRE.ReplaceAllString(m.renderMessages(), "")

	if strings.Contains(view, "…") {
		t.Errorf("message was cut:\n%s", view)
	}
	var text []string
	for _, line := range strings.Split(view, "\n") {
		line = strings.Trim(line, "│ ")
		if strings.Contains(line, "⚠") {
			line = line[strings.Index(line, "⚠")+len("⚠ "):]
		}
		if line != "" {
			text = append(text, line)
		}
	}
	if joined := strings.Join(text, " "); !strings.Contains(joined, "Not copied: config/secret.key (missing in the main worktree)") {
		t.Errorf("the whole message is not in the view:\n%s", view)
	}
}

// The toast fits an 80-column terminal whatever the worktree is called: when
// the name cannot fit whole, it is shortened in the middle, never the
// instruction. The history keeps the full name.
func TestProvisionToast_FitsAt80Columns(t *testing.T) {
	const long = "feature-checkout-redesign-v2-x" // 30 characters
	for _, tc := range []struct {
		name, worktree string
		err            error
		instruction    string
		wholeName      bool
	}{
		{"files, short name", "feature", fileErr(failed(worktree.FieldCopyFiles, "config/secret.key")),
			". 1 file not copied. See ␣\u00a0messages.", true},
		{"files, 30-character name", long, fileErr(failed(worktree.FieldCopyFiles, "config/secret.key")),
			". 1 file not copied. See ␣\u00a0messages.", true},
		{"state, 30-character name", long, errStateFailed,
			", but grove could not register it. See ␣\u00a0messages.", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(withItems(1), withSize(80, 24))

			m = sendMsg(m, creationDoneMsg{source: "create", name: tc.worktree, path: "/src/myapp-" + tc.worktree, bootErr: tc.err})

			got := m.toast.Message()
			if w := lipgloss.Width(got); w > 80-6 {
				t.Errorf("toast is %d cells, more than the %d an 80-column toast shows: %q", w, 80-6, got)
			}
			if view := ansiStripRE.ReplaceAllString(m.toast.View(80), ""); !strings.Contains(view, tc.instruction) || strings.Contains(view, tc.instruction+"…") {
				t.Errorf("the instruction was cut:\n%s", view)
			}
			if whole := strings.HasPrefix(got, "Created "+tc.worktree+tc.instruction); whole != tc.wholeName {
				t.Errorf("toast = %q; whole name = %v, want %v", got, whole, tc.wholeName)
			}
			if !tc.wholeName && (!strings.HasPrefix(got, "Created feature-") || !strings.Contains(got, "…")) {
				t.Errorf("toast = %q, want the name shortened in the middle", got)
			}
			if h := history(m); !strings.Contains(strings.Join(h[1:], "\n"), tc.worktree) {
				t.Errorf("history lost the full name: %q", h)
			}
		})
	}
}
