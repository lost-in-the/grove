package hooks

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"time"
)

// ExecutionContext provides context for hook action execution
type ExecutionContext struct {
	// Event being processed
	Event string

	// Worktree information
	Worktree     string // Short worktree name (e.g., "testing")
	WorktreeFull string // Full worktree directory name (e.g., "grove-cli-testing")
	Branch       string // Branch name (e.g., "testing" or "feature/testing")

	// Project information
	Project string // Project name (e.g., "grove-cli")

	// Paths
	MainPath string // Absolute path to main worktree
	NewPath  string // Absolute path to new/target worktree
	PrevPath string // Absolute path to previous worktree (for switch events)

	// Optional extras
	Port int // Allocated port (if any)
}

// Executor runs user-configured hooks for lifecycle events
type Executor struct {
	config *HooksConfig
	Output io.Writer // destination for status messages; defaults to os.Stdout
}

// NewExecutor creates a new hook executor with loaded configuration
func NewExecutor() (*Executor, error) {
	cfg, err := LoadHooksConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load hooks config: %w", err)
	}

	return &Executor{config: cfg, Output: os.Stdout}, nil
}

// NewExecutorWithConfig creates an executor with a provided config (useful for testing)
func NewExecutorWithConfig(cfg *HooksConfig) *Executor {
	return &Executor{config: cfg, Output: os.Stdout}
}

// printf writes to the executor's output writer.
func (e *Executor) printf(format string, args ...any) {
	w := e.Output
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, format, args...)
}

// Execute runs all configured hooks for an event
// Returns an error if a required hook fails, otherwise just logs warnings
func (e *Executor) Execute(event string, ctx *ExecutionContext) error {
	if e.config == nil {
		return nil
	}

	actions := e.config.GetActionsForEvent(event)
	if len(actions) == 0 {
		return nil
	}

	// Apply per-branch/worktree overrides
	if len(e.config.Overrides) > 0 {
		override := e.config.FindOverride(ctx.Branch, ctx.Worktree)
		actions = ApplyOverride(actions, override, ctx.MainPath)
		if len(actions) == 0 {
			return nil
		}
	}

	vars := e.buildVariables(ctx)
	var firstRequiredErr error

	for _, action := range actions {
		err := e.executeAction(&action, ctx, vars)

		if err != nil {
			// Determine how to handle the error
			if action.Required || action.OnFailure == "fail" {
				e.printf("✗ Hook failed: %v\n", err)
				if firstRequiredErr == nil {
					firstRequiredErr = err
				}
			} else if action.OnFailure == "ignore" {
				// Silent - do nothing
			} else {
				// Default: warn
				e.printf("⚠ Hook warning: %v\n", err)
			}
		}
	}

	return firstRequiredErr
}

// HasHooksForEvent returns true if there are any hooks configured for the event
func (e *Executor) HasHooksForEvent(event string) bool {
	if e.config == nil {
		return false
	}
	return e.config.HasActionsForEvent(event)
}

// executeAction runs a single hook action
func (e *Executor) executeAction(action *HookAction, ctx *ExecutionContext, vars *Variables) error {
	switch action.Type {
	case "copy":
		return e.executeCopy(action, ctx, vars)
	case "symlink":
		return e.executeSymlink(action, ctx, vars)
	case "command":
		return e.executeCommand(action, ctx, vars)
	case "template":
		return e.executeTemplate(action, ctx, vars)
	default:
		return fmt.Errorf("unknown hook action type: %s", action.Type)
	}
}

// buildVariables creates the variable context for interpolation
func (e *Executor) buildVariables(ctx *ExecutionContext) *Variables {
	// Get current username
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	now := time.Now()

	return &Variables{
		Worktree:     ctx.Worktree,
		WorktreeFull: ctx.WorktreeFull,
		Branch:       ctx.Branch,
		Project:      ctx.Project,
		MainPath:     ctx.MainPath,
		NewPath:      ctx.NewPath,
		PrevPath:     ctx.PrevPath,
		Port:         ctx.Port,
		User:         username,
		Timestamp:    now.Unix(),
		Date:         now.Format("2006-01-02"),
	}
}

// Variables holds all available template variables
type Variables struct {
	Worktree     string // Short worktree name
	WorktreeFull string // Full worktree directory name
	Branch       string // Branch name
	Project      string // Project name
	MainPath     string // Main worktree path
	NewPath      string // New worktree path
	PrevPath     string // Previous worktree path
	Port         int    // Allocated port
	User         string // Current username
	Timestamp    int64  // Unix timestamp
	Date         string // ISO date (YYYY-MM-DD)
}

// Interpolate replaces template variables in a string using {{.variable}} syntax
func (v *Variables) Interpolate(s string) string {
	// Simple interpolation without full text/template for basic cases
	// This handles the common {{.variable}} pattern
	replacements := map[string]string{
		"{{.worktree}}":      v.Worktree,
		"{{.worktree_full}}": v.WorktreeFull,
		"{{.branch}}":        v.Branch,
		"{{.project}}":       v.Project,
		"{{.main_path}}":     v.MainPath,
		"{{.new_path}}":      v.NewPath,
		"{{.prev_path}}":     v.PrevPath,
		"{{.port}}":          fmt.Sprintf("%d", v.Port),
		"{{.user}}":          v.User,
		"{{.timestamp}}":     fmt.Sprintf("%d", v.Timestamp),
		"{{.date}}":          v.Date,
	}

	result := s
	for pattern, value := range replacements {
		result = replaceAll(result, pattern, value)
	}

	return result
}

// replaceAll is a simple string replacement (avoiding regexp for performance)
func replaceAll(s, old, new string) string {
	if old == "" {
		return s
	}
	result := s
	for {
		i := indexOf(result, old)
		if i < 0 {
			break
		}
		result = result[:i] + new + result[i+len(old):]
	}
	return result
}

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// executeCopy performs a copy action
func (e *Executor) executeCopy(action *HookAction, ctx *ExecutionContext, vars *Variables) error {
	from := vars.Interpolate(action.From)
	to := vars.Interpolate(action.To)

	// Resolve paths
	srcPath := resolvePath(from, ctx.MainPath)
	dstPath := resolvePath(to, ctx.NewPath)

	// Check if source exists
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("copy source does not exist: %s", srcPath)
		}
		return fmt.Errorf("copy: cannot access source %s: %w", srcPath, err)
	}

	// Perform copy
	if srcInfo.IsDir() {
		if err := copyDir(srcPath, dstPath); err != nil {
			return fmt.Errorf("copy directory %s to %s: %w", from, to, err)
		}
	} else {
		if err := copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("copy file %s to %s: %w", from, to, err)
		}
	}

	e.printf("✓ Copied %s\n", to)
	return nil
}

// executeSymlink performs a symlink action
func (e *Executor) executeSymlink(action *HookAction, ctx *ExecutionContext, vars *Variables) error {
	from := vars.Interpolate(action.From)
	to := vars.Interpolate(action.To)

	// Resolve paths
	srcPath := resolvePath(from, ctx.MainPath)
	linkPath := resolvePath(to, ctx.NewPath)

	// Check if source exists
	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("symlink source does not exist: %s", srcPath)
		}
		return fmt.Errorf("symlink: cannot access source %s: %w", srcPath, err)
	}

	// Remove existing file/link at destination if exists
	if _, err := os.Lstat(linkPath); err == nil {
		if err := os.RemoveAll(linkPath); err != nil {
			return fmt.Errorf("symlink: cannot remove existing %s: %w", linkPath, err)
		}
	}

	// Create symlink
	if err := os.Symlink(srcPath, linkPath); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", to, from, err)
	}

	e.printf("✓ Symlinked %s\n", to)
	return nil
}

// executeCommand performs a command action
func (e *Executor) executeCommand(action *HookAction, ctx *ExecutionContext, vars *Variables) error {
	command := vars.Interpolate(action.Command)

	// Determine working directory
	var workDir string
	switch action.WorkingDir {
	case "main":
		workDir = ctx.MainPath
	case "new", "":
		workDir = ctx.NewPath
	default:
		workDir = vars.Interpolate(action.WorkingDir)
	}

	// Execute command with timeout
	timeout := time.Duration(action.Timeout) * time.Second
	start := time.Now()

	w := e.Output
	if w == nil {
		w = os.Stdout
	}
	if err := runCommand(command, workDir, timeout, w, w); err != nil {
		return fmt.Errorf("command '%s': %w", command, err)
	}

	elapsed := time.Since(start)
	e.printf("✓ Ran: %s (%.1fs)\n", command, elapsed.Seconds())
	return nil
}

// executeTemplate performs a template action
func (e *Executor) executeTemplate(action *HookAction, ctx *ExecutionContext, vars *Variables) error {
	from := vars.Interpolate(action.From)
	to := vars.Interpolate(action.To)

	// Resolve paths
	srcPath := resolvePath(from, ctx.MainPath)
	dstPath := resolvePath(to, ctx.NewPath)

	// Read template file
	content, err := os.ReadFile(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("template source does not exist: %s", srcPath)
		}
		return fmt.Errorf("template: cannot read %s: %w", srcPath, err)
	}

	// Create extended variables with action-specific vars
	extVars := *vars
	result := string(content)

	// Apply action-specific vars first
	for k, v := range action.Vars {
		pattern := "{{." + k + "}}"
		result = replaceAll(result, pattern, vars.Interpolate(v))
	}

	// Apply standard variables
	result = extVars.Interpolate(result)

	// Ensure destination directory exists
	if dir := dirOf(dstPath); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("template: cannot create directory: %w", err)
		}
	}

	// Write output file
	if err := os.WriteFile(dstPath, []byte(result), 0644); err != nil {
		return fmt.Errorf("template: cannot write %s: %w", dstPath, err)
	}

	e.printf("✓ Generated %s from template\n", to)
	return nil
}

// dirOf returns the directory portion of a path
func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return ""
}
