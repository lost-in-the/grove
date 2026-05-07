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

func TestTeeBuffer_SingleWriteLargerThanCap(t *testing.T) {
	tb := &teeBuffer{}
	// Build a payload larger than the 8KB cap.
	payload := make([]byte, stderrBufferLimit+1024)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	n, err := tb.Write(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(payload) {
		t.Errorf("Write returned n=%d, want %d", n, len(payload))
	}

	// Buffer must not exceed the cap.
	if len(tb.buf) > stderrBufferLimit {
		t.Errorf("buf len %d exceeds cap %d", len(tb.buf), stderrBufferLimit)
	}

	// Tail bytes must be preserved.
	tail := payload[len(payload)-stderrBufferLimit:]
	if string(tb.buf) != string(tail) {
		t.Errorf("buf does not contain the tail of the written payload")
	}
}

func TestTeeBuffer_MultipleSmallWrites(t *testing.T) {
	tb := &teeBuffer{}
	chunk := []byte(strings.Repeat("x", 1024))

	// Write 10 chunks of 1KB each — total 10KB, cap is 8KB.
	for i := 0; i < 10; i++ {
		if _, err := tb.Write(chunk); err != nil {
			t.Fatalf("Write %d failed: %v", i, err)
		}
	}

	if len(tb.buf) > stderrBufferLimit {
		t.Errorf("buf len %d exceeds cap %d after multiple writes", len(tb.buf), stderrBufferLimit)
	}
	// The buffer should hold exactly the last 8 chunks.
	want := strings.Repeat("x", stderrBufferLimit)
	if tb.String() != want {
		t.Errorf("buf content mismatch: got len %d, want len %d", len(tb.buf), stderrBufferLimit)
	}
}

func TestTeeBuffer_ExactCapWrite(t *testing.T) {
	tb := &teeBuffer{}
	payload := make([]byte, stderrBufferLimit)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	if _, err := tb.Write(payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tb.buf) != stderrBufferLimit {
		t.Errorf("buf len %d, want %d", len(tb.buf), stderrBufferLimit)
	}
}
