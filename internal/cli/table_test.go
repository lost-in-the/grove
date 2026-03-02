package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestTable_Basic(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)

	tbl := NewTable(w,
		Column{Title: "NAME", MinWidth: 10},
		Column{Title: "STATUS", MinWidth: 8},
	)
	tbl.AddRow("main", "clean")
	tbl.AddRow("feature", "dirty")
	tbl.Render()

	got := buf.String()
	if !strings.Contains(got, "NAME") {
		t.Error("missing NAME header")
	}
	if !strings.Contains(got, "STATUS") {
		t.Error("missing STATUS header")
	}
	if !strings.Contains(got, "main") {
		t.Error("missing 'main' row")
	}
	if !strings.Contains(got, "feature") {
		t.Error("missing 'feature' row")
	}
	if !strings.Contains(got, "─") {
		t.Error("missing separator")
	}
}

func TestTable_Empty(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)

	tbl := NewTable(w, Column{Title: "NAME"})
	tbl.Render()

	if buf.Len() != 0 {
		t.Errorf("expected empty output for empty table, got %q", buf.String())
	}
}

func TestTable_MaxWidth(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)

	tbl := NewTable(w,
		Column{Title: "NAME", MaxWidth: 5},
	)
	tbl.AddRow("very-long-name")
	tbl.Render()

	got := buf.String()
	// The 14-char value must be truncated to fit MaxWidth 5, producing "very…"
	if !strings.Contains(got, "…") {
		t.Errorf("expected truncation with ellipsis '…', got %q", got)
	}
}

func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{
			name:     "no truncation needed",
			input:    "abc",
			maxWidth: 10,
			want:     "abc…", // always appends ellipsis when called
		},
		{
			name:     "truncates long string",
			input:    "abcdefghij",
			maxWidth: 5,
			want:     "abcd…",
		},
		{
			name:     "maxWidth 1 returns just ellipsis",
			input:    "abcdef",
			maxWidth: 1,
			want:     "…",
		},
		{
			name:     "maxWidth 0 returns just ellipsis",
			input:    "abcdef",
			maxWidth: 0,
			want:     "…",
		},
		{
			name:     "empty string",
			input:    "",
			maxWidth: 5,
			want:     "…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateToWidth(tt.input, tt.maxWidth)
			if got != tt.want {
				t.Errorf("truncateToWidth(%q, %d) = %q, want %q", tt.input, tt.maxWidth, got, tt.want)
			}
		})
	}
}

func TestTable_AddRow_PadsToColumnCount(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)

	tbl := NewTable(w,
		Column{Title: "A", MinWidth: 3},
		Column{Title: "B", MinWidth: 3},
		Column{Title: "C", MinWidth: 3},
	)
	// Only provide 1 value — should pad remaining columns
	tbl.AddRow("x")
	tbl.Render()

	got := buf.String()
	if !strings.Contains(got, "x") {
		t.Error("missing value in output")
	}
	// Should have 3 columns in header
	if !strings.Contains(got, "A") || !strings.Contains(got, "B") || !strings.Contains(got, "C") {
		t.Errorf("missing column headers in %q", got)
	}
}

func TestTable_MultipleColumns(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)

	tbl := NewTable(w,
		Column{Title: "NAME", MinWidth: 8},
		Column{Title: "BRANCH", MinWidth: 8},
		Column{Title: "STATUS", MinWidth: 6},
	)
	tbl.AddRow("main", "main", "clean")
	tbl.AddRow("testing", "feat/x", "dirty")
	tbl.Render()

	got := buf.String()
	lines := strings.Split(got, "\n")
	// header + separator + 2 data rows + trailing newline = 5
	if len(lines) < 4 {
		t.Errorf("expected at least 4 lines, got %d: %q", len(lines), got)
	}
}
