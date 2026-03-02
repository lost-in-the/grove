package tui

import "testing"

func TestValidateWorktreeName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"", ""},
		{"valid-name", ""},
		{"also_valid", ""},
		{"has space", "name contains invalid characters"},
		{"has/slash", "name contains invalid characters"},
		{"-starts-dash", "name cannot start with - or ."},
		{".starts-dot", "name cannot start with - or ."},
		{"has:colon", "name contains invalid characters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateWorktreeName(tt.name)
			if got != tt.want {
				t.Errorf("ValidateWorktreeName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		max  int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"hello", 1, "h"},
		{"hello", 0, ""},
		{"hello", -1, ""},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.s, tt.max)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
		}
	}
}

func TestCompactAge(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"5 minutes ago", "5m ago"},
		{"1 minute ago", "1m ago"},
		{"2 hours ago", "2h ago"},
		{"1 hour ago", "1h ago"},
		{"3 days ago", "3d ago"},
		{"1 day ago", "1d ago"},
		{"2 weeks ago", "2w ago"},
		{"6 months ago", "6mo ago"},
		{"1 year ago", "1y ago"},
		{"just now", "just now"},
	}
	for _, tt := range tests {
		got := compactAge(tt.input)
		if got != tt.want {
			t.Errorf("compactAge(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFilterItems(t *testing.T) {
	items := []WorktreeItem{
		{ShortName: "main", Branch: "main"},
		{ShortName: "feature-auth", Branch: "feature/auth"},
		{ShortName: "fix-bug", Branch: "fix/bug-123"},
	}

	t.Run("empty query returns all", func(t *testing.T) {
		result := filterItems(items, "")
		if len(result) != 3 {
			t.Errorf("expected 3, got %d", len(result))
		}
	})

	t.Run("matches short name", func(t *testing.T) {
		result := filterItems(items, "auth")
		if len(result) != 1 {
			t.Errorf("expected 1, got %d", len(result))
		}
	})

	t.Run("matches branch", func(t *testing.T) {
		result := filterItems(items, "bug-123")
		if len(result) != 1 {
			t.Errorf("expected 1, got %d", len(result))
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		result := filterItems(items, "MAIN")
		if len(result) != 1 {
			t.Errorf("expected 1, got %d", len(result))
		}
	})

	t.Run("no matches", func(t *testing.T) {
		result := filterItems(items, "nonexistent")
		if len(result) != 0 {
			t.Errorf("expected 0, got %d", len(result))
		}
	})
}

func TestFilteredBranches(t *testing.T) {
	branches := []string{"main", "feature/auth", "fix/bug-123"}

	t.Run("empty filter returns all", func(t *testing.T) {
		result := filteredBranches(branches, "")
		if len(result) != 3 {
			t.Errorf("expected 3, got %d", len(result))
		}
	})

	t.Run("filters by substring", func(t *testing.T) {
		result := filteredBranches(branches, "fix")
		if len(result) != 1 {
			t.Errorf("expected 1, got %d", len(result))
		}
	})
}

func TestScrollWindow(t *testing.T) {
	tests := []struct {
		name       string
		total      int
		cursor     int
		maxVisible int
		wantStart  int
		wantEnd    int
	}{
		{"empty list", 0, 0, 10, 0, 0},
		{"cursor at start", 5, 0, 10, 0, 5},
		{"cursor in middle, fits", 5, 2, 10, 0, 5},
		{"cursor at end, fits", 5, 4, 10, 0, 5},
		{"cursor scrolls window", 15, 12, 10, 3, 13},
		{"cursor at end of long list", 20, 19, 10, 10, 20},
		{"cursor at max boundary", 15, 9, 10, 0, 10},
		{"cursor just past max", 15, 10, 10, 1, 11},
		{"max zero", 5, 0, 0, 0, 0},
		{"single item", 1, 0, 10, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := scrollWindow(tt.total, tt.cursor, tt.maxVisible)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("scrollWindow(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.total, tt.cursor, tt.maxVisible, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hi", 5, "hi   "},
		{"hello", 5, "hello"},
		{"toolong", 3, "toolong"},
	}
	for _, tt := range tests {
		got := padRight(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("padRight(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}
