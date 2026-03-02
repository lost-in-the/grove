package tmux

import (
	"testing"
)

func TestIsInsideTmux(t *testing.T) {
	tests := []struct {
		name     string
		tmuxEnv  string
		expected bool
	}{
		{
			name:     "inside tmux",
			tmuxEnv:  "/tmp/tmux-1000/default,12345,0",
			expected: true,
		},
		{
			name:     "not inside tmux",
			tmuxEnv:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TMUX", tt.tmuxEnv)

			result := IsInsideTmux()
			if result != tt.expected {
				t.Errorf("IsInsideTmux() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSessionCreate(t *testing.T) {
	// Skip if tmux is not available
	if !IsTmuxAvailable() {
		t.Skip("tmux not available")
	}

	tests := []struct {
		name    string
		session string
		path    string
		wantErr bool
	}{
		{
			name:    "create session",
			session: "test-session-create",
			path:    "/tmp",
			wantErr: false,
		},
		{
			name:    "empty name",
			session: "",
			path:    "/tmp",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing session
			if tt.session != "" {
				_ = KillSession(tt.session)
			}

			err := CreateSession(tt.session, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSession() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify session exists
				exists, err := SessionExists(tt.session)
				if err != nil {
					t.Errorf("SessionExists() error = %v", err)
				}
				if !exists {
					t.Errorf("Session %s does not exist after creation", tt.session)
				}

				// Clean up
				_ = KillSession(tt.session)
			}
		})
	}
}

func TestListSessions(t *testing.T) {
	// Skip if tmux is not available
	if !IsTmuxAvailable() {
		t.Skip("tmux not available")
	}

	// Create a test session
	testSession := "test-session-list"
	if err := CreateSession(testSession, "/tmp"); err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}
	defer func() { _ = KillSession(testSession) }()

	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	// Should have at least our test session
	found := false
	for _, s := range sessions {
		if s.Name == testSession {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Test session %s not found in list", testSession)
	}
}

func TestGetLastSession(t *testing.T) {
	// Test the last session storage
	testSession := "test-last-session"

	err := StoreLastSession(testSession)
	if err != nil {
		t.Fatalf("StoreLastSession() error = %v", err)
	}

	last, err := GetLastSession()
	if err != nil {
		t.Fatalf("GetLastSession() error = %v", err)
	}

	if last != testSession {
		t.Errorf("GetLastSession() = %s, want %s", last, testSession)
	}
}

func TestCreateSessionWithCommand(t *testing.T) {
	if !IsTmuxAvailable() {
		t.Skip("tmux not available")
	}

	tests := []struct {
		name    string
		session string
		path    string
		command string
		wantErr bool
	}{
		{
			name:    "create session with no command (default shell)",
			session: "test-session-cmd-default",
			path:    "/tmp",
			command: "",
			wantErr: false,
		},
		{
			name:    "create session with command",
			session: "test-session-cmd-custom",
			path:    "/tmp",
			command: "sleep 60",
			wantErr: false,
		},
		{
			name:    "empty name",
			session: "",
			path:    "/tmp",
			command: "echo hello",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.session != "" {
				_ = KillSession(tt.session)
			}

			err := CreateSessionWithCommand(tt.session, tt.path, tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSessionWithCommand() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				exists, err := SessionExists(tt.session)
				if err != nil {
					t.Errorf("SessionExists() error = %v", err)
				}
				if !exists {
					t.Errorf("Session %s does not exist after creation", tt.session)
				}

				_ = KillSession(tt.session)
			}
		})
	}
}

func TestCreateSessionWithCommand_AlreadyExists(t *testing.T) {
	if !IsTmuxAvailable() {
		t.Skip("tmux not available")
	}

	session := "test-session-cmd-dup"
	_ = KillSession(session)

	if err := CreateSessionWithCommand(session, "/tmp", ""); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	defer func() { _ = KillSession(session) }()

	// Idempotent: second create should succeed (no-op)
	if err := CreateSessionWithCommand(session, "/tmp", ""); err != nil {
		t.Errorf("expected idempotent create to succeed, got: %v", err)
	}
}

func TestIsCommandRunning(t *testing.T) {
	if !IsTmuxAvailable() {
		t.Skip("tmux not available")
	}

	session := "test-session-cmd-running"
	_ = KillSession(session)

	if err := CreateSession(session, "/tmp"); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	defer func() { _ = KillSession(session) }()

	// Session should be running a shell by default
	shellCommands := []string{"bash", "zsh", "fish", "sh"}
	runningAny := false
	for _, sh := range shellCommands {
		if IsCommandRunning(session, sh) {
			runningAny = true
			break
		}
	}

	if !runningAny {
		t.Error("expected session to be running a shell command")
	}

	// Should not be running a non-existent command
	if IsCommandRunning(session, "definitely-not-a-real-command") {
		t.Error("expected IsCommandRunning to return false for non-existent command")
	}
}

func TestDisplayPopup_NotInsideTmux(t *testing.T) {
	if !IsTmuxAvailable() {
		t.Skip("tmux not available")
	}

	// Ensure TMUX env is empty so IsInsideTmux() returns false
	t.Setenv("TMUX", "")

	err := DisplayPopup("any-session", "80%", "80%")
	if err == nil {
		t.Error("expected error when not inside tmux")
	}
}

func TestParseIntOrZero(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "valid integer", input: "42", want: 42},
		{name: "zero", input: "0", want: 0},
		{name: "invalid string", input: "abc", want: 0},
		{name: "empty string", input: "", want: 0},
		{name: "negative", input: "-5", want: -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIntOrZero(tt.input)
			if got != tt.want {
				t.Errorf("parseIntOrZero(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestPaneInfoIsShell(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{name: "bash", command: "bash", want: true},
		{name: "zsh", command: "zsh", want: true},
		{name: "fish", command: "fish", want: true},
		{name: "sh", command: "sh", want: true},
		{name: "vim", command: "vim", want: false},
		{name: "node", command: "node", want: false},
		{name: "python", command: "python", want: false},
		{name: "empty", command: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PaneInfo{CurrentCommand: tt.command}
			got := p.IsShell()
			if got != tt.want {
				t.Errorf("PaneInfo{CurrentCommand: %q}.IsShell() = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestKillSession_EmptyName(t *testing.T) {
	if !IsTmuxAvailable() {
		t.Skip("tmux not available")
	}

	err := KillSession("")
	if err == nil {
		t.Error("KillSession(\"\") expected error, got nil")
	}
}

func TestKillSession_NonExistent(t *testing.T) {
	if !IsTmuxAvailable() {
		t.Skip("tmux not available")
	}

	err := KillSession("test-grove-nonexistent-xyz")
	if err == nil {
		t.Error("KillSession(nonexistent) expected error, got nil")
	}
}

func TestSessionExists(t *testing.T) {
	if !IsTmuxAvailable() {
		t.Skip("tmux not available")
	}

	sessionName := "test-grove-exists-001"
	_ = KillSession(sessionName)
	if err := CreateSession(sessionName, "/tmp"); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	defer func() { _ = KillSession(sessionName) }()

	tests := []struct {
		name    string
		session string
		want    bool
	}{
		{name: "existing session", session: sessionName, want: true},
		{name: "non-existent session", session: "test-grove-no-such-session", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SessionExists(tt.session)
			if err != nil {
				t.Errorf("SessionExists(%q) error = %v", tt.session, err)
				return
			}
			if got != tt.want {
				t.Errorf("SessionExists(%q) = %v, want %v", tt.session, got, tt.want)
			}
		})
	}
}

func TestGetSessionStatus(t *testing.T) {
	if !IsTmuxAvailable() {
		t.Skip("tmux not available")
	}

	sessionName := "test-grove-status-001"
	_ = KillSession(sessionName)
	if err := CreateSession(sessionName, "/tmp"); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	defer func() { _ = KillSession(sessionName) }()

	status := GetSessionStatus(sessionName)
	if status != "detached" {
		t.Errorf("GetSessionStatus(%q) = %q, want %q", sessionName, status, "detached")
	}

	status = GetSessionStatus("test-grove-no-such-session")
	if status != "none" {
		t.Errorf("GetSessionStatus(nonexistent) = %q, want %q", status, "none")
	}
}

func TestAttachSession_EmptyName(t *testing.T) {
	if !IsTmuxAvailable() {
		t.Skip("tmux not available")
	}

	err := AttachSession("")
	if err == nil {
		t.Error("AttachSession(\"\") expected error, got nil")
	}
}

func TestAttachSession_NonExistent(t *testing.T) {
	if !IsTmuxAvailable() {
		t.Skip("tmux not available")
	}

	err := AttachSession("test-grove-no-such-session")
	if err == nil {
		t.Error("AttachSession(nonexistent) expected error, got nil")
	}
}

func TestSwitchSession_NotInTmux(t *testing.T) {
	t.Setenv("TMUX", "")

	err := SwitchSession("any-session")
	if err == nil {
		t.Error("SwitchSession() expected error when not in tmux, got nil")
	}
}

func TestSwitchSession_EmptyName(t *testing.T) {
	if !IsTmuxAvailable() {
		t.Skip("tmux not available")
	}

	err := SwitchSession("")
	if err == nil {
		t.Error("SwitchSession(\"\") expected error, got nil")
	}
}

func TestGetCurrentSession_NotInTmux(t *testing.T) {
	t.Setenv("TMUX", "")

	_, err := GetCurrentSession()
	if err == nil {
		t.Error("GetCurrentSession() expected error when not in tmux, got nil")
	}
}
