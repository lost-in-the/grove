package tracker

import (
	"os"
	"strings"
	"testing"
	"time"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParsePRDetail(t *testing.T) {
	d, err := parsePRDetail(readFixture(t, "pr_detail.json"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Number != 42 || d.HeadSHA != "0123456789abcdef0123456789abcdef01234567" || !d.UpdatedAt.Equal(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("header fields: %+v", d)
	}
	if len(d.Comments) != 1 || d.Comments[0].Author != "octocat" || d.Comments[0].Body != "Looks close." {
		t.Errorf("comments: %+v", d.Comments)
	}
	if len(d.Reviews) != 1 || d.Reviews[0].Author != "hubot" || d.Reviews[0].State != "APPROVED" {
		t.Errorf("reviews: %+v", d.Reviews)
	}
	want := []PRCheck{
		{Name: "Lint", State: "SUCCESS", URL: "https://github.com/o/r/actions/runs/1/job/1"},
		{Name: "Test", State: "IN_PROGRESS", URL: "https://github.com/o/r/actions/runs/1/job/2"},
		{Name: "ci/legacy", State: "PENDING", URL: "https://ci.example.com/42"},
	}
	if len(d.Checks) != len(want) {
		t.Fatalf("checks: %+v", d.Checks)
	}
	for i := range want {
		if d.Checks[i] != want[i] {
			t.Errorf("check %d = %+v, want %+v", i, d.Checks[i], want[i])
		}
	}
	if len(d.Commits) != 2 || d.Commits[1].SHA != "bbbbbbb" || d.Commits[1].Message != "fix: second" {
		t.Errorf("commits: %+v", d.Commits)
	}
	if len(d.Files) != 2 || d.Files[1] != (PRFile{Path: "docs/README.md", Additions: 10}) {
		t.Errorf("files: %+v", d.Files)
	}
}

// Input is gh api --paginate --slurp output: an array of page arrays.
func TestParseReviewComments(t *testing.T) {
	cs, err := parseReviewComments(readFixture(t, "review_comments.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 4 {
		t.Fatalf("got %d comments, want 4 across both pages", len(cs))
	}
	if cs[1].InReplyTo != cs[0].ID || cs[0].InReplyTo != 0 {
		t.Errorf("reply: %d -> %d", cs[1].ID, cs[1].InReplyTo)
	}
	if c := cs[0]; c.Path != "main.go" || c.Line != 12 || c.Side != "RIGHT" || c.Author != "hubot" || c.Body != "Why this?" || c.DiffHunk != "@@ -10,3 +10,4 @@ func main() {" {
		t.Errorf("first: %+v", c)
	}
	if c := cs[2]; c.Line != 0 || c.OriginalLine != 4 {
		t.Errorf("outdated: line %d, original %d; want 0, 4", c.Line, c.OriginalLine)
	}
	if c := cs[3]; c.StartLine != 18 || c.Line != 20 {
		t.Errorf("multi-line: start %d, line %d; want 18, 20", c.StartLine, c.Line)
	}
}

// gh api takes a GitHub Enterprise host as --hostname, not in the path.
func TestReviewCommentsArgs(t *testing.T) {
	cases := map[string]string{
		"o/r":                      "api --paginate --slurp repos/o/r/pulls/7/comments?per_page=100",
		"ghe.example.com/team/app": "api --hostname ghe.example.com --paginate --slurp repos/team/app/pulls/7/comments?per_page=100",
		"":                         "api --paginate --slurp repos/{owner}/{repo}/pulls/7/comments?per_page=100",
	}
	for repo, want := range cases {
		if got := strings.Join(reviewCommentsArgs(repo, 7), " "); got != want {
			t.Errorf("%q: %s\nwant %s", repo, got, want)
		}
	}
}

// The diff comes over REST: gh pr diff looks the PR up through GraphQL,
// which can be rate limited while REST still answers.
func TestPRDiffArgs(t *testing.T) {
	want := "api --hostname ghe.example.com -H Accept: application/vnd.github.diff repos/team/app/pulls/7"
	if got := strings.Join(prDiffArgs("ghe.example.com/team/app", 7), " "); got != want {
		t.Errorf("%s\nwant %s", got, want)
	}
	if got := strings.Join(prDiffArgs("o/r", 7), " "); got != "api -H Accept: application/vnd.github.diff repos/o/r/pulls/7" {
		t.Errorf("%s", got)
	}
}

func TestCommitDiffArgs(t *testing.T) {
	want := "api -H Accept: application/vnd.github.diff repos/o/r/commits/bbbbbbb2222"
	if got := strings.Join(commitDiffArgs("o/r", "bbbbbbb2222"), " "); got != want {
		t.Errorf("%s\nwant %s", got, want)
	}
}

// Commits keep their full id, for fetching one commit's diff.
func TestParsePRDetail_CommitOID(t *testing.T) {
	d, err := parsePRDetail(readFixture(t, "pr_detail.json"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Commits[1].OID != "bbbbbbb2222222222222222222222222222222222" {
		t.Errorf("OID %q", d.Commits[1].OID)
	}
}
