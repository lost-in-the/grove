package docker

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
)

// externalStrategy implements the docker mode for projects whose services are defined
// in an external, shared docker-compose setup (e.g., a central shared-compose directory).
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
	if !s.getAutoStart() {
		return nil
	}

	worktreePath := ctx.WorktreePath
	if worktreePath == "" {
		return fmt.Errorf("worktree path not available in hook context")
	}

	return s.startServices(worktreePath)
}

func (s *externalStrategy) OnPostCreate(ctx *hooks.Context) error {
	if ctx.WorktreePath == "" || ctx.MainPath == "" {
		return nil
	}

	return s.setupWorktree(ctx.WorktreePath, ctx.MainPath)
}

func (s *externalStrategy) Up(worktreePath string, detach bool) error {
	args := []string{"up"}
	if detach {
		args = append(args, "-d")
	}
	args = append(args, s.ext.Services...)

	cmd := composeCommand(s.composePath(), s.envForWorktree(worktreePath), args...)
	cmd.Stdout = os.Stdout
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

	cmd := composeCommand(s.composePath(), nil, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (s *externalStrategy) Run(worktreePath string, service string, command string) error {
	cmd := composeCommand(s.composePath(), s.envForWorktree(worktreePath), "run", "--rm", service, "bash", "-cil", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (s *externalStrategy) Restart(_ string, service string) error {
	args := []string{"restart"}
	if service != "" {
		args = append(args, service)
	} else {
		args = append(args, s.ext.Services...)
	}

	cmd := composeCommand(s.composePath(), nil, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// stopServices stops (not removes) the configured services in the external compose.
func (s *externalStrategy) stopServices() error {
	args := append([]string{"stop"}, s.ext.Services...)
	cmd := composeCommand(s.composePath(), nil, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// startServices starts the configured services with the env var pointing to the worktree.
func (s *externalStrategy) startServices(worktreePath string) error {
	args := append([]string{"up", "-d"}, s.ext.Services...)
	cmd := composeCommand(s.composePath(), s.envForWorktree(worktreePath), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// setupWorktree copies credentials and creates symlinks in a newly created worktree.
func (s *externalStrategy) setupWorktree(newPath, mainPath string) error {
	var firstErr error

	// Copy files from main worktree
	for _, relPath := range s.ext.CopyFiles {
		src := filepath.Join(mainPath, relPath)
		dst := filepath.Join(newPath, relPath)

		if err := copyFile(src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to copy %s: %v\n", relPath, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Printf("  copied %s\n", relPath)
	}

	// Create symlinks to main worktree directories
	for _, relPath := range s.ext.SymlinkDirs {
		src := filepath.Join(mainPath, relPath)
		dst := filepath.Join(newPath, relPath)

		if err := createSymlink(src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to symlink %s: %v\n", relPath, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Printf("  symlinked %s\n", relPath)
	}

	return firstErr
}

// composePath returns the resolved absolute path to the external compose directory.
func (s *externalStrategy) composePath() string {
	path := s.ext.Path
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	return path
}

// envForWorktree returns the environment variable setting for the given worktree path.
// The value is the relative path from the compose directory (e.g., "./admin-feature-x").
func (s *externalStrategy) envForWorktree(worktreePath string) []string {
	rel := s.relativeWorktreePath(worktreePath)
	return []string{s.ext.EnvVar + "=" + rel}
}

// relativeWorktreePath converts an absolute worktree path to a relative path from
// the external compose directory. Returns a ./ prefixed path (e.g., "./admin-feature-x").
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
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

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
			os.Remove(dst)
		} else {
			return fmt.Errorf("%s already exists and is not a symlink", dst)
		}
	}

	return os.Symlink(src, dst)
}
