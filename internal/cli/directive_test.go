package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestCdDirective_RequiresExistingFile(t *testing.T) {
	t.Run("writes to an existing cd file", func(t *testing.T) {
		cdFile := filepath.Join(t.TempDir(), "cd")
		if err := os.WriteFile(cdFile, nil, 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GROVE_CD_FILE", cdFile)
		if !CdDirective("/target") {
			t.Fatal("expected true for an existing cd file")
		}
		if got, _ := os.ReadFile(cdFile); string(got) != "/target" {
			t.Errorf("cd file = %q, want %q", got, "/target")
		}
	})

	t.Run("does not resurrect a deleted cd file", func(t *testing.T) {
		cdFile := filepath.Join(t.TempDir(), "gone")
		t.Setenv("GROVE_CD_FILE", cdFile)
		t.Setenv("GROVE_SHELL", "") // not shell integration → falls through to false
		if CdDirective("/target") {
			t.Error("expected fall-through (false) for a missing cd file")
		}
		if _, err := os.Stat(cdFile); !os.IsNotExist(err) {
			t.Error("CdDirective recreated the deleted cd file")
		}
	})

	t.Run("missing cd file falls through to stdout under shell integration", func(t *testing.T) {
		cdFile := filepath.Join(t.TempDir(), "gone")
		t.Setenv("GROVE_CD_FILE", cdFile)
		t.Setenv("GROVE_SHELL", "1")
		if !CdDirective("/target") {
			t.Error("expected a stdout directive (true) under GROVE_SHELL=1")
		}
		if _, err := os.Stat(cdFile); !os.IsNotExist(err) {
			t.Error("CdDirective recreated the deleted cd file")
		}
	})
}

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
