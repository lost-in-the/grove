//go:build !windows

package tui

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestCaptureStdio_BoundedDrainWithLingeringWriter(t *testing.T) {
	orig := captureDrainTimeout
	captureDrainTimeout = 200 * time.Millisecond
	defer func() { captureDrainTimeout = orig }()

	var straggler int
	start := time.Now()
	out, err := captureStdio(func() error {
		fmt.Fprint(os.Stdout, "partial")
		// Duplicate the pipe's write end, exactly as a subprocess inheriting
		// fd 1 would, and keep it open past fn's return.
		fd, dupErr := syscall.Dup(int(os.Stdout.Fd()))
		if dupErr != nil {
			t.Fatalf("dup: %v", dupErr)
		}
		straggler = fd
		return nil
	})
	elapsed := time.Since(start)
	defer func() { _ = syscall.Close(straggler) }()

	if err != nil {
		t.Fatalf("captureStdio: %v", err)
	}
	if out != "partial" {
		t.Errorf("captured = %q, want %q", out, "partial")
	}
	if elapsed >= 2*time.Second {
		t.Errorf("captureStdio took %v with a lingering writer; want return shortly after the %v drain timeout", elapsed, captureDrainTimeout)
	}
}
