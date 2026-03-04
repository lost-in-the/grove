package docker

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeahArmstrong/grove-cli/internal/cli"
	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

// externalStrategy implements the docker mode for projects whose services are defined
// in an external, shared docker-compose setup (e.g., a central shared-infra directory).
type externalStrategy struct {
	cfg *config.Config
	ext *config.ExternalComposeConfig
}

func newExternalStrategy(cfg *config.Config) *externalStrategy {
	return &externalStrategy{
		cfg: cfg,
		ext: cfg.Plugins.Docker.External,
	}
}

func (s *externalStrategy) OnPreSwitch(ctx *hooks.Context) error {
	if !s.getAutoStop() {
		return nil
	}

	return s.stopServices()
}

func (s *externalStrategy) OnPostSwitch(ctx *hooks.Context) error {
	worktreePath := ctx.WorktreePath
	if worktreePath == "" {
		return fmt.Errorf("worktree path not available in hook context")
	}

	// Always persist the env var so manual docker compose commands use the right directory.
	// This runs even when auto_start is disabled — the env file must stay in sync.
	if err := s.persistEnvVar(worktreePath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to persist %s to %s: %v\n", s.ext.EnvVar, s.ext.EnvFileName(), err)
	}

	// Export the env var into the user's shell so manual docker compose commands work
	s.emitEnvDirective(worktreePath)

	if !s.getAutoStart() {
		return nil
	}

	return s.startServices(worktreePath)
}

func (s *externalStrategy) OnPostCreate(ctx *hooks.Context) error {
	if ctx.WorktreePath == "" || ctx.MainPath == "" {
		return nil
	}

	if err := s.persistEnvVar(ctx.WorktreePath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to persist %s to %s: %v\n", s.ext.EnvVar, s.ext.EnvFileName(), err)
	}
	s.emitEnvDirective(ctx.WorktreePath)

	return setupWorktreeFiles(s.ext, ctx.WorktreePath, ctx.MainPath)
}

func (s *externalStrategy) Up(worktreePath string, detach bool) error {
	// Persist so manual docker compose commands also resolve to this worktree
	if err := s.persistEnvVar(worktreePath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to persist %s to %s: %v\n", s.ext.EnvVar, s.ext.EnvFileName(), err)
	}
	s.emitEnvDirective(worktreePath)

	args := []string{"up"}
	if detach {
		args = append(args, "-d")
	}
	args = append(args, s.ext.Services...)

	cmd := composeCommand(s.composePath(), s.ext.EnvFileName(), s.envForWorktree(worktreePath), args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *externalStrategy) Down(_ string) error {
	return s.stopServices()
}

func (s *externalStrategy) Logs(_ string, service string, follow bool) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if service != "" {
		args = append(args, service)
	} else {
		args = append(args, s.ext.Services...)
	}

	cmd := composeCommand(s.composePath(), s.ext.EnvFileName(), nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (s *externalStrategy) Run(worktreePath string, service string, command string) error {
	// Persist so the env file stays consistent with what we're running against
	if err := s.persistEnvVar(worktreePath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to persist %s to %s: %v\n", s.ext.EnvVar, s.ext.EnvFileName(), err)
	}
	s.emitEnvDirective(worktreePath)

	env := s.envForWorktree(worktreePath)

	// Add TEST_ENV_NUMBER for test commands so parallel test runs use isolated DB slots
	if isTestCommand(command) {
		wtName := filepath.Base(worktreePath)
		envNum := worktree.TestEnvNumber(wtName)
		env = append(env, fmt.Sprintf("TEST_ENV_NUMBER=%d", envNum))
	}

	cmd := composeCommand(s.composePath(), s.ext.EnvFileName(), env, "run", "--rm", service, "bash", "-cil", command)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// isTestCommand reports whether the given command string looks like a test invocation.
func isTestCommand(cmd string) bool {
	testPatterns := []string{
		"rspec",
		"rails test",
		"bin/rails test",
		"rake test",
		"minitest",
		"bin/rspec",
		"bundle exec rspec",
	}
	for _, pattern := range testPatterns {
		if strings.Contains(cmd, pattern) {
			return true
		}
	}
	return false
}

func (s *externalStrategy) Restart(_ string, service string) error {
	args := []string{"restart"}
	if service != "" {
		args = append(args, service)
	} else {
		args = append(args, s.ext.Services...)
	}

	cmd := composeCommand(s.composePath(), s.ext.EnvFileName(), nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// stopServices stops (not removes) the configured services in the external compose.
func (s *externalStrategy) stopServices() error {
	args := append([]string{"stop"}, s.ext.Services...)
	cmd := composeCommand(s.composePath(), s.ext.EnvFileName(), nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// startServices starts the configured services with the env var pointing to the worktree.
func (s *externalStrategy) startServices(worktreePath string) error {
	args := append([]string{"up", "-d"}, s.ext.Services...)
	cmd := composeCommand(s.composePath(), s.ext.EnvFileName(), s.envForWorktree(worktreePath), args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// emitEnvDirective emits an env: shell directive so the user's shell exports
// the env var after grove commands. Only emitted under shell integration.
func (s *externalStrategy) emitEnvDirective(worktreePath string) {
	rel := s.relativeWorktreePath(worktreePath)
	cli.EnvDirective(s.ext.EnvVar, rel)
}

// composePath returns the resolved absolute path to the external compose directory.
func (s *externalStrategy) composePath() string {
	return resolveComposePath(s.ext.Path)
}

// envFilePath returns the full path to the env file where the env var is persisted.
func (s *externalStrategy) envFilePath() string {
	return filepath.Join(s.composePath(), s.ext.EnvFileName())
}

// persistEnvVar writes the env_var (e.g., APP_DIR) to the configured env file in
// the compose directory so that subsequent docker compose commands use the correct value.
func (s *externalStrategy) persistEnvVar(worktreePath string) error {
	envFile := s.envFilePath()
	key := s.ext.EnvVar
	rel := s.relativeWorktreePath(worktreePath)
	line := key + "=" + rel

	content, err := os.ReadFile(envFile)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(envFile, []byte(line+"\n"), 0644)
		}
		return fmt.Errorf("failed to read %s: %w", s.ext.EnvFileName(), err)
	}

	lines := strings.Split(string(content), "\n")
	prefix := key + "="
	found := false
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			if lines[i] == line {
				return nil // skip write — value unchanged
			}
			lines[i] = line
			found = true
			break
		}
	}

	if !found {
		// Insert before trailing empty line (preserving final newline)
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = append(lines[:len(lines)-1], line, "")
		} else {
			lines = append(lines, line)
		}
	}

	return os.WriteFile(envFile, []byte(strings.Join(lines, "\n")), 0644)
}

// removeEnvVar removes the env_var entry from the env file if its value matches
// the given worktree. Called on grove rm to clean up stale entries.
func (s *externalStrategy) removeEnvVar(worktreePath string) error {
	envFile := s.envFilePath()
	key := s.ext.EnvVar
	rel := s.relativeWorktreePath(worktreePath)
	expected := key + "=" + rel

	content, err := os.ReadFile(envFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", s.ext.EnvFileName(), err)
	}

	lines := strings.Split(string(content), "\n")
	filtered := make([]string, 0, len(lines))
	removed := false
	for _, l := range lines {
		if l == expected {
			removed = true
			continue
		}
		filtered = append(filtered, l)
	}

	if !removed {
		return nil
	}

	result := strings.Join(filtered, "\n")
	if result == "" {
		result = "\n"
	}
	return os.WriteFile(envFile, []byte(result), 0644)
}

// envForWorktree returns the environment variable setting for the given worktree path.
// The value is the relative path from the compose directory (e.g., "./myapp-feature-x").
func (s *externalStrategy) envForWorktree(worktreePath string) []string {
	rel := s.relativeWorktreePath(worktreePath)
	return []string{s.ext.EnvVar + "=" + rel}
}

// relativeWorktreePath converts an absolute worktree path to a relative path from
// the external compose directory. Returns a ./ prefixed path (e.g., "./myapp-feature-x").
func (s *externalStrategy) relativeWorktreePath(absPath string) string {
	composePath := s.composePath()
	rel, err := filepath.Rel(composePath, absPath)
	if err != nil {
		// Fall back to just the directory name
		return "./" + filepath.Base(absPath)
	}
	if !strings.HasPrefix(rel, ".") {
		rel = "./" + rel
	}
	return rel
}

func (s *externalStrategy) getAutoStart() bool {
	if s.cfg != nil && s.cfg.Plugins.Docker.AutoStart != nil {
		return *s.cfg.Plugins.Docker.AutoStart
	}
	// External mode defaults to true for auto_start
	return true
}

func (s *externalStrategy) getAutoStop() bool {
	if s.cfg != nil && s.cfg.Plugins.Docker.AutoStop != nil {
		return *s.cfg.Plugins.Docker.AutoStop
	}
	// External mode defaults to true for auto_stop (unlike local's false)
	return true
}

// copyFile copies a single file from src to dst, creating parent directories as needed.
func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("source file not found: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// createSymlink creates a symbolic link from src to dst, creating parent directories
// as needed. If dst already exists and is a symlink, it is replaced.
func createSymlink(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source directory not found: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Remove existing symlink if present
	if info, err := os.Lstat(dst); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(dst)
		} else {
			return fmt.Errorf("%s already exists and is not a symlink", dst)
		}
	}

	return os.Symlink(src, dst)
}
