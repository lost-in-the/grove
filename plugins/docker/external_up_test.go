package docker

import (
	"errors"
	"testing"
)

func TestUpResult_FailureBecomesSuccessIfBlockersHealthy(t *testing.T) {
	statuses := []ServiceStatus{
		{Name: "app", Status: ServiceRunning},
		{Name: "asset_precompile", Status: ServiceExitedError},
	}
	cmdErr := errors.New("exit status 1")

	got := finalizeUpResult(cmdErr, statuses, []string{"asset_precompile"})

	if got != nil {
		t.Errorf("expected nil error (only non-blocking service failed), got: %v", got)
	}
}

func TestUpResult_FailurePreservedIfBlockerFailed(t *testing.T) {
	statuses := []ServiceStatus{
		{Name: "app", Status: ServiceExitedError},
	}
	cmdErr := errors.New("exit status 1")

	got := finalizeUpResult(cmdErr, statuses, nil)
	if got == nil {
		t.Errorf("expected non-nil error (app is blocking)")
	}
}

func TestUpResult_NilOnNoCmdError(t *testing.T) {
	statuses := []ServiceStatus{{Name: "app", Status: ServiceRunning}}
	if got := finalizeUpResult(nil, statuses, nil); got != nil {
		t.Errorf("expected nil, got: %v", got)
	}
}

func TestUpResult_PreservesErrWhenProbeReturnsNoStatuses(t *testing.T) {
	cmdErr := errors.New("compose up failed")
	// Empty statuses simulates probe success but no services — finalizeUpResult
	// currently treats this as healthy=true and returns nil. Verify behavior:
	// with an empty status list and no non-blocking config, finalizeUpResult
	// should NOT swallow the cmdErr.
	got := finalizeUpResult(cmdErr, nil, nil)
	if got == nil {
		t.Errorf("expected cmdErr propagated when no statuses available, got nil")
	}
}
