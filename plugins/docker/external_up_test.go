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
	// Empty statuses (nil) means the probe returned no usable data — either it
	// failed, timed out, or compose ps reported nothing. finalizeUpResult must
	// propagate cmdErr in this case rather than swallowing it as healthy.
	got := finalizeUpResult(cmdErr, nil, nil)
	if got == nil {
		t.Errorf("expected cmdErr propagated when no statuses available, got nil")
	}
}
