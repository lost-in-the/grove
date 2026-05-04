package cli

import (
	"testing"
)

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
