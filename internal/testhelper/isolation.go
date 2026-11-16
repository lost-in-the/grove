package testhelper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolatedHome is set by RunIsolated and read by AssertHomeIsolated.
var isolatedHome string

// RunIsolated runs a package's tests with HOME, XDG_CONFIG_HOME, and git's
// global config pointed at a throwaway directory, and returns m.Run's exit
// code. Call it from TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(testhelper.RunIsolated(m)) }
//
// grove resolves its global config, global hooks.toml, last-session file,
// update cache, and logs from $HOME, so without this a test run reads (and can
// execute hooks from) the developer's real ~/.config/grove.
func RunIsolated(m *testing.M) int {
	dir, err := os.MkdirTemp("", "grove-test-home-")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	// macOS temp dirs are symlinks; resolve so path comparisons are stable.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	gitConfig := filepath.Join(dir, "gitconfig")
	const neutralGitConfig = "[user]\n\tname = Test User\n\temail = test@test.com\n[init]\n\tdefaultBranch = main\n[commit]\n\tgpgsign = false\n"
	if err := os.WriteFile(gitConfig, []byte(neutralGitConfig), 0o600); err != nil {
		panic(err)
	}

	for k, v := range map[string]string{
		"HOME":                dir,
		"XDG_CONFIG_HOME":     filepath.Join(dir, ".config"),
		"GIT_CONFIG_GLOBAL":   gitConfig,
		"GIT_CONFIG_NOSYSTEM": "1",
	} {
		if err := os.Setenv(k, v); err != nil {
			panic(err)
		}
	}
	// A developer's override would bypass the fake HOME.
	_ = os.Unsetenv("GROVE_CONFIG")

	isolatedHome = dir
	return m.Run()
}

// AssertHomeIsolated fails the test unless the process runs under
// RunIsolated's fake HOME, i.e. the package has a TestMain that calls it.
func AssertHomeIsolated(t *testing.T) {
	t.Helper()
	if isolatedHome == "" {
		t.Fatal("package TestMain does not call testhelper.RunIsolated: tests can read the real home directory")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if home != isolatedHome {
		t.Errorf("UserHomeDir() resolves outside the isolated home (got a different directory than the fake one)")
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); !strings.HasPrefix(xdg, isolatedHome) {
		t.Errorf("XDG_CONFIG_HOME is outside the isolated home")
	}
	if os.Getenv("GROVE_CONFIG") != "" {
		t.Errorf("GROVE_CONFIG is set, which bypasses the isolated home")
	}
}
