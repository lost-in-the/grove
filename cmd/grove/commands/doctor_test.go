package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

		detail, err := checkConfigSymlinks(groveDir)
		if err != nil {
			t.Errorf("expected pass, got error: %v", err)
		}
		if !strings.Contains(detail, "worktrees checked") {
			t.Errorf("expected 'worktrees checked' in detail, got %q", detail)
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
		if !strings.Contains(err.Error(), "broken symlinks") {
			t.Errorf("expected 'broken symlinks' in error, got %q", err.Error())
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
