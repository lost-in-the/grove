package tracker

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
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

// FetchIssue retrieves an issue by number.
func (g *GitHubAdapter) FetchIssue(number int) (*Issue, error) {
	args := []string{"issue", "view", strconv.Itoa(number), "--json", "number,title,body,state,author,labels,createdAt,updatedAt,url"}
	if g.repo != "" {
		args = append(args, "--repo", g.repo)
	}

	output, err := g.runGH(args...)
	if err != nil {
		return nil, fmt.Errorf("fetch issue %d: %w", number, err)
	}

	var ghIssue struct {
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

	if err := json.Unmarshal(output, &ghIssue); err != nil {
		return nil, fmt.Errorf("parse issue response: %w", err)
	}

	labels := make([]string, len(ghIssue.Labels))
	for i, l := range ghIssue.Labels {
		labels[i] = l.Name
	}

	return &Issue{
		Number:    ghIssue.Number,
		Title:     ghIssue.Title,
		Body:      ghIssue.Body,
		State:     strings.ToLower(ghIssue.State),
		Author:    ghIssue.Author.Login,
		Labels:    labels,
		CreatedAt: ghIssue.CreatedAt,
		UpdatedAt: ghIssue.UpdatedAt,
		URL:       ghIssue.URL,
	}, nil
}

// FetchPR retrieves a pull request by number.
func (g *GitHubAdapter) FetchPR(number int) (*PullRequest, error) {
	args := []string{"pr", "view", strconv.Itoa(number), "--json", "number,title,body,state,author,labels,headRefName,baseRefName,isDraft,commits,additions,deletions,reviewDecision,createdAt,updatedAt,url"}
	if g.repo != "" {
		args = append(args, "--repo", g.repo)
	}

	output, err := g.runGH(args...)
	if err != nil {
		return nil, fmt.Errorf("fetch pr %d: %w", number, err)
	}

	var ghPR struct {
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
		HeadRefName    string `json:"headRefName"`
		BaseRefName    string `json:"baseRefName"`
		IsDraft        bool   `json:"isDraft"`
		Commits        []struct {
			Oid string `json:"oid"`
		} `json:"commits"`
		Additions      int       `json:"additions"`
		Deletions      int       `json:"deletions"`
		ReviewDecision string    `json:"reviewDecision"`
		CreatedAt      time.Time `json:"createdAt"`
		UpdatedAt      time.Time `json:"updatedAt"`
		URL            string    `json:"url"`
	}

	if err := json.Unmarshal(output, &ghPR); err != nil {
		return nil, fmt.Errorf("parse pr response: %w", err)
	}

	labels := make([]string, len(ghPR.Labels))
	for i, l := range ghPR.Labels {
		labels[i] = l.Name
	}

	return &PullRequest{
		Number:         ghPR.Number,
		Title:          ghPR.Title,
		Body:           ghPR.Body,
		State:          strings.ToLower(ghPR.State),
		Author:         ghPR.Author.Login,
		Labels:         labels,
		Branch:         ghPR.HeadRefName,
		BaseBranch:     ghPR.BaseRefName,
		IsDraft:        ghPR.IsDraft,
		CommitCount:    len(ghPR.Commits),
		Additions:      ghPR.Additions,
		Deletions:      ghPR.Deletions,
		ReviewDecision: ghPR.ReviewDecision,
		CreatedAt:      ghPR.CreatedAt,
		UpdatedAt:      ghPR.UpdatedAt,
		URL:            ghPR.URL,
	}, nil
}

// ListIssues retrieves issues with optional filtering.
func (g *GitHubAdapter) ListIssues(opts ListOptions) ([]*Issue, error) {
	args := []string{"issue", "list", "--json", "number,title,state,author,labels,createdAt,updatedAt,url"}

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

	var ghIssues []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
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

	if err := json.Unmarshal(output, &ghIssues); err != nil {
		return nil, fmt.Errorf("parse issues response: %w", err)
	}

	issues := make([]*Issue, len(ghIssues))
	for i, gh := range ghIssues {
		labels := make([]string, len(gh.Labels))
		for j, l := range gh.Labels {
			labels[j] = l.Name
		}

		issues[i] = &Issue{
			Number:    gh.Number,
			Title:     gh.Title,
			State:     strings.ToLower(gh.State),
			Author:    gh.Author.Login,
			Labels:    labels,
			CreatedAt: gh.CreatedAt,
			UpdatedAt: gh.UpdatedAt,
			URL:       gh.URL,
		}
	}

	return issues, nil
}

// ListPRs retrieves pull requests with optional filtering.
func (g *GitHubAdapter) ListPRs(opts ListOptions) ([]*PullRequest, error) {
	args := []string{"pr", "list", "--json", "number,title,body,state,author,labels,headRefName,baseRefName,isDraft,commits,additions,deletions,reviewDecision,createdAt,updatedAt,url"}

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

	var ghPRs []struct {
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
		HeadRefName    string `json:"headRefName"`
		BaseRefName    string `json:"baseRefName"`
		IsDraft        bool   `json:"isDraft"`
		Commits        []struct {
			Oid string `json:"oid"`
		} `json:"commits"`
		Additions      int       `json:"additions"`
		Deletions      int       `json:"deletions"`
		ReviewDecision string    `json:"reviewDecision"`
		CreatedAt      time.Time `json:"createdAt"`
		UpdatedAt      time.Time `json:"updatedAt"`
		URL            string    `json:"url"`
	}

	if err := json.Unmarshal(output, &ghPRs); err != nil {
		return nil, fmt.Errorf("parse prs response: %w", err)
	}

	prs := make([]*PullRequest, len(ghPRs))
	for i, gh := range ghPRs {
		labels := make([]string, len(gh.Labels))
		for j, l := range gh.Labels {
			labels[j] = l.Name
		}

		prs[i] = &PullRequest{
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
			Additions:      gh.Additions,
			Deletions:      gh.Deletions,
			ReviewDecision: gh.ReviewDecision,
			CreatedAt:      gh.CreatedAt,
			UpdatedAt:      gh.UpdatedAt,
			URL:            gh.URL,
		}
	}

	return prs, nil
}

// runGH executes a gh CLI command and returns the output.
func (g *GitHubAdapter) runGH(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" {
			return nil, fmt.Errorf("failed to run gh %s: %w\nOutput: %s", strings.Join(args, " "), err, outputStr)
		}
		return nil, fmt.Errorf("failed to run gh %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

// IsGHInstalled checks if the gh CLI is installed and authenticated.
func IsGHInstalled() bool {
	cmd := exec.Command("gh", "auth", "status")
	return cmd.Run() == nil
}

// DetectRepo tries to detect the GitHub repository from the current directory.
func DetectRepo() (string, error) {
	cmd := exec.Command("gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("detect repo: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
