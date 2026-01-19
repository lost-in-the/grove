package notify

import (
	"testing"
)

func TestSend(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		message string
		wantErr bool
	}{
		{
			name:    "valid notification",
			title:   "Test Title",
			message: "Test message",
			wantErr: false,
		},
		{
			name:    "empty title",
			title:   "",
			message: "Test message",
			wantErr: true,
		},
		{
			name:    "title with quotes",
			title:   `Title with "quotes"`,
			message: "Message",
			wantErr: false,
		},
		{
			name:    "message with backslashes",
			title:   "Title",
			message: `Message with \backslashes\`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Send(tt.title, tt.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsAvailable(t *testing.T) {
	// Should always return a boolean without error
	available := IsAvailable()

	// Just verify it returns something (can be true or false depending on platform)
	_ = available
}

func TestEscapeAppleScript(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no special characters",
			input: "Hello World",
			want:  "Hello World",
		},
		{
			name:  "quotes",
			input: `Message with "quotes"`,
			want:  `Message with \"quotes\"`,
		},
		{
			name:  "backslashes",
			input: `Path\to\file`,
			want:  `Path\\to\\file`,
		},
		{
			name:  "mixed special characters",
			input: `"Path\to\file"`,
			want:  `\"Path\\to\\file\"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeAppleScript(tt.input)
			if got != tt.want {
				t.Errorf("escapeAppleScript() = %q, want %q", got, tt.want)
			}
		})
	}
}
