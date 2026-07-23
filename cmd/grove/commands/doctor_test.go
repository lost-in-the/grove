package commands

// TODO(v0.7.1): Add orchestration smoke test for runExternalModeChecks,
// checkProvisioningSources, and runCheck/runInfo (all at 0.0% coverage).
// The test should call the doctor RunE with a mock filesystem representing
// an external-mode project and assert the check output contains expected
// messages. This was deferred from v0.7.0 due to scope — the RunE function
// pulls in live filesystem, env, and git checks that need a full fixture.
// See release-audit/coverage-gaps.md §"grove doctor orchestration smoke test".

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
)

// newTestWriter returns a *cli.Writer backed by a bytes.Buffer for capturing
// output in tests. Color is disabled (isTTY=false) so output is plain text.
func newTestWriter(buf *bytes.Buffer) *cli.Writer {
	return cli.NewWriter(buf, false)
}

func TestCheckHooksDockerRouting_FlagsHostBundleInstall(t *testing.T) {
	root := t.TempDir()
	groveDir := filepath.Join(root, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Make it a Docker project
	if err := os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM ruby"), 0644); err != nil {
		t.Fatal(err)
	}
	hooksToml := `[[hooks.post_create]]
type = "command"
command = "bundle install --quiet"
`
	if err := os.WriteFile(filepath.Join(groveDir, "hooks.toml"), []byte(hooksToml), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := checkHooksDockerRouting(root, groveDir)
	if err == nil {
		t.Fatal("expected error for host bundle install in docker project")
	}
	if !strings.Contains(err.Error(), "bundle install") {
		t.Errorf("expected error to mention bundle install, got %v", err)
	}
}

func TestCheckHooksDockerRouting_HappyPath(t *testing.T) {
	root := t.TempDir()
	groveDir := filepath.Join(root, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM ruby"), 0644); err != nil {
		t.Fatal(err)
	}
	hooksToml := `[[hooks.post_create]]
type = "docker:compose"
service = "app"
command = "bundle install --quiet"
`
	if err := os.WriteFile(filepath.Join(groveDir, "hooks.toml"), []byte(hooksToml), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := checkHooksDockerRouting(root, groveDir); err != nil {
		t.Errorf("expected pass for compose-typed hook, got %v", err)
	}
}

func TestCheckHooksDockerRouting_NoDockerNoOp(t *testing.T) {
	root := t.TempDir()
	groveDir := filepath.Join(root, ".grove")
	_ = os.MkdirAll(groveDir, 0755)

	got, err := checkHooksDockerRouting(root, groveDir)
	if err != nil {
		t.Fatalf("unexpected error in no-docker dir: %v", err)
	}
	if !strings.Contains(got, "n/a") {
		t.Errorf("expected n/a hint, got %q", got)
	}
}

func TestCheckStrayBackup_FlagsExistingDir(t *testing.T) {
	groveDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(groveDir, ".grove-backup"), 0755); err != nil {
		t.Fatal(err)
	}
	_, err := checkStrayBackup(groveDir)
	if err == nil {
		t.Fatal("expected error when .grove-backup exists")
	}
	if !strings.Contains(err.Error(), "not grove-managed") {
		t.Errorf("expected explanation, got %v", err)
	}
}

func TestCheckStrayBackup_HappyPath(t *testing.T) {
	groveDir := t.TempDir()
	if _, err := checkStrayBackup(groveDir); err != nil {
		t.Errorf("expected no error on clean dir, got %v", err)
	}
}

func TestIsLikelyHostInstallCommand(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"bundle install", true},
		{"bundle install --quiet", true},
		{"npm install && npm test", true},
		{"npm test && npm install", true},
		{"yarn install --frozen-lockfile", true},
		{"pip install -r requirements.txt", true},
		{"echo \"to set up: bundle install\"", false},
		{"echo bundle install required", false}, // false positive risk; we want to NOT match
		{"bundle exec rspec", false},
		{"bundlerinstall", false}, // pattern guard against substring matches
		{"", false},
	}
	for _, c := range cases {
		got := isLikelyHostInstallCommand(c.cmd)
		if got != c.want {
			t.Errorf("isLikelyHostInstallCommand(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}

func TestRewriteHostInstallsToCompose(t *testing.T) {
	src := `# Grove hooks
[[hooks.post_create]]
type = "copy"
from = ".env"
to = ".env"

[[hooks.post_create]]
type = "command"
command = "bundle install --quiet"
timeout = 300
on_failure = "warn"

[[hooks.post_create]]
type = "command"
command = "echo unrelated"
timeout = 60
`
	got, n := rewriteHostInstallsToCompose(src, "web")
	if n != 1 {
		t.Errorf("expected 1 rewrite, got %d", n)
	}
	if !strings.Contains(got, `type = "docker:compose"`) {
		t.Errorf("expected docker:compose type in output, got:\n%s", got)
	}
	if !strings.Contains(got, `service = "web"`) {
		t.Errorf("expected service=web in output")
	}
	// Untouched blocks preserved.
	if !strings.Contains(got, `command = "echo unrelated"`) {
		t.Errorf("non-install command should be untouched")
	}
	// User comment preserved.
	if !strings.Contains(got, "# Grove hooks") {
		t.Errorf("user comment should be preserved")
	}
}

// TestRewriteHostInstallsToCompose_EdgeCases covers malformed and unusual TOML
// shapes that the textual rewriter must tolerate without panicking or
// corrupting the file. The rewriter is deliberately conservative: when in
// doubt, it leaves the block untouched.
func TestRewriteHostInstallsToCompose_EdgeCases(t *testing.T) {
	t.Run("empty input returns empty with zero count", func(t *testing.T) {
		got, n := rewriteHostInstallsToCompose("", "web")
		if got != "" || n != 0 {
			t.Errorf("empty input: got (%q, %d), want (\"\", 0)", got, n)
		}
	})

	t.Run("no post_create hooks leaves input unchanged", func(t *testing.T) {
		src := "[[hooks.pre_create]]\ntype = \"command\"\ncommand = \"bundle install\"\n"
		got, n := rewriteHostInstallsToCompose(src, "web")
		if n != 0 {
			t.Errorf("expected 0 rewrites for non-post_create hook, got %d", n)
		}
		if got != src {
			t.Errorf("input mutated when no post_create blocks present:\nwant: %q\ngot:  %q", src, got)
		}
	})

	t.Run("preserves trailing-newline absence", func(t *testing.T) {
		// No trailing newline on input — output should match.
		src := `[[hooks.post_create]]
type = "command"
command = "echo hi"`
		got, _ := rewriteHostInstallsToCompose(src, "web")
		if strings.HasSuffix(got, "\n") {
			t.Errorf("output gained trailing newline that input didn't have:\n%q", got)
		}
	})

	t.Run("type = command without matching install command is untouched", func(t *testing.T) {
		src := `[[hooks.post_create]]
type = "command"
command = "rake db:migrate"
`
		got, n := rewriteHostInstallsToCompose(src, "web")
		if n != 0 {
			t.Errorf("non-install command should not be rewritten, got n=%d", n)
		}
		if strings.Contains(got, "docker:compose") {
			t.Error("non-install command should not be converted to docker:compose")
		}
	})

	t.Run("malformed block without type line is untouched", func(t *testing.T) {
		// Block declares header but no type=command line — rewriter should
		// not touch it (no typeLine match).
		src := `[[hooks.post_create]]
command = "bundle install"
`
		got, n := rewriteHostInstallsToCompose(src, "web")
		if n != 0 {
			t.Errorf("malformed block (no type line) should be skipped, got n=%d", n)
		}
		if !strings.Contains(got, `command = "bundle install"`) {
			t.Error("original command line should be preserved verbatim")
		}
	})

	t.Run("multiple matching blocks all rewritten", func(t *testing.T) {
		src := `[[hooks.post_create]]
type = "command"
command = "bundle install"

[[hooks.post_create]]
type = "command"
command = "npm install"
`
		got, n := rewriteHostInstallsToCompose(src, "app")
		if n != 2 {
			t.Errorf("expected 2 rewrites, got %d", n)
		}
		if strings.Count(got, `type = "docker:compose"`) != 2 {
			t.Errorf("expected two docker:compose type lines, got:\n%s", got)
		}
		if strings.Count(got, `service = "app"`) != 2 {
			t.Errorf("expected two service=app lines, got:\n%s", got)
		}
	})

	t.Run("block terminated by EOF (no trailing blank/section) is handled", func(t *testing.T) {
		// blockEnd only advances past blank lines or new sections; a block
		// at EOF must still be rewritten.
		src := `[[hooks.post_create]]
type = "command"
command = "bundle install"`
		got, n := rewriteHostInstallsToCompose(src, "web")
		if n != 1 {
			t.Errorf("expected 1 rewrite for EOF-terminated block, got %d", n)
		}
		if !strings.Contains(got, `service = "web"`) {
			t.Errorf("expected service line in output:\n%s", got)
		}
	})

	// Pinning tests for issue #37: known fragile TOML shapes. Each asserts
	// the conservative, non-corrupting behavior the rewriter promises — see
	// the contract docstring on rewriteHostInstallsToCompose for the full
	// list of limitations these tests document.

	t.Run("indented header is still detected and rewritten", func(t *testing.T) {
		// strings.TrimSpace makes the header match position-insensitive.
		// The 3 emitted replacement lines (type/service/mode) are written
		// at column 0, which drops indentation on those specific lines.
		// That's cosmetic — the file is still valid TOML.
		src := "    [[hooks.post_create]]\n    type = \"command\"\n    command = \"bundle install\"\n"
		got, n := rewriteHostInstallsToCompose(src, "web")
		if n != 1 {
			t.Errorf("expected indented header to be detected, got n=%d", n)
		}
		if !strings.Contains(got, `type = "docker:compose"`) {
			t.Errorf("expected docker:compose type line in output:\n%s", got)
		}
	})

	t.Run("quoted keys are left untouched", func(t *testing.T) {
		// HasPrefix(t, "type") fails on `"type" = ...` because the line
		// starts with a quote. Block stays verbatim.
		src := `[[hooks.post_create]]
"type" = "command"
"command" = "bundle install"
`
		got, n := rewriteHostInstallsToCompose(src, "web")
		if n != 0 {
			t.Errorf("quoted-key block should not be rewritten, got n=%d", n)
		}
		if got != src {
			t.Errorf("quoted-key block should be preserved verbatim:\nwant: %q\ngot:  %q", src, got)
		}
	})

	t.Run("inline array-of-tables is left untouched", func(t *testing.T) {
		// `hooks.post_create = [{...}]` is not a `[[hooks.post_create]]`
		// header, so it never enters the block path. Grove's TOML decoder
		// (internal/hooks/config.go) also rejects this shape for arrays
		// of tables, so it cannot appear in a real config — this test
		// pins the no-corruption guarantee for an unsupported shape.
		src := `hooks.post_create = [{ type = "command", command = "bundle install" }]
`
		got, n := rewriteHostInstallsToCompose(src, "web")
		if n != 0 {
			t.Errorf("inline array-of-tables should not be rewritten, got n=%d", n)
		}
		if got != src {
			t.Errorf("inline array-of-tables should be preserved verbatim:\nwant: %q\ngot:  %q", src, got)
		}
	})

	t.Run("multi-line string body containing literal header is preserved", func(t *testing.T) {
		// The inner `[[hooks.post_create]]` line breaks block-walking via
		// the `[` prefix check, but the resulting "header" has no type
		// line so the surrounding block falls through unchanged. The
		// install command inside the string is not detected because
		// isLikelyHostInstallCommand requires a command-token boundary
		// which the surrounding `"""` defeats.
		src := `[[hooks.post_create]]
type = "command"
command = """multi-line body
[[hooks.post_create]]
echo done"""
`
		got, n := rewriteHostInstallsToCompose(src, "web")
		if n != 0 {
			t.Errorf("multi-line string with embedded literal header should not trigger a rewrite, got n=%d", n)
		}
		if got != src {
			t.Errorf("multi-line string content should be preserved verbatim:\nwant: %q\ngot:  %q", src, got)
		}
	})

	t.Run("weird whitespace in keys is tolerated", func(t *testing.T) {
		// HasPrefix(t, "type") and HasPrefix(t, "command") tolerate any
		// amount of whitespace before the `=`. The Trim(`"`) on the
		// extracted value also handles surrounding quotes regardless of
		// padding.
		src := `[[hooks.post_create]]
type   =   "command"
command  =  "bundle install"
`
		_, n := rewriteHostInstallsToCompose(src, "web")
		if n != 1 {
			t.Errorf("loose-whitespace block should still be detected, got n=%d", n)
		}
	})

	t.Run("comments on non-replaced lines are preserved verbatim", func(t *testing.T) {
		// Acceptance criterion for issue #37: "User comments are preserved
		// verbatim." This test exercises every comment position the
		// rewriter must preserve: a leading header comment, an inline
		// comment on the command line, and inline comments on auxiliary
		// fields (timeout, on_failure). The comment on the type line
		// itself is intentionally dropped because that whole line is
		// being replaced — that's the rewrite, not a regression.
		src := `# Local overrides for the web service.
[[hooks.post_create]]
type = "command"  # legacy host install
command = "bundle install" # keep this
timeout = 300 # 5m
on_failure = "warn"  # tolerate flakes
`
		got, n := rewriteHostInstallsToCompose(src, "web")
		if n != 1 {
			t.Fatalf("expected 1 rewrite, got %d", n)
		}
		for _, want := range []string{
			"# Local overrides for the web service.",
			`command = "bundle install" # keep this`,
			"timeout = 300 # 5m",
			`on_failure = "warn"  # tolerate flakes`,
		} {
			if !strings.Contains(got, want) {
				t.Errorf("expected comment/line preserved verbatim: %q\nfull output:\n%s", want, got)
			}
		}
		// Field ordering of non-replaced lines is preserved (command
		// before timeout before on_failure).
		cmdIdx := strings.Index(got, "command =")
		toIdx := strings.Index(got, "timeout =")
		ofIdx := strings.Index(got, "on_failure =")
		if cmdIdx >= toIdx || toIdx >= ofIdx {
			t.Errorf("non-replaced field ordering should be preserved (command < timeout < on_failure):\n%s", got)
		}
	})
}

// TestRewriteHostInstallsToCompose_FurtherEdgeCases pins the *current*
// regex-based behavior on TOML shapes the rewriter doesn't formally
// understand. Each subtest documents the limitation so that the
// companion `refactor(doctor): use TOML parser for hook rewrites
// instead of regex` issue can update the assertions when it lands.
func TestRewriteHostInstallsToCompose_FurtherEdgeCases(t *testing.T) {
	t.Run("indented header is currently treated as a header", func(t *testing.T) {
		// TODO(toml-refactor): a TOML parser would either treat indented
		// headers as syntax errors or normalize indentation on output.
		// Today: strings.TrimSpace strips the indent so the header IS
		// matched, but the injected `service` / `mode` lines come out
		// unindented while the surrounding `command =` line keeps its
		// original indent. Asserting the count here so a future change
		// in either direction surfaces the regression.
		src := "  [[hooks.post_create]]\n  type = \"command\"\n  command = \"bundle install\"\n"
		_, n := rewriteHostInstallsToCompose(src, "web")
		if n != 1 {
			t.Errorf("indented header: expected n=1 under current behavior, got %d", n)
		}
	})

	t.Run("multi-line triple-quoted command is not detected", func(t *testing.T) {
		// TODO(toml-refactor): a real TOML parser would resolve the
		// multi-line value to "bundle install" and trigger the rewrite.
		// Today: the regex extracts the value between the first pair of
		// `"`, which is empty, so isLikelyHostInstallCommand("") is false.
		src := "[[hooks.post_create]]\ntype = \"command\"\ncommand = \"\"\"\nbundle install\n\"\"\"\n"
		got, n := rewriteHostInstallsToCompose(src, "web")
		if n != 0 {
			t.Errorf("multi-line command: expected n=0 under current behavior, got %d", n)
		}
		if !strings.Contains(got, "bundle install") {
			t.Errorf("multi-line command should be preserved verbatim, got:\n%s", got)
		}
	})

	t.Run("quoted key form \"type\" = \"command\" is not detected", func(t *testing.T) {
		// TODO(toml-refactor): TOML allows quoted keys; the regex requires
		// the literal prefix `type` so it misses this shape.
		src := "[[hooks.post_create]]\n\"type\" = \"command\"\ncommand = \"bundle install\"\n"
		got, n := rewriteHostInstallsToCompose(src, "web")
		if n != 0 {
			t.Errorf("quoted key: expected n=0 under current behavior, got %d", n)
		}
		if strings.Contains(got, "docker:compose") {
			t.Errorf("quoted-key block should not be rewritten, got:\n%s", got)
		}
	})

	t.Run("inline-table form is not detected", func(t *testing.T) {
		// TODO(toml-refactor): inline tables (`hooks = [{...}]`) are valid
		// TOML; the regex only knows about array-of-tables headers.
		src := "hooks = [{ type = \"command\", command = \"bundle install\" }]\n"
		got, n := rewriteHostInstallsToCompose(src, "web")
		if n != 0 {
			t.Errorf("inline table: expected n=0 under current behavior, got %d", n)
		}
		if got != src {
			t.Errorf("inline table should be passed through verbatim:\nwant: %q\ngot:  %q", src, got)
		}
	})
}

// projectOpts configures a synthetic project root materialized by
// testProject. Each string field is the file body; empty means "don't
// create that file".
type projectOpts struct {
	Dockerfile        string
	Compose           string // written as docker-compose.yml
	HooksToml         string
	ReadOnlyHooksFile bool // chmod 0444 hooks.toml after writing — exercises write failures
}

// testProject materializes a temporary project root + .grove dir under
// t.TempDir(). It centralizes the t.TempDir() + os.WriteFile pattern that
// existed inline in TestCheckHooksDockerRouting_* so doctor tests can
// declare project shape declaratively.
func testProject(t *testing.T, opts projectOpts) (root, groveDir string) {
	t.Helper()
	root = t.TempDir()
	groveDir = filepath.Join(root, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatalf("mkdir .grove: %v", err)
	}
	if opts.Dockerfile != "" {
		if err := os.WriteFile(filepath.Join(root, "Dockerfile"), []byte(opts.Dockerfile), 0644); err != nil {
			t.Fatalf("write Dockerfile: %v", err)
		}
	}
	if opts.Compose != "" {
		if err := os.WriteFile(filepath.Join(root, "docker-compose.yml"), []byte(opts.Compose), 0644); err != nil {
			t.Fatalf("write compose: %v", err)
		}
	}
	if opts.HooksToml != "" {
		if err := os.WriteFile(filepath.Join(groveDir, "hooks.toml"), []byte(opts.HooksToml), 0644); err != nil {
			t.Fatalf("write hooks.toml: %v", err)
		}
	}
	if opts.ReadOnlyHooksFile {
		// chmod the file (not the dir): Linux file write needs write perm
		// on the file itself + execute on path components, NOT write on
		// the parent dir. Chmod'ing the dir to 0555 still lets OpenFile
		// open and truncate an existing file inside it.
		hooksPath := filepath.Join(groveDir, "hooks.toml")
		if err := os.Chmod(hooksPath, 0444); err != nil {
			t.Fatalf("chmod readonly: %v", err)
		}
		// Restore writable perms so t.TempDir's auto-cleanup can rm -r.
		t.Cleanup(func() { _ = os.Chmod(hooksPath, 0644) })
	}
	return root, groveDir
}

// TestFixHostInstallsInDockerProject covers the early-return and error
// paths that the inline t.TempDir() pattern in
// TestCheckHooksDockerRouting_* didn't reach.
func TestFixHostInstallsInDockerProject(t *testing.T) {
	t.Run("not a docker project returns (0, nil)", func(t *testing.T) {
		root, groveDir := testProject(t, projectOpts{
			HooksToml: `[[hooks.post_create]]` + "\n" + `type = "command"` + "\n" + `command = "bundle install"` + "\n",
		})
		n, err := fixHostInstallsInDockerProject(root, groveDir)
		if err != nil {
			t.Errorf("expected nil error in non-docker dir, got %v", err)
		}
		if n != 0 {
			t.Errorf("expected 0 rewrites in non-docker dir, got %d", n)
		}
	})

	t.Run("dockerfile present but no compose file returns (0, nil)", func(t *testing.T) {
		root, groveDir := testProject(t, projectOpts{
			Dockerfile: "FROM ruby",
			HooksToml:  `[[hooks.post_create]]` + "\n" + `type = "command"` + "\n" + `command = "bundle install"` + "\n",
		})
		n, err := fixHostInstallsInDockerProject(root, groveDir)
		if err != nil {
			t.Errorf("expected nil error when no compose file, got %v", err)
		}
		if n != 0 {
			t.Errorf("expected 0 rewrites when no compose, got %d", n)
		}
	})

	t.Run("compose with only infra services can't infer app, returns error", func(t *testing.T) {
		// All services in infraServiceNames → pickAppService returns
		// ("", false), so InferAppService fails.
		compose := `services:
  postgres:
    image: postgres:15
  redis:
    image: redis:7
`
		root, groveDir := testProject(t, projectOpts{
			Dockerfile: "FROM ruby",
			Compose:    compose,
			HooksToml:  `[[hooks.post_create]]` + "\n" + `type = "command"` + "\n" + `command = "bundle install"` + "\n",
		})
		_, err := fixHostInstallsInDockerProject(root, groveDir)
		if err == nil {
			t.Fatal("expected error when no app service can be inferred")
		}
		if !strings.Contains(err.Error(), "can't infer app service") {
			t.Errorf("error should mention inference failure, got %v", err)
		}
	})

	t.Run("missing hooks.toml returns wrapped read error", func(t *testing.T) {
		compose := "services:\n  web:\n    image: ruby\n"
		root, groveDir := testProject(t, projectOpts{
			Dockerfile: "FROM ruby",
			Compose:    compose,
			// HooksToml intentionally omitted.
		})
		_, err := fixHostInstallsInDockerProject(root, groveDir)
		if err == nil {
			t.Fatal("expected error for missing hooks.toml")
		}
		if !strings.Contains(err.Error(), "read hooks.toml") {
			t.Errorf("error should be wrapped as read failure, got %v", err)
		}
	})

	t.Run("read-only hooks.toml returns wrapped write error", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("chmod 0444 is bypassed when running as root")
		}
		compose := "services:\n  web:\n    image: ruby\n"
		hooks := `[[hooks.post_create]]
type = "command"
command = "bundle install"
`
		root, groveDir := testProject(t, projectOpts{
			Dockerfile:        "FROM ruby",
			Compose:           compose,
			HooksToml:         hooks,
			ReadOnlyHooksFile: true,
		})
		_, err := fixHostInstallsInDockerProject(root, groveDir)
		if err == nil {
			t.Fatal("expected write error on read-only .grove")
		}
		if !strings.Contains(err.Error(), "write hooks.toml") {
			t.Errorf("error should be wrapped as write failure, got %v", err)
		}
	})

	t.Run("happy path rewrites and reports count", func(t *testing.T) {
		compose := "services:\n  web:\n    image: ruby\n"
		hooks := `[[hooks.post_create]]
type = "command"
command = "bundle install --quiet"
`
		root, groveDir := testProject(t, projectOpts{
			Dockerfile: "FROM ruby",
			Compose:    compose,
			HooksToml:  hooks,
		})
		n, err := fixHostInstallsInDockerProject(root, groveDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 1 {
			t.Errorf("expected 1 rewrite, got %d", n)
		}
		got, err := os.ReadFile(filepath.Join(groveDir, "hooks.toml"))
		if err != nil {
			t.Fatalf("read back hooks.toml: %v", err)
		}
		if !strings.Contains(string(got), `type = "docker:compose"`) {
			t.Errorf("hooks.toml not rewritten:\n%s", got)
		}
		if !strings.Contains(string(got), `service = "web"`) {
			t.Errorf("hooks.toml missing service=web:\n%s", got)
		}
	})
}

func TestCheckEnvFileConfig_NonDefault(t *testing.T) {
	direnvFound := func(name string) (string, error) {
		if name == "direnv" {
			return "/usr/bin/direnv", nil
		}
		return "", fmt.Errorf("not found")
	}
	miseFound := func(name string) (string, error) {
		if name == "mise" {
			return "/usr/bin/mise", nil
		}
		return "", fmt.Errorf("not found")
	}
	bothFound := func(name string) (string, error) {
		if name == "direnv" {
			return "/usr/bin/direnv", nil
		}
		if name == "mise" {
			return "/usr/bin/mise", nil
		}
		return "", fmt.Errorf("not found")
	}
	neitherFound := func(name string) (string, error) { return "", fmt.Errorf("not found") }

	tests := []struct {
		name           string
		envFileName    string
		envrcContent   string // "" means no .envrc file
		miseContent    string // "" means no .mise.toml file
		lookPath       func(string) (string, error)
		wantLoader     bool
		wantLoaderName string
		wantConfig     bool
		wantLoads      bool
		wantLoaderErr  bool
		wantConfigErr  bool
	}{
		{
			name:           "direnv installed and envrc references file",
			envFileName:    ".env.local",
			envrcContent:   "dotenv_if_exists .env.local",
			lookPath:       direnvFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     true,
			wantLoads:      true,
		},
		{
			name:           "mise installed and mise.toml references file",
			envFileName:    ".env.local",
			miseContent:    "[env]\nfile = \".env.local\"",
			lookPath:       miseFound,
			wantLoader:     true,
			wantLoaderName: "mise",
			wantConfig:     true,
			wantLoads:      true,
		},
		{
			name:           "both installed, direnv preferred",
			envFileName:    ".env.local",
			envrcContent:   "dotenv_if_exists .env.local",
			lookPath:       bothFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     true,
			wantLoads:      true,
		},
		{
			name:          "neither direnv nor mise installed",
			envFileName:   ".env.local",
			envrcContent:  "dotenv_if_exists .env.local",
			lookPath:      neitherFound,
			wantLoader:    false,
			wantConfig:    true,
			wantLoads:     true,
			wantLoaderErr: true,
		},
		{
			name:           "direnv installed but no config files",
			envFileName:    ".env.local",
			lookPath:       direnvFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     false,
			wantConfigErr:  true,
		},
		{
			name:           "envrc exists but does not reference file",
			envFileName:    ".env.local",
			envrcContent:   "layout ruby",
			lookPath:       direnvFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     true,
			wantLoads:      false,
			wantConfigErr:  true,
		},
		{
			name:           "mise installed with mise.toml not referencing file",
			envFileName:    ".env.local",
			miseContent:    "[tools]\nnode = \"20\"",
			lookPath:       miseFound,
			wantLoader:     true,
			wantLoaderName: "mise",
			wantConfig:     true,
			wantLoads:      false,
			wantConfigErr:  true,
		},
		{
			name:           "custom env file name with direnv",
			envFileName:    ".env.grove",
			envrcContent:   "dotenv_if_exists .env.grove",
			lookPath:       direnvFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     true,
			wantLoads:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.envrcContent != "" {
				if err := os.WriteFile(filepath.Join(tmpDir, ".envrc"), []byte(tt.envrcContent), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.miseContent != "" {
				if err := os.WriteFile(filepath.Join(tmpDir, ".mise.toml"), []byte(tt.miseContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			result := checkEnvFileConfig(tt.envFileName, tmpDir, tt.lookPath)

			if result.loaderInstalled != tt.wantLoader {
				t.Errorf("loaderInstalled = %v, want %v", result.loaderInstalled, tt.wantLoader)
			}
			if result.loaderName != tt.wantLoaderName {
				t.Errorf("loaderName = %q, want %q", result.loaderName, tt.wantLoaderName)
			}
			if result.configExists != tt.wantConfig {
				t.Errorf("configExists = %v, want %v", result.configExists, tt.wantConfig)
			}
			if result.configLoadsFile != tt.wantLoads {
				t.Errorf("configLoadsFile = %v, want %v", result.configLoadsFile, tt.wantLoads)
			}
			if (result.loaderErr != "") != tt.wantLoaderErr {
				t.Errorf("loaderErr = %q, wantErr = %v", result.loaderErr, tt.wantLoaderErr)
			}
			if (result.configErr != "") != tt.wantConfigErr {
				t.Errorf("configErr = %q, wantErr = %v", result.configErr, tt.wantConfigErr)
			}
		})
	}
}

func TestCheckEnvFileConfig_DefaultEnv(t *testing.T) {
	noopLookPath := func(name string) (string, error) { return "", nil }

	tests := []struct {
		name         string
		envrcContent string
		miseContent  string
		wantHint     bool
	}{
		{
			name:         "envrc with env.local support shows hint",
			envrcContent: "dotenv_if_exists .env.local",
			wantHint:     true,
		},
		{
			name:        "mise.toml with env.local support shows hint",
			miseContent: "[env]\nfile = \".env.local\"",
			wantHint:    true,
		},
		{
			name:         "envrc without env.local support no hint",
			envrcContent: "layout ruby",
			wantHint:     false,
		},
		{
			name:         "no config files no hint",
			envrcContent: "",
			wantHint:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.envrcContent != "" {
				if err := os.WriteFile(filepath.Join(tmpDir, ".envrc"), []byte(tt.envrcContent), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.miseContent != "" {
				if err := os.WriteFile(filepath.Join(tmpDir, ".mise.toml"), []byte(tt.miseContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			result := checkEnvFileConfig(".env", tmpDir, noopLookPath)

			if result.hintAvailable != tt.wantHint {
				t.Errorf("hintAvailable = %v, want %v", result.hintAvailable, tt.wantHint)
			}
			if result.loaderInstalled {
				t.Error("loaderInstalled should be false in default mode")
			}
			if result.configExists {
				t.Error("configExists should be false in default mode")
			}
		})
	}
}

func TestCheckGroveBinary(t *testing.T) {
	tests := []struct {
		name     string
		lookPath func(string) (string, error)
		wantPass bool
		wantMsg  string
	}{
		{
			name: "binary found",
			lookPath: func(name string) (string, error) {
				return "/usr/local/bin/grove", nil
			},
			wantPass: true,
			wantMsg:  "grove",
		},
		{
			name: "binary not found",
			lookPath: func(name string) (string, error) {
				return "", fmt.Errorf("not found")
			},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detail, err := checkGroveBinary(tt.lookPath)
			if tt.wantPass && err != nil {
				t.Errorf("expected pass, got error: %v", err)
			}
			if !tt.wantPass && err == nil {
				t.Errorf("expected fail, got pass with: %s", detail)
			}
			if tt.wantPass && !strings.Contains(detail, tt.wantMsg) {
				t.Errorf("expected detail to contain %q, got %q", tt.wantMsg, detail)
			}
		})
	}
}

// testRunGit runs a git command in the given directory, failing the test on error.
func testRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
		"GIT_CONFIG_GLOBAL=/dev/null",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestCheckConfigSymlinks(t *testing.T) {
	t.Run("all symlinks valid", func(t *testing.T) {
		mainDir := t.TempDir()
		testRunGit(t, mainDir, "init")
		testRunGit(t, mainDir, "commit", "--allow-empty", "-m", "init")

		// Create .grove/config.toml in main worktree
		groveDir := filepath.Join(mainDir, ".grove")
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			t.Fatal(err)
		}
		configPath := filepath.Join(groveDir, "config.toml")
		if err := os.WriteFile(configPath, []byte("[grove]"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a secondary worktree
		wtDir := filepath.Join(t.TempDir(), "worktree")
		testRunGit(t, mainDir, "worktree", "add", wtDir, "-b", "test-branch")

		// Create .grove with valid symlink in worktree
		wtGrove := filepath.Join(wtDir, ".grove")
		if err := os.MkdirAll(wtGrove, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(configPath, filepath.Join(wtGrove, "config.toml")); err != nil {
			t.Fatal(err)
		}

		// Any grove-created config symlink is legacy debris now — config
		// resolves from the main worktree, and with config.toml no longer
		// git-ignored the symlink shows up as dirt in its worktree.
		_, err := checkConfigSymlinks(groveDir)
		if err == nil {
			t.Fatal("expected legacy-symlink finding, got pass")
		}
		if !strings.Contains(err.Error(), "legacy config symlinks") {
			t.Errorf("expected 'legacy config symlinks' in error, got %q", err.Error())
		}
	})

	t.Run("broken symlink detected", func(t *testing.T) {
		mainDir := t.TempDir()
		testRunGit(t, mainDir, "init")
		testRunGit(t, mainDir, "commit", "--allow-empty", "-m", "init")

		// Create .grove in main (no config.toml — target will be missing)
		groveDir := filepath.Join(mainDir, ".grove")
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create a secondary worktree
		wtDir := filepath.Join(t.TempDir(), "worktree")
		testRunGit(t, mainDir, "worktree", "add", wtDir, "-b", "test-branch")

		// Create .grove with broken symlink in worktree
		wtGrove := filepath.Join(wtDir, ".grove")
		if err := os.MkdirAll(wtGrove, 0755); err != nil {
			t.Fatal(err)
		}
		// Point to non-existent target
		if err := os.Symlink(filepath.Join(groveDir, "config.toml"), filepath.Join(wtGrove, "config.toml")); err != nil {
			t.Fatal(err)
		}

		_, err := checkConfigSymlinks(groveDir)
		if err == nil {
			t.Fatal("expected error for broken symlink, got nil")
		}
		if !strings.Contains(err.Error(), "legacy config symlinks") || !strings.Contains(err.Error(), "(broken)") {
			t.Errorf("expected legacy finding with broken annotation, got %q", err.Error())
		}
	})

	t.Run("no worktrees besides main", func(t *testing.T) {
		mainDir := t.TempDir()
		testRunGit(t, mainDir, "init")
		testRunGit(t, mainDir, "commit", "--allow-empty", "-m", "init")

		groveDir := filepath.Join(mainDir, ".grove")
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			t.Fatal(err)
		}

		detail, err := checkConfigSymlinks(groveDir)
		if err != nil {
			t.Errorf("expected pass, got error: %v", err)
		}
		if !strings.Contains(detail, "1 worktrees checked") {
			t.Errorf("expected '1 worktrees checked', got %q", detail)
		}
	})
}

// ── runCheck / runInfo ─────────────────────────────────────────────────────

func TestRunCheck(t *testing.T) {
	t.Run("passing check emits success line", func(t *testing.T) {
		var buf bytes.Buffer
		w := newTestWriter(&buf)
		ok := runCheck(w, "My check", func() (string, error) {
			return "detail text", nil
		})
		if !ok {
			t.Error("expected runCheck to return true on success")
		}
		if !strings.Contains(buf.String(), "My check") {
			t.Errorf("expected 'My check' in output, got %q", buf.String())
		}
		if !strings.Contains(buf.String(), "detail text") {
			t.Errorf("expected detail in output, got %q", buf.String())
		}
	})

	t.Run("passing check with empty detail", func(t *testing.T) {
		var buf bytes.Buffer
		w := newTestWriter(&buf)
		ok := runCheck(w, "No detail", func() (string, error) {
			return "", nil
		})
		if !ok {
			t.Error("expected runCheck to return true")
		}
		if !strings.Contains(buf.String(), "No detail") {
			t.Errorf("expected check name in output, got %q", buf.String())
		}
	})

	t.Run("failing check emits error line and returns false", func(t *testing.T) {
		var buf bytes.Buffer
		w := newTestWriter(&buf)
		ok := runCheck(w, "Bad check", func() (string, error) {
			return "", fmt.Errorf("something went wrong")
		})
		if ok {
			t.Error("expected runCheck to return false on error")
		}
		if !strings.Contains(buf.String(), "Bad check") {
			t.Errorf("expected check name in output, got %q", buf.String())
		}
		if !strings.Contains(buf.String(), "something went wrong") {
			t.Errorf("expected error text in output, got %q", buf.String())
		}
	})
}

func TestRunInfo(t *testing.T) {
	var buf bytes.Buffer
	w := newTestWriter(&buf)
	runInfo(w, "Project", "not in a grove project")
	out := buf.String()
	if !strings.Contains(out, "Project") {
		t.Errorf("expected 'Project' in output, got %q", out)
	}
	if !strings.Contains(out, "not in a grove project") {
		t.Errorf("expected detail in output, got %q", out)
	}
}

// ── runExternalModeChecks ──────────────────────────────────────────────────

func TestRunExternalModeChecks_LocalMode(t *testing.T) {
	var buf bytes.Buffer
	w := newTestWriter(&buf)
	cfg := config.LoadDefaults() // Mode defaults to "" (local)
	allPassed := true
	runExternalModeChecks(w, cfg, t.TempDir(), &allPassed)
	out := buf.String()
	if !strings.Contains(out, "not configured") {
		t.Errorf("expected 'not configured' info line for local mode, got %q", out)
	}
	if !allPassed {
		t.Error("expected allPassed to remain true in local mode")
	}
}

func TestRunExternalModeChecks_NilConfig(t *testing.T) {
	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	runExternalModeChecks(w, nil, t.TempDir(), &allPassed)
	out := buf.String()
	if !strings.Contains(out, "not configured") {
		t.Errorf("expected 'not configured' for nil config, got %q", out)
	}
}

func TestRunExternalModeChecks_ExternalModeClean(t *testing.T) {
	// A clean external-mode project: path exists, no provisioning files.
	extDir := t.TempDir()
	projectRoot := t.TempDir()

	cfg := config.LoadDefaults()
	cfg.Plugins.Docker.Mode = "external"
	cfg.Plugins.Docker.External = &config.ExternalComposeConfig{
		Path: extDir,
	}

	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	runExternalModeChecks(w, cfg, projectRoot, &allPassed)
	out := buf.String()

	if !strings.Contains(out, "External compose path") {
		t.Errorf("expected 'External compose path' check in output, got %q", out)
	}
	if !allPassed {
		t.Errorf("expected allPassed=true for clean external project, output:\n%s", out)
	}
}

func TestRunExternalModeChecks_ExternalPathMissing(t *testing.T) {
	// Path is set but does not exist.
	projectRoot := t.TempDir()
	cfg := config.LoadDefaults()
	cfg.Plugins.Docker.Mode = "external"
	cfg.Plugins.Docker.External = &config.ExternalComposeConfig{
		Path: filepath.Join(projectRoot, "nonexistent"),
	}

	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	runExternalModeChecks(w, cfg, projectRoot, &allPassed)
	out := buf.String()

	if allPassed {
		t.Errorf("expected allPassed=false when ext path is missing, output:\n%s", out)
	}
	if !strings.Contains(out, "External compose path") {
		t.Errorf("expected 'External compose path' check in output, got %q", out)
	}
}

func TestRunExternalModeChecks_ExternalPathIsFile(t *testing.T) {
	// Path exists but is a file, not a directory.
	projectRoot := t.TempDir()
	filePath := filepath.Join(projectRoot, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := config.LoadDefaults()
	cfg.Plugins.Docker.Mode = "external"
	cfg.Plugins.Docker.External = &config.ExternalComposeConfig{
		Path: filePath,
	}

	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	runExternalModeChecks(w, cfg, projectRoot, &allPassed)
	if allPassed {
		t.Error("expected allPassed=false when ext path is a file, not a directory")
	}
}

// ── checkProvisioningSources ───────────────────────────────────────────────

func TestCheckProvisioningSources_AllPresent(t *testing.T) {
	projectRoot := t.TempDir()
	// Create the files referenced in copy_files and symlink_files.
	for _, name := range []string{".env", "credentials.json"} {
		if err := os.WriteFile(filepath.Join(projectRoot, name), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}
	ext := &config.ExternalComposeConfig{
		CopyFiles:    []string{".env"},
		SymlinkFiles: []string{"credentials.json"},
	}
	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	checkProvisioningSources(w, ext, projectRoot, &allPassed)
	if !allPassed {
		t.Errorf("expected allPassed=true when all files present, output:\n%s", buf.String())
	}
}

func TestCheckProvisioningSources_MissingFile(t *testing.T) {
	projectRoot := t.TempDir()
	// Do NOT create the file — it should be flagged as missing.
	ext := &config.ExternalComposeConfig{
		CopyFiles: []string{"credentials.json"},
	}
	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	checkProvisioningSources(w, ext, projectRoot, &allPassed)
	if allPassed {
		t.Errorf("expected allPassed=false when file is missing, output:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "credentials.json") {
		t.Errorf("expected missing file name in output, got %q", buf.String())
	}
}

func TestCheckProvisioningSources_Empty(t *testing.T) {
	// No provisioning entries — no checks emitted.
	ext := &config.ExternalComposeConfig{}
	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	checkProvisioningSources(w, ext, t.TempDir(), &allPassed)
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty provisioning config, got %q", buf.String())
	}
	if !allPassed {
		t.Error("expected allPassed unchanged (true) for empty provisioning config")
	}
}

func TestCheckProvisioningSources_SymlinkDirMissing(t *testing.T) {
	projectRoot := t.TempDir()
	ext := &config.ExternalComposeConfig{
		SymlinkDirs: []string{"vendor"},
	}
	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	checkProvisioningSources(w, ext, projectRoot, &allPassed)
	if allPassed {
		t.Errorf("expected allPassed=false for missing symlink_dirs entry, output:\n%s", buf.String())
	}
}

// ── checkEnvFileChecks ─────────────────────────────────────────────────────

func TestCheckEnvFileChecks_DefaultEnvSkips(t *testing.T) {
	// Default .env with no hint: function should emit nothing.
	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	checkEnvFileChecks(w, ".env", envFileCheckResult{hintAvailable: false}, &allPassed)
	if buf.Len() != 0 {
		t.Errorf("expected no output for default .env without hint, got %q", buf.String())
	}
}

func TestCheckEnvFileChecks_DefaultEnvHint(t *testing.T) {
	// Default .env with hint: should emit an info line, not fail.
	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	checkEnvFileChecks(w, ".env", envFileCheckResult{hintAvailable: true}, &allPassed)
	out := buf.String()
	if !strings.Contains(out, "Env file hint") {
		t.Errorf("expected 'Env file hint' in output, got %q", out)
	}
	if !allPassed {
		t.Error("expected allPassed unchanged (true) for hint-only case")
	}
}

func TestCheckEnvFileChecks_CustomEnvLoaderFound(t *testing.T) {
	// Custom env file, loader present, config loads the file.
	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	result := envFileCheckResult{
		loaderInstalled: true,
		loaderName:      "direnv",
		configExists:    true,
		configLoadsFile: true,
	}
	checkEnvFileChecks(w, ".env.local", result, &allPassed)
	out := buf.String()
	if !strings.Contains(out, "Env file target") {
		t.Errorf("expected 'Env file target' check, got %q", out)
	}
	if !allPassed {
		t.Errorf("expected allPassed=true, output:\n%s", out)
	}
}

func TestCheckEnvFileChecks_CustomEnvLoaderMissing(t *testing.T) {
	// Loader not found — runCheck emits an error (non-fatal, result not gated).
	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	result := envFileCheckResult{
		loaderInstalled: false,
		loaderErr:       "neither direnv nor mise found",
		configLoadsFile: false,
		configErr:       "no .envrc or .mise.toml found",
	}
	checkEnvFileChecks(w, ".env.local", result, &allPassed)
	out := buf.String()
	if !strings.Contains(out, "Env file loader") {
		t.Errorf("expected 'Env file loader' check in output, got %q", out)
	}
}

// ── checkAgentStacks ───────────────────────────────────────────────────────

func TestCheckAgentStacks_NotEnabled(t *testing.T) {
	// Agent section absent — should emit "not enabled" info and return.
	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	ext := &config.ExternalComposeConfig{} // Agent is nil
	cfg := config.LoadDefaults()
	cfg.Plugins.Docker.Mode = "external"
	cfg.Plugins.Docker.External = ext
	checkAgentStacks(w, cfg, ext, &allPassed)
	if !strings.Contains(buf.String(), "not enabled") {
		t.Errorf("expected 'not enabled' info, got %q", buf.String())
	}
	if !allPassed {
		t.Error("expected allPassed unchanged when agent not enabled")
	}
}

func TestCheckAgentStacks_EnabledMissingTemplate(t *testing.T) {
	// Agent enabled but template_path missing — should fail.
	extDir := t.TempDir()
	enabled := true
	ext := &config.ExternalComposeConfig{
		Path: extDir,
		Agent: &config.AgentStackConfig{
			Enabled:      &enabled,
			Services:     []string{"app"},
			TemplatePath: "agent-compose.yml", // does not exist
		},
	}
	cfg := config.LoadDefaults()
	cfg.Plugins.Docker.Mode = "external"
	cfg.Plugins.Docker.External = ext

	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	checkAgentStacks(w, cfg, ext, &allPassed)
	out := buf.String()
	if allPassed {
		t.Errorf("expected allPassed=false when template missing, output:\n%s", out)
	}
	if !strings.Contains(out, "Agent template path") {
		t.Errorf("expected 'Agent template path' check in output, got %q", out)
	}
}

func TestCheckAgentStacks_EnabledNoServices(t *testing.T) {
	// Agent enabled but services list is empty — should fail agent config check.
	extDir := t.TempDir()
	enabled := true
	ext := &config.ExternalComposeConfig{
		Path: extDir,
		Agent: &config.AgentStackConfig{
			Enabled:      &enabled,
			Services:     []string{}, // empty
			TemplatePath: "agent.yml",
		},
	}
	cfg := config.LoadDefaults()
	cfg.Plugins.Docker.Mode = "external"
	cfg.Plugins.Docker.External = ext

	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	checkAgentStacks(w, cfg, ext, &allPassed)
	out := buf.String()
	if allPassed {
		t.Errorf("expected allPassed=false for empty services, output:\n%s", out)
	}
	if !strings.Contains(out, "agent.services is empty") {
		t.Errorf("expected 'agent.services is empty' in output, got %q", out)
	}
}

// ── checkAgentNetwork ─────────────────────────────────────────────────────

func TestCheckAgentNetwork_EmptyNetworkNoOp(t *testing.T) {
	// Empty network string: function should return immediately without output.
	var buf bytes.Buffer
	w := newTestWriter(&buf)
	allPassed := true
	checkAgentNetwork(w, "", &allPassed)
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty network, got %q", buf.String())
	}
	if !allPassed {
		t.Error("expected allPassed unchanged for empty network")
	}
}

func TestFixConfigSymlinks_RemovesLegacySymlinks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	mainWt := filepath.Join(base, "main")
	if err := os.MkdirAll(filepath.Join(mainWt, ".grove"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(base, "init", mainWt)
	testWrite(t, filepath.Join(mainWt, ".grove", "config.toml"), "project_name = \"m\"\n")
	testWrite(t, filepath.Join(mainWt, "f"), "x")
	git(mainWt, "add", "-A")
	git(mainWt, "commit", "-m", "init")

	linked := filepath.Join(base, "linked")
	git(mainWt, "worktree", "add", linked, "-b", "feat")

	// Replace the linked worktree's checked-out config.toml with a legacy
	// per-worktree symlink (what pre-0.10 grove created).
	lc := filepath.Join(linked, ".grove", "config.toml")
	if err := os.Remove(lc); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(mainWt, ".grove", "config.toml"), lc); err != nil {
		t.Fatal(err)
	}

	groveDir := filepath.Join(mainWt, ".grove")
	if _, err := checkConfigSymlinks(groveDir); err == nil {
		t.Fatal("checkConfigSymlinks should flag the planted symlink")
	}

	removed, err := fixConfigSymlinks(groveDir)
	if err != nil {
		t.Fatalf("fixConfigSymlinks: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	// The symlink must be gone (the committed real file is restored in its place).
	if fi, err := os.Lstat(lc); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("config.toml is still a symlink after --fix")
	}
	if _, err := checkConfigSymlinks(groveDir); err != nil {
		t.Errorf("checkConfigSymlinks should pass after fix, got: %v", err)
	}
}

func testWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func resetDoctorActions() { doctorActions = nil }

func TestRunFixableCheck(t *testing.T) {
	t.Run("fix repairs a failing check and the run reflects the post-fix state", func(t *testing.T) {
		resetDoctorActions()
		var buf bytes.Buffer
		w := newTestWriter(&buf)
		healthy := false
		ok := runFixableCheck(w, "Config symlinks",
			func() (string, error) {
				if healthy {
					return "3 worktrees checked", nil
				}
				return "", fmt.Errorf("legacy config symlinks in: a, b")
			},
			true,
			func() (string, error) {
				healthy = true
				return "Removed 2 legacy config symlink(s)", nil
			})
		if !ok {
			t.Error("expected fixed check to count as passed")
		}
		out := buf.String()
		for _, want := range []string{"legacy config symlinks in: a, b", "Removed 2 legacy config symlink(s)", "3 worktrees checked"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in output:\n%s", want, out)
			}
		}
		if len(doctorActions) != 0 {
			t.Errorf("fixed check must not leave an action item, got %v", doctorActions)
		}
	})

	t.Run("without --fix the failure is recorded as an action item", func(t *testing.T) {
		resetDoctorActions()
		var buf bytes.Buffer
		w := newTestWriter(&buf)
		ok := runFixableCheck(w, "Config symlinks",
			func() (string, error) { return "", fmt.Errorf("legacy config symlinks in: a") },
			false,
			func() (string, error) { t.Fatal("fixer must not run without --fix"); return "", nil })
		if ok {
			t.Error("expected failing check to return false")
		}
		if len(doctorActions) != 1 || doctorActions[0].name != "Config symlinks" {
			t.Errorf("expected one recorded action, got %v", doctorActions)
		}
	})

	t.Run("failing fixer records the action and warns", func(t *testing.T) {
		resetDoctorActions()
		var buf bytes.Buffer
		w := newTestWriter(&buf)
		ok := runFixableCheck(w, "Hooks Docker-routing",
			func() (string, error) { return "", fmt.Errorf("host installs detected") },
			true,
			func() (string, error) { return "", fmt.Errorf("rewrite failed") })
		if ok {
			t.Error("expected check to stay failed when the fixer errors")
		}
		if !strings.Contains(buf.String(), "rewrite failed") {
			t.Errorf("expected fixer error in output:\n%s", buf.String())
		}
		if len(doctorActions) != 1 {
			t.Errorf("expected one recorded action, got %v", doctorActions)
		}
	})

	t.Run("fixer that does not resolve the check keeps it failed", func(t *testing.T) {
		resetDoctorActions()
		var buf bytes.Buffer
		w := newTestWriter(&buf)
		ok := runFixableCheck(w, "Stubborn",
			func() (string, error) { return "", fmt.Errorf("still broken") },
			true,
			func() (string, error) { return "tried something", nil })
		if ok {
			t.Error("expected check to stay failed after ineffective fix")
		}
		if len(doctorActions) != 1 {
			t.Errorf("expected one recorded action, got %v", doctorActions)
		}
	})

	t.Run("passing check never invokes the fixer", func(t *testing.T) {
		resetDoctorActions()
		var buf bytes.Buffer
		w := newTestWriter(&buf)
		ok := runFixableCheck(w, "Fine",
			func() (string, error) { return "all good", nil },
			true,
			func() (string, error) { t.Fatal("fixer must not run for a passing check"); return "", nil })
		if !ok {
			t.Error("expected passing check to return true")
		}
	})
}

func TestDoctorActionSummary(t *testing.T) {
	t.Run("failures render a numbered action list at the end", func(t *testing.T) {
		resetDoctorActions()
		var buf bytes.Buffer
		w := newTestWriter(&buf)
		runCheck(w, "Docker network 'x'", func() (string, error) {
			return "", fmt.Errorf("network not found (is the main stack running?)")
		})
		runCheck(w, "Config symlinks", func() (string, error) {
			return "", fmt.Errorf("legacy config symlinks in: a — run `grove doctor --fix` to remove them")
		})
		buf.Reset()
		printDoctorSummary(w, false)
		out := buf.String()
		for _, want := range []string{
			"2 checks failed — action required:",
			"1. Docker network 'x' — network not found (is the main stack running?)",
			"2. Config symlinks — legacy config symlinks in: a — run `grove doctor --fix` to remove them",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in summary:\n%s", want, out)
			}
		}
	})

	t.Run("all passed prints the success line and no action list", func(t *testing.T) {
		resetDoctorActions()
		var buf bytes.Buffer
		w := newTestWriter(&buf)
		printDoctorSummary(w, true)
		out := buf.String()
		if !strings.Contains(out, "All checks passed") {
			t.Errorf("expected success line, got:\n%s", out)
		}
		if strings.Contains(out, "Action required") {
			t.Errorf("unexpected action list on a clean run:\n%s", out)
		}
	})

	t.Run("optional checks do not record action items", func(t *testing.T) {
		resetDoctorActions()
		var buf bytes.Buffer
		w := newTestWriter(&buf)
		runOptionalCheck(w, "Tmux", func() (string, error) {
			return "", fmt.Errorf("tmux not found in PATH (optional)")
		})
		if len(doctorActions) != 0 {
			t.Errorf("optional check must not record an action, got %v", doctorActions)
		}
	})
}
