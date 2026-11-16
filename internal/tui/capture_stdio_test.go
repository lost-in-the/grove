package tui

import (
	"fmt"
	"os"
	"slices"
	"testing"
	"time"
)

func TestCaptureStdio_CapturesStdoutAndStderr(t *testing.T) {
	out, err := captureStdio(func() error {
		fmt.Fprint(os.Stdout, "to-stdout ")
		fmt.Fprint(os.Stderr, "to-stderr")
		return nil
	})
	if err != nil {
		t.Fatalf("captureStdio: %v", err)
	}
	if out != "to-stdout to-stderr" {
		t.Errorf("captured = %q, want %q", out, "to-stdout to-stderr")
	}
}

// TestCaptureStdio_PanicRestoresStdio pins the panic-safety of captureStdio:
// if fn panics, the os.Stdout/os.Stderr swap must be undone and stdioMu
// released — otherwise every later capture deadlocks and bubbletea's panic
// recovery prints its diagnostics into the abandoned pipe.
func TestCaptureStdio_PanicRestoresStdio(t *testing.T) {
	origOut, origErr := os.Stdout, os.Stderr

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected the panic to propagate out of captureStdio")
			}
		}()
		_, _ = captureStdio(func() error { panic("boom") })
	}()

	if os.Stdout != origOut {
		t.Error("os.Stdout not restored after panic")
	}
	if os.Stderr != origErr {
		t.Error("os.Stderr not restored after panic")
	}

	// stdioMu must be unlocked again — a follow-up capture must complete.
	type result struct {
		out string
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		out, err := captureStdio(func() error {
			fmt.Fprint(os.Stdout, "after-panic")
			return nil
		})
		resCh <- result{out, err}
	}()
	select {
	case res := <-resCh:
		if res.err != nil {
			t.Fatalf("follow-up capture: %v", res.err)
		}
		if res.out != "after-panic" {
			t.Errorf("follow-up capture = %q, want %q", res.out, "after-panic")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("captureStdio deadlocked after a panicking capture (stdioMu still held?)")
	}
}

// TestCaptureStdio_BoundedDrainWithLingeringWriter simulates a backgrounded
// grandchild (`sh -c 'x &'` spawned by a hook/docker) that inherited a copy of
// the pipe's write end and keeps it open after fn returns. The drain can never
// reach EOF, so captureStdio must return the partial capture after
// captureDrainTimeout instead of hanging the tea.Cmd forever.
func TestJoinCaptured(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want string
	}{
		{"unterminated first chunk gets newline guard", "last line", "first plugin line\n", "last line\nfirst plugin line\n"},
		{"terminated first chunk joins as-is", "line\n", "next\n", "line\nnext\n"},
		{"empty first chunk", "", "only\n", "only\n"},
		{"empty second chunk", "only", "", "only"},
		{"both empty", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := joinCaptured(tc.a, tc.b); got != tc.want {
				t.Errorf("joinCaptured(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// lineWriter sends whole lines, joining a line split across writes, dropping
// a trailing \r and empty lines, flushing the partial last line on Close,
// and ignoring writes after Close.
func TestLineWriter(t *testing.T) {
	var got []string
	lw := &lineWriter{emit: func(line string) { got = append(got, line) }}
	for _, chunk := range []string{"one\ntw", "o\r\n\n", "three"} {
		if _, err := lw.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	if want := []string{"one", "two"}; !slices.Equal(got, want) {
		t.Errorf("before Close: %q, want %q", got, want)
	}
	_ = lw.Close()
	if _, err := lw.Write([]byte("late\n")); err != nil {
		t.Fatal(err)
	}
	_ = lw.Close()
	if want := []string{"one", "two", "three"}; !slices.Equal(got, want) {
		t.Errorf("after Close: %q, want %q", got, want)
	}
}
