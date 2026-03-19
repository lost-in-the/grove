package commands

import (
	"testing"
)

func TestIssuesCmd(t *testing.T) {
	if issuesCmd == nil {
		t.Fatal("issuesCmd is nil")
	}

	if issuesCmd.Use != "issues" {
		t.Errorf("issuesCmd.Use = %v, want 'issues'", issuesCmd.Use)
	}

	if issuesCmd.Short == "" {
		t.Error("issuesCmd.Short is empty")
	}

	if issuesCmd.RunE == nil {
		t.Error("issuesCmd.RunE is nil")
	}

	// Check flags
	flags := issuesCmd.Flags()
	if !flags.HasFlags() {
		t.Error("issuesCmd should have flags")
	}

	stateFlag := flags.Lookup("state")
	if stateFlag == nil {
		t.Error("issuesCmd missing --state flag")
	}

	labelFlag := flags.Lookup("label")
	if labelFlag == nil {
		t.Error("issuesCmd missing --label flag")
	}

	assigneeFlag := flags.Lookup("assignee")
	if assigneeFlag == nil {
		t.Error("issuesCmd missing --assignee flag")
	}

	authorFlag := flags.Lookup("author")
	if authorFlag == nil {
		t.Error("issuesCmd missing --author flag")
	}

	limitFlag := flags.Lookup("limit")
	if limitFlag == nil {
		t.Error("issuesCmd missing --limit flag")
	}
}

func TestPrsCmd(t *testing.T) {
	if prsCmd == nil {
		t.Fatal("prsCmd is nil")
	}

	if prsCmd.Use != "prs" {
		t.Errorf("prsCmd.Use = %v, want 'prs'", prsCmd.Use)
	}

	if prsCmd.Short == "" {
		t.Error("prsCmd.Short is empty")
	}

	if prsCmd.RunE == nil {
		t.Error("prsCmd.RunE is nil")
	}

	// Check flags
	flags := prsCmd.Flags()
	if !flags.HasFlags() {
		t.Error("prsCmd should have flags")
	}

	stateFlag := flags.Lookup("state")
	if stateFlag == nil {
		t.Error("prsCmd missing --state flag")
	}

	labelFlag := flags.Lookup("label")
	if labelFlag == nil {
		t.Error("prsCmd missing --label flag")
	}

	assigneeFlag := flags.Lookup("assignee")
	if assigneeFlag == nil {
		t.Error("prsCmd missing --assignee flag")
	}

	authorFlag := flags.Lookup("author")
	if authorFlag == nil {
		t.Error("prsCmd missing --author flag")
	}

	limitFlag := flags.Lookup("limit")
	if limitFlag == nil {
		t.Error("prsCmd missing --limit flag")
	}
}

func TestPrsCmd_HasFzfFlag(t *testing.T) {
	flag := prsCmd.Flags().Lookup("fzf")
	if flag == nil {
		t.Fatal("prs command should have --fzf flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--fzf should default to false, got %s", flag.DefValue)
	}
}

func TestIssuesCmd_HasFzfFlag(t *testing.T) {
	flag := issuesCmd.Flags().Lookup("fzf")
	if flag == nil {
		t.Fatal("issues command should have --fzf flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--fzf should default to false, got %s", flag.DefValue)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "long string",
			input:  "this is a very long string",
			maxLen: 10,
			want:   "this is...",
		},
		{
			name:   "truncate at boundary",
			input:  "hello world",
			maxLen: 8,
			want:   "hello...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBrowseCmd(t *testing.T) {
	if browseCmd == nil {
		t.Fatal("browseCmd is nil")
	}

	if browseCmd.Use != "browse" {
		t.Errorf("browseCmd.Use = %v, want 'browse'", browseCmd.Use)
	}

	if browseCmd.Short == "" {
		t.Error("browseCmd.Short is empty")
	}

	if browseCmd.RunE == nil {
		t.Error("browseCmd.RunE is nil")
	}

	// Verify the "b" alias is registered
	aliases := browseCmd.Aliases
	found := false
	for _, a := range aliases {
		if a == "b" {
			found = true
			break
		}
	}
	if !found {
		t.Error("browseCmd should have 'b' alias")
	}
}

func TestBrowseCmd_HasJSONFlag(t *testing.T) {
	flag := browseCmd.Flags().Lookup("json")
	if flag == nil {
		t.Fatal("browse command should have --json flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--json should default to false, got %s", flag.DefValue)
	}
}

func TestParseIssueNumberFromWorktreeName(t *testing.T) {
	tests := []struct {
		name      string
		shortName string
		want      int
	}{
		{
			name:      "issue worktree with slug",
			shortName: "issue-123-fix-auth-bug",
			want:      123,
		},
		{
			name:      "issue worktree without slug",
			shortName: "issue-456",
			want:      456,
		},
		{
			name:      "pr worktree is not an issue",
			shortName: "pr-789-my-feature",
			want:      0,
		},
		{
			name:      "feature branch",
			shortName: "feat-cool-thing",
			want:      0,
		},
		{
			name:      "bare name",
			shortName: "main",
			want:      0,
		},
		{
			name:      "issue prefix but non-numeric",
			shortName: "issue-abc-something",
			want:      0,
		},
		{
			name:      "issue prefix with leading zeros",
			shortName: "issue-007-james-bond",
			want:      7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIssueNumberFromWorktreeName(tt.shortName)
			if got != tt.want {
				t.Errorf("parseIssueNumberFromWorktreeName(%q) = %d, want %d", tt.shortName, got, tt.want)
			}
		})
	}
}
