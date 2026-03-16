package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/lost-in-the/grove/internal/cmdexec"
)

const sessionStatusNone = "none"

// Session represents a tmux session
type Session struct {
	Name     string
	Windows  int
	Attached bool
	Created  string
}

// IsInsideTmux checks if we're currently inside a tmux session
func IsInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// IsControlModeTerminal returns true if the current terminal supports tmux control mode.
// Currently this means iTerm2.
func IsControlModeTerminal() bool {
	return os.Getenv("TERM_PROGRAM") == "iTerm2"
}

// ShouldUseControlMode determines whether to use tmux -CC based on config and terminal.
// Returns true when: config allows it (nil or true) AND terminal supports it.
func ShouldUseControlMode(controlModeCfg *bool) bool {
	if controlModeCfg != nil && !*controlModeCfg {
		return false
	}
	return IsControlModeTerminal()
}

// AttachSessionControlMode attaches to a session using tmux -CC (control mode).
// This is for iTerm2 integration where -CC goes before the subcommand.
func AttachSessionControlMode(name string) error {
	return attachSession(name, "-CC", "attach-session", "-t", name)
}

var (
	tmuxAvailableOnce   sync.Once
	tmuxAvailableResult bool
)

// IsTmuxAvailable checks if tmux is installed. The result is cached for the
// lifetime of the process since tmux availability doesn't change at runtime.
func IsTmuxAvailable() bool {
	tmuxAvailableOnce.Do(func() {
		_, err := exec.LookPath("tmux")
		tmuxAvailableResult = err == nil
	})
	return tmuxAvailableResult
}

// CreateSession creates a new detached tmux session.
// If the session already exists, this is a no-op (idempotent).
func CreateSession(name, path string) error {
	return CreateSessionWithCommand(name, path, "")
}

// AttachSession attaches to an existing session.
// This is interactive (blocks until detach) — no timeout applied.
func AttachSession(name string) error {
	return attachSession(name, "attach-session", "-t", name)
}

// attachSession is the shared implementation for AttachSession and AttachSessionControlMode.
// It validates the session, falls back to SwitchSession when inside tmux,
// and otherwise runs tmux with the given args interactively.
func attachSession(name string, tmuxArgs ...string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	exists, err := SessionExists(name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("session '%s' does not exist", name)
	}

	if IsInsideTmux() {
		return SwitchSession(name)
	}

	cmd := exec.Command("tmux", tmuxArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// SwitchSession switches to a different session from within tmux.
// Callers are responsible for checking SessionExists beforehand if needed.
func SwitchSession(name string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	if !IsInsideTmux() {
		return fmt.Errorf("not inside tmux session")
	}

	output, err := cmdexec.CombinedOutput(context.TODO(), "tmux", []string{"switch-client", "-t", name}, "", cmdexec.Tmux)
	if err != nil {
		return fmt.Errorf("failed to switch session: %s: %w", string(output), err)
	}

	return nil
}

// RenameSession renames an existing tmux session.
// Callers are responsible for checking SessionExists beforehand if needed.
func RenameSession(oldName, newName string) error {
	if oldName == "" {
		return fmt.Errorf("old session name cannot be empty")
	}
	if newName == "" {
		return fmt.Errorf("new session name cannot be empty")
	}

	output, err := cmdexec.CombinedOutput(context.TODO(), "tmux", []string{"rename-session", "-t", oldName, newName}, "", cmdexec.Tmux)
	if err != nil {
		return fmt.Errorf("failed to rename tmux session: %s: %w", string(output), err)
	}

	return nil
}

// KillSession kills a tmux session.
// Callers are responsible for checking SessionExists beforehand if needed.
func KillSession(name string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	output, err := cmdexec.CombinedOutput(context.TODO(), "tmux", []string{"kill-session", "-t", name}, "", cmdexec.Tmux)
	if err != nil {
		return fmt.Errorf("failed to kill session: %s: %w", string(output), err)
	}

	return nil
}

// SessionExists checks if a session exists
func SessionExists(name string) (bool, error) {
	err := cmdexec.Run(context.TODO(), "tmux", []string{"has-session", "-t", name}, "", cmdexec.Tmux)
	if err != nil {
		// Exit code 1 means session doesn't exist
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

// ListSessions returns all tmux sessions
func ListSessions() ([]*Session, error) {
	output, err := cmdexec.Output(context.TODO(), "tmux", []string{"list-sessions", "-F", "#{session_name}|#{session_windows}|#{session_attached}|#{session_created}"}, "", cmdexec.Tmux)
	if err != nil {
		// If no sessions exist, that's okay
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return []*Session{}, nil
			}
		}
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	sessions := make([]*Session, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}

		session := &Session{
			Name:     parts[0],
			Windows:  parseIntOrZero(parts[1]),
			Attached: parts[2] == "1",
			Created:  parts[3],
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// GetCurrentSession returns the name of the current tmux session
func GetCurrentSession() (string, error) {
	if !IsInsideTmux() {
		return "", fmt.Errorf("not inside tmux session")
	}

	output, err := cmdexec.Output(context.TODO(), "tmux", []string{"display-message", "-p", "#{session_name}"}, "", cmdexec.Tmux)
	if err != nil {
		return "", fmt.Errorf("failed to get current session: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// StoreLastSession stores the name of the last session
func StoreLastSession(name string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(homeDir, ".config", "grove")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	lastSessionFile := filepath.Join(configDir, "last_session")
	tmpFile := lastSessionFile + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(name), 0644); err != nil {
		return fmt.Errorf("write last session: %w", err)
	}
	if err := os.Rename(tmpFile, lastSessionFile); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("save last session: %w", err)
	}
	return nil
}

// GetLastSession retrieves the name of the last session
func GetLastSession() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	lastSessionFile := filepath.Join(homeDir, ".config", "grove", "last_session")
	data, err := os.ReadFile(lastSessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no last session stored")
		}
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

// GetSessionStatus returns the status of a tmux session
// Returns "attached", "detached", or "none" if session doesn't exist
func GetSessionStatus(name string) string {
	if !IsTmuxAvailable() {
		return sessionStatusNone
	}

	sessions, err := ListSessions()
	if err != nil {
		return sessionStatusNone
	}

	for _, s := range sessions {
		if s.Name == name {
			if s.Attached {
				return "attached"
			}
			return "detached"
		}
	}

	return sessionStatusNone
}

// PaneInfo holds the current state of a session's active pane
type PaneInfo struct {
	CurrentPath    string
	CurrentCommand string
}

// GetPaneInfo returns the current path and command for a session's active pane
func GetPaneInfo(sessionName string) (*PaneInfo, error) {
	output, err := cmdexec.Output(context.TODO(), "tmux", []string{"display-message", "-t", sessionName, "-p", "#{pane_current_path}|#{pane_current_command}"}, "", cmdexec.Tmux)
	if err != nil {
		return nil, fmt.Errorf("failed to get pane info: %w", err)
	}

	parts := strings.SplitN(strings.TrimSpace(string(output)), "|", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("unexpected pane info format")
	}

	return &PaneInfo{
		CurrentPath:    parts[0],
		CurrentCommand: parts[1],
	}, nil
}

// IsShell returns true if the pane's current command is a known shell
func (p *PaneInfo) IsShell() bool {
	shells := map[string]bool{
		"bash": true,
		"zsh":  true,
		"fish": true,
		"sh":   true,
	}
	return shells[p.CurrentCommand]
}

// SendKeys sends keys to the active pane of a tmux session
func SendKeys(sessionName string, keys string) error {
	output, err := cmdexec.CombinedOutput(context.TODO(), "tmux", []string{"send-keys", "-t", sessionName, keys, "Enter"}, "", cmdexec.Tmux)
	if err != nil {
		return fmt.Errorf("failed to send keys: %s: %w", string(output), err)
	}
	return nil
}

// parseIntOrZero attempts to parse an integer, returning 0 on error
func parseIntOrZero(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

// CreateSessionWithCommand creates a new detached tmux session running a specific command.
// If command is empty, behaves identically to CreateSession (runs default shell).
// If the session already exists, this is a no-op (idempotent).
func CreateSessionWithCommand(name, path, command string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	exists, err := SessionExists(name)
	if err != nil {
		return fmt.Errorf("check session exists: %w", err)
	}
	if exists {
		return nil
	}

	args := []string{"new-session", "-d", "-s", name, "-c", path}
	if command != "" {
		args = append(args, command)
	}

	output, err := cmdexec.CombinedOutput(context.TODO(), "tmux", args, "", cmdexec.Tmux)
	if err != nil {
		return fmt.Errorf("failed to create session: %s: %w", string(output), err)
	}

	return nil
}

// DisplayPopup opens a tmux display-popup attached to an existing session.
// Width and height are tmux percentage strings (e.g., "80%").
// This is interactive — no timeout applied.
func DisplayPopup(sessionName, width, height string) error {
	if sessionName == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	if !IsInsideTmux() {
		return fmt.Errorf("display-popup requires being inside tmux")
	}

	args := []string{"display-popup"}
	if width != "" {
		args = append(args, "-w", width)
	}
	if height != "" {
		args = append(args, "-h", height)
	}
	// Session name is single-quoted to prevent shell injection. Tmux session
	// names follow {project}-{name} and cannot contain single quotes.
	args = append(args, "-E", fmt.Sprintf("tmux attach-session -t '%s'", sessionName))

	cmd := exec.Command("tmux", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// IsCommandRunning checks if a session's active pane is running a specific command.
// Returns false if the session doesn't exist or pane info can't be retrieved.
func IsCommandRunning(sessionName, command string) bool {
	pane, err := GetPaneInfo(sessionName)
	if err != nil {
		return false
	}
	return pane.CurrentCommand == command
}
