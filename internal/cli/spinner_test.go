package cli

import (
	"errors"
	"testing"
)

func TestSpinWithResult_Success(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	result, err := SpinWithResult("testing", func() (string, error) {
		return "hello", nil
	})

	if err != nil {
		t.Errorf("SpinWithResult() unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("SpinWithResult() result = %q, want %q", result, "hello")
	}
}

func TestSpinWithResult_FnError(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	fnErr := errors.New("function error")
	result, err := SpinWithResult("testing", func() (string, error) {
		return "partial", fnErr
	})

	if err != fnErr {
		t.Errorf("SpinWithResult() error = %v, want fn error %v", err, fnErr)
	}
	// Result may be set even on error — just verify error propagation
	_ = result
}

func TestSpinWithResult_ErrorPrecedence(t *testing.T) {
	// SpinWithResult error precedence: fn error takes priority over framework errors.
	// When fnErr != nil, it is always returned (even if Spin itself also returned an error).
	// This test verifies the fn error is returned unchanged.
	t.Setenv("NO_COLOR", "1")

	sentinel := errors.New("sentinel fn error")
	_, err := SpinWithResult("testing", func() (int, error) {
		return 0, sentinel
	})

	if !errors.Is(err, sentinel) {
		t.Errorf("SpinWithResult() should return fn error: got %v, want %v", err, sentinel)
	}
}

func TestSpinWithResult_IntResult(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	count, err := SpinWithResult("counting", func() (int, error) {
		return 42, nil
	})

	if err != nil {
		t.Errorf("SpinWithResult() unexpected error: %v", err)
	}
	if count != 42 {
		t.Errorf("SpinWithResult() = %d, want 42", count)
	}
}
