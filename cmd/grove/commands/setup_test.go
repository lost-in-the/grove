package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runSetup executes the setup command against a temp HOME with the given
// shell and --alias value ("" = no alias). Returns the rc file path.
func runSetup(t *testing.T, alias string) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")

	prev := setupAliasFlag
	setupAliasFlag = alias
	t.Cleanup(func() { setupAliasFlag = prev })

	if err := setupCmd.RunE(setupCmd, nil); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	return filepath.Join(home, ".zshrc")
}

func TestSetup_FreshInstallAppendsEvalLine(t *testing.T) {
	rcFile := runSetup(t, "")

	data, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatalf("rc file not created: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "# Grove shell integration") {
		t.Errorf("expected marker comment in rc file, got: %q", content)
	}
	if !strings.Contains(content, `eval "$(grove install zsh)"`) {
		t.Errorf("expected eval line in rc file, got: %q", content)
	}
	if strings.Contains(content, "--alias") {
		t.Errorf("no alias requested, but rc file contains --alias: %q", content)
	}
}

func TestSetup_AliasFlagWritesAliasEvalLine(t *testing.T) {
	rcFile := runSetup(t, "w")

	data, _ := os.ReadFile(rcFile)
	if !strings.Contains(string(data), `eval "$(grove install zsh --alias=w)"`) {
		t.Errorf("expected aliased eval line in rc file, got: %q", string(data))
	}
}

func TestSetup_AliasFlagValidated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")

	prev := setupAliasFlag
	setupAliasFlag = "w; rm -rf /"
	t.Cleanup(func() { setupAliasFlag = prev })

	if err := setupCmd.RunE(setupCmd, nil); err == nil {
		t.Error("setup should reject an invalid alias name")
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Error("setup must not write the rc file when the alias is invalid")
	}
}

// setupWithExistingRC runs setup against a temp HOME whose .zshrc already
// has the given content, returning the rc content afterwards.
func setupWithExistingRC(t *testing.T, existing, alias string) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")
	rcFile := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(rcFile, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	prev := setupAliasFlag
	setupAliasFlag = alias
	t.Cleanup(func() { setupAliasFlag = prev })

	if err := setupCmd.RunE(setupCmd, nil); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	data, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSetup_MigratesDeprecatedInitLine(t *testing.T) {
	existing := "export FOO=1\n" +
		`eval "$(grove init zsh)"` + "\n" +
		"alias ll='ls -l'\n"

	content := setupWithExistingRC(t, existing, "")

	if strings.Contains(content, "grove init zsh") {
		t.Errorf("deprecated 'grove init' line should be replaced, got: %q", content)
	}
	if !strings.Contains(content, `eval "$(grove install zsh)"`) {
		t.Errorf("expected canonical eval line after migration, got: %q", content)
	}
	// The rest of the file must be untouched.
	if !strings.Contains(content, "export FOO=1\n") || !strings.Contains(content, "alias ll='ls -l'\n") {
		t.Errorf("unrelated rc content was modified: %q", content)
	}
	if strings.Count(content, "grove install") != 1 {
		t.Errorf("expected exactly one grove line after migration, got: %q", content)
	}
}

func TestSetup_MigratesMismatchedInstallLine(t *testing.T) {
	// A wrong-shell install line in the zshrc is genuinely stale and gets
	// replaced with the canonical line. (An install line that differs only
	// by --alias is NOT treated as stale — the alias choice is preserved,
	// see TestSetup_PreservesExistingAlias.)
	existing := `eval "$(grove install bash)"` + "\n"

	content := setupWithExistingRC(t, existing, "")

	if strings.Contains(content, "grove install bash") {
		t.Errorf("stale install line should be replaced, got: %q", content)
	}
	if !strings.Contains(content, `eval "$(grove install zsh)"`) {
		t.Errorf("expected canonical eval line, got: %q", content)
	}
	if strings.Count(content, "grove install") != 1 {
		t.Errorf("expected exactly one grove line, got: %q", content)
	}
}

func TestSetup_AlreadyConfiguredLeavesFileUntouched(t *testing.T) {
	existing := "# Grove shell integration\n" + `eval "$(grove install zsh)"` + "\n"

	content := setupWithExistingRC(t, existing, "")

	if content != existing {
		t.Errorf("rc file changed despite being already configured:\n got: %q\nwant: %q", content, existing)
	}
}

// Plain `grove setup` (e.g. re-run after a ShellVersion bump because doctor
// said so) must NOT strip a previously opted-in alias from the rc line.
func TestSetup_PreservesExistingAlias(t *testing.T) {
	existing := "# Grove shell integration\n" + `eval "$(grove install zsh --alias=g)"` + "\n"

	content := setupWithExistingRC(t, existing, "")

	if content != existing {
		t.Errorf("plain setup must preserve the existing --alias=g line:\n got: %q\nwant: %q", content, existing)
	}
}

func TestSetup_PreservesExistingBareAlias(t *testing.T) {
	// A bare `--alias` in the rc means "w" (NoOptDefVal); plain setup should
	// keep an alias (normalizing to the explicit form is fine).
	existing := `eval "$(grove install zsh --alias)"` + "\n"

	content := setupWithExistingRC(t, existing, "")

	if !strings.Contains(content, "--alias") {
		t.Errorf("plain setup dropped the existing bare --alias opt-in: %q", content)
	}
}

// An explicit --alias on the command line wins over the rc's existing choice.
func TestSetup_ExplicitAliasOverridesExisting(t *testing.T) {
	existing := `eval "$(grove install zsh --alias=g)"` + "\n"

	content := setupWithExistingRC(t, existing, "q")

	if !strings.Contains(content, `eval "$(grove install zsh --alias=q)"`) {
		t.Errorf("explicit --alias=q should replace the rc line, got: %q", content)
	}
	if strings.Contains(content, "--alias=g") {
		t.Errorf("old alias line should be gone, got: %q", content)
	}
}

// A stale line must be healed even when the canonical line is ALSO present —
// a leftover deprecated `grove init` line errors on every shell startup.
func TestSetup_RemovesStaleLineWhenCanonicalPresent(t *testing.T) {
	existing := "# Grove shell integration\n" +
		`eval "$(grove install zsh)"` + "\n" +
		`eval "$(grove init zsh)"` + "\n"

	content := setupWithExistingRC(t, existing, "")

	if strings.Contains(content, "grove init zsh") {
		t.Errorf("stale init line should be removed even when canonical exists, got: %q", content)
	}
	if strings.Count(content, `eval "$(grove install zsh)"`) != 1 {
		t.Errorf("expected exactly one canonical line, got: %q", content)
	}
}

// Multiple stale lines are all healed in one run — first replaced, rest deleted.
func TestSetup_MigratesAllStaleLines(t *testing.T) {
	existing := `eval "$(grove init zsh)"` + "\n" +
		"export FOO=1\n" +
		`eval "$(grove install zsh --alias=q)"` + "\n"

	// Explicit alias so the outcome is deterministic (existing rc alias
	// would otherwise be adopted).
	content := setupWithExistingRC(t, existing, "w")

	if strings.Contains(content, "grove init zsh") || strings.Contains(content, "--alias=q") {
		t.Errorf("all stale lines should be healed in one run, got: %q", content)
	}
	if strings.Count(content, "eval \"$(grove install") != 1 {
		t.Errorf("expected exactly one grove line after healing, got: %q", content)
	}
	if !strings.Contains(content, "export FOO=1\n") {
		t.Errorf("unrelated content must be untouched, got: %q", content)
	}
}

func TestSetup_RejectsPositionalArgs(t *testing.T) {
	// `grove setup --alias g` (space form) must error loudly, not silently
	// install alias "w" and drop "g".
	if err := setupCmd.Args(setupCmd, []string{"g"}); err == nil {
		t.Error("setup should reject positional arguments")
	}
}
