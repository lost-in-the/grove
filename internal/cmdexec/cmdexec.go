// Package cmdexec wraps exec.CommandContext with timeout handling for external commands.
package cmdexec

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Timeout categories for external commands.
const (
	GitLocal  = 10 * time.Second
	GitRemote = 30 * time.Second
	Tmux      = 5 * time.Second
	GHCLI     = 15 * time.Second
)

// Output runs a command with timeout and returns its stdout.
// dir may be empty to use the current working directory.
func Output(ctx context.Context, name string, args []string, dir string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("%s timed out after %v", name, timeout)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CombinedOutput runs a command with timeout and returns its combined stdout+stderr.
// dir may be empty to use the current working directory.
func CombinedOutput(ctx context.Context, name string, args []string, dir string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("%s timed out after %v", name, timeout)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Run runs a command with timeout, discarding output.
// dir may be empty to use the current working directory.
func Run(ctx context.Context, name string, args []string, dir string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("%s timed out after %v", name, timeout)
	}
	return err
}
