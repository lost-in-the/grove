package tui

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lost-in-the/grove/internal/worktree"
)

// provisionReport is what the dashboard shows when bootstrapping a worktree it
// created or copied returned an error: one warning toast, and detail lines for
// the message history, in the CLI's wording.
type provisionReport struct {
	toast   string
	history []string // display order
}

// newProvisionReport builds the report for err. done opens the toast
// ("Created feature"); name and path are the new worktree's. ok is false for
// an error that is not a file, state, or required hook failure.
func newProvisionReport(done, name, path string, err error) (r provisionReport, ok bool) {
	var fe *worktree.FileSetupError
	hasFiles := errors.As(err, &fe)
	var se *worktree.StateRegistrationError
	hasState := errors.As(err, &se)
	hookErr := requiredHookError(err)

	switch {
	case hookErr != nil:
		r.toast = done + ", but a required hook failed. Setup stopped."
		r.history = append(r.history, capitalize(hookErr.Error()))
	case hasState:
		r.toast = done + ", but grove could not register it. Run grove adopt"
	case hasFiles:
		r.toast = fmt.Sprintf("%s. %s. Run grove doctor %s --fix", done, filesNotSetUp(fe.Failed), name)
	default:
		return r, false
	}
	if hasState {
		r.history = append(r.history,
			"Not registered: "+se.Err.Error(),
			fmt.Sprintf("The worktree exists, but grove could not register it. Run `grove adopt %s`.", path))
	}
	if hasFiles {
		// Lines ends with the CLI's remedy line; the toast already says it.
		r.history = append(r.history, fe.Lines(name, "")[:len(fe.Failed)]...)
	}
	return r, true
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
// shows the toast, so the history reads toast first, then the details.
func (m *Model) showProvisionReport(r provisionReport) {
	for i := len(r.history) - 1; i >= 0; i-- {
		m.toast.Record(NewToast(r.history[i], ToastWarning))
	}
	m.toast.Show(NewToast(r.toast, ToastWarning))
}

// streamProvisionReport sends the report's detail lines to the creation
// activity log.
func streamProvisionReport(ch chan<- creationEvent, name, path string, err error) {
	r, ok := newProvisionReport("Created "+name, name, path, err)
	if !ok {
		ch <- creationEvent{line: fmt.Sprintf("Bootstrap failed: %v", err)}
		return
	}
	for _, line := range r.history {
		ch <- creationEvent{line: strings.TrimSpace(line)}
	}
}
