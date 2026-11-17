//go:build integration

// End-to-end coverage for behavior every command shares: the --json error
// shape, exit codes for the same condition, and the version flag.
package integration_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/testhelper"
)

type jsonErrorDoc struct {
	Error   bool   `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// lastJSONError parses stderr as one JSON error object. Stderr must hold
// nothing else: a script reads it whole.
func lastJSONError(t *testing.T, stderr string) jsonErrorDoc {
	t.Helper()
	var doc jsonErrorDoc
	if err := json.Unmarshal([]byte(stderr), &doc); err != nil {
		t.Fatalf("stderr is not one JSON error object: %v\nstderr: %q", err, stderr)
	}
	return doc
}

func TestPsJSON_NoAgentStacksPrintsEmptyArray(t *testing.T) {
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)

	res := runGrove(t, binary, repo, "ps", "--json")

	if res.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", res.code, res.stderr)
	}
	if got := strings.TrimSpace(res.stdout); got != "[]" {
		t.Errorf("stdout = %q, want []", got)
	}
	if res.stderr != "" {
		t.Errorf("stderr = %q, want empty", res.stderr)
	}
}

func TestJSONFlag_ErrorsUseOneShape(t *testing.T) {
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)

	cases := []struct {
		name string
		args []string
	}{
		{"to", []string{"to", "ghost", "--json"}},
		{"diff", []string{"diff", "ghost", "--json"}},
		{"join", []string{"join", "ghost", "--json"}},
		{"open", []string{"open", "ghost", "--no-create", "--json"}},
		{"graft", []string{"graft", "ghost", "--json"}},
		{"sync", []string{"sync", "ghost", "--json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := runGrove(t, binary, repo, tc.args...)

			if res.code == 0 {
				t.Fatalf("exit code = 0, want non-zero\nstdout: %s", res.stdout)
			}
			doc := lastJSONError(t, res.stderr)
			if !doc.Error {
				t.Errorf(`"error" = false, want true`)
			}
			if doc.Code != res.code {
				t.Errorf(`"code" = %d, exit code = %d: they must match`, doc.Code, res.code)
			}
			if doc.Message == "" {
				t.Errorf(`"message" is empty`)
			}
		})
	}
}

func TestWithoutJSONFlag_ErrorsStayPlainText(t *testing.T) {
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)

	res := runGrove(t, binary, repo, "open", "ghost", "--no-create")

	if res.code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.HasPrefix(res.stderr, "Error: ") {
		t.Errorf("stderr = %q, want it to start with %q", res.stderr, "Error: ")
	}
}

func TestNew_ExistingNameExitsResourceExists(t *testing.T) {
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)

	first := runGrove(t, binary, repo, "new", "twice", "--no-switch", "--here")
	if first.code != 0 {
		t.Fatalf("first create: exit code = %d\nstderr: %s", first.code, first.stderr)
	}

	t.Run("text", func(t *testing.T) {
		res := runGrove(t, binary, repo, "new", "twice", "--no-switch", "--here")
		if res.code != exitcode.ResourceExists {
			t.Errorf("exit code = %d, want %d", res.code, exitcode.ResourceExists)
		}
		if !strings.Contains(res.stderr, "already exists") {
			t.Errorf("stderr = %q, want it to say the worktree already exists", res.stderr)
		}
	})

	t.Run("json", func(t *testing.T) {
		res := runGrove(t, binary, repo, "new", "twice", "--no-switch", "--here", "--json")
		if res.code != exitcode.ResourceExists {
			t.Errorf("exit code = %d, want %d", res.code, exitcode.ResourceExists)
		}
		if doc := lastJSONError(t, res.stderr); doc.Code != exitcode.ResourceExists {
			t.Errorf(`"code" = %d, want %d`, doc.Code, exitcode.ResourceExists)
		}
	})
}

func TestVersionFlagMatchesVersionCommand(t *testing.T) {
	binary := buildGroveBinary(t)
	dir := t.TempDir()

	flag := runGrove(t, binary, dir, "--version")
	if flag.code != 0 {
		t.Fatalf("grove --version: exit code = %d\nstderr: %s", flag.code, flag.stderr)
	}
	sub := runGrove(t, binary, dir, "version")
	want, _, _ := strings.Cut(sub.stdout, "\n")
	if got := strings.TrimSpace(flag.stdout); got != want {
		t.Errorf("grove --version = %q, want the first line of grove version: %q", got, want)
	}
}

// runGroveWithFakeGh runs grove with a gh on PATH that knows issue #7.
func runGroveWithFakeGh(t *testing.T, binary, dir string, args ...string) runResult {
	t.Helper()
	t.Setenv("PATH", installFakeGh(t)+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GROVE_TEST_GH", "issue-found")
	return runGrove(t, binary, dir, args...)
}

func TestFetchJSON_PrintsTheCreateResult(t *testing.T) {
	binary := buildGroveBinary(t)

	type result struct {
		SwitchTo string `json:"switch_to"`
		Name     string `json:"name"`
		Path     string `json:"path"`
		Created  bool   `json:"created"`
	}
	parse := func(t *testing.T, res runResult) result {
		t.Helper()
		if res.code != 0 {
			t.Fatalf("exit code = %d\nstderr: %s", res.code, res.stderr)
		}
		var got result
		if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
			t.Fatalf("stdout is not one JSON object: %v\nstdout: %q", err, res.stdout)
		}
		if !got.Created || got.Path == "" || !strings.Contains(got.Name, "7") {
			t.Errorf("result = %+v, want created, a path, and a name with the issue number", got)
		}
		return got
	}

	t.Run("switches by default", func(t *testing.T) {
		repo := testhelper.SetupRailsFixture(t)
		got := parse(t, runGroveWithFakeGh(t, binary, repo, "fetch", "issue/7", "--json", "--here"))
		if got.SwitchTo != got.Path {
			t.Errorf("switch_to = %q, want the path %q", got.SwitchTo, got.Path)
		}
	})

	t.Run("no-switch leaves switch_to out", func(t *testing.T) {
		repo := testhelper.SetupRailsFixture(t)
		got := parse(t, runGroveWithFakeGh(t, binary, repo, "fetch", "issue/7", "-j", "-n", "--here"))
		if got.SwitchTo != "" {
			t.Errorf("switch_to = %q, want it left out with --no-switch", got.SwitchTo)
		}
	})
}

func TestFetchNoSwitch_DoesNotChangeDirectory(t *testing.T) {
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)
	t.Setenv("GROVE_SHELL", "1")

	res := runGroveWithFakeGh(t, binary, repo, "fetch", "issue/7", "--no-switch", "--here")

	if res.code != 0 {
		t.Fatalf("exit code = %d\nstderr: %s", res.code, res.stderr)
	}
	if strings.Contains(res.stdout, "cd:") {
		t.Errorf("stdout has a cd: directive with --no-switch:\n%s", res.stdout)
	}
	if !strings.Contains(res.stdout, "grove to ") {
		t.Errorf("stdout = %q, want it to say how to move there", res.stdout)
	}
}

func TestCreateErrorsUnderJSON(t *testing.T) {
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)
	branch := strings.TrimSpace(testhelper.RunGit(t, repo, "rev-parse", "--abbrev-ref", "HEAD"))
	testhelper.RunGit(t, repo, "branch", branch+"-taken")

	t.Run("fork onto an existing branch", func(t *testing.T) {
		res := runGrove(t, binary, repo, "fork", "taken", "--json", "--no-wip", "--here")
		if res.code != exitcode.ResourceExists {
			t.Errorf("exit code = %d, want %d", res.code, exitcode.ResourceExists)
		}
		if doc := lastJSONError(t, res.stderr); doc.Code != exitcode.ResourceExists {
			t.Errorf(`"code" = %d, want %d`, doc.Code, exitcode.ResourceExists)
		}
	})

	t.Run("new mirroring a missing remote branch", func(t *testing.T) {
		res := runGrove(t, binary, repo, "new", "env", "--mirror", "origin/ghost", "--json", "--here")
		if res.code != exitcode.ResourceNotFound {
			t.Errorf("exit code = %d, want %d", res.code, exitcode.ResourceNotFound)
		}
		if doc := lastJSONError(t, res.stderr); doc.Code != exitcode.ResourceNotFound {
			t.Errorf(`"code" = %d, want %d`, doc.Code, exitcode.ResourceNotFound)
		}
	})
}
