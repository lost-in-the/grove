package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeahArmstrong/grove-cli/internal/detect"
	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/grove"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/spf13/cobra"
)

var (
	initWithTesting bool
	initWithScratch bool
	initFull        bool
	initNoHooks     bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new grove project",
	Long: `Initialize the current git repository as a grove project.

This command creates a .grove directory with configuration and state files.
It must be run from the root of a git repository (not from a worktree).

Auto-detects project type (Rails, Node, Go, Python, Docker) and generates
a .grove/hooks.toml with sensible defaults for file copying, symlinks,
and setup commands.

Flags:
  --with-testing  Also create a 'testing' worktree
  --with-scratch  Also create a 'scratch' worktree
  --full          Create testing, scratch, and hotfix worktrees
  --no-hooks      Skip hooks.toml generation

Example:
  cd my-project
  grove init
  grove init --full
  grove init --no-hooks`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func runInit() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	if !isGitRepo(cwd) {
		PrintError("not a git repository")
		PrintSuggestion("run 'git init' first, or cd to an existing repository")
		ExitWithCode(exitcode.InvalidInput)
		return nil
	}

	isWorktree, err := grove.IsInsideWorktree()
	if err != nil {
		return fmt.Errorf("failed to check worktree status: %w", err)
	}
	if isWorktree {
		PrintError("cannot initialize from a worktree")
		PrintSuggestion("run 'grove init' from the main repository")
		ExitWithCode(exitcode.NotGroveProject)
		return nil
	}

	groveDir := filepath.Join(cwd, ".grove")
	if _, err := os.Stat(groveDir); err == nil {
		PrintError("grove project already initialized")
		PrintSuggestion("use 'grove ls' to see worktrees, or remove .grove to reinitialize")
		ExitWithCode(exitcode.ResourceExists)
		return nil
	}

	projectName := detectProjectName(cwd)

	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("failed to create .grove directory: %w", err)
	}

	configPath := filepath.Join(groveDir, "config.toml")
	configContent := fmt.Sprintf(`# Grove project configuration
project_name = %q

[switch]
dirty_handling = "prompt"  # auto-stash, prompt, refuse

[naming]
pattern = "{project}-{name}"

[tmux]
prefix = ""  # Optional prefix for tmux session names
`, projectName)

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to create config.toml: %w", err)
	}

	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		return fmt.Errorf("failed to initialize state: %w", err)
	}

	if err := stateMgr.SetProject(projectName); err != nil {
		return fmt.Errorf("failed to set project name: %w", err)
	}

	mainBranch := detectMainBranch(cwd)
	mainState := &state.WorktreeState{
		Path:   cwd,
		Branch: mainBranch,
		Root:   true,
	}
	if err := stateMgr.AddWorktree("main", mainState); err != nil {
		return fmt.Errorf("failed to register main worktree: %w", err)
	}

	if err := updateGitignore(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update .gitignore: %v\n", err)
	}

	envrcPath := filepath.Join(groveDir, ".envrc")
	envrcContent := `# Grove shell integration
# Source this in your .envrc: source_env .grove/.envrc
export GROVE_PROJECT="` + projectName + `"
`
	if err := os.WriteFile(envrcPath, []byte(envrcContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create .envrc: %v\n", err)
	}

	fmt.Printf("✓ Initialized grove project '%s'\n", projectName)
	fmt.Printf("  Config: %s\n", configPath)

	// Auto-detect project type and generate hooks.toml
	if !initNoHooks {
		hooksPath := filepath.Join(groveDir, "hooks.toml")
		if _, err := os.Stat(hooksPath); os.IsNotExist(err) {
			profile := detect.Detect(cwd)
			if profile.Type != "unknown" {
				hooksContent := generateHooksToml(profile)
				if err := os.WriteFile(hooksPath, []byte(hooksContent), 0644); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to create hooks.toml: %v\n", err)
				} else {
					typeName := profile.Type
					if profile.Type == "mixed" {
						typeName = "mixed (" + strings.Join(profile.Types, ", ") + ")"
					}
					fmt.Printf("\nDetected: %s project\n", typeName)
					fmt.Printf("  Generated: .grove/hooks.toml\n")
					for _, f := range profile.Copy {
						fmt.Printf("    • copy %s\n", f)
					}
					for _, s := range profile.Symlinks {
						fmt.Printf("    • symlink %s\n", s)
					}
					for _, c := range profile.Commands {
						fmt.Printf("    • run: %s\n", c)
					}
					fmt.Printf("\n  Edit hooks: grove config --hooks -e\n")
				}
			}
		}
	}

	// Create additional worktrees if requested
	if initFull {
		initWithTesting = true
		initWithScratch = true
	}

	if initWithTesting {
		if err := createWorktree(cwd, projectName, "testing"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create testing worktree: %v\n", err)
		} else {
			fmt.Println("✓ Created 'testing' worktree")
		}
	}

	if initWithScratch {
		if err := createWorktree(cwd, projectName, "scratch"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create scratch worktree: %v\n", err)
		} else {
			fmt.Println("✓ Created 'scratch' worktree")
		}
	}

	if initFull {
		if err := createWorktree(cwd, projectName, "hotfix"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create hotfix worktree: %v\n", err)
		} else {
			fmt.Println("✓ Created 'hotfix' worktree")
		}
	}

	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("  grove new <name>   Create a new worktree")
	fmt.Println("  grove ls           List all worktrees")
	fmt.Println("  grove to <name>    Switch to a worktree")

	return nil
}

// generateHooksToml creates hooks.toml content from a detected profile
func generateHooksToml(profile *detect.ProjectProfile) string {
	var b strings.Builder

	b.WriteString("# Grove hooks configuration\n")
	b.WriteString("# Auto-generated for detected project type: " + profile.Type + "\n")
	b.WriteString("#\n")
	b.WriteString("# Edit: grove config --hooks -e\n")
	b.WriteString("# Docs: https://github.com/LeahArmstrong/grove-cli#hooks\n\n")

	for _, f := range profile.Copy {
		b.WriteString("[[hooks.post_create]]\n")
		b.WriteString("type = \"copy\"\n")
		b.WriteString(fmt.Sprintf("from = %q\n", f))
		b.WriteString(fmt.Sprintf("to = %q\n", f))
		b.WriteString("required = false\n\n")
	}

	for _, s := range profile.Symlinks {
		b.WriteString("[[hooks.post_create]]\n")
		b.WriteString("type = \"symlink\"\n")
		b.WriteString(fmt.Sprintf("from = %q\n", s))
		b.WriteString(fmt.Sprintf("to = %q\n", s))
		b.WriteString("\n")
	}

	for _, c := range profile.Commands {
		b.WriteString("[[hooks.post_create]]\n")
		b.WriteString("type = \"command\"\n")
		b.WriteString(fmt.Sprintf("command = %q\n", c))
		b.WriteString("timeout = 300\n")
		b.WriteString("on_failure = \"warn\"\n\n")
	}

	return b.String()
}

func init() {
	initCmd.Flags().BoolVar(&initWithTesting, "with-testing", false, "Also create a testing worktree")
	initCmd.Flags().BoolVar(&initWithScratch, "with-scratch", false, "Also create a scratch worktree")
	initCmd.Flags().BoolVar(&initFull, "full", false, "Create testing, scratch, and hotfix worktrees")
	initCmd.Flags().BoolVar(&initNoHooks, "no-hooks", false, "Skip hooks.toml generation")
	rootCmd.AddCommand(initCmd)
}
