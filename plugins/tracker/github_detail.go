package tracker

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lost-in-the/grove/internal/cmdexec"
)

// prDetailFields is the gh --json field list matching ghPRDetail.
const prDetailFields = "number,updatedAt,headRefOid,comments,reviews,statusCheckRollup,commits,files"

type ghLogin struct {
	Login string `json:"login"`
}

// ghPRDetail mirrors the JSON gh returns for prDetailFields.
type ghPRDetail struct {
	Number     int       `json:"number"`
	UpdatedAt  time.Time `json:"updatedAt"`
	HeadRefOid string    `json:"headRefOid"`
	Comments   []struct {
		Author    ghLogin   `json:"author"`
		Body      string    `json:"body"`
		CreatedAt time.Time `json:"createdAt"`
	} `json:"comments"`
	Reviews []struct {
		Author      ghLogin   `json:"author"`
		State       string    `json:"state"`
		Body        string    `json:"body"`
		SubmittedAt time.Time `json:"submittedAt"`
	} `json:"reviews"`
	// statusCheckRollup mixes CheckRun and StatusContext entries.
	StatusCheckRollup []struct {
		Typename   string `json:"__typename"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		DetailsURL string `json:"detailsUrl"`
		Context    string `json:"context"`
		State      string `json:"state"`
		TargetURL  string `json:"targetUrl"`
	} `json:"statusCheckRollup"`
	Commits []struct {
		Oid             string `json:"oid"`
		MessageHeadline string `json:"messageHeadline"`
	} `json:"commits"`
	Files []struct {
		Path      string `json:"path"`
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
	} `json:"files"`
}

// parsePRDetail converts gh pr view --json prDetailFields output.
func parsePRDetail(data []byte) (*PRDetail, error) {
	var gh ghPRDetail
	if err := json.Unmarshal(data, &gh); err != nil {
		return nil, fmt.Errorf("failed to parse PR detail: %w", err)
	}
	d := &PRDetail{Number: gh.Number, UpdatedAt: gh.UpdatedAt, HeadSHA: gh.HeadRefOid}
	for _, c := range gh.Comments {
		d.Comments = append(d.Comments, PRComment{Author: c.Author.Login, Body: c.Body, CreatedAt: c.CreatedAt})
	}
	for _, r := range gh.Reviews {
		d.Reviews = append(d.Reviews, PRReview{Author: r.Author.Login, State: r.State, Body: r.Body, SubmittedAt: r.SubmittedAt})
	}
	for _, c := range gh.StatusCheckRollup {
		check := PRCheck{Name: c.Name, State: c.Status, URL: c.DetailsURL}
		if c.Typename == "StatusContext" {
			check = PRCheck{Name: c.Context, State: c.State, URL: c.TargetURL}
		} else if c.Status == "COMPLETED" {
			check.State = c.Conclusion
		}
		check.State = strings.ToUpper(check.State)
		d.Checks = append(d.Checks, check)
	}
	for _, c := range gh.Commits {
		sha := c.Oid
		if len(sha) > 7 {
			sha = sha[:7]
		}
		d.Commits = append(d.Commits, PRCommit{SHA: sha, Message: c.MessageHeadline})
	}
	for _, f := range gh.Files {
		d.Files = append(d.Files, PRFile{Path: f.Path, Additions: f.Additions, Deletions: f.Deletions})
	}
	return d, nil
}

// ghReviewComment mirrors one entry of the REST pull request comments list.
// Nullable numbers decode to 0.
type ghReviewComment struct {
	ID           int64     `json:"id"`
	InReplyToID  int64     `json:"in_reply_to_id"`
	Path         string    `json:"path"`
	Line         int       `json:"line"`
	StartLine    int       `json:"start_line"`
	OriginalLine int       `json:"original_line"`
	Side         string    `json:"side"`
	DiffHunk     string    `json:"diff_hunk"`
	CommitID     string    `json:"commit_id"`
	User         ghLogin   `json:"user"`
	Body         string    `json:"body"`
	CreatedAt    time.Time `json:"created_at"`
}

// parseReviewComments converts gh api --paginate --slurp output, an array of
// page arrays, into one list in GitHub's order.
func parseReviewComments(data []byte) ([]ReviewComment, error) {
	var pages [][]ghReviewComment
	if err := json.Unmarshal(data, &pages); err != nil {
		return nil, fmt.Errorf("failed to parse review comments: %w", err)
	}
	var out []ReviewComment
	for _, page := range pages {
		for _, c := range page {
			out = append(out, ReviewComment{
				ID: c.ID, InReplyTo: c.InReplyToID, Path: c.Path,
				Line: c.Line, StartLine: c.StartLine, OriginalLine: c.OriginalLine,
				Side: c.Side, DiffHunk: c.DiffHunk, CommitID: c.CommitID,
				Author: c.User.Login, Body: c.Body, CreatedAt: c.CreatedAt,
			})
		}
	}
	return out, nil
}

// FetchPRDetail retrieves a pull request's conversation, checks, commits,
// and changed files.
func (g *GitHubAdapter) FetchPRDetail(number int) (*PRDetail, error) {
	args := []string{"pr", "view", strconv.Itoa(number), "--json", prDetailFields}
	if g.repo != "" {
		args = append(args, "--repo", g.repo)
	}
	output, err := g.runGH(args...)
	if err != nil {
		return nil, err
	}
	return parsePRDetail(output)
}

// FetchReviewComments retrieves every inline review comment on a pull
// request. gh pr view has no review threads, so this uses the REST API.
func (g *GitHubAdapter) FetchReviewComments(number int) ([]ReviewComment, error) {
	output, err := g.runGHTimeout(cmdexec.GHCLISlow, reviewCommentsArgs(g.repo, number)...)
	if err != nil {
		return nil, err
	}
	return parseReviewComments(output)
}

// apiArgs is a gh api call on a path under the repository. repo is
// "owner/repo", "host/owner/repo" for GitHub Enterprise (whose host gh
// takes as --hostname), or "" for gh's own {owner}/{repo} placeholders.
func apiArgs(repo, path string, flags ...string) []string {
	args := []string{"api"}
	if parts := strings.Split(repo, "/"); len(parts) == 3 {
		args = append(args, "--hostname", parts[0])
		repo = parts[1] + "/" + parts[2]
	}
	if repo == "" {
		repo = "{owner}/{repo}" // gh fills these in from the current repository
	}
	args = append(args, flags...)
	return append(args, "repos/"+repo+"/"+path)
}

// reviewCommentsArgs is the gh api call for a PR's review comments.
func reviewCommentsArgs(repo string, number int) []string {
	return apiArgs(repo, fmt.Sprintf("pulls/%d/comments?per_page=100", number), "--paginate", "--slurp")
}

// prDiffArgs is the gh api call for a PR's diff, over REST.
func prDiffArgs(repo string, number int) []string {
	return apiArgs(repo, fmt.Sprintf("pulls/%d", number), "-H", "Accept: application/vnd.github.diff")
}

// FetchPRDiff retrieves a pull request's unified diff, for git.ParseDiff.
//
// It comes over REST, since gh pr diff looks the PR up through GraphQL,
// which can be rate limited while REST still answers. The diff is
// verbatim: by default gh neutralizes terminal escape sequences (and,
// writing to a pipe, refuses a response that holds them), so grove asks
// for the raw text and the caller must neutralize control characters
// before drawing any line. A gh too old to know the flag is asked again
// without it.
func (g *GitHubAdapter) FetchPRDiff(number int) (string, error) {
	args := prDiffArgs(g.repo, number)
	output, err := g.runGHTimeout(cmdexec.GHCLISlow, append(args, "--allow-escape-sequences")...)
	if err != nil && strings.Contains(err.Error(), "unknown flag: --allow-escape-sequences") {
		output, err = g.runGHTimeout(cmdexec.GHCLISlow, args...)
	}
	if err != nil {
		return "", err
	}
	return string(output), nil
}
