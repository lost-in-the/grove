package tracker

import (
	"testing"
	"time"
)

// mockTracker implements Tracker interface for testing.
type mockTracker struct {
	name string
}

func (m *mockTracker) Name() string {
	return m.name
}

func (m *mockTracker) FetchIssue(number int) (*Issue, error) {
	return &Issue{
		Number: number,
		Title:  "Test Issue",
	}, nil
}

func (m *mockTracker) FetchPR(number int) (*PullRequest, error) {
	return &PullRequest{
		Number: number,
		Title:  "Test PR",
	}, nil
}

func (m *mockTracker) ListIssues(opts ListOptions) ([]*Issue, error) {
	return []*Issue{
		{Number: 1, Title: "Issue 1"},
		{Number: 2, Title: "Issue 2"},
	}, nil
}

func (m *mockTracker) ListPRs(opts ListOptions) ([]*PullRequest, error) {
	return []*PullRequest{
		{Number: 1, Title: "PR 1"},
		{Number: 2, Title: "PR 2"},
	}, nil
}

func TestRegister(t *testing.T) {
	// Clear registry for clean test
	registry = make(map[string]Tracker)

	tracker := &mockTracker{name: "test"}
	Register("test", tracker)

	got, err := Get("test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.Name() != "test" {
		t.Errorf("Get().Name() = %v, want %v", got.Name(), "test")
	}
}

func TestGet_NotFound(t *testing.T) {
	// Clear registry for clean test
	registry = make(map[string]Tracker)

	_, err := Get("nonexistent")
	if err == nil {
		t.Error("Get() expected error for nonexistent tracker")
	}
}

func TestList(t *testing.T) {
	// Clear registry for clean test
	registry = make(map[string]Tracker)

	Register("github", &mockTracker{name: "github"})
	Register("linear", &mockTracker{name: "linear"})

	names := List()
	if len(names) != 2 {
		t.Errorf("List() len = %v, want 2", len(names))
	}
}

func TestGenerateWorktreeName(t *testing.T) {
	tests := []struct {
		name      string
		issueType string
		number    int
		title     string
		want      string
	}{
		{
			name:      "simple title",
			issueType: "issue",
			number:    123,
			title:     "Fix Auth Bug",
			want:      "issue-123-fix-auth-bug",
		},
		{
			name:      "title with special chars",
			issueType: "pr",
			number:    456,
			title:     "Add User API [WIP]",
			want:      "pr-456-add-user-api-wip",
		},
		{
			name:      "very long title",
			issueType: "issue",
			number:    789,
			title:     "This is a very long issue title that should be truncated to a reasonable length for worktree names",
			want:      "issue-789-this-is-a-very-long-issue-title-that-sho",
		},
		{
			name:      "title with numbers",
			issueType: "pr",
			number:    100,
			title:     "Update dependencies to v2.0",
			want:      "pr-100-update-dependencies-to-v20",
		},
		{
			name:      "title with multiple spaces",
			issueType: "issue",
			number:    200,
			title:     "Fix   multiple   spaces",
			want:      "issue-200-fix-multiple-spaces",
		},
		{
			name:      "title starting with space",
			issueType: "pr",
			number:    300,
			title:     "  Leading spaces",
			want:      "pr-300-leading-spaces",
		},
		{
			name:      "title ending with dash",
			issueType: "issue",
			number:    400,
			title:     "Trailing dash -",
			want:      "issue-400-trailing-dash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateWorktreeName(tt.issueType, tt.number, tt.title)
			if got != tt.want {
				t.Errorf("GenerateWorktreeName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "lowercase letters",
			input: "hello world",
			want:  "hello-world",
		},
		{
			name:  "uppercase letters",
			input: "Hello World",
			want:  "hello-world",
		},
		{
			name:  "numbers",
			input: "test 123",
			want:  "test-123",
		},
		{
			name:  "special characters",
			input: "hello@world#test!",
			want:  "helloworldtest",
		},
		{
			name:  "multiple spaces",
			input: "hello   world",
			want:  "hello-world",
		},
		{
			name:  "leading and trailing spaces",
			input: "  hello world  ",
			want:  "hello-world",
		},
		{
			name:  "already slugified",
			input: "hello-world",
			want:  "hello-world",
		},
		{
			name:  "underscores",
			input: "hello_world_test",
			want:  "hello-world-test",
		},
		{
			name:  "mixed case and symbols",
			input: "Fix [Bug] in User_API v2.0",
			want:  "fix-bug-in-user-api-v20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIssueStruct(t *testing.T) {
	now := time.Now()
	issue := Issue{
		Number:    123,
		Title:     "Test Issue",
		Body:      "Test body",
		State:     "open",
		Author:    "testuser",
		Labels:    []string{"bug", "priority"},
		CreatedAt: now,
		UpdatedAt: now,
		URL:       "https://example.com/issues/123",
	}

	if issue.Number != 123 {
		t.Errorf("Issue.Number = %v, want 123", issue.Number)
	}
	if issue.Title != "Test Issue" {
		t.Errorf("Issue.Title = %v, want Test Issue", issue.Title)
	}
	if issue.Body != "Test body" {
		t.Errorf("Issue.Body = %v, want Test body", issue.Body)
	}
	if issue.State != "open" {
		t.Errorf("Issue.State = %v, want open", issue.State)
	}
	if issue.Author != "testuser" {
		t.Errorf("Issue.Author = %v, want testuser", issue.Author)
	}
	if len(issue.Labels) != 2 {
		t.Errorf("len(Issue.Labels) = %v, want 2", len(issue.Labels))
	}
	if issue.CreatedAt != now {
		t.Errorf("Issue.CreatedAt = %v, want %v", issue.CreatedAt, now)
	}
	if issue.UpdatedAt != now {
		t.Errorf("Issue.UpdatedAt = %v, want %v", issue.UpdatedAt, now)
	}
	if issue.URL != "https://example.com/issues/123" {
		t.Errorf("Issue.URL = %v, want https://example.com/issues/123", issue.URL)
	}
}

func TestPullRequestStruct(t *testing.T) {
	now := time.Now()
	pr := PullRequest{
		Number:     456,
		Title:      "Test PR",
		Body:       "Test body",
		State:      "open",
		Author:     "testuser",
		Labels:     []string{"enhancement"},
		Branch:     "feature-branch",
		BaseBranch: "main",
		CreatedAt:  now,
		UpdatedAt:  now,
		URL:        "https://example.com/pulls/456",
	}

	if pr.Number != 456 {
		t.Errorf("PullRequest.Number = %v, want 456", pr.Number)
	}
	if pr.Title != "Test PR" {
		t.Errorf("PullRequest.Title = %v, want Test PR", pr.Title)
	}
	if pr.Body != "Test body" {
		t.Errorf("PullRequest.Body = %v, want Test body", pr.Body)
	}
	if pr.State != "open" {
		t.Errorf("PullRequest.State = %v, want open", pr.State)
	}
	if pr.Author != "testuser" {
		t.Errorf("PullRequest.Author = %v, want testuser", pr.Author)
	}
	if len(pr.Labels) != 1 {
		t.Errorf("len(PullRequest.Labels) = %v, want 1", len(pr.Labels))
	}
	if pr.Branch != "feature-branch" {
		t.Errorf("PullRequest.Branch = %v, want feature-branch", pr.Branch)
	}
	if pr.BaseBranch != "main" {
		t.Errorf("PullRequest.BaseBranch = %v, want main", pr.BaseBranch)
	}
	if pr.CreatedAt != now {
		t.Errorf("PullRequest.CreatedAt = %v, want %v", pr.CreatedAt, now)
	}
	if pr.UpdatedAt != now {
		t.Errorf("PullRequest.UpdatedAt = %v, want %v", pr.UpdatedAt, now)
	}
	if pr.URL != "https://example.com/pulls/456" {
		t.Errorf("PullRequest.URL = %v, want https://example.com/pulls/456", pr.URL)
	}
}

func TestPullRequestEnhancedFields(t *testing.T) {
	tests := []struct {
		name string
		pr   PullRequest
	}{
		{
			name: "draft PR with review data",
			pr: PullRequest{
				Number:         1,
				Title:          "Draft PR",
				IsDraft:        true,
				CommitCount:    5,
				Additions:      234,
				Deletions:      89,
				ReviewDecision: "",
			},
		},
		{
			name: "approved PR with changes",
			pr: PullRequest{
				Number:         2,
				Title:          "Approved PR",
				IsDraft:        false,
				CommitCount:    12,
				Additions:      1203,
				Deletions:      445,
				ReviewDecision: "APPROVED",
			},
		},
		{
			name: "PR with changes requested",
			pr: PullRequest{
				Number:         3,
				Title:          "Needs work",
				IsDraft:        false,
				CommitCount:    1,
				Additions:      5,
				Deletions:      5,
				ReviewDecision: "CHANGES_REQUESTED",
			},
		},
		{
			name: "PR with zero stats",
			pr: PullRequest{
				Number:         4,
				Title:          "Empty PR",
				IsDraft:        false,
				CommitCount:    0,
				Additions:      0,
				Deletions:      0,
				ReviewDecision: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify fields are accessible and hold correct values
			if tt.pr.IsDraft != (tt.name == "draft PR with review data") {
				t.Errorf("IsDraft = %v, unexpected for %s", tt.pr.IsDraft, tt.name)
			}
			if tt.pr.CommitCount < 0 {
				t.Errorf("CommitCount = %v, should be non-negative", tt.pr.CommitCount)
			}
			if tt.pr.Additions < 0 {
				t.Errorf("Additions = %v, should be non-negative", tt.pr.Additions)
			}
			if tt.pr.Deletions < 0 {
				t.Errorf("Deletions = %v, should be non-negative", tt.pr.Deletions)
			}
		})
	}
}

func TestPullRequestReviewDecisionValues(t *testing.T) {
	validDecisions := []string{"", "APPROVED", "CHANGES_REQUESTED", "REVIEW_REQUIRED"}
	for _, decision := range validDecisions {
		pr := PullRequest{ReviewDecision: decision}
		if pr.ReviewDecision != decision {
			t.Errorf("ReviewDecision = %v, want %v", pr.ReviewDecision, decision)
		}
	}
}

func TestListOptions(t *testing.T) {
	opts := ListOptions{
		State:    "open",
		Labels:   []string{"bug", "critical"},
		Assignee: "user1",
		Author:   "user2",
		Limit:    50,
	}

	if opts.State != "open" {
		t.Errorf("ListOptions.State = %v, want open", opts.State)
	}
	if len(opts.Labels) != 2 {
		t.Errorf("len(ListOptions.Labels) = %v, want 2", len(opts.Labels))
	}
	if opts.Assignee != "user1" {
		t.Errorf("ListOptions.Assignee = %v, want user1", opts.Assignee)
	}
	if opts.Author != "user2" {
		t.Errorf("ListOptions.Author = %v, want user2", opts.Author)
	}
	if opts.Limit != 50 {
		t.Errorf("ListOptions.Limit = %v, want 50", opts.Limit)
	}
}
