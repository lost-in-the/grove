package commands

import (
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/shell"
)

// The alias is opt-in: default output has none, --alias adds it, and the
// name must survive the generator's validation (see internal/shell).
func TestInstall_AliasFlag(t *testing.T) {
	t.Run("no alias by default", func(t *testing.T) {
		output, err := shell.GenerateZshIntegration("")
		if err != nil {
			t.Fatalf("GenerateZshIntegration error: %v", err)
		}
		if strings.Contains(output, "alias ") {
			t.Error("default install output must not define an alias")
		}
	})

	t.Run("bare --alias resolves to w", func(t *testing.T) {
		// NoOptDefVal wiring: `grove install zsh --alias` must behave like
		// --alias=w.
		flag := installCmd.Flags().Lookup("alias")
		if flag == nil {
			t.Fatal("install command has no --alias flag")
		}
		if flag.NoOptDefVal != "w" {
			t.Errorf("bare --alias should default to 'w', got %q", flag.NoOptDefVal)
		}
	})

	t.Run("alias appears in output", func(t *testing.T) {
		output, err := shell.GenerateZshIntegration("w")
		if err != nil {
			t.Fatalf("GenerateZshIntegration error: %v", err)
		}
		if !strings.Contains(output, "alias w=grove") {
			t.Error("expected 'alias w=grove' in output with alias set")
		}
	})
}
