package cli

import (
	"fmt"
	"os"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/LeahArmstrong/grove-cli/internal/theme"
)

// spinnerModel is a minimal Bubbletea model for a CLI spinner.
type spinnerModel struct {
	spinner spinner.Model
	message string
	done    bool
	err     error
	fn      func() error
}

type spinDoneMsg struct{ err error }

func (m spinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		err := m.fn()
		return spinDoneMsg{err: err}
	})
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinDoneMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.err = fmt.Errorf("operation canceled")
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m spinnerModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	return tea.NewView(fmt.Sprintf("%s %s", m.spinner.View(), m.message))
}

// Spin shows a spinner on stderr while fn executes.
// Falls back to a simple message when not a TTY or NO_COLOR is set.
func Spin(message string, fn func() error) error {
	w := NewStderr()
	if !w.IsTTY() || theme.IsNoColor() {
		fmt.Fprintf(os.Stderr, "%s...", message)
		err := fn()
		if err != nil {
			fmt.Fprintln(os.Stderr, " failed")
		} else {
			fmt.Fprintln(os.Stderr, " done")
		}
		return err
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.Colors.Primary)

	m := spinnerModel{
		spinner: s,
		message: message,
		fn:      fn,
	}

	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	result, err := p.Run()
	if err != nil {
		return err
	}

	final, ok := result.(spinnerModel)
	if !ok {
		return fmt.Errorf("spinner: unexpected result type %T", result)
	}
	return final.err
}

// SpinWithResult shows a spinner while fn executes and returns both the result and error.
//
// Error precedence: if fn returns an error, that error is returned (most specific).
// If fn succeeds but the Bubbletea program itself fails (e.g., terminal error) or
// the user cancels with ctrl+c, the framework error is returned instead.
func SpinWithResult[T any](message string, fn func() (T, error)) (T, error) {
	var result T
	var fnErr error

	err := Spin(message, func() error {
		result, fnErr = fn()
		return fnErr
	})

	if err != nil && fnErr == nil {
		return result, err
	}
	return result, fnErr
}
