package cli

import (
	"io"
	"os"

	"golang.org/x/term"

	"github.com/lost-in-the/grove/internal/theme"
)

// Writer wraps an io.Writer with TTY detection.
type Writer struct {
	w     io.Writer
	isTTY bool
}

// NewStdout creates a Writer for stdout with TTY detection.
func NewStdout() *Writer {
	return &Writer{
		w:     os.Stdout,
		isTTY: term.IsTerminal(int(os.Stdout.Fd())),
	}
}

// NewStderr creates a Writer for stderr with TTY detection.
func NewStderr() *Writer {
	return &Writer{
		w:     os.Stderr,
		isTTY: term.IsTerminal(int(os.Stderr.Fd())),
	}
}

// NewWriter creates a Writer wrapping the given io.Writer.
// isTTY must be provided since arbitrary writers don't have fd's.
func NewWriter(w io.Writer, isTTY bool) *Writer {
	return &Writer{w: w, isTTY: isTTY}
}

// IsTTY returns whether the underlying writer is a terminal.
func (w *Writer) IsTTY() bool {
	return w.isTTY
}

// UseColor returns whether color output should be used.
// Returns true only when writing to a TTY with NO_COLOR not set.
func (w *Writer) UseColor() bool {
	return w.isTTY && !theme.IsNoColor()
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	return w.w.Write(p)
}
