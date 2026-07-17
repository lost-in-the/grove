package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/detect"
	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/worktree"
)

var (
	initWithTesting bool
	initWithScratch bool
	initFull        bool
	initNoHooks     bool
	initAuto        bool
	initWalkthrough bool
	initYes         bool
)

func initArgs(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		shell := args[0]
		if shell == shellZsh || shell == shellBash || shell == "fish" {
			return fmt.Errorf("to set up shell integration, use: grove install %s\n\n  eval \"$(grove install %s)\"", shell, shell)
		}
		return fmt.Errorf("unknown argument %q\n\nUsage: grove init [flags]", shell)
	}
	return nil
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new grove project",
	Long: `Initialize the current git repository as a grove project.

This command creates a .grove directory with configuration and state files.
It must be run from the root of a git repository (not from a worktree).

Auto-detects project type (Rails, Node, Go, Python, Docker) and generates
a .grove/hooks.toml with sensible defaults. When Docker is also detected,
install commands (bundle/npm/pip) are routed as compose hooks rather than
host commands so they run inside the container.

By default in an interactive terminal, you'll be asked whether to use auto
generation (with a preview confirm) or step through a walkthrough. Pass
--auto/--walkthrough/--yes to skip the prompt for scripted use.

Flags:
  --with-testing  Also create a 'testing' worktree
  --with-scratch  Also create a 'scratch' worktree
  --full          Create testing, scratch, and hotfix worktrees
  --no-hooks      Skip hooks.toml generation
  --auto          Generate hooks.toml from detection (default for non-TTY)
  --walkthrough   Step through detected items interactively
  --yes           Skip the preview/confirm prompt (CI mode)

Example:
  cd my-project
  grove init                  # interactive: pick auto or walkthrough
  grove init --auto           # auto, with confirm preview
  grove init --auto --yes     # CI / scripted
  grove init --walkthrough    # step-by-step
  grove init --no-hooks`,
	Args: initArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func runInit() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	if err := validateInitPreconditions(cwd); err != nil {
		return err
	}

	groveDir := filepath.Join(cwd, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("failed to create .grove directory: %w", err)
	}

	projectName := detectProjectName(cwd)

	configPath, err := writeInitConfig(groveDir, projectName)
	if err != nil {
		return err
	}

	if err := initializeState(groveDir, cwd, projectName); err != nil {
		return err
	}

	if err := grove.EnsureGroveExcludes(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to record grove's git excludes: %v\n", err)
	}

	writeEnvrc(groveDir, projectName)

	fmt.Printf("✓ Initialized grove project '%s'\n", projectName)
	fmt.Printf("  Config: %s\n", configPath)

	if !initNoHooks {
		generateAndWriteHooks(groveDir, cwd, cli.StdPrompter{})
	}

	createInitialWorktrees(groveDir, cwd, projectName)

	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("  grove new <name>   Create a new worktree")
	fmt.Println("  grove ls           List all worktrees")
	fmt.Println("  grove to <name>    Switch to a worktree")

	return nil
}

// validateInitPreconditions checks that we're in a valid state to initialize.
func validateInitPreconditions(cwd string) error {
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

	return nil
}

func writeInitConfig(groveDir, projectName string) (string, error) {
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
		return "", fmt.Errorf("failed to create config.toml: %w", err)
	}
	return configPath, nil
}

func initializeState(groveDir, cwd, projectName string) error {
	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		return fmt.Errorf("failed to initialize state: %w", err)
	}

	if err := stateMgr.SetProject(projectName); err != nil {
		return fmt.Errorf("failed to set project name: %w", err)
	}

	mainBranch := detectMainBranch(cwd)
	now := time.Now()
	mainState := &state.WorktreeState{
		Path:           cwd,
		Branch:         mainBranch,
		Root:           true,
		CreatedAt:      now,
		LastAccessedAt: now,
	}
	// Key the root under "root", the name DisplayName()/DisplayNameForPath()
	// return for the main worktree. Registering it as "main" meant every
	// runtime TouchWorktree("root")/GetWorktree("root") missed it, so the
	// root's last_accessed_at was frozen at init time (B22).
	return stateMgr.AddWorktree("root", mainState)
}

func writeEnvrc(groveDir, projectName string) {
	envrcPath := filepath.Join(groveDir, ".envrc")
	envrcContent := `# Grove shell integration
# Source this in your .envrc: source_env .grove/.envrc
export GROVE_PROJECT="` + projectName + `"
`
	if err := os.WriteFile(envrcPath, []byte(envrcContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create .envrc: %v\n", err)
	}
}

func generateAndWriteHooks(groveDir, cwd string, prompter cli.Prompter) {
	hooksPath := filepath.Join(groveDir, "hooks.toml")
	if _, err := os.Stat(hooksPath); !os.IsNotExist(err) {
		return
	}

	profile := detect.Detect(cwd)
	cfg, _ := config.Load()
	if cfg != nil && cfg.IsExternalDockerMode() && cfg.Plugins.Docker.External != nil {
		filterProfileForExternalDocker(profile, cfg.Plugins.Docker.External)
	}

	if profile.Type == "unknown" && !profile.HasDocker {
		return
	}

	decision := resolveInitMode(prompter)
	switch decision.Mode {
	case initModeSkip:
		fmt.Println("\nSkipped hooks.toml generation (per --skip / user choice).")
		return
	case initModeWalkthrough:
		profile = walkthroughProfile(profile, prompter)
	}

	hooksContent := generateHooksToml(profile)

	if decision.Mode == initModeAuto && !decision.SkipConfirm && prompter.IsInteractive() {
		// Show preview, confirm.
		fmt.Println("\nProposed .grove/hooks.toml:")
		fmt.Println(strings.TrimRight(indentLines(hooksContent, "  "), "\n"))
		fmt.Println()
		ok, err := prompter.Confirm("Write this to .grove/hooks.toml?", true)
		if err != nil || !ok {
			fmt.Println("Skipped hooks.toml generation.")
			return
		}
	}

	if err := os.WriteFile(hooksPath, []byte(hooksContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create hooks.toml: %v\n", err)
		return
	}

	printHooksSummary(profile)
}

// init UX modes
const (
	initModeAuto        = "auto"
	initModeWalkthrough = "walkthrough"
	initModeSkip        = "skip"
)

// initModeDecision captures the resolved mode plus the confirm-skip decision.
// Returned (vs mutating package globals) so runInit is reentrant.
type initModeDecision struct {
	Mode        string // initModeAuto / initModeWalkthrough / initModeSkip
	SkipConfirm bool   // true = don't show preview/confirm prompt (CI mode)
}

// resolveInitMode picks how init should generate hooks based on flags + TTY.
// Precedence: explicit flag > interactive prompt > non-TTY default (auto+yes).
// Reads package-level flag globals (set by cobra) but does NOT write them.
func resolveInitMode(prompter cli.Prompter) initModeDecision {
	switch {
	case initWalkthrough:
		return initModeDecision{Mode: initModeWalkthrough, SkipConfirm: initYes}
	case initAuto:
		return initModeDecision{Mode: initModeAuto, SkipConfirm: initYes}
	case initYes:
		// --yes alone implies --auto without confirm.
		return initModeDecision{Mode: initModeAuto, SkipConfirm: true}
	}
	if !prompter.IsInteractive() {
		// Non-TTY (CI, pipes): preserve historical zero-prompt behavior.
		return initModeDecision{Mode: initModeAuto, SkipConfirm: true}
	}
	// Interactive without flags: ask. Index-based dispatch — labels are
	// user-facing copy and may evolve; we rely on stable positions.
	const (
		idxAuto = iota
		idxWalkthrough
		idxSkip
	)
	idx, err := prompter.ChooseIndex(
		"How would you like to configure hooks.toml?",
		[]string{
			"auto (generate from detection, with preview)",
			"walkthrough (review each item interactively)",
			"skip (don't generate hooks.toml)",
		},
	)
	if err != nil {
		// Canceled/error → safest path is skip.
		return initModeDecision{Mode: initModeSkip}
	}
	switch idx {
	case idxAuto:
		return initModeDecision{Mode: initModeAuto}
	case idxWalkthrough:
		return initModeDecision{Mode: initModeWalkthrough}
	default:
		return initModeDecision{Mode: initModeSkip}
	}
}

// commandRouting is the resolved choice from a routing prompt.
type commandRouting int

const (
	routeHost commandRouting = iota
	routeContainer
	routeSkip
)

// walkthroughProfile prompts the user about each detected item so they can
// keep, route to a different runner, or drop it. Returns a new profile —
// does not mutate the input.
func walkthroughProfile(p *detect.ProjectProfile, prompter cli.Prompter) *detect.ProjectProfile {
	out := *p

	// Copy/symlink prompts: just keep/skip. filterByPrompt allocates a fresh
	// slice so the input profile's backing array isn't shared.
	out.Copy = filterByPrompt(p.Copy, "Copy file from main worktree?", prompter)
	out.Symlinks = filterByPrompt(p.Symlinks, "Symlink dir from main worktree?", prompter)

	// Host commands: route host/container/skip.
	if len(p.Commands) > 0 {
		keptHost := make([]string, 0, len(p.Commands))
		var routedToContainer []detect.ContainerCommand
		for _, cmd := range p.Commands {
			switch promptCommandRouting(cmd, p.HasDocker, p.DockerService, prompter) {
			case routeHost:
				keptHost = append(keptHost, cmd)
			case routeContainer:
				svc := p.DockerService
				if svc == "" {
					svc = "app"
				}
				routedToContainer = append(routedToContainer, detect.ContainerCommand{Service: svc, Command: cmd})
			case routeSkip:
				// drop
			}
		}
		out.Commands = keptHost
		// out.ContainerCommands may share backing array with p; allocate fresh.
		merged := make([]detect.ContainerCommand, 0, len(out.ContainerCommands)+len(routedToContainer))
		merged = append(merged, out.ContainerCommands...)
		merged = append(merged, routedToContainer...)
		out.ContainerCommands = merged
	}

	// Container commands: keep/route-to-host/skip.
	if len(p.ContainerCommands) > 0 {
		keptContainer := make([]detect.ContainerCommand, 0, len(p.ContainerCommands))
		var demoted []string
		for _, cc := range p.ContainerCommands {
			switch promptCommandRouting(cc.Command, true, cc.Service, prompter) {
			case routeHost:
				demoted = append(demoted, cc.Command)
			case routeContainer:
				keptContainer = append(keptContainer, cc)
			case routeSkip:
				// drop
			}
		}
		out.ContainerCommands = keptContainer
		merged := make([]string, 0, len(out.Commands)+len(demoted))
		merged = append(merged, out.Commands...)
		merged = append(merged, demoted...)
		out.Commands = merged
	}

	return &out
}

func filterByPrompt(items []string, q string, prompter cli.Prompter) []string {
	if len(items) == 0 {
		return items
	}
	kept := make([]string, 0, len(items))
	for _, item := range items {
		ok, err := prompter.Confirm(fmt.Sprintf("%s %q", q, item), true)
		if err == nil && ok {
			kept = append(kept, item)
		}
	}
	return kept
}

func promptCommandRouting(cmd string, hasDocker bool, service string, prompter cli.Prompter) commandRouting {
	// Build options + parallel routing slice so dispatch is index-keyed,
	// not label-text-keyed (label copy can change without breaking dispatch).
	options := []string{"host (run on host machine)"}
	routings := []commandRouting{routeHost}
	if hasDocker {
		opt := "container (run via docker compose)"
		if service != "" {
			opt = fmt.Sprintf("container (run via docker compose, service: %s)", service)
		}
		options = append(options, opt)
		routings = append(routings, routeContainer)
	}
	options = append(options, "skip (don't run this command)")
	routings = append(routings, routeSkip)

	idx, err := prompter.ChooseIndex(fmt.Sprintf("How to run %q?", cmd), options)
	if err != nil || idx < 0 || idx >= len(routings) {
		return routeSkip
	}
	return routings[idx]
}

func indentLines(s, prefix string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(prefix)
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func printHooksSummary(profile *detect.ProjectProfile) {
	typeName := profile.Type
	if profile.Type == "mixed" {
		typeName = "mixed (" + strings.Join(profile.Types, ", ") + ")"
	}
	if profile.HasDocker && !strings.Contains(typeName, "docker") {
		typeName += " + docker"
	}
	fmt.Printf("\nDetected: %s project\n", typeName)
	fmt.Printf("  Generated: .grove/hooks.toml\n")
	for _, f := range profile.Copy {
		fmt.Printf("    • copy %s\n", f)
	}
	for _, s := range profile.Symlinks {
		fmt.Printf("    • symlink %s\n", s)
	}
	for _, cc := range profile.ContainerCommands {
		fmt.Printf("    • compose run (%s): %s\n", cc.Service, cc.Command)
	}
	for _, c := range profile.Commands {
		fmt.Printf("    • run: %s\n", c)
	}
	fmt.Printf("\n  Edit hooks: grove config --hooks -e\n")
}

// createInitialWorktrees creates the --with-testing/--with-scratch/--full
// worktrees through the same path as `grove new`: worktree.Manager.Create
// (which honors the [naming] pattern instead of hardcoding {project}-{name})
// followed by setupCreatedWorktree (state registration, config symlink,
// hooks.toml post_create hooks, docker auto-start). Without this, worktrees
// grove itself just created weren't registered in state, so entering one and
// running any grove command reported it as an unrecognized drifted worktree.
func createInitialWorktrees(groveDir, cwd, projectName string) {
	if initFull {
		initWithTesting = true
		initWithScratch = true
	}

	names := []string{}
	if initWithTesting {
		names = append(names, "testing")
	}
	if initWithScratch {
		names = append(names, "scratch")
	}
	if initFull {
		names = append(names, "hotfix")
	}
	if len(names) == 0 {
		return
	}

	mgr, err := worktree.NewManager(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize worktree manager: %v\n", err)
		return
	}
	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to open state for initial worktrees: %v\n", err)
		return
	}
	cfg, err := config.LoadFromGroveDir(groveDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config for initial worktrees: %v\n", err)
		return
	}
	ctx := &GroveContext{GroveDir: groveDir, ProjectRoot: cwd, State: stateMgr, Config: cfg}
	w := cli.NewStdout()

	for _, name := range names {
		if err := mgr.Create(name, name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create %s worktree: %v\n", name, err)
			continue
		}
		if _, err := setupCreatedWorktree(ctx, mgr, name, name, worktreeSetupOpts{}, w); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set up %s worktree: %v\n", name, err)
			continue
		}
		fmt.Printf("✓ Created '%s' worktree\n", name)
	}
}

// filterProfileForExternalDocker removes entries from a profile that are already
// handled by the external docker plugin, preventing conflicts in hooks.toml.
func filterProfileForExternalDocker(profile *detect.ProjectProfile, ext *config.ExternalComposeConfig) {
	// Build sets for quick lookup
	symlinkSet := make(map[string]bool, len(ext.SymlinkDirs))
	for _, s := range ext.SymlinkDirs {
		symlinkSet[s] = true
	}
	copySet := make(map[string]bool, len(ext.CopyFiles)+len(ext.SymlinkFiles))
	for _, f := range ext.CopyFiles {
		copySet[f] = true
	}
	for _, f := range ext.SymlinkFiles {
		copySet[f] = true
	}

	// Filter symlinks
	filtered := profile.Symlinks[:0]
	for _, s := range profile.Symlinks {
		if !symlinkSet[s] {
			filtered = append(filtered, s)
		}
	}
	profile.Symlinks = filtered

	// Filter copy files
	filteredCopy := profile.Copy[:0]
	for _, f := range profile.Copy {
		if !copySet[f] {
			filteredCopy = append(filteredCopy, f)
		}
	}
	profile.Copy = filteredCopy

	// Filter commands that operate on symlinked dirs
	filteredCmds := profile.Commands[:0]
	for _, c := range profile.Commands {
		if !shouldSkipCommand(symlinkSet, c) {
			filteredCmds = append(filteredCmds, c)
		}
	}
	profile.Commands = filteredCmds

	// Same for compose-routed commands.
	if len(profile.ContainerCommands) > 0 {
		filtered := profile.ContainerCommands[:0]
		for _, cc := range profile.ContainerCommands {
			if !shouldSkipCommand(symlinkSet, cc.Command) {
				filtered = append(filtered, cc)
			}
		}
		profile.ContainerCommands = filtered
	}
}

// shouldSkipCommand returns true if a command is redundant because the
// directory it would populate is already provided via symlink.
func shouldSkipCommand(symlinkSet map[string]bool, cmd string) bool {
	if symlinkSet["vendor/bundle"] && strings.Contains(cmd, "bundle install") {
		return true
	}
	if symlinkSet["node_modules"] && strings.Contains(cmd, "npm install") {
		return true
	}
	return symlinkSet[".venv"] && strings.Contains(cmd, "pip install")
}

// generateHooksToml creates hooks.toml content from a detected profile
func generateHooksToml(profile *detect.ProjectProfile) string {
	var b strings.Builder

	header := "# Auto-generated for detected project type: " + profile.Type
	if profile.HasDocker {
		header += " (Docker detected)"
	}

	b.WriteString("# Grove hooks configuration\n")
	b.WriteString(header + "\n")
	b.WriteString("#\n")
	b.WriteString("# Edit: grove config --hooks -e\n")
	b.WriteString("# Docs: https://github.com/lost-in-the/grove#hooks\n\n")

	for _, f := range profile.Copy {
		b.WriteString("[[hooks.post_create]]\n")
		b.WriteString("type = \"copy\"\n")
		fmt.Fprintf(&b, "from = %q\n", f)
		fmt.Fprintf(&b, "to = %q\n", f)
		b.WriteString("required = false\n\n")
	}

	for _, s := range profile.Symlinks {
		b.WriteString("[[hooks.post_create]]\n")
		b.WriteString("type = \"symlink\"\n")
		fmt.Fprintf(&b, "from = %q\n", s)
		fmt.Fprintf(&b, "to = %q\n", s)
		b.WriteString("\n")
	}

	if len(profile.ContainerCommands) > 0 && profile.DockerServiceInferred {
		b.WriteString("# NOTE: service name was inferred. If your compose file uses a different\n")
		b.WriteString("# name for the application service, edit the `service = ...` lines below.\n\n")
	}
	for _, cc := range profile.ContainerCommands {
		b.WriteString("[[hooks.post_create]]\n")
		b.WriteString("type = \"docker:compose\"\n")
		fmt.Fprintf(&b, "service = %q\n", cc.Service)
		fmt.Fprintf(&b, "command = %q\n", cc.Command)
		b.WriteString("mode = \"run\"\n")
		b.WriteString("timeout = 900\n")
		b.WriteString("on_failure = \"warn\"\n\n")
	}

	if profile.DockerComposeMissing && len(profile.Commands) > 0 {
		b.WriteString("# NOTE: Docker detected but no compose file found (or no app service\n")
		b.WriteString("# could be inferred). The commands below are kept as host commands.\n")
		b.WriteString("# If your toolchain lives in a container, replace each with a\n")
		b.WriteString("# `type = \"docker:compose\"` block (with service = \"...\") or a\n")
		b.WriteString("# `type = \"docker:exec\"` block (with container = \"...\").\n")
		b.WriteString("# See: docs/CONFIGURATION_REFERENCE.md\n\n")
	}

	for _, c := range profile.Commands {
		b.WriteString("[[hooks.post_create]]\n")
		b.WriteString("type = \"command\"\n")
		fmt.Fprintf(&b, "command = %q\n", c)
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
	initCmd.Flags().BoolVar(&initAuto, "auto", false, "Auto-generate hooks.toml from detection (default for non-TTY)")
	initCmd.Flags().BoolVar(&initWalkthrough, "walkthrough", false, "Step through detected items interactively")
	initCmd.Flags().BoolVar(&initYes, "yes", false, "Skip the preview/confirm prompt (CI mode)")
	initCmd.MarkFlagsMutuallyExclusive("auto", "walkthrough")
	initCmd.MarkFlagsMutuallyExclusive("no-hooks", "auto")
	initCmd.MarkFlagsMutuallyExclusive("no-hooks", "walkthrough")
	rootCmd.AddCommand(initCmd)
}
