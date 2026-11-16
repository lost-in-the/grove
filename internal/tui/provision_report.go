package tui

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/internal/worktree"
)

// provisionReport is what the dashboard shows when bootstrapping a worktree it
// created or copied returned an error: one short warning toast that names the
// worktree once and points to the message history, and the detail lines for
// that history in the CLI's wording, remedy included.
type provisionReport struct {
	verb    string // "Created" or "Copied to"
	name    string
	rest    string   // the toast after the name
	history []string // display order
}

// seeMessages points to the message history the way the footer writes a key
// and its action ("␣ find"): Find (␣), then its "messages" action. The
// history has no key of its own. The no-break space keeps the key and its
// action on one line when the message history wraps.
const seeMessages = "See ␣\u00a0messages."

// newProvisionReport builds the report for err. verb opens the toast
// ("Created"); name and path are the new worktree's. ok is false for an error
// that is not a file, state, or required hook failure.
func newProvisionReport(verb, name, path string, err error) (r provisionReport, ok bool) {
	r.verb, r.name = verb, name
	var fe *worktree.FileSetupError
	hasFiles := errors.As(err, &fe)
	var se *worktree.StateRegistrationError
	hasState := errors.As(err, &se)
	hookErr := requiredHookError(err)

	switch {
	case hookErr != nil:
		r.rest = ", but a required hook failed. Setup stopped."
		r.history = append(r.history, capitalize(hookErr.Error()))
	case hasState:
		r.rest = ", but grove could not register it. " + seeMessages
	case hasFiles:
		r.rest = ". " + filesNotSetUp(fe.Failed) + ". " + seeMessages
	default:
		return r, false
	}
	if hasState {
		r.history = append(r.history,
			"Not registered: "+se.Err.Error(),
			fmt.Sprintf("The worktree exists, but grove could not register it. Run `grove adopt %s`.", path))
	}
	if hasFiles {
		r.history = append(r.history, fe.Lines(name, "The worktree was created")...)
	}
	return r, true
}

// toastText is the toast in at most maxCells cells. Only the name gives way,
// shortened in the middle, so the instruction is never cut.
func (r provisionReport) toastText(maxCells int) string {
	name := r.name
	budget := maxCells - lipgloss.Width(r.verb+" "+r.rest)
	if maxCells > 0 && lipgloss.Width(name) > budget {
		name = middleTruncate(name, max(1, budget))
	}
	return r.verb + " " + name + r.rest
}

// filesNotSetUp counts the failed entries: "1 file not copied", "2 files not
// linked", or "3 files not set up" when copies and links both failed.
func filesNotSetUp(failed []worktree.FileResult) string {
	copies, links := 0, 0
	for _, r := range failed {
		if r.Field == worktree.FieldCopyFiles {
			copies++
		} else {
			links++
		}
	}
	verb := "set up"
	switch {
	case links == 0:
		verb = "copied"
	case copies == 0:
		verb = "linked"
	}
	noun := "files"
	if len(failed) == 1 {
		noun = "file"
	}
	return fmt.Sprintf("%d %s not %s", len(failed), noun, verb)
}

// requiredHookError returns the part of err that is the required hook
// failure, without the file or state errors joined to it. Nil when there is
// none.
func requiredHookError(err error) error {
	if !errors.Is(err, worktree.ErrRequiredHookFailed) {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range joined.Unwrap() {
			if found := requiredHookError(e); found != nil {
				return found
			}
		}
	}
	return err
}

func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// showProvisionReport records the detail lines in the message history, then
// shows the toast, so the history reads toast first, then the details. The
// toast box takes 6 cells of the terminal width (ToastModel.View).
func (m *Model) showProvisionReport(r provisionReport) {
	for i := len(r.history) - 1; i >= 0; i-- {
		m.toast.Record(NewToast(r.history[i], ToastWarning))
	}
	maxCells := 0
	if m.width > 0 {
		maxCells = m.width - 6
	}
	m.toast.Show(NewToast(r.toastText(maxCells), ToastWarning))
}

// streamProvisionReport sends the report's detail lines to the creation
// activity log.
func streamProvisionReport(ch chan<- creationEvent, name, path string, err error) {
	r, ok := newProvisionReport("Created", name, path, err)
	if !ok {
		ch <- creationEvent{line: fmt.Sprintf("Bootstrap failed: %v", err)}
		return
	}
	for _, line := range r.history {
		ch <- creationEvent{line: strings.TrimSpace(line)}
	}
}
