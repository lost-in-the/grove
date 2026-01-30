package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestHeaderView(t *testing.T) {
	tests := []struct {
		name       string
		header     Header
		width      int
		wantParts  []string
		dontWant   []string
	}{
		{
			name: "shows project name and worktree count",
			header: Header{
				ProjectName:   "acupoll",
				WorktreeCount: 5,
				CurrentBranch: "main",
				CurrentName:   "fix/diff-review",
			},
			width:     80,
			wantParts: []string{"acupoll", "5 worktrees", "main"},
		},
		{
			name: "shows current worktree indicator",
			header: Header{
				ProjectName:   "grove-cli",
				WorktreeCount: 3,
				CurrentBranch: "feat/tui",
				CurrentName:   "feat/tui",
			},
			width:     100,
			wantParts: []string{"grove-cli", "3 worktrees", "feat/tui"},
		},
		{
			name: "singular worktree",
			header: Header{
				ProjectName:   "myproject",
				WorktreeCount: 1,
				CurrentBranch: "main",
				CurrentName:   "main",
			},
			width:     80,
			wantParts: []string{"myproject", "1 worktree"},
			dontWant:  []string{"1 worktrees"},
		},
		{
			name: "zero worktrees",
			header: Header{
				ProjectName:   "empty",
				WorktreeCount: 0,
				CurrentBranch: "",
				CurrentName:   "",
			},
			width:     80,
			wantParts: []string{"empty", "0 worktrees"},
		},
		{
			name: "narrow terminal truncates gracefully",
			header: Header{
				ProjectName:   "acupoll",
				WorktreeCount: 5,
				CurrentBranch: "main",
				CurrentName:   "fix/diff-review-cleanup-long-name",
			},
			width:     40,
			wantParts: []string{"acupoll"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := tt.header.View(tt.width)
			plain := stripAnsi(view)

			for _, part := range tt.wantParts {
				if !strings.Contains(plain, part) {
					t.Errorf("View() missing %q in:\n%s", part, plain)
				}
			}

			for _, part := range tt.dontWant {
				if strings.Contains(plain, part) {
					t.Errorf("View() should not contain %q in:\n%s", part, plain)
				}
			}

			// View width should not exceed requested width
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > tt.width {
					t.Errorf("line exceeds width %d: width=%d line=%q",
						tt.width, lipgloss.Width(line), line)
				}
			}
		})
	}
}

func TestHeaderCurrentIndicator(t *testing.T) {
	h := Header{
		ProjectName:   "proj",
		WorktreeCount: 2,
		CurrentBranch: "main",
		CurrentName:   "feature-x",
	}
	view := stripAnsi(h.View(80))

	// Should contain the dot indicator for current worktree
	if !strings.Contains(view, "●") && !strings.Contains(view, "feature-x") {
		t.Errorf("expected current worktree indicator, got:\n%s", view)
	}
}

// stripAnsi removes ANSI escape codes for plain text comparison.
func stripAnsi(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}
