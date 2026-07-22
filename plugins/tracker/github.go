package tracker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/lost-in-the/grove/internal/cmdexec"
)

// GitHubAdapter implements the Tracker interface using the GitHub CLI.
type GitHubAdapter struct {
	repo string // format: "owner/repo"
}

// NewGitHubAdapter creates a new GitHub adapter.
// If repo is empty, it will be detected from the current repository.
func NewGitHubAdapter(repo string) *GitHubAdapter {
	return &GitHubAdapter{repo: repo}
}

// Name returns the tracker name.
func (g *GitHubAdapter) Name() string {
	return "github"
}

// issueJSONFields is the gh --json field list matching ghIssue. Keep in sync
// with the ghIssue struct below.
const issueJSONFields = "number,title,body,state,author,labels,createdAt,updatedAt,url"

// ghIssue mirrors the JSON fields requested from gh for issues.
type ghIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	URL       string    `json:"url"`
}

// toIssue converts a gh issue response into the tracker Issue type.
func (gh ghIssue) toIssue() *Issue {
	labels := make([]string, len(gh.Labels))
	for i, l := range gh.Labels {
		labels[i] = l.Name
	}

	return &Issue{
		Number:    gh.Number,
		Title:     gh.Title,
		Body:      gh.Body,
		State:     strings.ToLower(gh.State),
		Author:    gh.Author.Login,
		Labels:    labels,
		CreatedAt: gh.CreatedAt,
		UpdatedAt: gh.UpdatedAt,
		URL:       gh.URL,
	}
}

// prJSONFields is the gh --json field list matching ghPR. Keep in sync with
// the ghPR struct below.
const prJSONFields = "number,title,body,state,author,labels,headRefName,baseRefName,isDraft,commits,additions,deletions,reviewDecision,createdAt,updatedAt,url"

// ghPR mirrors the JSON fields requested from gh for pull requests.
type ghPR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	HeadRefName string `json:"headRefName"`
	BaseRefName string `json:"baseRefName"`
	IsDraft     bool   `json:"isDraft"`
	Commits     []struct {
		Oid             string `json:"oid"`
		MessageHeadline string `json:"messageHeadline"`
	} `json:"commits"`
	Additions      int       `json:"additions"`
	Deletions      int       `json:"deletions"`
	ReviewDecision string    `json:"reviewDecision"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	URL            string    `json:"url"`
}

// toPullRequest converts a gh PR response into the tracker PullRequest type,
// flattening labels and truncating commit SHAs to 7 characters.
func (gh ghPR) toPullRequest() *PullRequest {
	labels := make([]string, len(gh.Labels))
	for i, l := range gh.Labels {
		labels[i] = l.Name
	}

	commits := make([]PRCommit, len(gh.Commits))
	for i, c := range gh.Commits {
		sha := c.Oid
		if len(sha) > 7 {
			sha = sha[:7]
		}
		commits[i] = PRCommit{SHA: sha, Message: c.MessageHeadline}
	}

	return &PullRequest{
		Number:         gh.Number,
		Title:          gh.Title,
		Body:           gh.Body,
		State:          strings.ToLower(gh.State),
		Author:         gh.Author.Login,
		Labels:         labels,
		Branch:         gh.HeadRefName,
		BaseBranch:     gh.BaseRefName,
		IsDraft:        gh.IsDraft,
		CommitCount:    len(gh.Commits),
		Commits:        commits,
		Additions:      gh.Additions,
		Deletions:      gh.Deletions,
		ReviewDecision: gh.ReviewDecision,
		CreatedAt:      gh.CreatedAt,
		UpdatedAt:      gh.UpdatedAt,
		URL:            gh.URL,
	}
}

// FetchIssue retrieves an issue by number.
func (g *GitHubAdapter) FetchIssue(number int) (*Issue, error) {
	args := []string{"issue", "view", strconv.Itoa(number), "--json", issueJSONFields}
	if g.repo != "" {
		args = append(args, "--repo", g.repo)
	}

	output, err := g.runGH(args...)
	if err != nil {
		return nil, fmt.Errorf("fetch issue %d: %w", number, err)
	}

	var gh ghIssue
	if err := json.Unmarshal(output, &gh); err != nil {
		return nil, fmt.Errorf("parse issue response: %w", err)
	}

	return gh.toIssue(), nil
}

// FetchPR retrieves a pull request by number.
func (g *GitHubAdapter) FetchPR(number int) (*PullRequest, error) {
	args := []string{"pr", "view", strconv.Itoa(number), "--json", prJSONFields}
	if g.repo != "" {
		args = append(args, "--repo", g.repo)
	}

	output, err := g.runGH(args...)
	if err != nil {
		return nil, fmt.Errorf("fetch pr %d: %w", number, err)
	}

	var gh ghPR
	if err := json.Unmarshal(output, &gh); err != nil {
		return nil, fmt.Errorf("parse pr response: %w", err)
	}

	return gh.toPullRequest(), nil
}

// ListIssues retrieves issues with optional filtering.
func (g *GitHubAdapter) ListIssues(opts ListOptions) ([]*Issue, error) {
	args := []string{"issue", "list", "--json", issueJSONFields}

	if opts.State != "" && opts.State != "all" {
		args = append(args, "--state", opts.State)
	}

	if opts.Assignee != "" {
		args = append(args, "--assignee", opts.Assignee)
	}

	if opts.Author != "" {
		args = append(args, "--author", opts.Author)
	}

	if len(opts.Labels) > 0 {
		args = append(args, "--label", strings.Join(opts.Labels, ","))
	}

	if opts.Limit > 0 {
		args = append(args, "--limit", strconv.Itoa(opts.Limit))
	}

	if g.repo != "" {
		args = append(args, "--repo", g.repo)
	}

	output, err := g.runGH(args...)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	var ghIssues []ghIssue
	if err := json.Unmarshal(output, &ghIssues); err != nil {
		return nil, fmt.Errorf("parse issues response: %w", err)
	}

	issues := make([]*Issue, len(ghIssues))
	for i, gh := range ghIssues {
		issues[i] = gh.toIssue()
	}

	return issues, nil
}

// ListPRs retrieves pull requests with optional filtering.
func (g *GitHubAdapter) ListPRs(opts ListOptions) ([]*PullRequest, error) {
	args := []string{"pr", "list", "--json", prJSONFields}

	if opts.State != "" && opts.State != "all" {
		args = append(args, "--state", opts.State)
	}

	if opts.Assignee != "" {
		args = append(args, "--assignee", opts.Assignee)
	}

	if opts.Author != "" {
		args = append(args, "--author", opts.Author)
	}

	if len(opts.Labels) > 0 {
		args = append(args, "--label", strings.Join(opts.Labels, ","))
	}

	if opts.Limit > 0 {
		args = append(args, "--limit", strconv.Itoa(opts.Limit))
	}

	if g.repo != "" {
		args = append(args, "--repo", g.repo)
	}

	output, err := g.runGH(args...)
	if err != nil {
		return nil, fmt.Errorf("list prs: %w", err)
	}

	var ghPRs []ghPR
	if err := json.Unmarshal(output, &ghPRs); err != nil {
		return nil, fmt.Errorf("parse prs response: %w", err)
	}

	prs := make([]*PullRequest, len(ghPRs))
	for i, gh := range ghPRs {
		prs[i] = gh.toPullRequest()
	}

	return prs, nil
}

// runGH executes a gh CLI command and returns its stdout. Uses Output (not
// CombinedOutput) so gh's stderr notices — e.g. "A new release of gh is
// available" printed on otherwise-successful commands — don't get interleaved
// into the JSON the callers parse (B30). On failure, gh's stderr is still
// available via the ExitError for the diagnostic message.
func (g *GitHubAdapter) runGH(args ...string) ([]byte, error) {
	output, err := cmdexec.Output(context.TODO(), "gh", args, "", cmdexec.GHCLI)
	if err != nil {
		var stderr string
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		if stderr != "" {
			return nil, fmt.Errorf("failed to run gh %s: %w\nOutput: %s", strings.Join(args, " "), err, stderr)
		}
		return nil, fmt.Errorf("failed to run gh %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

// IsGHInstalled checks if the gh CLI is installed and authenticated.
func IsGHInstalled() bool {
	return cmdexec.Run(context.TODO(), "gh", []string{"auth", "status"}, "", cmdexec.GHCLI) == nil
}

// DetectRepo tries to detect the GitHub repository from the current directory.
func DetectRepo() (string, error) {
	output, err := cmdexec.Output(context.TODO(), "gh", []string{"repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"}, "", cmdexec.GHCLI)
	if err != nil {
		return "", fmt.Errorf("detect repo: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetPRForBranch looks up the open PR for a given branch name.
// Returns nil with no error if no PR exists for the branch.
func (g *GitHubAdapter) GetPRForBranch(branch string) (*PullRequest, error) {
	// gh pr view takes the branch as a positional argument — it has no --head
	// flag (that belongs to gh pr list).
	args := []string{"pr", "view", branch, "--json", "number,title,state,url"}
	if g.repo != "" {
		args = append(args, "--repo", g.repo)
	}

	output, err := g.runGH(args...)
	if err != nil {
		// gh exits non-zero when no PR is found — treat as "no PR"
		return nil, nil
	}

	// Only a subset of prJSONFields is requested; the remaining ghPR fields
	// stay at their zero values.
	var gh ghPR
	if err := json.Unmarshal(output, &gh); err != nil {
		return nil, fmt.Errorf("parse pr response: %w", err)
	}

	return gh.toPullRequest(), nil
}

// GetRepoViewURL returns the base HTTPS URL of the GitHub repository.
func GetRepoViewURL(repo string) string {
	return "https://github.com/" + repo
}
