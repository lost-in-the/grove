package docker

import (
	"os/exec"
	"strings"
	"testing"
)

// TestRunWithErrorTranslation_Success verifies that a zero-exit command returns nil.
func TestRunWithErrorTranslation_Success(t *testing.T) {
	cmd := exec.Command("true")
	if err := runWithErrorTranslation(cmd, false); err != nil {
		t.Fatalf("expected nil for zero-exit cmd, got: %v", err)
	}
}

// TestRunWithErrorTranslation_FailureEmptyStderr verifies that a non-zero exit with no
// stderr output passes the original error through unchanged.
func TestRunWithErrorTranslation_FailureEmptyStderr(t *testing.T) {
	cmd := exec.Command("false")
	err := runWithErrorTranslation(cmd, false)
	if err == nil {
		t.Fatal("expected non-nil error for non-zero exit")
	}
	// translateRunError returns original on empty stderr, so error text should
	// not include any grove hint.
	if strings.Contains(err.Error(), "--with-deps") {
		t.Errorf("unexpected hint in error for empty-stderr case: %v", err)
	}
	if strings.Contains(err.Error(), "dependency") {
		t.Errorf("unexpected dependency rewrite for empty-stderr case: %v", err)
	}
}

// TestRunWithErrorTranslation_ConnectionRefused_NoIncludeDeps verifies that a
// connection-refused stderr produces the --with-deps hint when includeDeps=false.
func TestRunWithErrorTranslation_ConnectionRefused_NoIncludeDeps(t *testing.T) {
	cmd := exec.Command("sh", "-c", "echo 'connection refused' >&2; exit 1")
	err := runWithErrorTranslation(cmd, false)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "--with-deps") {
		t.Errorf("expected --with-deps hint, got: %v", err)
	}
}

// TestRunWithErrorTranslation_ConnectionRefused_IncludeDeps verifies that the
// --with-deps hint is suppressed when includeDeps=true.
func TestRunWithErrorTranslation_ConnectionRefused_IncludeDeps(t *testing.T) {
	cmd := exec.Command("sh", "-c", "echo 'connection refused' >&2; exit 1")
	err := runWithErrorTranslation(cmd, true)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if strings.Contains(err.Error(), "--with-deps") {
		t.Errorf("expected no --with-deps hint when includeDeps=true, got: %v", err)
	}
}

// TestRunWithErrorTranslation_DependencyFailure verifies that a compose
// "didn't complete successfully" stderr triggers the dependency-failure rewrite.
func TestRunWithErrorTranslation_DependencyFailure(t *testing.T) {
	cmd := exec.Command("sh", "-c", `echo 'service "postgres" didn'"'"'t complete successfully: exit 1' >&2; exit 1`)
	err := runWithErrorTranslation(cmd, false)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "postgres") {
		t.Errorf("expected service name 'postgres' in rewritten error, got: %v", err)
	}
	if !strings.Contains(msg, "unrelated") {
		t.Errorf("expected 'unrelated' in rewritten dependency-failure message, got: %v", err)
	}
}
