package docker

import (
	"errors"
	"strings"
	"testing"
)

func TestTranslateRunError_DependencyDidntComplete(t *testing.T) {
	stderr := `Container my-stack-asset_precompile-1  Error
service "asset_precompile" didn't complete successfully: exit 1`
	original := errors.New("exit status 1")

	got := translateRunError(stderr, original)

	if got == nil {
		t.Fatal("expected translated error, got nil")
	}
	msg := got.Error()
	if !strings.Contains(msg, "asset_precompile") {
		t.Errorf("expected service name in message, got: %s", msg)
	}
	if !strings.Contains(msg, "include_deps") && !strings.Contains(msg, "ephemeral") {
		t.Errorf("expected actionable hint mentioning include_deps or ephemeral, got: %s", msg)
	}
}

func TestTranslateRunError_PassThroughOnUnknownPattern(t *testing.T) {
	stderr := "some other docker error"
	original := errors.New("exit status 1")

	got := translateRunError(stderr, original)

	if got != original {
		t.Errorf("expected original error pass-through, got: %v", got)
	}
}

func TestTranslateRunError_PassThroughOnEmptyStderr(t *testing.T) {
	original := errors.New("exit status 1")
	got := translateRunError("", original)
	if got != original {
		t.Errorf("expected original error pass-through, got: %v", got)
	}
}
