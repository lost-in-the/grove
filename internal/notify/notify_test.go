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
