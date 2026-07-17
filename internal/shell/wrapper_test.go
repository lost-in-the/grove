package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildFakeGrove compiles the test binary and returns its path.
func buildFakeGrove(t *testing.T) string {
	return buildFakeGroveNamed(t, "fakegrove")
}

// buildFakeGroveNamed compiles the test binary under the given name.
// Use name "grove" to put a real `grove` executable on PATH for tests that
// exercise the integration's own binary resolution.
func buildFakeGroveNamed(t *testing.T, name string) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), name)
	src := filepath.Join("testdata", "fakegrove.go")
	cmd := exec.Command("go", "build", "-o", binPath, src)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build fakegrove: %v", err)
	}
	return binPath
}

// runZshWrapper sources the zsh template with __GROVE_BIN pointed at fakegrove,
// then runs the given grove invocation and returns stdout, stderr, exit code.
func runZshWrapper(t *testing.T, binPath string, groveArgs string, env ...string) (string, string, int) {
	t.Helper()

	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not available")
	}

	// Extract only the grove() function from the template (skip compdef/completion)
	funcEnd := strings.Index(zshTemplate, "\n# Tab completion")
	wrapperFunc := zshTemplate
	if funcEnd > 0 {
		wrapperFunc = zshTemplate[:funcEnd]
	}

	// Build a zsh script that defines the wrapper and invokes it
	script := `
__GROVE_BIN="` + binPath + `"
` + wrapperFunc + `
# Invoke the wrapper
grove ` + groveArgs + `
`
	cmd := exec.Command("zsh", "-c", script)
	cmd.Env = append(os.Environ(), env...)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run zsh wrapper: %v", err)
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

func TestWrapper_BareInvocation_RunsDirectly(t *testing.T) {
	binPath := buildFakeGrove(t)

	// Bare "grove" (no args) should run the binary directly — not captured.
	// fakegrove prints "TUI_RENDERED" to stdout in this case.
	stdout, stderr, exitCode := runZshWrapper(t, binPath, "" /* no args */, "GROVE_DEBUG=1")

	t.Logf("stdout: %q", stdout)
	t.Logf("stderr: %q", stderr)
	t.Logf("exit: %d", exitCode)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	// The key assertion: TUI_RENDERED must appear in stdout because the
	// wrapper runs the binary directly (not captured in a subshell).
	if !strings.Contains(stdout, "TUI_RENDERED") {
		t.Errorf("bare grove should print TUI_RENDERED directly to stdout, got: %q", stdout)
	}

	// Debug log should show TUI mode
	if !strings.Contains(stderr, "TUI mode") {
		t.Errorf("expected debug log about TUI mode, got stderr: %q", stderr)
	}
}

func TestWrapper_SubcommandLs_Passthrough(t *testing.T) {
	binPath := buildFakeGrove(t)

	stdout, _, exitCode := runZshWrapper(t, binPath, "ls")

	t.Logf("stdout: %q", stdout)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	// ls runs as passthrough (not captured), output appears directly
	if !strings.Contains(stdout, "root") {
		t.Errorf("expected 'root' in ls output, got: %q", stdout)
	}
	if !strings.Contains(stdout, "feature-auth") {
		t.Errorf("expected 'feature-auth' in ls output, got: %q", stdout)
	}
}

func TestWrapper_Passthrough_RunsDirectly(t *testing.T) {
	binPath := buildFakeGrove(t)

	// "version" is a non-directive command — should run directly (passthrough)
	stdout, _, exitCode := runZshWrapper(t, binPath, "version")

	t.Logf("stdout: %q", stdout)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	if !strings.Contains(stdout, "grove v1.0.0-test") {
		t.Errorf("expected 'grove v1.0.0-test' in passthrough output, got: %q", stdout)
	}
}

func TestWrapper_Passthrough_StderrSeparate(t *testing.T) {
	binPath := buildFakeGrove(t)

	// "logs" produces both stdout and stderr — passthrough should keep them separate
	stdout, stderr, exitCode := runZshWrapper(t, binPath, "logs")

	t.Logf("stdout: %q", stdout)
	t.Logf("stderr: %q", stderr)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	// stdout should have the log lines
	if !strings.Contains(stdout, "line1: starting up") {
		t.Errorf("expected 'line1: starting up' in stdout, got: %q", stdout)
	}

	// stderr should be separate (not merged into stdout like the old 2>&1 behavior)
	if !strings.Contains(stderr, "stderr: debug info") {
		t.Errorf("expected 'stderr: debug info' in stderr, got: %q", stderr)
	}
	if strings.Contains(stdout, "stderr: debug info") {
		t.Errorf("stderr content should NOT appear in stdout for passthrough commands, got: %q", stdout)
	}
}

func TestWrapper_ToCommand_ParsesCdDirective(t *testing.T) {
	binPath := buildFakeGrove(t)

	// "grove to testing" should emit cd:/tmp/fakegrove-testing
	// The wrapper should parse that and NOT print it to stdout.
	// (It would try to cd, which will fail since the dir doesn't exist,
	//  but we're checking the parsing, not the cd.)

	// Create the target directory so cd succeeds
	targetDir := "/tmp/fakegrove-testing"
	_ = os.MkdirAll(targetDir, 0755)
	defer func() { _ = os.RemoveAll(targetDir) }()

	// Don't use GROVE_DEBUG here — debug stderr gets merged into captured
	// output by the wrapper's 2>&1, which would contain "cd:" in log lines.
	stdout, stderr, exitCode := runZshWrapper(t, binPath, "to testing")

	t.Logf("stdout: %q", stdout)
	t.Logf("stderr: %q", stderr)
	t.Logf("exit: %d", exitCode)

	// cd: directive should NOT appear in stdout (wrapper consumed it)
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if strings.HasPrefix(line, "cd:") {
			t.Errorf("cd: directive should be consumed by wrapper, but appeared in stdout: %q", line)
		}
	}
}

func TestWrapper_NewCommand_ParsesCdDirective(t *testing.T) {
	binPath := buildFakeGrove(t)

	// "grove new" emits a cd: directive when outside tmux (or --no-tmux).
	// Regression test: "new" was missing from the wrapper's capture case
	// list, so the directive printed raw and the shell never cd'd.
	targetDir := "/tmp/fakegrove-myfeature"
	_ = os.MkdirAll(targetDir, 0755)
	defer func() { _ = os.RemoveAll(targetDir) }()

	stdout, stderr, exitCode := runZshWrapper(t, binPath, "new myfeature")

	t.Logf("stdout: %q", stdout)
	t.Logf("stderr: %q", stderr)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	// Progress output should still appear
	if !strings.Contains(stdout, "Created worktree fakegrove-myfeature") {
		t.Errorf("expected progress output in stdout, got: %q", stdout)
	}

	// cd: directive must be consumed by the wrapper, not printed
	if strings.Contains(stdout, "cd:/tmp") {
		t.Errorf("cd: directive leaked to stdout: %q", stdout)
	}
}

func TestWrapper_IssuesBrowser_RoutesCdThroughFile(t *testing.T) {
	binPath := buildFakeGrove(t)

	// B27: `grove issues`/`grove prs` used to run in passthrough, so the cd:
	// directive they emit when a worktree is selected printed raw on the
	// terminal instead of changing directory. The wrapper now runs them with
	// GROVE_CD_FILE, so the directive is consumed via the file, not stdout.
	stdout, _, exitCode := runZshWrapper(t, binPath, "issues")
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "BROWSER_RENDERED") {
		t.Errorf("expected browser output in stdout, got: %q", stdout)
	}
	if strings.Contains(stdout, "cd:/tmp") {
		t.Errorf("cd: directive leaked to stdout for issues browser: %q", stdout)
	}
}

func TestWrapper_IssuesPrs_UseCdFile(t *testing.T) {
	// Both wrappers must run issues/prs through the GROVE_CD_FILE path.
	for _, tmpl := range []struct{ name, body string }{{"zsh", zshTemplate}, {"bash", bashTemplate}} {
		idx := strings.Index(tmpl.body, "issues|prs)")
		if idx < 0 {
			t.Errorf("%s template missing issues|prs case", tmpl.name)
			continue
		}
		end := strings.Index(tmpl.body[idx:], ";;")
		if end < 0 {
			t.Errorf("%s issues|prs case has no terminator", tmpl.name)
			continue
		}
		if !strings.Contains(tmpl.body[idx:idx+end], "GROVE_CD_FILE") {
			t.Errorf("%s issues|prs case does not use GROVE_CD_FILE", tmpl.name)
		}
	}
}

func TestWrapper_NewAliases_AreCaptured(t *testing.T) {
	// grove new's aliases (spawn, n) must also be in the capture case list,
	// otherwise the cd: directive prints raw when invoked via an alias.
	if !strings.Contains(zshTemplate, "new|spawn|n|") {
		t.Error("zsh template capture list missing new|spawn|n")
	}
	if !strings.Contains(bashTemplate, "new|spawn|n|") {
		t.Error("bash template capture list missing new|spawn|n")
	}
}

func TestWrapper_ForkCommand_SeparatesDirectivesFromText(t *testing.T) {
	binPath := buildFakeGrove(t)

	targetDir := "/tmp/fakegrove-mixed"
	_ = os.MkdirAll(targetDir, 0755)
	defer func() { _ = os.RemoveAll(targetDir) }()

	// fork is a directive command — mixed output should have cd: parsed out
	stdout, _, exitCode := runZshWrapper(t, binPath, "fork mixed")

	t.Logf("stdout: %q", stdout)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	// Non-directive lines should appear
	if !strings.Contains(stdout, "some output before") {
		t.Errorf("expected 'some output before' in output, got: %q", stdout)
	}
	if !strings.Contains(stdout, "some output after") {
		t.Errorf("expected 'some output after' in output, got: %q", stdout)
	}

	// cd: directive should be consumed (not leaked to stdout)
	if strings.Contains(stdout, "cd:/tmp") {
		t.Errorf("cd: directive leaked to stdout: %q", stdout)
	}
}

func TestWrapper_BareInvocation_WritesCdFile(t *testing.T) {
	binPath := buildFakeGrove(t)

	// Create a target directory for the cd
	targetDir := filepath.Join(t.TempDir(), "cd-target")
	_ = os.MkdirAll(targetDir, 0755)

	// The wrapper creates its own cd file, but fakegrove needs FAKEGROVE_CD_TARGET
	// to simulate a TUI selection writing to the cd file.
	// We test the full flow: wrapper creates temp file, passes via GROVE_CD_FILE,
	// fakegrove writes target to it, wrapper reads and cd's.
	stdout, stderr, exitCode := runZshWrapper(t, binPath, "", /* no args */
		"GROVE_DEBUG=1",
		"FAKEGROVE_CD_TARGET="+targetDir,
	)

	t.Logf("stdout: %q", stdout)
	t.Logf("stderr: %q", stderr)
	t.Logf("exit: %d", exitCode)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	// TUI output should still appear (binary runs directly, not captured)
	if !strings.Contains(stdout, "TUI_RENDERED") {
		t.Errorf("expected TUI_RENDERED in stdout, got: %q", stdout)
	}
}

func TestWrapper_BareInvocation_NoCd_WhenEmpty(t *testing.T) {
	binPath := buildFakeGrove(t)

	// Without FAKEGROVE_CD_TARGET, fakegrove won't write to the cd file.
	// The wrapper should not attempt to cd.
	stdout, _, exitCode := runZshWrapper(t, binPath, "" /* no args */)

	t.Logf("stdout: %q", stdout)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	if !strings.Contains(stdout, "TUI_RENDERED") {
		t.Errorf("expected TUI_RENDERED in stdout, got: %q", stdout)
	}
}

func TestWrapper_RecursionGuard_BinaryNotFound(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not available")
	}

	// Simulate the failure mode: __GROVE_BIN set to bare "grove" (what
	// happens when command -v grove fails and falls back to echo grove).
	// Without the guard, the grove() function calls itself infinitely.
	funcEnd := strings.Index(zshTemplate, "\n# Tab completion")
	wrapperFunc := zshTemplate
	if funcEnd > 0 {
		wrapperFunc = zshTemplate[:funcEnd]
	}

	script := `__GROVE_BIN="grove"
` + wrapperFunc + `
grove version 2>/tmp/grove-recursion-test-stderr
echo "exit:$?"
`
	cmd := exec.Command("zsh", "-c", script)
	var stdout strings.Builder
	cmd.Stdout = &stdout

	_ = cmd.Run()

	stderrBytes, _ := os.ReadFile("/tmp/grove-recursion-test-stderr")
	stderr := string(stderrBytes)
	_ = os.Remove("/tmp/grove-recursion-test-stderr")

	t.Logf("stdout: %q", stdout.String())
	t.Logf("stderr: %q", stderr)

	// Must NOT get the recursion error
	if strings.Contains(stderr, "job table full") || strings.Contains(stderr, "recursion") {
		t.Errorf("recursion guard failed — got infinite recursion error")
	}

	// Should get the clean error message from the guard
	if !strings.Contains(stderr, "binary not found") {
		t.Errorf("expected 'binary not found' warning from recursion guard, got stderr: %q", stderr)
	}

	// Exit code should be 127 (command not found convention)
	if !strings.Contains(stdout.String(), "exit:127") {
		t.Errorf("expected exit code 127 from recursion guard, got stdout: %q", stdout.String())
	}
}

func TestWrapper_RecursionGuard_EmptyBin(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not available")
	}

	// Empty __GROVE_BIN should also be caught
	funcEnd := strings.Index(zshTemplate, "\n# Tab completion")
	wrapperFunc := zshTemplate
	if funcEnd > 0 {
		wrapperFunc = zshTemplate[:funcEnd]
	}

	script := `__GROVE_BIN=""
` + wrapperFunc + `
grove version 2>/tmp/grove-empty-bin-stderr
echo "exit:$?"
`
	cmd := exec.Command("zsh", "-c", script)
	var stdout strings.Builder
	cmd.Stdout = &stdout

	_ = cmd.Run()

	stderrBytes, _ := os.ReadFile("/tmp/grove-empty-bin-stderr")
	stderr := string(stderrBytes)
	_ = os.Remove("/tmp/grove-empty-bin-stderr")

	t.Logf("stdout: %q", stdout.String())
	t.Logf("stderr: %q", stderr)

	if strings.Contains(stderr, "job table full") || strings.Contains(stderr, "recursion") {
		t.Errorf("recursion guard failed for empty __GROVE_BIN")
	}

	if !strings.Contains(stderr, "binary not found") {
		t.Errorf("expected 'binary not found' warning, got stderr: %q", stderr)
	}
}

// runResourcedIntegration sources the full generated integration TWICE in
// one shell (simulating an rc re-source), then runs `grove version`.
// Nothing is stripped: the completion registration must be safe to source
// in a bare non-interactive shell (zsh guards compdef behind a compinit
// check; bash's `complete` is a builtin).
func runResourcedIntegration(t *testing.T, shellBin, integration, binDir string) (string, string) {
	t.Helper()

	if _, err := exec.LookPath(shellBin); err != nil {
		t.Skipf("%s not available", shellBin)
	}

	script := `export PATH="` + binDir + `:$PATH"
` + integration + `
` + integration + `
grove version
echo "exit:$?"
`
	cmd := exec.Command(shellBin, "-c", script)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()
	return stdout.String(), stderr.String()
}

// Re-sourcing the rc file must not disable the integration: the second
// source runs the binary resolver while the grove() function from the first
// source is already defined, and the resolver must still find the BINARY,
// not the function (#137).
func TestWrapper_ResourceIdempotent_Zsh(t *testing.T) {
	binDir := filepath.Dir(buildFakeGroveNamed(t, "grove"))

	integration, err := GenerateZshIntegration("")
	if err != nil {
		t.Fatalf("GenerateZshIntegration error = %v", err)
	}

	stdout, stderr := runResourcedIntegration(t, "zsh", integration, binDir)
	t.Logf("stdout: %q", stdout)
	t.Logf("stderr: %q", stderr)

	if strings.Contains(stderr, "binary not found") {
		t.Errorf("re-source tripped the recursion guard, stderr: %q", stderr)
	}
	if !strings.Contains(stdout, "grove v1.0.0-test") {
		t.Errorf("expected wrapper to run the binary after re-source, stdout: %q", stdout)
	}
	if !strings.Contains(stdout, "exit:0") {
		t.Errorf("expected exit 0 after re-source, stdout: %q", stdout)
	}
}

func TestWrapper_ResourceIdempotent_Bash(t *testing.T) {
	binDir := filepath.Dir(buildFakeGroveNamed(t, "grove"))

	integration, err := GenerateBashIntegration("")
	if err != nil {
		t.Fatalf("GenerateBashIntegration error = %v", err)
	}

	stdout, stderr := runResourcedIntegration(t, "bash", integration, binDir)
	t.Logf("stdout: %q", stdout)
	t.Logf("stderr: %q", stderr)

	if strings.Contains(stderr, "binary not found") {
		t.Errorf("re-source tripped the recursion guard, stderr: %q", stderr)
	}
	if !strings.Contains(stdout, "grove v1.0.0-test") {
		t.Errorf("expected wrapper to run the binary after re-source, stdout: %q", stdout)
	}
	if !strings.Contains(stdout, "exit:0") {
		t.Errorf("expected exit 0 after re-source, stdout: %q", stdout)
	}
}

// The full integration (completion registration included) must be sourceable
// in a shell where compinit has NOT run — e.g. an eval line placed above
// compinit in the zshrc. Registration degrades to no completion instead of
// erroring with `command not found: compdef`.
func TestWrapper_FullIntegration_SourcesWithoutCompinit(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not available")
	}
	binDir := filepath.Dir(buildFakeGroveNamed(t, "grove"))

	integration, err := GenerateZshIntegration("")
	if err != nil {
		t.Fatalf("GenerateZshIntegration error = %v", err)
	}

	script := `export PATH="` + binDir + `:$PATH"
` + integration + `
grove version
echo "exit:$?"
`
	cmd := exec.Command("zsh", "-c", script)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()

	t.Logf("stdout: %q", stdout.String())
	t.Logf("stderr: %q", stderr.String())

	if strings.Contains(stderr.String(), "compdef") || strings.Contains(stderr.String(), "command not found") {
		t.Errorf("sourcing without compinit must not error, stderr: %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "exit:0") {
		t.Errorf("expected exit 0, stdout: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "grove v1.0.0-test") {
		t.Errorf("wrapper should still run the binary, stdout: %q", stdout.String())
	}
}

// When compinit IS loaded before the integration, completion must still
// register — the guard must not turn into a silent never-registers.
func TestWrapper_CompletionRegisters_WithCompinit(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not available")
	}
	binDir := filepath.Dir(buildFakeGroveNamed(t, "grove"))

	integration, err := GenerateZshIntegration("")
	if err != nil {
		t.Fatalf("GenerateZshIntegration error = %v", err)
	}

	script := `export PATH="` + binDir + `:$PATH"
autoload -Uz compinit && compinit -u
` + integration + `
whence -w _grove_completion
echo "exit:$?"
`
	cmd := exec.Command("zsh", "-c", script)
	// compinit writes ~/.zcompdump — keep it out of the developer's real HOME.
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), "ZDOTDIR="+t.TempDir())
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()

	t.Logf("stdout: %q", stdout.String())
	t.Logf("stderr: %q", stderr.String())

	if !strings.Contains(stdout.String(), "_grove_completion: function") {
		t.Errorf("completion function should be defined with compinit loaded, stdout: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "exit:0") {
		t.Errorf("expected exit 0, stdout: %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "command not found") {
		t.Errorf("unexpected error during registration, stderr: %q", stderr.String())
	}
}

func TestWrapper_FailCommand_PropagatesExitCode(t *testing.T) {
	binPath := buildFakeGrove(t)

	_, _, exitCode := runZshWrapper(t, binPath, "failnow")

	if exitCode == 0 {
		t.Errorf("expected non-zero exit code for fail command")
	}
}
