//go:build integration

// End-to-end coverage for create commands reporting files that were not
// copied or linked: exit code, stderr lines, --json fields, a config that
// fails to load, and the notice inside a worktree grove did not create.
package integration_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/testhelper"
)

// writeExternalConfig replaces the fixture's project config with an external
// compose config whose file setup lists are set from fileLists (raw TOML).
func writeExternalConfig(t *testing.T, repo, fileLists string) {
	t.Helper()
	compose := filepath.Join(filepath.Dir(repo), "shared-compose")
	if err := os.MkdirAll(compose, 0o755); err != nil {
		t.Fatal(err)
	}
	toml := `project_name = "myapp"

[plugins.docker]
mode = "external"

[plugins.docker.external]
path = "` + compose + `"
env_var = "APP_DIR"
services = ["app"]
` + fileLists
	testhelper.WriteFile(t, filepath.Join(repo, ".grove", "config.toml"), toml)
}

type runResult struct {
	stdout, stderr string
	code           int
}

func runGrove(t *testing.T, binary, dir string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("run grove %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return runResult{stdout: out.String(), stderr: errOut.String(), code: code}
}

type createJSON struct {
	Created     bool   `json:"created"`
	Path        string `json:"path"`
	Provisioned *bool  `json:"provisioned"`
	FileSetup   *struct {
		Copied []string `json:"copied"`
		Linked []string `json:"linked"`
		Failed []struct {
			Field string `json:"field"`
			Path  string `json:"path"`
			Error string `json:"error"`
		} `json:"failed"`
	} `json:"file_setup"`
}

func TestNewJSON_MissingCopySourceExitsNonZero(t *testing.T) {
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)
	writeExternalConfig(t, repo, `copy_files = ["config/secret.key", "config/database.yml"]`)

	res := runGrove(t, binary, repo, "new", "newwork", "--json", "--no-switch", "--here")

	if res.code != exitcode.CreatedNotProvisioned {
		t.Fatalf("exit code = %d, want %d\nstderr: %s", res.code, exitcode.CreatedNotProvisioned, res.stderr)
	}
	for _, want := range []string{
		"Not copied: config/secret.key (missing in the main worktree)",
		"The worktree was created, but 1 file is missing. Fix the source, then run `grove doctor newwork --fix`.",
	} {
		if !strings.Contains(res.stderr, want) {
			t.Errorf("stderr missing %q\nstderr: %s", want, res.stderr)
		}
	}

	var got createJSON
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, res.stdout)
	}
	if !got.Created || got.Provisioned == nil || *got.Provisioned {
		t.Errorf("created=%v provisioned=%v, want true/false", got.Created, got.Provisioned)
	}
	if got.FileSetup == nil || len(got.FileSetup.Failed) != 1 ||
		got.FileSetup.Failed[0].Field != "copy_files" || got.FileSetup.Failed[0].Path != "config/secret.key" {
		t.Fatalf("file_setup.failed = %+v", got.FileSetup)
	}
	if len(got.FileSetup.Copied) != 1 || got.FileSetup.Copied[0] != "config/database.yml" {
		t.Errorf("file_setup.copied = %v, want [config/database.yml]", got.FileSetup.Copied)
	}
	if _, err := os.Stat(filepath.Join(got.Path, "config", "database.yml")); err != nil {
		t.Errorf("worktree was not kept or the good entry was not copied: %v", err)
	}
}

func TestNewJSON_AllFilesCopiedIsProvisioned(t *testing.T) {
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)
	writeExternalConfig(t, repo, `copy_files = ["config/database.yml"]`)

	res := runGrove(t, binary, repo, "new", "newwork", "--json", "--no-switch", "--here")
	if res.code != 0 {
		t.Fatalf("exit code = %d\nstderr: %s", res.code, res.stderr)
	}
	var got createJSON
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, res.stdout)
	}
	if got.Provisioned == nil || !*got.Provisioned || got.FileSetup == nil || len(got.FileSetup.Failed) != 0 {
		t.Errorf("provisioned=%v file_setup=%+v, want true and no failures", got.Provisioned, got.FileSetup)
	}
}

func TestNew_ConfigThatFailsToLoadCreatesNoWorktree(t *testing.T) {
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)
	writeExternalConfig(t, repo, `copy_files = ["config/*.key"]`)

	res := runGrove(t, binary, repo, "new", "newwork", "--json", "--no-switch", "--here")

	if res.code == 0 {
		t.Fatalf("exit code = 0, want non-zero\nstdout: %s", res.stdout)
	}
	// --json is set, so the one error is the JSON error object on stderr.
	doc := lastJSONError(t, res.stderr)
	if doc.Code != res.code {
		t.Errorf(`"code" = %d, exit code = %d: they must match`, doc.Code, res.code)
	}
	if res.stdout != "" {
		t.Errorf("stdout = %q, want empty (no worktree, no result)", res.stdout)
	}
	if n := strings.Count(doc.Message, `"config/*.key" has a glob character`); n != 1 {
		t.Errorf("the reason appears %d times in the message, want once:\n%s", n, doc.Message)
	}
	if strings.Contains(res.stderr, "using defaults") {
		t.Errorf("stderr says it is using defaults, but the command refused to:\n%s", res.stderr)
	}
	if !strings.HasPrefix(doc.Message, "grove did not create the worktree: the config failed to load: plugins.docker.external.copy_files[0]") {
		t.Errorf("message does not say why no worktree was created:\n%s", doc.Message)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(repo), "myapp-newwork")); !os.IsNotExist(err) {
		t.Errorf("a worktree directory exists after a failed config load (stat err = %v)", err)
	}
	if out := testhelper.RunGit(t, repo, "branch", "--list", "newwork"); strings.TrimSpace(out) != "" {
		t.Errorf("branch newwork was created: %q", out)
	}
}

func TestDriftNotice_NamesMissingFiles(t *testing.T) {
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)
	writeExternalConfig(t, repo, `copy_files = ["config/database.yml", ".env.local"]`+"\n"+`symlink_dirs = ["node_modules"]`)

	outside := filepath.Join(filepath.Dir(repo), "plain-worktree")
	testhelper.RunGit(t, repo, "worktree", "add", "-b", "plain", outside)

	res := runGrove(t, binary, outside, "ls")

	for _, want := range []string{
		"2 configured files are missing here: .env.local, node_modules",
		"grove adopt",
	} {
		if !strings.Contains(res.stderr, want) {
			t.Errorf("stderr missing %q\nstderr: %s", want, res.stderr)
		}
	}
}
