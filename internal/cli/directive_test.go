package cli

import (
	"os"
	"regexp"
	"testing"
)

func TestDirective_NoANSI(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	t.Cleanup(func() { os.Stdout = oldStdout })
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	os.Stdout = w

	Directive("cd", "/some/path/to/worktree")

	_ = w.Close()

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	_ = r.Close()
	output := string(buf[:n])

	// Verify format
	expected := "cd:/some/path/to/worktree\n"
	if output != expected {
		t.Errorf("got %q, want %q", output, expected)
	}

	// Verify NO ANSI escape sequences
	ansiRegex := regexp.MustCompile(`\x1b\[`)
	if ansiRegex.MatchString(output) {
		t.Errorf("directive contains ANSI escape sequences: %q", output)
	}
}

func TestDirective_TmuxAttach_NoANSI(t *testing.T) {
	oldStdout := os.Stdout
	t.Cleanup(func() { os.Stdout = oldStdout })
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	os.Stdout = w

	Directive("tmux-attach", "grove-cli-feature")

	_ = w.Close()

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	_ = r.Close()
	output := string(buf[:n])

	expected := "tmux-attach:grove-cli-feature\n"
	if output != expected {
		t.Errorf("got %q, want %q", output, expected)
	}

	ansiRegex := regexp.MustCompile(`\x1b\[`)
	if ansiRegex.MatchString(output) {
		t.Errorf("directive contains ANSI escape sequences: %q", output)
	}
}
