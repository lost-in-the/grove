package commands

import "testing"

func TestUncommittedNotice(t *testing.T) {
	tests := []struct {
		files int
		want  string
	}{
		{files: 3, want: "feature has 3 uncommitted files"},
		{files: 1, want: "feature has 1 uncommitted file"},
		{files: 0, want: "feature has uncommitted changes"},
	}
	for _, tt := range tests {
		if got := uncommittedNotice("feature", tt.files); got != tt.want {
			t.Errorf("uncommittedNotice(feature, %d) = %q, want %q", tt.files, got, tt.want)
		}
	}
}
