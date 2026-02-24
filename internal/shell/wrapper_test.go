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
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "fakegrove")
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

func TestWrapper_FailCommand_PropagatesExitCode(t *testing.T) {
	binPath := buildFakeGrove(t)

	_, _, exitCode := runZshWrapper(t, binPath, "failnow")

	if exitCode == 0 {
		t.Errorf("expected non-zero exit code for fail command")
	}
}
