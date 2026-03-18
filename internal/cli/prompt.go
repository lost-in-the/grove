package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// ErrPromptCanceled is returned when the user cancels a prompt with Escape or Ctrl+C.
var ErrPromptCanceled = errors.New("canceled")

// IsInteractive returns true if stdin is connected to a terminal.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// ReadLine reads a single line of input from stdin, displaying the given prompt.
// Uses raw terminal mode to properly handle Escape and Ctrl+C as cancellation.
// Returns ErrPromptCanceled if the user cancels.
func ReadLine(prompt string) (string, error) {
	_, _ = fmt.Fprint(os.Stderr, prompt)

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		// Non-terminal: fall back to simple line read
		return readLineCooked()
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return readLineCooked()
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	var buf []byte
	b := make([]byte, 1)

	for {
		_, err := os.Stdin.Read(b)
		if err != nil {
			_, _ = fmt.Fprint(os.Stderr, "\r\n")
			return "", err
		}

		switch b[0] {
		case 0x03: // Ctrl+C
			_, _ = fmt.Fprint(os.Stderr, "\r\n")
			return "", ErrPromptCanceled
		case 0x1b: // Escape — may be start of an escape sequence (arrow keys, etc.)
			drainEscapeSequence()
			_, _ = fmt.Fprint(os.Stderr, "\r\n")
			return "", ErrPromptCanceled
		case 0x04: // Ctrl+D (EOF)
			_, _ = fmt.Fprint(os.Stderr, "\r\n")
			return "", ErrPromptCanceled
		case 0x0d, 0x0a: // Enter (CR or LF)
			_, _ = fmt.Fprint(os.Stderr, "\r\n")
			return strings.TrimSpace(string(buf)), nil
		case 0x7f, 0x08: // Backspace / Delete
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				_, _ = fmt.Fprint(os.Stderr, "\b \b")
			}
		default:
			if b[0] >= 0x20 && b[0] < 0x7f { // Printable ASCII
				buf = append(buf, b[0])
				_, _ = os.Stderr.Write(b)
			}
		}
	}
}

// drainEscapeSequence reads and discards any bytes that follow an ESC byte
// as part of an escape sequence (e.g., arrow keys send ESC [ A).
// Uses a goroutine with a short timeout to avoid blocking on standalone Escape.
func drainEscapeSequence() {
	done := make(chan struct{})
	go func() {
		// Try to read escape sequence continuation bytes.
		// If it was a standalone Escape, this blocks until program exit (harmless).
		seq := make([]byte, 8)
		_, _ = os.Stdin.Read(seq)
		close(done)
	}()

	// Wait briefly — escape sequences arrive as a burst, so if nothing
	// comes within 20ms, it was a standalone Escape press.
	select {
	case <-done:
		// Drained the escape sequence
	case <-time.After(20 * time.Millisecond):
		// Standalone Escape — goroutine will be cleaned up at process exit
	}
}

// readLineCooked reads a line using a basic scanner (for non-terminal input).
func readLineCooked() (string, error) {
	var buf [4096]byte
	n, err := os.Stdin.Read(buf[:])
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(buf[:n]))
	// Check if the line ended with a newline
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line), nil
}

// Confirm asks the user a yes/no question.
// Returns an error if stdin is not an interactive terminal.
func Confirm(question string, defaultYes bool) (bool, error) {
	if !IsInteractive() {
		return false, fmt.Errorf("not an interactive terminal")
	}

	hint := "y/N"
	if defaultYes {
		hint = "Y/n"
	}

	input, err := ReadLine(fmt.Sprintf("%s [%s]: ", question, hint))
	if err != nil {
		if errors.Is(err, ErrPromptCanceled) {
			return false, err
		}
		return defaultYes, err
	}
	input = strings.ToLower(input)

	switch input {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	case "":
		return defaultYes, nil
	default:
		return false, fmt.Errorf("invalid response %q: expected y or n", input)
	}
}

// ConfirmWithDetails shows information before asking for confirmation.
func ConfirmWithDetails(w *Writer, header string, details []string, question string, defaultYes bool) (bool, error) {
	if !IsInteractive() {
		return false, fmt.Errorf("not an interactive terminal")
	}

	// Print styled header and details
	Bold(w, "%s", header)
	for _, detail := range details {
		_, _ = fmt.Fprintf(w, "  %s\n", detail)
	}
	_, _ = fmt.Fprintln(w)

	return Confirm(question, defaultYes)
}

// Choose presents a numbered selection menu and returns the chosen option.
func Choose(title string, options []string) (string, error) {
	if !IsInteractive() {
		return "", fmt.Errorf("not an interactive terminal")
	}

	if len(options) == 0 {
		return "", fmt.Errorf("no options provided")
	}

	_, _ = fmt.Fprintf(os.Stderr, "%s\n", title)
	for i, opt := range options {
		_, _ = fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, opt)
	}

	input, err := ReadLine(fmt.Sprintf("Choice [1-%d]: ", len(options)))
	if err != nil {
		return "", err
	}

	var choice int
	if _, err := fmt.Sscanf(input, "%d", &choice); err != nil || choice < 1 || choice > len(options) {
		return "", fmt.Errorf("invalid choice %q: expected a number between 1 and %d", input, len(options))
	}

	return options[choice-1], nil
}
