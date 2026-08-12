package docker

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseUnsetTemplateVars_SingleWarning(t *testing.T) {
	stderr := `warning: The "AGENT_MYAPP_DIR" variable is not set. Defaulting to a blank string.
Error response from daemon: invalid spec: :/app:delegated: empty section between colons`

	got := parseUnsetTemplateVars(stderr)

	if len(got) != 1 || got[0] != "AGENT_MYAPP_DIR" {
		t.Errorf("parseUnsetTemplateVars() = %v, want [AGENT_MYAPP_DIR]", got)
	}
}

func TestParseUnsetTemplateVars_MultipleWarnings_Dedup(t *testing.T) {
	stderr := `warning: The "AGENT_MYAPP_DIR" variable is not set. Defaulting to a blank string.
warning: The "AGENT_MYAPP_DIR" variable is not set. Defaulting to a blank string.
warning: The "AGENT_OTHER" variable is not set. Defaulting to a blank string.`

	got := parseUnsetTemplateVars(stderr)

	want := []string{"AGENT_MYAPP_DIR", "AGENT_OTHER"}
	if len(got) != len(want) {
		t.Fatalf("parseUnsetTemplateVars() = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("parseUnsetTemplateVars()[%d] = %q, want %q", i, got[i], name)
		}
	}
}

func TestParseUnsetTemplateVars_NoWarning(t *testing.T) {
	stderr := "Error response from daemon: pull access denied"

	got := parseUnsetTemplateVars(stderr)

	if got != nil {
		t.Errorf("parseUnsetTemplateVars() = %v, want nil", got)
	}
}

func TestParseUnsetTemplateVars_EmptyStderr(t *testing.T) {
	got := parseUnsetTemplateVars("")
	if got != nil {
		t.Errorf("parseUnsetTemplateVars(\"\") = %v, want nil", got)
	}
}

func TestExportedEnvNames(t *testing.T) {
	env := []string{"APP_DIR=/tmp/some/worktree/path", "AGENT_SLOT=3"}

	got := exportedEnvNames(env)

	want := []string{"APP_DIR", "AGENT_SLOT"}
	if len(got) != len(want) {
		t.Fatalf("exportedEnvNames() = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("exportedEnvNames()[%d] = %q, want %q", i, got[i], name)
		}
	}
}

func TestExportedEnvNames_NoValueSeparator(t *testing.T) {
	// Defensive: a malformed entry without '=' should pass through unchanged
	// rather than panicking or dropping it silently.
	got := exportedEnvNames([]string{"MALFORMED"})
	if len(got) != 1 || got[0] != "MALFORMED" {
		t.Errorf("exportedEnvNames() = %v, want [MALFORMED]", got)
	}
}

func TestExportedEnvNames_NeverIncludesValues(t *testing.T) {
	env := []string{"APP_DIR=/Users/leah/secret/worktree/path"}

	got := exportedEnvNames(env)

	for _, name := range got {
		if strings.Contains(name, "/") {
			t.Errorf("exportedEnvNames() leaked a path-like value: %q", name)
		}
	}
}

func TestEnrichAgentStackStartError_UnsetVariable_NamesBoth(t *testing.T) {
	unsetVars := []string{"AGENT_MYAPP_DIR"}
	env := []string{"APP_DIR=/tmp/wt", "AGENT_SLOT=2"}
	original := errors.New("exit status 1")

	got := enrichAgentStackStartError(unsetVars, env, original)

	if got == nil {
		t.Fatal("expected enriched error, got nil")
	}
	msg := got.Error()
	if !strings.Contains(msg, "AGENT_MYAPP_DIR") {
		t.Errorf("expected unset variable name in message, got: %s", msg)
	}
	if !strings.Contains(msg, "APP_DIR") || !strings.Contains(msg, "AGENT_SLOT") {
		t.Errorf("expected exported variable names in message, got: %s", msg)
	}
	if strings.Contains(msg, "/tmp/wt") {
		t.Errorf("expected exported values NOT to appear in message, got: %s", msg)
	}
	if !errors.Is(got, original) {
		t.Errorf("expected original error to be chained via %%w, got: %v", got)
	}
}

func TestEnrichAgentStackStartError_NoWarning_PreservesOriginalWrapping(t *testing.T) {
	env := []string{"APP_DIR=/tmp/wt"}
	original := errors.New("exit status 1")

	got := enrichAgentStackStartError(nil, env, original)

	if got.Error() != "failed to start agent stack: exit status 1" {
		t.Errorf("got %q, want unchanged 'failed to start agent stack: exit status 1' wrapping", got.Error())
	}
	if !errors.Is(got, original) {
		t.Errorf("expected original error to be chained via %%w, got: %v", got)
	}
}

func TestEnrichAgentStackStartError_NilUnsetVars_PreservesOriginalWrapping(t *testing.T) {
	original := errors.New("exit status 1")

	got := enrichAgentStackStartError(nil, []string{"APP_DIR=/tmp/wt"}, original)

	if got.Error() != "failed to start agent stack: exit status 1" {
		t.Errorf("got %q, want unchanged 'failed to start agent stack: exit status 1' wrapping", got.Error())
	}
}

func TestUnsetVarScanWriter_SingleWarning(t *testing.T) {
	var inner bytes.Buffer
	w := newUnsetVarScanWriter(&inner)

	_, err := w.Write([]byte("warning: The \"AGENT_MYAPP_DIR\" variable is not set. Defaulting to a blank string.\n"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got := w.UnsetVars()
	if len(got) != 1 || got[0] != "AGENT_MYAPP_DIR" {
		t.Errorf("UnsetVars() = %v, want [AGENT_MYAPP_DIR]", got)
	}
	if inner.String() != "warning: The \"AGENT_MYAPP_DIR\" variable is not set. Defaulting to a blank string.\n" {
		t.Errorf("inner writer did not receive the full tee: %q", inner.String())
	}
}

func TestUnsetVarScanWriter_SplitAcrossWrites(t *testing.T) {
	var inner bytes.Buffer
	w := newUnsetVarScanWriter(&inner)

	full := "warning: The \"AGENT_MYAPP_DIR\" variable is not set. Defaulting to a blank string.\n"
	mid := len(full) / 2

	if _, err := w.Write([]byte(full[:mid])); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	// Before the newline arrives, nothing should have been detected yet.
	if got := w.UnsetVars(); len(got) != 0 {
		t.Errorf("UnsetVars() before line completion = %v, want none", got)
	}
	if _, err := w.Write([]byte(full[mid:])); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got := w.UnsetVars()
	if len(got) != 1 || got[0] != "AGENT_MYAPP_DIR" {
		t.Errorf("UnsetVars() = %v, want [AGENT_MYAPP_DIR]", got)
	}
	if inner.String() != full {
		t.Errorf("inner writer did not receive the full tee across split writes: %q", inner.String())
	}
}

func TestUnsetVarScanWriter_WarningThenLargeNoise_StillDetected(t *testing.T) {
	var inner bytes.Buffer
	w := newUnsetVarScanWriter(&inner)

	warning := "warning: The \"AGENT_MYAPP_DIR\" variable is not set. Defaulting to a blank string.\n"
	if _, err := w.Write([]byte(warning)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Simulate a stack that emits well over stderrBufferLimit (8KB) of noise
	// after the warning — the old fixed trailing-window buffer would evict
	// the warning here, but a streaming scan must not.
	noise := strings.Repeat("some unrelated pull/build output line\n", 1000)
	if len(noise) <= stderrBufferLimit {
		t.Fatalf("test setup: noise (%d bytes) must exceed stderrBufferLimit (%d bytes)", len(noise), stderrBufferLimit)
	}
	if _, err := w.Write([]byte(noise)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got := w.UnsetVars()
	if len(got) != 1 || got[0] != "AGENT_MYAPP_DIR" {
		t.Errorf("UnsetVars() = %v, want [AGENT_MYAPP_DIR] even after >8KB of trailing noise", got)
	}
}

func TestUnsetVarScanWriter_NoTrailingNewline_RequiresFlush(t *testing.T) {
	var inner bytes.Buffer
	w := newUnsetVarScanWriter(&inner)

	// Process died mid-line: no trailing newline ever arrives.
	partial := "warning: The \"AGENT_MYAPP_DIR\" variable is not set. Defaulting to a blank string."
	if _, err := w.Write([]byte(partial)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if got := w.UnsetVars(); len(got) != 0 {
		t.Errorf("UnsetVars() before Flush = %v, want none", got)
	}

	w.Flush()

	got := w.UnsetVars()
	if len(got) != 1 || got[0] != "AGENT_MYAPP_DIR" {
		t.Errorf("UnsetVars() after Flush = %v, want [AGENT_MYAPP_DIR]", got)
	}

	// Flush must be idempotent.
	w.Flush()
	got = w.UnsetVars()
	if len(got) != 1 || got[0] != "AGENT_MYAPP_DIR" {
		t.Errorf("UnsetVars() after second Flush = %v, want [AGENT_MYAPP_DIR] (no duplication)", got)
	}
}

func TestUnsetVarScanWriter_DedupAndCap(t *testing.T) {
	var inner bytes.Buffer
	w := newUnsetVarScanWriter(&inner)

	for i := 0; i < maxUnsetVars+5; i++ {
		line := fmt.Sprintf("warning: The \"AGENT_VAR_%d\" variable is not set. Defaulting to a blank string.\n", i)
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		// Duplicate each warning to confirm dedup as well as the cap.
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	got := w.UnsetVars()
	if len(got) != maxUnsetVars {
		t.Fatalf("UnsetVars() len = %d, want %d (cap)", len(got), maxUnsetVars)
	}
	seen := make(map[string]bool, len(got))
	for _, name := range got {
		if seen[name] {
			t.Errorf("UnsetVars() contains duplicate: %s", name)
		}
		seen[name] = true
	}
}

func TestUnsetVarScanWriter_NoWarning(t *testing.T) {
	var inner bytes.Buffer
	w := newUnsetVarScanWriter(&inner)

	if _, err := w.Write([]byte("Error response from daemon: pull access denied\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	w.Flush()

	if got := w.UnsetVars(); got != nil {
		t.Errorf("UnsetVars() = %v, want nil", got)
	}
}
