package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/detect"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/shell"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/plugins/docker"
)

var doctorFix bool

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Apply automatic fixes for detected issues (currently: rewrites host install hooks to docker:compose)")
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system health for grove",
	Long: `Run diagnostic checks to verify that grove's dependencies and configuration
are set up correctly.

System checks (binary, PATH, shell integration, git, tmux) run anywhere.
Project checks (Docker, config, symlinks) run when inside a grove project.

Pass --fix to apply automatic fixes for detected issues. Currently fixable:
  - Host bundle/npm/pip-install hooks in a Docker project are rewritten to
    docker:compose hooks.

Examples:
  grove doctor`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cli.NewStdout()
		cli.Header(w, "grove doctor")

		allPassed := true

		// ── Tier 1: System checks (always run) ──────────────────────

		// Check: Grove binary resolution
		allPassed = runCheck(w, "Grove binary", func() (string, error) {
			return checkGroveBinary(exec.LookPath)
		}) && allPassed

		// Check: Shell integration
		if v := os.Getenv("GROVE_SHELL_VERSION"); v != "" {
			allPassed = runCheck(w, "Shell integration", func() (string, error) {
				shellVer, err := strconv.Atoi(v)
				if err != nil {
					return "", fmt.Errorf("invalid version %q: %w", v, err)
				}
				if shellVer < shell.ShellVersion {
					return "", fmt.Errorf("outdated (v%d, current v%d) — re-run: grove setup", shellVer, shell.ShellVersion)
				}
				return fmt.Sprintf("v%d (current)", shellVer), nil
			}) && allPassed
		} else if os.Getenv("GROVE_SHELL") == "1" {
			runInfo(w, "Shell integration", "version not set (pre-v2 shell integration)")
		} else {
			runInfo(w, "Shell integration", "not active (running outside shell wrapper)")
		}

		// Check: Git available
		allPassed = runCheck(w, "Git", func() (string, error) {
			out, err := cmdexec.Output(context.TODO(), "git", []string{"--version"}, "", cmdexec.GitLocal)
			if err != nil {
				return "", fmt.Errorf("git not found in PATH")
			}
			return strings.TrimSpace(strings.TrimPrefix(string(out), "git version ")), nil
		}) && allPassed

		// Check: tmux available
		runCheck(w, "Tmux", func() (string, error) {
			out, err := cmdexec.Output(context.TODO(), "tmux", []string{"-V"}, "", cmdexec.Tmux)
			if err != nil {
				return "", fmt.Errorf("tmux not found in PATH (optional, needed for session management)")
			}
			return strings.TrimSpace(string(out)), nil
		})

		// Check: aggressive-resize warning for iTerm2 control mode
		if tmux.IsControlModeTerminal() {
			runCheck(w, "Tmux control mode", func() (string, error) {
				out, err := cmdexec.Output(context.TODO(), "tmux", []string{"show-option", "-gv", "aggressive-resize"}, "", cmdexec.Tmux)
				if err == nil && strings.TrimSpace(string(out)) == "on" {
					return "", fmt.Errorf("aggressive-resize is on — may cause display issues with tmux -CC in iTerm2. Run: tmux set-option -g aggressive-resize off")
				}
				return "iTerm2 detected, control mode available", nil
			})
		}

		// Check: gh CLI available
		runCheck(w, "GitHub CLI", func() (string, error) {
			if _, err := exec.LookPath("gh"); err != nil {
				return "", fmt.Errorf("gh not found in PATH (optional, needed for grove fetch)")
			}
			return "found", nil
		})

		// Check: Docker available (informational in Tier 1 — project checks validate Docker usage)
		runCheck(w, "Docker available", func() (string, error) {
			if _, err := exec.LookPath("docker"); err != nil {
				return "", fmt.Errorf("docker not found in PATH (optional, needed for grove new/to)")
			}
			return "found in PATH", nil
		})

		// Check: Docker daemon running
		runCheck(w, "Docker running", func() (string, error) {
			out, err := cmdexec.Output(context.TODO(), "docker", []string{"info", "--format", "{{.ServerVersion}}"}, "", cmdexec.Docker)
			if err != nil {
				return "", fmt.Errorf("docker daemon not responding (optional, needed for grove new/to)")
			}
			return "v" + strings.TrimSpace(string(out)), nil
		})

		// ── Tier 2: Project checks (only when in a grove project) ──

		groveDir, err := grove.FindRoot("")
		if err != nil {
			runInfo(w, "Project", fmt.Sprintf("detection error: %v", err))
		} else if groveDir == "" {
			_, _ = fmt.Fprintln(w)
			runInfo(w, "Project", "not in a grove project — skipping project checks")
		} else {
			_, _ = fmt.Fprintln(w)
			runInfo(w, "Project", filepath.Dir(groveDir))

			// Load config
			cfg, cfgErr := config.LoadFromGroveDir(groveDir)
			if cfgErr != nil {
				cli.Warning(w, "Config: %v (using defaults)", cfgErr)
				cfg = config.LoadDefaults()
			} else {
				allPassed = runCheck(w, "Config", func() (string, error) {
					return "loaded", nil
				}) && allPassed
			}

			// Check: config symlinks across worktrees
			allPassed = runCheck(w, "Config symlinks", func() (string, error) {
				return checkConfigSymlinks(groveDir)
			}) && allPassed

			// Existing Tier 2 checks (external compose, agent stacks, env files)
			runExternalModeChecks(w, cfg, filepath.Dir(groveDir), &allPassed)

			// Detect host install hooks in a Docker project (issue #28).
			projectRoot := filepath.Dir(groveDir)
			allPassed = runCheck(w, "Hooks Docker-routing", func() (string, error) {
				return checkHooksDockerRouting(projectRoot, groveDir)
			}) && allPassed

			// Auto-fix host installs when --fix is set.
			if doctorFix {
				if changed, err := fixHostInstallsInDockerProject(projectRoot, groveDir); err != nil {
					cli.Warning(w, "auto-fix failed: %v", err)
				} else if changed > 0 {
					cli.Success(w, "Rewrote %d host install hook(s) to docker:compose", changed)
				}
			}

			// Warn on stray .grove-backup directories (issue #28).
			allPassed = runCheck(w, "Backup directory", func() (string, error) {
				return checkStrayBackup(groveDir)
			}) && allPassed
		}

		_, _ = fmt.Fprintln(w)
		if allPassed {
			cli.Success(w, "All checks passed")
		} else {
			cli.Warning(w, "Some checks failed — see above for details")
		}

		return nil
	},
}

// checkGroveBinary verifies the grove binary is resolvable.
// lookPath is injected for testability.
func checkGroveBinary(lookPath func(string) (string, error)) (string, error) {
	path, err := lookPath("grove")
	if err != nil {
		return "", groveNotFoundError()
	}

	// Resolve symlinks for display
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = resolved
	}
	return path, nil
}

func groveNotFoundError() error {
	// Check common install locations for a helpful hint
	hints := []string{
		"/opt/homebrew/bin/grove", // Homebrew (Apple Silicon)
		"/usr/local/bin/grove",    // Homebrew (Intel) / manual
	}
	for _, hint := range hints {
		if _, statErr := os.Stat(hint); statErr == nil {
			return fmt.Errorf("grove not found in PATH, but exists at %s — add its directory to PATH in ~/.zshenv", hint)
		}
	}

	// Check GOPATH/bin
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		gobin := filepath.Join(gopath, "bin", "grove")
		if _, statErr := os.Stat(gobin); statErr == nil {
			return fmt.Errorf("grove not found in PATH, but exists at %s — add $GOPATH/bin to PATH", gobin)
		}
	}
	homeGobin := filepath.Join(os.Getenv("HOME"), "go", "bin", "grove")
	if _, statErr := os.Stat(homeGobin); statErr == nil {
		return fmt.Errorf("grove not found in PATH, but exists at %s — add ~/go/bin to PATH", homeGobin)
	}

	return fmt.Errorf("grove binary not found in PATH")
}

// checkConfigSymlinks validates .grove/config.toml symlinks across all worktrees.
func checkConfigSymlinks(groveDir string) (string, error) {
	projectRoot := filepath.Dir(groveDir)

	out, err := cmdexec.Output(context.TODO(), "git", []string{"-C", projectRoot, "worktree", "list", "--porcelain"}, "", cmdexec.GitLocal)
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %w", err)
	}

	var broken []string
	var total int

	for _, line := range strings.Split(string(out), "\n") {
		path, found := strings.CutPrefix(line, "worktree ")
		if !found {
			continue
		}
		total++

		configPath := filepath.Join(path, ".grove", "config.toml")
		target, err := os.Readlink(configPath)
		if err != nil {
			continue // Not a symlink or no .grove — skip
		}

		// It's a symlink — check if target exists
		if _, statErr := os.Stat(configPath); statErr != nil {
			broken = append(broken, fmt.Sprintf("%s → %s", filepath.Base(path), target))
		}
	}

	if len(broken) > 0 {
		return "", fmt.Errorf("broken symlinks: %s", strings.Join(broken, ", "))
	}

	return fmt.Sprintf("%d worktrees checked", total), nil
}

// runExternalModeChecks runs all Docker external mode checks.
// Extracted from the old doctor to keep the restructured command readable.
func runExternalModeChecks(w *cli.Writer, cfg *config.Config, projectRoot string, allPassed *bool) {
	if cfg == nil || !cfg.IsExternalDockerMode() {
		runInfo(w, "External mode", "not configured (using local compose)")
		return
	}

	ext := cfg.Plugins.Docker.External
	*allPassed = runCheck(w, "External compose path", func() (string, error) {
		if ext.Path == "" {
			return "", fmt.Errorf("plugins.docker.external.path not set")
		}
		info, err := os.Stat(ext.Path)
		if err != nil {
			return "", fmt.Errorf("%s: %w", ext.Path, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%s is not a directory", ext.Path)
		}
		return ext.Path, nil
	}) && *allPassed

	checkProvisioningSources(w, ext, projectRoot, allPassed)

	envFileName := ext.EnvFileName()
	composePath := docker.ResolveComposePath(ext.Path)
	efResult := checkEnvFileConfig(envFileName, composePath, exec.LookPath)

	checkEnvFileChecks(w, envFileName, efResult, allPassed)
	checkAgentStacks(w, cfg, ext, allPassed)
}

// checkProvisioningSources verifies that every entry in copy_files /
// symlink_files / symlink_dirs exists in the project root. Catches typos
// before the first 'grove new', when the failure would only show up as a
// silent warning during worktree creation.
func checkProvisioningSources(w *cli.Writer, ext *config.ExternalComposeConfig, projectRoot string, allPassed *bool) {
	type entry struct {
		field string
		paths []string
	}
	groups := []entry{
		{"copy_files", ext.CopyFiles},
		{"symlink_files", ext.SymlinkFiles},
		{"symlink_dirs", ext.SymlinkDirs},
	}
	for _, g := range groups {
		if len(g.paths) == 0 {
			continue
		}
		missing := []string{}
		for _, rel := range g.paths {
			if _, err := os.Stat(filepath.Join(projectRoot, rel)); err != nil {
				missing = append(missing, rel)
			}
		}
		*allPassed = runCheck(w, "Provisioning "+g.field, func() (string, error) {
			if len(missing) > 0 {
				return "", fmt.Errorf("missing in main worktree: %s", strings.Join(missing, ", "))
			}
			return fmt.Sprintf("%d entries", len(g.paths)), nil
		}) && *allPassed
	}
}

// checkEnvFileChecks runs env file loader and configuration checks.
func checkEnvFileChecks(w *cli.Writer, envFileName string, efResult envFileCheckResult, allPassed *bool) {
	if envFileName == ".env" {
		if efResult.hintAvailable {
			runInfo(w, "Env file hint", "direnv/mise is configured for .env.local — consider setting env_file = \".env.local\" to avoid dirtying tracked .env")
		}
		return
	}

	*allPassed = runCheck(w, "Env file target", func() (string, error) {
		return envFileName, nil
	}) && *allPassed

	runCheck(w, "Env file loader", func() (string, error) {
		if !efResult.loaderInstalled {
			return "", fmt.Errorf("%s", efResult.loaderErr)
		}
		return efResult.loaderName + " found in PATH", nil
	})

	runCheck(w, "Env file loader configured", func() (string, error) {
		if efResult.configErr != "" {
			return "", fmt.Errorf("%s", efResult.configErr)
		}
		if !efResult.configLoadsFile {
			return "", fmt.Errorf("no loader config references %s", envFileName)
		}
		return "configured", nil
	})
}

// checkAgentStacks runs agent stack configuration and network checks.
func checkAgentStacks(w *cli.Writer, cfg *config.Config, ext *config.ExternalComposeConfig, allPassed *bool) {
	if ext.Agent == nil || ext.Agent.Enabled == nil || !*ext.Agent.Enabled {
		runInfo(w, "Agent stacks", "not enabled")
		return
	}

	*allPassed = runCheck(w, "Agent config", func() (string, error) {
		if len(ext.Agent.Services) == 0 {
			return "", fmt.Errorf("agent.services is empty")
		}
		if ext.Agent.TemplatePath == "" {
			return "", fmt.Errorf("agent.template_path not set")
		}
		return fmt.Sprintf("%d services, max %d slots", len(ext.Agent.Services), ext.Agent.MaxSlots), nil
	}) && *allPassed

	*allPassed = runCheck(w, "Agent template path", func() (string, error) {
		tmpl := ext.Agent.TemplatePath
		if !filepath.IsAbs(tmpl) {
			tmpl = filepath.Join(docker.ResolveComposePath(ext.Path), tmpl)
		}
		info, err := os.Stat(tmpl)
		if err != nil {
			return "", fmt.Errorf("%s: %w", tmpl, err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("%s is a directory, expected a compose file", tmpl)
		}
		return tmpl, nil
	}) && *allPassed

	checkAgentNetwork(w, ext.Agent.Network, allPassed)

	slots, err := docker.ListActiveSlots(cfg)
	if err != nil {
		return
	}
	maxSlots := ext.Agent.MaxSlots
	if maxSlots <= 0 {
		maxSlots = 5
	}
	runInfo(w, "Active stacks", fmt.Sprintf("%d/%d slots in use", len(slots), maxSlots))
}

// checkAgentNetwork verifies the Docker network exists.
func checkAgentNetwork(w *cli.Writer, network string, allPassed *bool) {
	if network == "" {
		return
	}
	*allPassed = runCheck(w, "Docker network '"+network+"'", func() (string, error) {
		out, err := cmdexec.Output(context.TODO(), "docker", []string{"network", "ls", "--format", "{{.Name}}"}, "", cmdexec.Docker)
		if err != nil {
			return "", fmt.Errorf("failed to list networks: %w", err)
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) == network {
				return "exists", nil
			}
		}
		return "", fmt.Errorf("network not found (is the main stack running?)")
	}) && *allPassed
}

func runCheck(w *cli.Writer, name string, check func() (string, error)) bool {
	detail, err := check()
	if err != nil {
		cli.Error(w, "%s: %v", name, err)
		return false
	}
	if detail != "" {
		cli.Success(w, "%s (%s)", name, detail)
	} else {
		cli.Success(w, "%s", name)
	}
	return true
}

func runInfo(w *cli.Writer, name string, detail string) {
	cli.Info(w, "%s: %s", name, detail)
}

// envFileCheckResult holds the outcome of an env file configuration check.
type envFileCheckResult struct {
	loaderInstalled bool   // whether direnv or mise was found in PATH
	loaderName      string // "direnv", "mise", or "" if neither found
	configExists    bool   // whether a config file (.envrc or .mise.toml/mise.toml) exists
	configLoadsFile bool   // whether the config file references the env file name
	hintAvailable   bool   // whether .env.local hint should be shown (default .env mode only)
	loaderErr       string // error message if loader check failed
	configErr       string // error message if config check failed
}

// checkEnvFileConfig inspects the compose directory for env file loader readiness.
// It checks for direnv (.envrc) and mise (.mise.toml/mise.toml) as env file loaders.
// lookPath is injected for testability (pass exec.LookPath in production).
func checkEnvFileConfig(envFileName, composePath string, lookPath func(string) (string, error)) envFileCheckResult {
	if envFileName != ".env" {
		return checkCustomEnvFile(envFileName, composePath, lookPath)
	}
	return checkDefaultEnvFile(composePath)
}

func checkCustomEnvFile(envFileName, composePath string, lookPath func(string) (string, error)) envFileCheckResult {
	var result envFileCheckResult

	// Check for direnv or mise
	if _, err := lookPath("direnv"); err == nil {
		result.loaderInstalled = true
		result.loaderName = "direnv"
	} else if _, err := lookPath("mise"); err == nil {
		result.loaderInstalled = true
		result.loaderName = "mise"
	} else {
		result.loaderErr = fmt.Sprintf("neither direnv nor mise found — install one if you run manual docker compose commands in %s", composePath)
	}

	// Check for config file: .envrc (direnv) or .mise.toml/mise.toml (mise)
	if found, name := checkEnvrcFile(composePath, envFileName); found {
		result.configExists = true
		result.configLoadsFile = true
		_ = name
	} else if checkMiseFile(composePath, envFileName) {
		result.configExists = true
		result.configLoadsFile = true
	} else {
		result.configExists, result.configErr = checkConfigExists(composePath, envFileName)
	}

	return result
}

func checkDefaultEnvFile(composePath string) envFileCheckResult {
	var result envFileCheckResult

	envrcPath := filepath.Join(composePath, ".envrc")
	if data, err := os.ReadFile(envrcPath); err == nil && strings.Contains(string(data), ".env.local") {
		result.hintAvailable = true
	}
	if !result.hintAvailable && checkMiseFile(composePath, ".env.local") {
		result.hintAvailable = true
	}

	return result
}

// checkEnvrcFile checks if .envrc exists and references the env file.
func checkEnvrcFile(composePath, envFileName string) (found bool, name string) {
	data, err := os.ReadFile(filepath.Join(composePath, ".envrc"))
	if err != nil {
		return false, ""
	}
	if strings.Contains(string(data), envFileName) {
		return true, ".envrc"
	}
	return false, ""
}

// checkMiseFile checks if .mise.toml or mise.toml exists and references the env file.
func checkMiseFile(composePath, envFileName string) bool {
	for _, fname := range []string{".mise.toml", "mise.toml"} {
		data, err := os.ReadFile(filepath.Join(composePath, fname))
		if err != nil {
			continue
		}
		if strings.Contains(string(data), envFileName) {
			return true
		}
	}
	return false
}

// checkConfigExists checks if any env loader config file exists but doesn't reference the env file.
func checkConfigExists(composePath, envFileName string) (exists bool, errMsg string) {
	// Check .envrc
	if data, err := os.ReadFile(filepath.Join(composePath, ".envrc")); err == nil {
		if !strings.Contains(string(data), envFileName) {
			return true, fmt.Sprintf(".envrc does not reference %s — add: dotenv_if_exists %s", envFileName, envFileName)
		}
	}
	// Check mise files
	for _, fname := range []string{".mise.toml", "mise.toml"} {
		if data, err := os.ReadFile(filepath.Join(composePath, fname)); err == nil {
			if !strings.Contains(string(data), envFileName) {
				return true, fmt.Sprintf("%s does not reference %s — add %s to [env] section", fname, envFileName, envFileName)
			}
		}
	}
	return false, fmt.Sprintf("no .envrc or .mise.toml found in %s — needed only for manual docker compose commands (grove handles env files automatically)", composePath)
}

// checkHooksDockerRouting flags host install commands (bundle/npm/pip install)
// in a Docker-based project — the original symptom from issue #28.
func checkHooksDockerRouting(projectRoot, groveDir string) (string, error) {
	if !detect.HasDocker(projectRoot) {
		return "no docker — n/a", nil
	}

	cfg, err := hooks.LoadHooksConfig(groveDir)
	if err != nil || cfg == nil {
		return "", fmt.Errorf("could not load hooks: %v", err)
	}

	var offenders []string
	for _, a := range cfg.Hooks.PostCreate {
		if a.Type != "command" {
			continue
		}
		if isLikelyHostInstallCommand(a.Command) {
			offenders = append(offenders, a.Command)
		}
	}
	if len(offenders) > 0 {
		return "", fmt.Errorf(
			"host install command(s) in a Docker project: %s — convert to type=\"docker:compose\" hooks (or run `grove doctor --fix`)",
			strings.Join(offenders, "; "),
		)
	}
	return "no host installs detected", nil
}

// hostInstallPrograms is the set of command tokens that, when they appear at
// a real shell-command boundary, indicate a host-side install. Matching is
// boundary-aware (start of string, after `&&`, `||`, `;`, `|`) so that
// `echo "to set up: bundle install"` does NOT match.
var hostInstallPrograms = []string{
	"bundle install",
	"npm install",
	"yarn install",
	"pnpm install",
	"pip install",
	"pip3 install",
	"poetry install",
}

func isLikelyHostInstallCommand(cmd string) bool {
	for _, prog := range hostInstallPrograms {
		if hasCommandTokenBoundary(cmd, prog) {
			return true
		}
	}
	return false
}

// hasCommandTokenBoundary reports whether `prog` appears in `cmd` at a real
// command boundary (start, or after &&, ||, ;, |, with optional whitespace).
// This rejects matches inside string literals and echo/comment positions.
func hasCommandTokenBoundary(cmd, prog string) bool {
	separators := []string{"", "&&", "||", ";", "|"}
	for _, sep := range separators {
		// Prefix patterns: "<sep> <prog>" or "<sep><prog>" at any position.
		if sep == "" {
			// Start of string only.
			trimmed := strings.TrimLeft(cmd, " \t")
			if strings.HasPrefix(trimmed, prog) {
				return true
			}
			continue
		}
		idx := 0
		for idx < len(cmd) {
			j := strings.Index(cmd[idx:], sep)
			if j == -1 {
				break
			}
			rest := strings.TrimLeft(cmd[idx+j+len(sep):], " \t")
			if strings.HasPrefix(rest, prog) {
				return true
			}
			idx += j + len(sep)
		}
	}
	return false
}

// fixHostInstallsInDockerProject rewrites host install hooks to docker:compose
// hooks in place. Returns the number of hooks changed and any error.
//
// Conservative: only rewrites within `[[hooks.post_create]]`, only when the
// command is in hostInstallPrograms, and only for type = "command".
func fixHostInstallsInDockerProject(projectRoot, groveDir string) (int, error) {
	if !detect.HasDocker(projectRoot) {
		return 0, nil
	}
	composePath := detect.FindComposeFile(projectRoot)
	if composePath == "" {
		return 0, nil
	}
	service, ok := detect.InferAppService(composePath)
	if !ok {
		return 0, fmt.Errorf("can't infer app service from %s — fix manually", composePath)
	}

	hooksPath := filepath.Join(groveDir, "hooks.toml")
	original, err := os.ReadFile(hooksPath)
	if err != nil {
		return 0, fmt.Errorf("read hooks.toml: %w", err)
	}
	rewritten, n := rewriteHostInstallsToCompose(string(original), service)
	if n == 0 {
		return 0, nil
	}
	if err := os.WriteFile(hooksPath, []byte(rewritten), 0644); err != nil {
		return 0, fmt.Errorf("write hooks.toml: %w", err)
	}
	return n, nil
}

// rewriteHostInstallsToCompose performs the textual rewrite. Operates on the
// raw TOML so user comments and unrelated keys are preserved. Each matching
// `[[hooks.post_create]]` block has its `type = "command"` line replaced
// with `type = "docker:compose"` + `service = "..."` + `mode = "run"`.
func rewriteHostInstallsToCompose(src, service string) (string, int) {
	var out strings.Builder
	count := 0
	lines := strings.Split(src, "\n")

	// Walk blocks: start at each `[[hooks.post_create]]` header and look at
	// the keys until the next blank line or section.
	i := 0
	for i < len(lines) {
		line := lines[i]
		header := strings.TrimSpace(line) == "[[hooks.post_create]]"
		if !header {
			out.WriteString(line)
			if i < len(lines)-1 {
				out.WriteString("\n")
			}
			i++
			continue
		}

		// Collect block lines until next blank or new section.
		blockEnd := i + 1
		for blockEnd < len(lines) {
			t := strings.TrimSpace(lines[blockEnd])
			if t == "" || strings.HasPrefix(t, "[") {
				break
			}
			blockEnd++
		}
		block := lines[i:blockEnd]

		// Inspect: type=command + command matches hostInstallPrograms.
		typeLine, cmdLine := -1, -1
		isCommand := false
		matchedInstall := false
		for k := i + 1; k < blockEnd; k++ {
			t := strings.TrimSpace(lines[k])
			if strings.HasPrefix(t, "type") && strings.Contains(t, `"command"`) {
				typeLine = k
				isCommand = true
			} else if strings.HasPrefix(t, "command") {
				cmdLine = k
				// Extract value between quotes.
				if eq := strings.Index(t, "="); eq >= 0 {
					val := strings.TrimSpace(t[eq+1:])
					val = strings.Trim(val, "\"")
					if isLikelyHostInstallCommand(val) {
						matchedInstall = true
					}
				}
			}
		}

		if isCommand && matchedInstall && typeLine >= 0 && cmdLine >= 0 {
			count++
			for k, bl := range block {
				if k == typeLine-i {
					out.WriteString(`type = "docker:compose"`)
					out.WriteString("\n")
					out.WriteString(fmt.Sprintf("service = %q\n", service))
					out.WriteString("mode = \"run\"\n")
					continue
				}
				out.WriteString(bl)
				out.WriteString("\n")
			}
		} else {
			for _, bl := range block {
				out.WriteString(bl)
				out.WriteString("\n")
			}
		}
		// Trailing newline behavior preserved by always writing \n above.
		i = blockEnd
	}

	result := out.String()
	// Strip the trailing newline we may have added past the original end.
	if !strings.HasSuffix(src, "\n") && strings.HasSuffix(result, "\n") {
		result = result[:len(result)-1]
	}
	return result, count
}

// checkStrayBackup warns when an unexplained .grove/.grove-backup/ directory
// exists. Grove does not create this directory; if present it likely came
// from manual experimentation or an editor's autosave.
func checkStrayBackup(groveDir string) (string, error) {
	backup := filepath.Join(groveDir, ".grove-backup")
	info, err := os.Stat(backup)
	if err != nil {
		return "no stray backup directory", nil
	}
	if info.IsDir() {
		return "", fmt.Errorf(
			"%s exists but is not grove-managed — safe to remove if you don't recognize it",
			backup,
		)
	}
	return "no stray backup directory", nil
}
