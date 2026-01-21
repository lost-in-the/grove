package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/grove"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/spf13/cobra"
)

var (
	setupWithTesting bool
	setupWithScratch bool
	setupFull        bool
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Initialize a new grove project",
	Long: `Initialize the current git repository as a grove project.

This command creates a .grove directory with configuration and state files.
It must be run from the root of a git repository (not from a worktree).

Flags:
  --with-testing  Also create a 'testing' worktree
  --with-scratch  Also create a 'scratch' worktree
  --full          Create testing, scratch, and hotfix worktrees

Example:
  cd my-project
  grove setup
  grove setup --full`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSetup()
	},
}

func runSetup() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if this is a git repository
	if !isGitRepo(cwd) {
		PrintError("not a git repository")
		PrintSuggestion("run 'git init' first, or cd to an existing repository")
		ExitWithCode(exitcode.InvalidInput)
		return nil
	}

	// Check if running from a worktree (not the main repo)
	isWorktree, err := grove.IsInsideWorktree()
	if err != nil {
		return fmt.Errorf("failed to check worktree status: %w", err)
	}
	if isWorktree {
		PrintError("cannot initialize from a worktree")
		PrintSuggestion("run 'grove setup' from the main repository")
		ExitWithCode(exitcode.NotGroveProject)
		return nil
	}

	// Check if already initialized
	groveDir := filepath.Join(cwd, ".grove")
	if _, err := os.Stat(groveDir); err == nil {
		PrintError("grove project already initialized")
		PrintSuggestion("use 'grove ls' to see worktrees, or remove .grove to reinitialize")
		ExitWithCode(exitcode.ResourceExists)
		return nil
	}

	// Detect project name
	projectName := detectProjectName(cwd)

	// Create .grove directory
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("failed to create .grove directory: %w", err)
	}

	// Create config.toml
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

	// Initialize state
	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		return fmt.Errorf("failed to initialize state: %w", err)
	}

	if err := stateMgr.SetProject(projectName); err != nil {
		return fmt.Errorf("failed to set project name: %w", err)
	}

	// Register the main worktree
	mainBranch := detectMainBranch(cwd)
	mainState := &state.WorktreeState{
		Path:   cwd,
		Branch: mainBranch,
		Root:   true,
	}
	if err := stateMgr.AddWorktree("main", mainState); err != nil {
		return fmt.Errorf("failed to register main worktree: %w", err)
	}

	// Update .gitignore
	if err := updateGitignore(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update .gitignore: %v\n", err)
	}

	// Create .envrc for direnv users
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
	fmt.Printf("  State:  %s\n", filepath.Join(groveDir, "state.json"))

	// Create additional worktrees if requested
	if setupFull {
		setupWithTesting = true
		setupWithScratch = true
	}

	if setupWithTesting {
		if err := createWorktree(cwd, projectName, "testing"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create testing worktree: %v\n", err)
		} else {
			fmt.Println("✓ Created 'testing' worktree")
		}
	}

	if setupWithScratch {
		if err := createWorktree(cwd, projectName, "scratch"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create scratch worktree: %v\n", err)
		} else {
			fmt.Println("✓ Created 'scratch' worktree")
		}
	}

	if setupFull {
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

func isGitRepo(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func detectProjectName(dir string) string {
	// Try to get from git remote
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err == nil {
		url := strings.TrimSpace(string(output))
		// Extract repo name from URL
		// Handle: git@github.com:user/repo.git or https://github.com/user/repo.git
		url = strings.TrimSuffix(url, ".git")
		parts := strings.Split(url, "/")
		if len(parts) > 0 {
			name := parts[len(parts)-1]
			// Also handle git@github.com:user/repo format
			if idx := strings.LastIndex(name, ":"); idx != -1 {
				name = name[idx+1:]
			}
			if name != "" {
				return name
			}
		}
	}

	// Fall back to directory name
	return filepath.Base(dir)
}

func detectMainBranch(dir string) string {
	// Try to detect main branch
	for _, branch := range []string{"main", "master"} {
		cmd := exec.Command("git", "rev-parse", "--verify", branch)
		cmd.Dir = dir
		if err := cmd.Run(); err == nil {
			return branch
		}
	}

	// Fall back to current branch
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	return "main"
}

func updateGitignore(dir string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")

	// Read existing content
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Check if already has grove entries
	if strings.Contains(string(content), ".grove/state.json") {
		return nil // Already configured
	}

	// Append grove entries
	entry := "\n# Grove (worktree manager)\n.grove/state.json\n.grove/state.json.bak\n.grove/.envrc\n"

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(entry)
	return err
}

func createWorktree(repoDir, projectName, name string) error {
	// Create worktree directory next to the main repo
	parentDir := filepath.Dir(repoDir)
	worktreeDir := filepath.Join(parentDir, fmt.Sprintf("%s-%s", projectName, name))

	// Get current branch
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	baseBranch := strings.TrimSpace(string(output))

	// Create new branch and worktree
	branchName := name
	cmd = exec.Command("git", "worktree", "add", "-b", branchName, worktreeDir, baseBranch)
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func init() {
	setupCmd.Flags().BoolVar(&setupWithTesting, "with-testing", false, "Also create a testing worktree")
	setupCmd.Flags().BoolVar(&setupWithScratch, "with-scratch", false, "Also create a scratch worktree")
	setupCmd.Flags().BoolVar(&setupFull, "full", false, "Create testing, scratch, and hotfix worktrees")
	rootCmd.AddCommand(setupCmd)
}
