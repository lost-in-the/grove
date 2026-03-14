// Package tracker provides integration with issue tracking systems.
// It defines the Tracker interface that adapters must implement to
// support fetching issues and pull requests for worktree creation.
package tracker

import (
	"fmt"
	"time"
)

const (
	// maxSlugLength is the maximum length of a slug in worktree names
	maxSlugLength = 40
)

// Issue represents an issue from a tracking system.
type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	Author    string    `json:"author"`
	Labels    []string  `json:"labels"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	URL       string    `json:"url"`
}

// PRCommit represents a single commit in a pull request.
type PRCommit struct {
	SHA     string `json:"sha"`     // short SHA (7 chars)
	Message string `json:"message"` // first line of commit message
}

// PullRequest represents a pull request from a tracking system.
type PullRequest struct {
	Number         int        `json:"number"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	State          string     `json:"state"`
	Author         string     `json:"author"`
	Labels         []string   `json:"labels"`
	Branch         string     `json:"branch"`
	BaseBranch     string     `json:"base_branch"`
	IsDraft        bool       `json:"is_draft"`
	CommitCount    int        `json:"commit_count"`
	Commits        []PRCommit `json:"commits"`
	Additions      int        `json:"additions"`
	Deletions      int        `json:"deletions"`
	ReviewDecision string     `json:"review_decision"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	URL            string     `json:"url"`
}

// ListOptions configures filtering for list operations.
type ListOptions struct {
	State    string   // open, closed, all
	Labels   []string // filter by labels
	Assignee string   // filter by assignee
	Author   string   // filter by author
	Limit    int      // max number of results
}

// Tracker defines the interface for issue tracking system adapters.
type Tracker interface {
	// Name returns the tracker name (e.g., "github", "linear").
	Name() string

	// FetchIssue retrieves an issue by number.
	FetchIssue(number int) (*Issue, error)

	// FetchPR retrieves a pull request by number.
	FetchPR(number int) (*PullRequest, error)

	// ListIssues retrieves issues with optional filtering.
	ListIssues(opts ListOptions) ([]*Issue, error)

	// ListPRs retrieves pull requests with optional filtering.
	ListPRs(opts ListOptions) ([]*PullRequest, error)
}

// registry holds registered tracker adapters.
var registry = make(map[string]Tracker)

// Register registers a tracker adapter by name.
func Register(name string, tracker Tracker) {
	registry[name] = tracker
}

// Get retrieves a registered tracker by name.
func Get(name string) (Tracker, error) {
	t, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("tracker %q not registered", name)
	}
	return t, nil
}

// List returns the names of all registered trackers.
func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// GenerateWorktreeName generates a worktree name from issue/PR metadata.
// Format: {type}-{number}-{slug}
// Example: "issue-123-fix-auth-bug"
func GenerateWorktreeName(issueType string, number int, title string) string {
	// Convert title to slug
	slug := slugify(title)

	// Limit slug length to keep names manageable
	if len(slug) > maxSlugLength {
		slug = slug[:maxSlugLength]
	}

	return fmt.Sprintf("%s-%d-%s", issueType, number, slug)
}

// slugify converts a string to a URL-safe slug.
func slugify(s string) string {
	var result []rune
	prevDash := false

	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			result = append(result, r)
			prevDash = false
		case r >= 'A' && r <= 'Z':
			result = append(result, r+32) // to lowercase
			prevDash = false
		case r >= '0' && r <= '9':
			result = append(result, r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_':
			if !prevDash && len(result) > 0 {
				result = append(result, '-')
				prevDash = true
			}
		}
	}

	// Trim trailing dash
	if len(result) > 0 && result[len(result)-1] == '-' {
		result = result[:len(result)-1]
	}

	return string(result)
}
