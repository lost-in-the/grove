package docker

import (
	"errors"
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
	stderr := `warning: The "AGENT_MYAPP_DIR" variable is not set. Defaulting to a blank string.
Error response from daemon: invalid spec: :/app:delegated: empty section between colons`
	env := []string{"APP_DIR=/tmp/wt", "AGENT_SLOT=2"}
	original := errors.New("exit status 1")

	got := enrichAgentStackStartError(stderr, env, original)

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
	stderr := "Error response from daemon: pull access denied"
	env := []string{"APP_DIR=/tmp/wt"}
	original := errors.New("exit status 1")

	got := enrichAgentStackStartError(stderr, env, original)

	if got.Error() != "failed to start agent stack: exit status 1" {
		t.Errorf("got %q, want unchanged 'failed to start agent stack: exit status 1' wrapping", got.Error())
	}
	if !errors.Is(got, original) {
		t.Errorf("expected original error to be chained via %%w, got: %v", got)
	}
}

func TestEnrichAgentStackStartError_EmptyStderr_PreservesOriginalWrapping(t *testing.T) {
	original := errors.New("exit status 1")

	got := enrichAgentStackStartError("", []string{"APP_DIR=/tmp/wt"}, original)

	if got.Error() != "failed to start agent stack: exit status 1" {
		t.Errorf("got %q, want unchanged 'failed to start agent stack: exit status 1' wrapping", got.Error())
	}
}
