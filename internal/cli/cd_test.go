package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanCd_MatchesCdDirective(t *testing.T) {
	existing := filepath.Join(t.TempDir(), "cd")
	if err := os.WriteFile(existing, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "gone")

	tests := []struct {
		name   string
		cdFile string
		shell  string
		want   bool
	}{
		{"no wrapper at all", "", "", false},
		{"shell function, captured command", "", "1", true},
		{"shell function, cd file", existing, "1", true},
		{"cd file alone", existing, "", true},
		{"stale cd file, no shell function", missing, "", false},
		{"stale cd file under the shell function", missing, "1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GROVE_CD_FILE", tt.cdFile)
			t.Setenv("GROVE_SHELL", tt.shell)
			if got := CanCd(); got != tt.want {
				t.Errorf("CanCd() = %v, want %v", got, tt.want)
			}
			if _, err := os.Stat(missing); !os.IsNotExist(err) {
				t.Fatal("CanCd created the stale cd file")
			}
			// CanCd promises what CdDirective then does.
			var got bool
			captured := captureStdout(t, func() { got = CdDirective("/target") })
			if got != tt.want {
				t.Errorf("CdDirective() = %v, CanCd() said %v (stdout %q)", got, tt.want, captured)
			}
		})
	}
}

func TestCanCd_DoesNotTruncateTheCdFile(t *testing.T) {
	cdFile := filepath.Join(t.TempDir(), "cd")
	if err := os.WriteFile(cdFile, []byte("/kept"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GROVE_CD_FILE", cdFile)
	t.Setenv("GROVE_SHELL", "")
	if !CanCd() {
		t.Fatal("CanCd() = false for an existing cd file")
	}
	if got, _ := os.ReadFile(cdFile); string(got) != "/kept" {
		t.Errorf("cd file = %q after CanCd, want it untouched", got)
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain path", in: "/src/proj-fix", want: "'/src/proj-fix'"},
		{name: "embedded single quote", in: "/src/Dev's Projects/app-fix", want: `'/src/Dev'\''s Projects/app-fix'`},
		{name: "empty string", in: "", want: "''"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShellQuote(tt.in); got != tt.want {
				t.Errorf("ShellQuote(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExplainNoCd_SaysWhyAndHowToMove(t *testing.T) {
	var buf bytes.Buffer
	ExplainNoCd(NewWriter(&buf, false), "/src/Dev's app/app-feature")
	got := buf.String()

	for _, want := range []string{
		"⚠ grove could not change your directory: shell integration is not loaded.",
		"Running the binary by its path skips that function",
		"needs a new shell",
		`eval "$(grove install zsh)"`,
		`eval "$(grove install bash)"`,
		`cd '/src/Dev'\''s app/app-feature'`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("explanation is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\x1b[") {
		t.Errorf("explanation to a non-terminal carries ANSI escapes:\n%q", got)
	}
}

func TestExplainCannotMove_NamesTheSessionToo(t *testing.T) {
	var buf bytes.Buffer
	ExplainCannotMove(NewWriter(&buf, false), "/src/app-feature")
	got := buf.String()

	first, _, _ := strings.Cut(got, "\n")
	if first != "⚠ grove could not move you: no session could be switched, and shell integration is not loaded." {
		t.Errorf("first line = %q", first)
	}
	// The rest is the CLI's explanation, word for word.
	var cli bytes.Buffer
	ExplainNoCd(NewWriter(&cli, false), "/src/app-feature")
	_, wantRest, _ := strings.Cut(cli.String(), "\n")
	if _, rest, _ := strings.Cut(got, "\n"); rest != wantRest {
		t.Errorf("explanation drifted from the CLI's:\n%s\nwant:\n%s", rest, wantRest)
	}
}

// captureStdout returns what fn wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}
