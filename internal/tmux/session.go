package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

// IsTmuxAvailable checks if tmux is installed
func IsTmuxAvailable() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// CreateSession creates a new tmux session
func CreateSession(name, path string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	// Check if session already exists
	exists, err := SessionExists(name)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("session '%s' already exists", name)
	}

	// Create detached session in the specified path
	cmd := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create session: %s: %w", string(output), err)
	}

	return nil
}

// AttachSession attaches to an existing session
func AttachSession(name string) error {
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

	// If we're inside tmux, switch; otherwise attach
	if IsInsideTmux() {
		return SwitchSession(name)
	}

	// Attach to session (this will block)
	cmd := exec.Command("tmux", "attach-session", "-t", name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	return cmd.Run()
}

// SwitchSession switches to a different session from within tmux
func SwitchSession(name string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	if !IsInsideTmux() {
		return fmt.Errorf("not inside tmux session")
	}

	exists, err := SessionExists(name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("session '%s' does not exist", name)
	}

	cmd := exec.Command("tmux", "switch-client", "-t", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to switch session: %s: %w", string(output), err)
	}

	return nil
}

// KillSession kills a tmux session
func KillSession(name string) error {
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

	cmd := exec.Command("tmux", "kill-session", "-t", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to kill session: %s: %w", string(output), err)
	}

	return nil
}

// SessionExists checks if a session exists
func SessionExists(name string) (bool, error) {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	err := cmd.Run()
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
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}|#{session_windows}|#{session_attached}|#{session_created}")
	output, err := cmd.Output()
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

	cmd := exec.Command("tmux", "display-message", "-p", "#{session_name}")
	output, err := cmd.Output()
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
	return os.WriteFile(lastSessionFile, []byte(name), 0644)
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

// parseIntOrZero attempts to parse an integer, returning 0 on error
func parseIntOrZero(s string) int {
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}
