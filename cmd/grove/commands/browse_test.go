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
