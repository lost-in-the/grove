package tmux

import (
	"os"
	"testing"
)

func TestIsInsideTmux(t *testing.T) {
	// Save original env
	originalTmux := os.Getenv("TMUX")
	defer func() {
		if originalTmux != "" {
			os.Setenv("TMUX", originalTmux)
		} else {
			os.Unsetenv("TMUX")
		}
	}()

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
			if tt.tmuxEnv != "" {
				os.Setenv("TMUX", tt.tmuxEnv)
			} else {
				os.Unsetenv("TMUX")
			}

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
				KillSession(tt.session)
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
				KillSession(tt.session)
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
	defer KillSession(testSession)

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
