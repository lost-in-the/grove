package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/cli"
	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/grove"
	"github.com/LeahArmstrong/grove-cli/internal/shell"
	"github.com/LeahArmstrong/grove-cli/plugins/docker"
)

func init() {
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system health for grove",
	Long: `Run diagnostic checks to verify that grove's dependencies and configuration
are set up correctly.

System checks (binary, PATH, shell integration, git, tmux) run anywhere.
Project checks (Docker, config, symlinks) run when inside a grove project.

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
			out, err := exec.Command("git", "--version").Output()
			if err != nil {
				return "", fmt.Errorf("git not found in PATH")
			}
			return strings.TrimSpace(strings.TrimPrefix(string(out), "git version ")), nil
		}) && allPassed

		// Check: tmux available
		runCheck(w, "Tmux", func() (string, error) {
			out, err := exec.Command("tmux", "-V").Output()
			if err != nil {
				return "", fmt.Errorf("tmux not found in PATH (optional, needed for session management)")
			}
			return strings.TrimSpace(string(out)), nil
		})

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
			cmd := exec.Command("docker", "info", "--format", "{{.ServerVersion}}")
			out, err := cmd.Output()
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
			runExternalModeChecks(w, cfg, &allPassed)
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
		// Check common install locations for a helpful hint
		hints := []string{
			"/opt/homebrew/bin/grove", // Homebrew (Apple Silicon)
			"/usr/local/bin/grove",    // Homebrew (Intel) / manual
		}
		for _, hint := range hints {
			if _, statErr := os.Stat(hint); statErr == nil {
				return "", fmt.Errorf("grove not found in PATH, but exists at %s — add its directory to PATH in ~/.zshenv", hint)
			}
		}

		// Check GOPATH/bin
		if gopath := os.Getenv("GOPATH"); gopath != "" {
			gobin := filepath.Join(gopath, "bin", "grove")
			if _, statErr := os.Stat(gobin); statErr == nil {
				return "", fmt.Errorf("grove not found in PATH, but exists at %s — add $GOPATH/bin to PATH", gobin)
			}
		}
		homeGobin := filepath.Join(os.Getenv("HOME"), "go", "bin", "grove")
		if _, statErr := os.Stat(homeGobin); statErr == nil {
			return "", fmt.Errorf("grove not found in PATH, but exists at %s — add ~/go/bin to PATH", homeGobin)
		}

		return "", fmt.Errorf("grove binary not found in PATH")
	}

	// Resolve symlinks for display
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = resolved
	}
	return path, nil
}

// checkConfigSymlinks validates .grove/config.toml symlinks across all worktrees.
func checkConfigSymlinks(groveDir string) (string, error) {
	projectRoot := filepath.Dir(groveDir)

	cmd := exec.Command("git", "-C", projectRoot, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
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
func runExternalModeChecks(w *cli.Writer, cfg *config.Config, allPassed *bool) {
	if cfg == nil || !cfg.IsExternalDockerMode() {
		runInfo(w, "External mode", "not configured (using local compose)")
		return
	}

	ext := cfg.Plugins.Docker.External
	*allPassed = runCheck(w, "External compose path", func() (string, error) {
		if ext.Path == "" {
			return "", fmt.Errorf("plugins.docker.external.path not set")
		}
		return ext.Path, nil
	}) && *allPassed

	envFileName := ext.EnvFileName()
	composePath := docker.ResolveComposePath(ext.Path)
	efResult := checkEnvFileConfig(envFileName, composePath, exec.LookPath)

	if envFileName != ".env" {
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
	} else if efResult.hintAvailable {
		runInfo(w, "Env file hint", "direnv/mise is configured for .env.local — consider setting env_file = \".env.local\" to avoid dirtying tracked .env")
	}

	if ext.Agent == nil || ext.Agent.Enabled == nil || !*ext.Agent.Enabled {
		runInfo(w, "Agent stacks", "not enabled")
	} else {
		*allPassed = runCheck(w, "Agent config", func() (string, error) {
			if len(ext.Agent.Services) == 0 {
				return "", fmt.Errorf("agent.services is empty")
			}
			if ext.Agent.TemplatePath == "" {
				return "", fmt.Errorf("agent.template_path not set")
			}
			return fmt.Sprintf("%d services, max %d slots", len(ext.Agent.Services), ext.Agent.MaxSlots), nil
		}) && *allPassed

		if ext.Agent.Network != "" {
			*allPassed = runCheck(w, "Docker network '"+ext.Agent.Network+"'", func() (string, error) {
				cmd := exec.Command("docker", "network", "ls", "--format", "{{.Name}}")
				out, err := cmd.Output()
				if err != nil {
					return "", fmt.Errorf("failed to list networks: %w", err)
				}
				for _, line := range strings.Split(string(out), "\n") {
					if strings.TrimSpace(line) == ext.Agent.Network {
						return "exists", nil
					}
				}
				return "", fmt.Errorf("network not found (is the main stack running?)")
			}) && *allPassed
		}

		slots, err := docker.ListActiveSlots(cfg)
		if err == nil {
			maxSlots := ext.Agent.MaxSlots
			if maxSlots <= 0 {
				maxSlots = 5
			}
			runInfo(w, "Active stacks", fmt.Sprintf("%d/%d slots in use", len(slots), maxSlots))
		}
	}
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
	var result envFileCheckResult

	if envFileName != ".env" {
		// Non-default env file: check for direnv or mise
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
		} else if found, name := checkMiseFile(composePath, envFileName); found {
			result.configExists = true
			result.configLoadsFile = true
			_ = name
		} else {
			// Check if config files exist but don't reference the env file
			result.configExists, result.configErr = checkConfigExists(composePath, envFileName)
		}
	} else {
		// Default .env: check if .env.local setup is available via direnv or mise
		envrcPath := filepath.Join(composePath, ".envrc")
		if data, err := os.ReadFile(envrcPath); err == nil {
			if strings.Contains(string(data), ".env.local") {
				result.hintAvailable = true
			}
		}
		if !result.hintAvailable {
			if found, _ := checkMiseFile(composePath, ".env.local"); found {
				result.hintAvailable = true
			}
		}
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
func checkMiseFile(composePath, envFileName string) (found bool, name string) {
	for _, fname := range []string{".mise.toml", "mise.toml"} {
		data, err := os.ReadFile(filepath.Join(composePath, fname))
		if err != nil {
			continue
		}
		if strings.Contains(string(data), envFileName) {
			return true, fname
		}
	}
	return false, ""
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
