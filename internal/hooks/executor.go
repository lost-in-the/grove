package hooks

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
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

	// Output is where handlers should print status messages. nil means stdout.
	Output io.Writer
}

// Out returns the context's output writer, falling back to stdout. Exported
// so plugin handlers can use the same fallback rule as built-ins.
func (c *ExecutionContext) Out() io.Writer {
	if c == nil || c.Output == nil {
		return os.Stdout
	}
	return c.Output
}

// Executor runs user-configured hooks for lifecycle events
type Executor struct {
	config *HooksConfig
	Output io.Writer // destination for status messages; defaults to os.Stdout
}

// NewExecutor creates a new hook executor with loaded configuration.
// If groveDir is provided, it is forwarded to LoadHooksConfig for project hook discovery.
func NewExecutor(groveDir ...string) (*Executor, error) {
	cfg, err := LoadHooksConfig(groveDir...)
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
	_, _ = fmt.Fprintf(w, format, args...)
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

	if ctx.Output == nil {
		ctx.Output = e.Output
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

// executeAction runs a single hook action by looking up its handler in the
// global registry. Built-in handlers (copy/symlink/command/template) are
// registered at package init; plugins register their own types during Init().
func (e *Executor) executeAction(action *HookAction, ctx *ExecutionContext, vars *Variables) error {
	if h, ok := LookupActionHandler(action.Type); ok {
		return h(action, ctx, vars)
	}
	// Distinguish "type belongs to a known plugin that's disabled" from "typo".
	if hint, ok := disabledTypeHint(action.Type); ok {
		return fmt.Errorf("unknown hook action type %q (%s)", action.Type, hint)
	}
	return fmt.Errorf("unknown hook action type: %s", action.Type)
}

// disabledTypeHint returns a hint when a known-but-not-registered type is
// referenced. Plugins claim names via RegisterActionHandler at startup; if
// nothing's registered, the plugin is likely disabled or unavailable.
func disabledTypeHint(typeName string) (string, bool) {
	switch typeName {
	case "docker:compose", "docker:exec":
		return "docker plugin disabled or unavailable", true
	}
	return "", false
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
		result = strings.ReplaceAll(result, pattern, value)
	}

	return result
}

// builtinCopy performs a copy action
func builtinCopy(action *HookAction, ctx *ExecutionContext, vars *Variables) error {
	from := vars.Interpolate(action.From)
	to := vars.Interpolate(action.To)

	srcPath, err := resolvePathSafe(from, ctx.MainPath)
	if err != nil {
		return fmt.Errorf("copy source path: %w", err)
	}
	dstPath, err := resolvePathSafe(to, ctx.NewPath)
	if err != nil {
		return fmt.Errorf("copy destination path: %w", err)
	}

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("copy source does not exist: %s", srcPath)
		}
		return fmt.Errorf("copy: cannot access source %s: %w", srcPath, err)
	}

	if srcInfo.IsDir() {
		if err := copyDir(srcPath, dstPath); err != nil {
			return fmt.Errorf("copy directory %s to %s: %w", from, to, err)
		}
	} else {
		if err := copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("copy file %s to %s: %w", from, to, err)
		}
	}

	_, _ = fmt.Fprintf(ctx.Out(), "✓ Copied %s\n", to)
	return nil
}

// builtinSymlink performs a symlink action
func builtinSymlink(action *HookAction, ctx *ExecutionContext, vars *Variables) error {
	from := vars.Interpolate(action.From)
	to := vars.Interpolate(action.To)

	srcPath, err := resolvePathSafe(from, ctx.MainPath)
	if err != nil {
		return fmt.Errorf("symlink source path: %w", err)
	}
	linkPath, err := resolvePathSafe(to, ctx.NewPath)
	if err != nil {
		return fmt.Errorf("symlink destination path: %w", err)
	}

	if _, err := os.Lstat(srcPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("symlink source does not exist: %s", srcPath)
		}
		return fmt.Errorf("symlink: cannot access source %s: %w", srcPath, err)
	}

	if _, err := os.Lstat(linkPath); err == nil {
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("symlink: cannot remove existing %s: %w", linkPath, err)
		}
	}

	if err := os.Symlink(srcPath, linkPath); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", to, from, err)
	}

	_, _ = fmt.Fprintf(ctx.Out(), "✓ Symlinked %s\n", to)
	return nil
}

// builtinCommand performs a command action
func builtinCommand(action *HookAction, ctx *ExecutionContext, vars *Variables) error {
	command := vars.Interpolate(action.Command)

	var workDir string
	switch action.WorkingDir {
	case "main":
		workDir = ctx.MainPath
	case "new", "":
		workDir = ctx.NewPath
	default:
		workDir = vars.Interpolate(action.WorkingDir)
	}

	timeout := time.Duration(action.Timeout) * time.Second
	start := time.Now()

	w := ctx.Out()
	if err := runCommand(command, workDir, timeout, w, w); err != nil {
		return fmt.Errorf("command '%s': %w", command, err)
	}

	elapsed := time.Since(start)
	_, _ = fmt.Fprintf(w, "✓ Ran: %s (%.1fs)\n", command, elapsed.Seconds())
	return nil
}

// builtinTemplate performs a template action
func builtinTemplate(action *HookAction, ctx *ExecutionContext, vars *Variables) error {
	from := vars.Interpolate(action.From)
	to := vars.Interpolate(action.To)

	srcPath, err := resolvePathSafe(from, ctx.MainPath)
	if err != nil {
		return fmt.Errorf("template source path: %w", err)
	}
	dstPath, err := resolvePathSafe(to, ctx.NewPath)
	if err != nil {
		return fmt.Errorf("template destination path: %w", err)
	}

	content, err := os.ReadFile(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("template source does not exist: %s", srcPath)
		}
		return fmt.Errorf("template: cannot read %s: %w", srcPath, err)
	}

	extVars := *vars
	result := string(content)

	for k, v := range action.Vars {
		pattern := "{{." + k + "}}"
		result = strings.ReplaceAll(result, pattern, vars.Interpolate(v))
	}

	result = extVars.Interpolate(result)

	if dir := filepath.Dir(dstPath); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("template: cannot create directory: %w", err)
		}
	}

	if err := os.WriteFile(dstPath, []byte(result), 0644); err != nil {
		return fmt.Errorf("template: cannot write %s: %w", dstPath, err)
	}

	_, _ = fmt.Fprintf(ctx.Out(), "✓ Generated %s from template\n", to)
	return nil
}

func init() {
	RegisterActionHandler("copy", builtinCopy)
	RegisterActionHandler("symlink", builtinSymlink)
	RegisterActionHandler("command", builtinCommand)
	RegisterActionHandler("template", builtinTemplate)
}
