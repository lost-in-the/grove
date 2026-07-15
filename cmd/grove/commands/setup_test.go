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
	existing := `eval "$(grove install zsh --alias=q)"` + "\n"

	content := setupWithExistingRC(t, existing, "")

	if strings.Contains(content, "--alias=q") {
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
