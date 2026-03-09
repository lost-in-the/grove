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

func TestTmuxAttachDirective(t *testing.T) {
	tests := []struct {
		name        string
		session     string
		controlMode bool
		expected    string
	}{
		{name: "normal mode", session: "test-session", controlMode: false, expected: "tmux-attach:test-session\n"},
		{name: "control mode", session: "test-session", controlMode: true, expected: "tmux-attach-cc:test-session\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdout := os.Stdout
			t.Cleanup(func() { os.Stdout = oldStdout })
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("pipe failed: %v", err)
			}
			os.Stdout = w

			TmuxAttachDirective(tt.session, tt.controlMode)

			_ = w.Close()

			buf := make([]byte, 1024)
			n, _ := r.Read(buf)
			_ = r.Close()
			got := string(buf[:n])

			if got != tt.expected {
				t.Errorf("TmuxAttachDirective(%q, %v) = %q, want %q", tt.session, tt.controlMode, got, tt.expected)
			}
		})
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
