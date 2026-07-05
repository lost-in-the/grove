package docker

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
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

	action := resolveFromConfig(s.getContainerSwitch(ctx))
	if action == ContainerSwitchOff {
		return nil
	}

	worktreePath := s.getWorktreePath(ctx.PrevWorktree)
	if !hasDockerCompose(worktreePath) {
		return nil
	}

	if action == ContainerSwitchPrompt {
		yes, err := cli.Confirm("Stop containers in previous worktree?", false)
		if err != nil || !yes {
			return nil
		}
	}

	return s.down(worktreePath)
}

func (s *localStrategy) OnPostSwitch(ctx *hooks.Context) error {
	if !s.getAutoStart() {
		return nil
	}

	action := resolveFromConfig(s.getContainerSwitch(ctx))
	if action == ContainerSwitchOff {
		return nil
	}

	worktreePath := s.getWorktreePath(ctx.Worktree)
	if !hasDockerCompose(worktreePath) {
		return nil
	}

	if action == ContainerSwitchPrompt {
		yes, err := cli.Confirm("Start containers for this worktree?", true)
		if err != nil || !yes {
			return nil
		}
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

	cmd := composeCommand(worktreePath, "", nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *localStrategy) Down(worktreePath string) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	cmd := composeCommand(worktreePath, "", nil, "down")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *localStrategy) Logs(worktreePath string, service string, follow bool) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if service != "" {
		args = append(args, service)
	}

	cmd := composeCommand(worktreePath, "", nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (s *localStrategy) Run(worktreePath string, service string, command string) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	args := buildRunArgs(s.cfg, worktreePath, service, command)
	cmd := composeCommand(worktreePath, "", nil, args...)
	return runWithErrorTranslation(cmd, s.cfg.Test.IncludeDepsValue())
}

// Exec runs a command in an already-running container (compose exec).
func (s *localStrategy) Exec(worktreePath string, service string, command string) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	cmd := composeCommand(worktreePath, "", nil, "exec", service, "bash", "-cil", command)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (s *localStrategy) Restart(worktreePath string, service string) error {
	if !hasDockerCompose(worktreePath) {
		return ErrNoComposeFile
	}

	args := []string{"restart"}
	if service != "" {
		args = append(args, service)
	}

	cmd := composeCommand(worktreePath, "", nil, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *localStrategy) down(worktreePath string) error {
	return s.Down(worktreePath)
}

func (s *localStrategy) up(worktreePath string, detach bool) error {
	return s.Up(worktreePath, detach)
}

func (s *localStrategy) getAutoStart() bool {
	if s.cfg != nil && s.cfg.Plugins.Docker.AutoStart != nil {
		return *s.cfg.Plugins.Docker.AutoStart
	}
	return true
}

func (s *localStrategy) getContainerSwitch(ctx *hooks.Context) string {
	if ctx.Config != nil {
		return ctx.Config.Switch.ContainerSwitch
	}
	return ""
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
		return filepath.Join(resolveComposePath(s.cfg.ProjectsDir), name)
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

// composeEnvFileArgs returns --env-file arguments for `docker compose` v2 that
// layer the default .env (if present in composePath) underneath the configured
// envFile. Returns nil when envFile is unset or already ".env" (compose reads
// .env by default in that case).
//
// Compose v2 supports multiple --env-file flags; later ones override earlier
// ones for interpolation. Without this layering, setting env_file=".env.local"
// would cause `docker compose` to ignore .env entirely, silently breaking
// interpolation of variables defined only in .env (issue #98).
func composeEnvFileArgs(composePath, envFile string) []string {
	if envFile == "" || envFile == ".env" {
		return nil
	}
	var args []string
	if _, err := os.Stat(filepath.Join(composePath, ".env")); err == nil {
		args = append(args, "--env-file", ".env")
	}
	args = append(args, "--env-file", envFile)
	return args
}

// composeCommand creates a `docker compose` (v2) command with optional
// environment variables. dir sets the working directory. envFile, when
// non-empty and not ".env", adds --env-file to the compose command for YAML
// variable interpolation; if a .env file also exists alongside, it is layered
// underneath so compose still reads defaults from .env. env is a list of
// extra KEY=VALUE strings to add to the process environment.
//
// The standalone docker-compose v1 binary is not supported: v1 is EOL and
// only accepts a single --env-file, which would silently break the .env
// layering above (issue #107).
//
// Callers set cmd.Stdout = os.Stderr intentionally: grove reserves stdout for
// shell-integration directives (cd:, env:, tmux-attach:), so Docker output
// must go to stderr to avoid corrupting the directive stream.
func composeCommand(dir string, envFile string, env []string, args ...string) *exec.Cmd {
	cmdArgs := []string{"compose"}
	cmdArgs = append(cmdArgs, composeEnvFileArgs(dir, envFile)...)
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("docker", cmdArgs...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd
}
