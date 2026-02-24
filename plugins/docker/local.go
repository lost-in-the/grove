package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
)

// localStrategy implements the docker mode for projects with their own docker-compose file.
type localStrategy struct {
	cfg *config.Config
}

func newLocalStrategy(cfg *config.Config) *localStrategy {
	return &localStrategy{cfg: cfg}
}

func (s *localStrategy) OnPreSwitch(ctx *hooks.Context) error {
	if !s.getAutoStop() {
		return nil
	}

	worktreePath := s.getWorktreePath(ctx.PrevWorktree)
	if !hasDockerCompose(worktreePath) {
		return nil
	}

	return s.down(worktreePath)
}

func (s *localStrategy) OnPostSwitch(ctx *hooks.Context) error {
	if !s.getAutoStart() {
		return nil
	}

	worktreePath := s.getWorktreePath(ctx.Worktree)
	if !hasDockerCompose(worktreePath) {
		return nil
	}

	return s.up(worktreePath, false)
}

func (s *localStrategy) OnPostCreate(_ *hooks.Context) error {
	// Local mode has no post-create behavior
	return nil
}

func (s *localStrategy) Up(worktreePath string, detach bool) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	args := []string{"up"}
	if detach {
		args = append(args, "-d")
	}

	cmd := composeCommand(worktreePath, nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *localStrategy) Down(worktreePath string) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	cmd := composeCommand(worktreePath, nil, "down")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *localStrategy) Logs(worktreePath string, service string, follow bool) error {
	if !hasDockerCompose(worktreePath) {
		return fmt.Errorf("no docker-compose file found in %s", worktreePath)
	}

	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if service != "" {
		args = append(args, service)
	}

	cmd := composeCommand(worktreePath, nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (s *localStrategy) Run(worktreePath string, service string, command string) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	cmd := composeCommand(worktreePath, nil, "run", "--rm", service, "bash", "-cil", command)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (s *localStrategy) Restart(worktreePath string, service string) error {
	if !hasDockerCompose(worktreePath) {
		return fmt.Errorf("no docker-compose file found in %s", worktreePath)
	}

	args := []string{"restart"}
	if service != "" {
		args = append(args, service)
	}

	cmd := composeCommand(worktreePath, nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *localStrategy) down(worktreePath string) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	cmd := composeCommand(worktreePath, nil, "down")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *localStrategy) up(worktreePath string, detach bool) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	args := []string{"up"}
	if detach {
		args = append(args, "-d")
	}

	cmd := composeCommand(worktreePath, nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *localStrategy) getAutoStart() bool {
	if s.cfg != nil && s.cfg.Plugins.Docker.AutoStart != nil {
		return *s.cfg.Plugins.Docker.AutoStart
	}
	return true
}

func (s *localStrategy) getAutoStop() bool {
	if s.cfg != nil && s.cfg.Plugins.Docker.AutoStop != nil {
		return *s.cfg.Plugins.Docker.AutoStop
	}
	return false
}

func (s *localStrategy) getWorktreePath(name string) string {
	if name == "" {
		return ""
	}

	if filepath.IsAbs(name) {
		return name
	}

	if s.cfg != nil && s.cfg.ProjectsDir != "" {
		projectsDir := s.cfg.ProjectsDir
		if strings.HasPrefix(projectsDir, "~/") {
			home, err := os.UserHomeDir()
			if err == nil {
				projectsDir = filepath.Join(home, projectsDir[2:])
			}
		}
		return filepath.Join(projectsDir, name)
	}

	cwd, _ := os.Getwd()
	return filepath.Join(cwd, name)
}

// hasDockerCompose checks if a directory has a docker-compose file
func hasDockerCompose(dir string) bool {
	composeFiles := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, file := range composeFiles {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// composeCommand creates a docker-compose command with optional environment variables.
// dir sets the working directory. env is a list of extra KEY=VALUE strings to add.
func composeCommand(dir string, env []string, args ...string) *exec.Cmd {
	if _, err := exec.LookPath("docker"); err == nil {
		cmdArgs := append([]string{"compose"}, args...)
		cmd := exec.Command("docker", cmdArgs...)
		cmd.Dir = dir
		if len(env) > 0 {
			cmd.Env = append(os.Environ(), env...)
		}
		return cmd
	}

	cmd := exec.Command("docker-compose", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd
}
