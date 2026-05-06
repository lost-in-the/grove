package cli

import (
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

// replaceStdin swaps os.Stdin for the read end of a pipe and returns a
// restore func. The test writes to wPipe and must close it when done.
func replaceStdin(t *testing.T) (wPipe *os.File, restore func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	old := os.Stdin
	os.Stdin = r
	return w, func() {
		os.Stdin = old
		_ = r.Close()
	}
}

// TestReadLineCooked_ReadsLine verifies readLineCooked returns the text written
// to the pipe, trimmed of the trailing newline.
func TestReadLineCooked_ReadsLine(t *testing.T) {
	w, restore := replaceStdin(t)
	defer restore()

	go func() {
		_, _ = io.WriteString(w, "hello world\n")
		_ = w.Close()
	}()

	got, err := readLineCooked()
	if err != nil {
		t.Fatalf("readLineCooked() unexpected error: %v", err)
	}
	if got != "hello world" {
		t.Errorf("readLineCooked() = %q, want %q", got, "hello world")
	}
}

// TestReadLineCooked_EOF verifies readLineCooked returns an error on EOF.
func TestReadLineCooked_EOF(t *testing.T) {
	w, restore := replaceStdin(t)
	defer restore()

	_ = w.Close() // immediate EOF

	_, err := readLineCooked()
	if err == nil {
		t.Fatal("readLineCooked() expected error on EOF, got nil")
	}
}

// TestDrainEscapeSequence_Timeout verifies drainEscapeSequence returns
// promptly (within 200ms) when no follow-up bytes arrive — i.e. it was a
// standalone Escape press, not a multi-byte escape sequence.
//
// We do NOT replace os.Stdin here: drainEscapeSequence spawns a goroutine
// that reads os.Stdin and may outlive the function (the goroutine leaks
// harmlessly until process exit, as documented in the production comment).
// Swapping os.Stdin while that goroutine is running would cause a data race.
// Instead we rely on the 20ms select timeout inside drainEscapeSequence to
// fire when stdin produces no bytes during the test.
func TestDrainEscapeSequence_Timeout(t *testing.T) {
	if IsInteractive() {
		t.Skip("stdin is a TTY — drainEscapeSequence would block on real terminal input")
	}

	done := make(chan struct{})
	go func() {
		drainEscapeSequence()
		close(done)
	}()

	select {
	case <-done:
		// returned within timeout — correct
	case <-time.After(200 * time.Millisecond):
		t.Fatal("drainEscapeSequence() did not return within 200ms for standalone ESC")
	}
}

// TestDrainEscapeSequence_ArrowKey verifies drainEscapeSequence consumes the
// continuation bytes of an arrow-key sequence (ESC [ A) and returns quickly.
func TestDrainEscapeSequence_ArrowKey(t *testing.T) {
	w, restore := replaceStdin(t)

	go func() {
		// Write the continuation bytes of ESC [ A (up-arrow sequence).
		_, _ = w.Write([]byte("[A"))
		_ = w.Close()
	}()

	done := make(chan struct{})
	go func() {
		drainEscapeSequence()
		close(done)
	}()

	select {
	case <-done:
		restore()
		// drained successfully
	case <-time.After(200 * time.Millisecond):
		restore()
		t.Fatal("drainEscapeSequence() did not return within 200ms after arrow-key bytes")
	}
}

// TestReadLine_NonTTY_CtrlC verifies that when stdin is a pipe (non-TTY),
// ReadLine falls through to readLineCooked and returns the input.
// The raw-mode Ctrl+C path requires a real TTY so it cannot be tested here.
// Coverage of ErrPromptCanceled in raw mode is guarded by the TTY check.
func TestReadLine_NonTTY_ReturnsInput(t *testing.T) {
	if IsInteractive() {
		t.Skip("stdin is a TTY — skipping non-TTY ReadLine test")
	}

	w, restore := replaceStdin(t)
	defer restore()

	go func() {
		_, _ = io.WriteString(w, "my answer\n")
		_ = w.Close()
	}()

	got, err := ReadLine("prompt: ")
	if err != nil {
		t.Fatalf("ReadLine() unexpected error: %v", err)
	}
	if got != "my answer" {
		t.Errorf("ReadLine() = %q, want %q", got, "my answer")
	}
}

// TestReadLine_NonTTY_EOF verifies ReadLine surfaces the EOF error from the
// non-TTY code path (readLineCooked) rather than hanging.
func TestReadLine_NonTTY_EOF(t *testing.T) {
	if IsInteractive() {
		t.Skip("stdin is a TTY — skipping non-TTY ReadLine test")
	}

	w, restore := replaceStdin(t)
	defer restore()

	_ = w.Close() // immediate EOF

	_, err := ReadLine("prompt: ")
	if err == nil {
		t.Fatal("ReadLine() expected error on EOF, got nil")
	}
	// Should NOT be ErrPromptCanceled — that's the raw-mode Ctrl+C path.
	if errors.Is(err, ErrPromptCanceled) {
		t.Errorf("ReadLine() returned ErrPromptCanceled for EOF, want an io error")
	}
}

func TestIsInteractive_NonTTY(t *testing.T) {
	// In test environments stdin is not a terminal.
	got := IsInteractive()
	if got {
		t.Log("IsInteractive() returned true — running in an interactive TTY")
	}
	// We can't assert false here because CI and local runs may differ,
	// but we can verify the function returns a consistent bool.
	_ = got
}

func TestConfirm_NonInteractive(t *testing.T) {
	// Confirm must return an error when stdin is not a terminal.
	if IsInteractive() {
		t.Skip("stdin is a TTY — skipping non-interactive test")
	}

	_, err := Confirm("continue?", false)
	if err == nil {
		t.Error("Confirm() expected error in non-interactive mode, got nil")
	}
}

func TestConfirm_NonInteractive_DefaultYes(t *testing.T) {
	if IsInteractive() {
		t.Skip("stdin is a TTY — skipping non-interactive test")
	}

	_, err := Confirm("continue?", true)
	if err == nil {
		t.Error("Confirm() expected error in non-interactive mode, got nil")
	}
}

func TestChoose_EmptyOptions(t *testing.T) {
	// Choose with no options always returns an error, regardless of TTY state.
	_, err := Choose("pick one", []string{})
	if err == nil {
		t.Error("Choose() with empty options expected error, got nil")
	}
}

func TestChoose_NonInteractive(t *testing.T) {
	if IsInteractive() {
		t.Skip("stdin is a TTY — skipping non-interactive test")
	}

	_, err := Choose("pick one", []string{"a", "b", "c"})
	if err == nil {
		t.Error("Choose() expected error in non-interactive mode, got nil")
	}
}

// TestStdPrompter_DelegatesToFreeFunctions verifies the default Prompter
// implementation forwards to the package-level helpers — IsInteractive,
// Confirm, ChooseIndex — so behavior in production code matches what
// tests see when they substitute a fake.
func TestStdPrompter_DelegatesToFreeFunctions(t *testing.T) {
	p := StdPrompter{}

	if got, want := p.IsInteractive(), IsInteractive(); got != want {
		t.Errorf("StdPrompter.IsInteractive() = %v, want %v (must match free function)", got, want)
	}

	if IsInteractive() {
		t.Skip("stdin is a TTY — skipping non-interactive delegation checks")
	}
	if _, err := p.Confirm("continue?", false); err == nil {
		t.Error("StdPrompter.Confirm() expected error in non-interactive mode")
	}
	if _, err := p.ChooseIndex("pick", []string{"a", "b"}); err == nil {
		t.Error("StdPrompter.ChooseIndex() expected error in non-interactive mode")
	}
}
