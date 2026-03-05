package cmdexec

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestOutput_Success(t *testing.T) {
	out, err := Output(context.Background(), "echo", []string{"hello"}, "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}

func TestOutput_Timeout(t *testing.T) {
	_, err := Output(context.Background(), "sleep", []string{"10"}, "", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

func TestCombinedOutput_CapturesStderr(t *testing.T) {
	out, err := CombinedOutput(context.Background(), "sh", []string{"-c", "echo err >&2"}, "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "err") {
		t.Fatalf("expected stderr in output, got: %q", string(out))
	}
}

func TestRun_Timeout(t *testing.T) {
	err := Run(context.Background(), "sleep", []string{"10"}, "", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

func TestOutput_WithDir(t *testing.T) {
	out, err := Output(context.Background(), "pwd", nil, "/tmp", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// /tmp may resolve to /private/tmp on macOS
	got := strings.TrimSpace(string(out))
	if got != "/tmp" && got != "/private/tmp" {
		t.Fatalf("expected /tmp or /private/tmp, got %q", got)
	}
}

func TestOutput_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := Output(ctx, "echo", []string{"hello"}, "", 5*time.Second)
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
}
