package commands

import "testing"

func TestShortSHA(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"f012", "f012"},        // 4-char abbreviation — must not panic
		{"50832", "50832"},      // 5-char (the reproduced graft --pick panic)
		{"abcdef7", "abcdef7"},  // exactly 7
		{"abcdef78", "abcdef7"}, // 8 → truncated
		{"0123456789abcdef0123456789abcdef01234567", "0123456"}, // full SHA
	}
	for _, tt := range tests {
		if got := shortSHA(tt.in); got != tt.want {
			t.Errorf("shortSHA(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
