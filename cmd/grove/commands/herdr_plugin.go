package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/state"
)

// This file implements the grove side of the herdr plugin shipped in
// integrations/herdr. herdr invokes plugin commands as plain argv processes and
// passes context through the environment, so these subcommands are hidden —
// they are an integration surface, not something a user types.
//
// See integrations/herdr/README.md for the manifest and install instructions.

// herdrContext is the invocation context herdr passes to a plugin action,
// decoded from HERDR_PLUGIN_CONTEXT_JSON.
type herdrContext struct {
	WorkspaceID    string `json:"workspace_id"`
	WorkspaceLabel string `json:"workspace_label"`
	WorkspaceCwd   string `json:"workspace_cwd"`
	FocusedPaneID  string `json:"focused_pane_id"`
	FocusedPaneCwd string `json:"focused_pane_cwd"`
	Worktree       *struct {
		RepoName     string `json:"repo_name"`
		RepoRoot     string `json:"repo_root"`
		CheckoutPath string `json:"checkout_path"`
	} `json:"worktree"`
}

// CheckoutPath returns the directory the invocation refers to: the workspace's
// git checkout, or its plain cwd when the workspace has no git provenance.
//
// It does not check the filesystem — use ResolveDir for that.
func (c *herdrContext) CheckoutPath() string {
	if c.Worktree != nil && c.Worktree.CheckoutPath != "" {
		return c.Worktree.CheckoutPath
	}
	return c.WorkspaceCwd
}

// ResolveDir returns the first directory in the context that still exists.
//
// herdr's worktree provenance is captured when a workspace is opened and does
// not follow the checkout afterwards, so a `grove rename` leaves
// worktree.checkout_path pointing at a directory that is no longer there. The
// workspace cwd and the focused pane's cwd are the live fallbacks.
func (c *herdrContext) ResolveDir() string {
	candidates := []string{c.CheckoutPath(), c.WorkspaceCwd, c.FocusedPaneCwd}
	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	// Nothing resolved; return the nominal path so the error names something
	// recognizable rather than being empty.
	return c.CheckoutPath()
}

// RepoRoot returns the main checkout of the workspace's repository, or empty
// when herdr reported no git provenance.
func (c *herdrContext) RepoRoot() string {
	if c.Worktree == nil {
		return ""
	}
	return c.Worktree.RepoRoot
}

func parseHerdrContext(raw string) (*herdrContext, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("no herdr plugin context (is grove being run as a herdr plugin?)")
	}
	var ctx herdrContext
	if err := json.Unmarshal([]byte(raw), &ctx); err != nil {
		return nil, fmt.Errorf("decode herdr plugin context: %w", err)
	}
	return &ctx, nil
}

// herdrEvent is the event envelope herdr passes to a plugin event hook,
// decoded from HERDR_PLUGIN_EVENT_JSON.
type herdrEvent struct {
	Event string
	Data  struct {
		WorkspaceID string `json:"workspace_id"`
		Worktree    *struct {
			Path  string `json:"path"`
			Label string `json:"label"`
		} `json:"worktree"`
		AlreadyOpen bool `json:"already_open"`
	}
	AlreadyOpen bool
}

// CheckoutPath returns the worktree checkout the event refers to.
func (e *herdrEvent) CheckoutPath() string {
	if e.Data.Worktree == nil {
		return ""
	}
	return e.Data.Worktree.Path
}

// normalizeHerdrEventName converts herdr's snake_case wire form to the dotted
// form used in manifests and HERDR_PLUGIN_EVENT, so both resolve alike.
func normalizeHerdrEventName(name string) string {
	return strings.ReplaceAll(name, "_", ".")
}

func parseHerdrEvent(raw string) (*herdrEvent, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("no herdr plugin event (is grove being run as a herdr event hook?)")
	}
	var envelope struct {
		Event string          `json:"event"`
		Data  json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, fmt.Errorf("decode herdr plugin event: %w", err)
	}

	event := &herdrEvent{Event: normalizeHerdrEventName(envelope.Event)}
	if len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, &event.Data); err != nil {
			return nil, fmt.Errorf("decode herdr event data: %w", err)
		}
	}
	event.AlreadyOpen = event.Data.AlreadyOpen
	return event, nil
}

var herdrEventCmd = &cobra.Command{
	Use:    "herdr-event",
	Short:  "Handle a herdr plugin event (internal)",
	Hidden: true,
	Long: `Handles a herdr event hook invocation.

herdr passes the event in HERDR_PLUGIN_EVENT_JSON. This command is registered
by the plugin in integrations/herdr and is not meant to be run by hand.

On worktree.opened it reports whether grove already tracks the checkout, so a
worktree created through herdr's own UI does not silently bypass grove's
bootstrap. It never modifies anything — adopting is left to 'grove adopt', which
runs post-create hooks the user should opt into.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cli.NewStderr()

		event, err := parseHerdrEvent(os.Getenv("HERDR_PLUGIN_EVENT_JSON"))
		if err != nil {
			return err
		}

		// HERDR_PLUGIN_EVENT carries the dotted name directly; prefer it when
		// present since it is the value the manifest matched on.
		name := event.Event
		if fromEnv := os.Getenv("HERDR_PLUGIN_EVENT"); fromEnv != "" {
			name = normalizeHerdrEventName(fromEnv)
		}

		if name != "worktree.opened" {
			return nil
		}
		// Focusing an already-open workspace re-fires the event. Staying quiet
		// keeps this from nagging every time the user clicks a workspace.
		if event.AlreadyOpen {
			return nil
		}

		path := event.CheckoutPath()
		if path == "" {
			return nil
		}

		tracked, err := groveTracksWorktree(path)
		if err != nil {
			// Not a grove project, or not readable — nothing to say.
			return nil //nolint:nilerr // a non-grove worktree is not an error
		}
		if tracked {
			return nil
		}

		cli.Warning(w, "grove does not track the worktree at %s", path)
		cli.Faint(w, "Run 'grove adopt %s' to bootstrap it (state, excludes, hooks).", path)
		return nil
	},
}

// groveContextForPath builds a minimal context for a directory, without the
// cwd-relative discovery, drift notices, or plugin registration that
// RequireGroveContext performs. herdr invokes plugin commands from an
// arbitrary cwd, so discovery has to start from the path herdr named.
func groveContextForPath(path string) (*GroveContext, error) {
	groveDir, err := grove.FindRoot(path)
	if err != nil {
		return nil, err
	}
	if groveDir == "" {
		return nil, fmt.Errorf("%s is not inside a grove project", path)
	}

	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize state: %w", err)
	}

	cfg, err := config.LoadFromGroveDir(groveDir)
	if err != nil {
		cfg = config.LoadDefaults()
	}

	return &GroveContext{
		GroveDir:    groveDir,
		ProjectRoot: grove.MustProjectRoot(groveDir),
		State:       stateMgr,
		Config:      cfg,
	}, nil
}

// groveTracksWorktree reports whether the checkout at path belongs to a grove
// project that already has it in state.
func groveTracksWorktree(path string) (bool, error) {
	ctx, err := groveContextForPath(path)
	if err != nil {
		return false, err
	}

	mgr, err := ctx.WorktreeManager()
	if err != nil {
		return false, err
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = path
	}
	// The main checkout is always implicitly tracked.
	if same, _ := samePath(resolved, ctx.ProjectRoot); same {
		return true, nil
	}

	name := mgr.DisplayNameForPath(resolved)
	if name == "" {
		return false, nil
	}
	ws, err := ctx.State.GetWorktree(name)
	return err == nil && ws != nil, nil
}

// samePath compares two paths with symlinks resolved on both sides.
func samePath(a, b string) (bool, error) {
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		ra = a
	}
	rb, err := filepath.EvalSymlinks(b)
	if err != nil {
		rb = b
	}
	return filepath.Clean(ra) == filepath.Clean(rb), nil
}

var herdrActionCmd = &cobra.Command{
	Use:    "herdr-action <action>",
	Short:  "Run a herdr plugin action (internal)",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	Long: `Runs a herdr plugin action against the invoking workspace.

herdr passes the workspace in HERDR_PLUGIN_CONTEXT_JSON. This command is
registered by the plugin in integrations/herdr and is not meant to be run by
hand.

Actions:
  status  Print grove's view of the workspace's worktree.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cli.NewStdout()

		hctx, err := parseHerdrContext(os.Getenv("HERDR_PLUGIN_CONTEXT_JSON"))
		if err != nil {
			return err
		}

		path := hctx.ResolveDir()
		if path == "" {
			return fmt.Errorf("herdr workspace has no directory to act on")
		}

		switch args[0] {
		case "status":
			tracked, err := groveTracksWorktree(path)
			if err != nil {
				return fmt.Errorf("%s is not inside a grove project", path)
			}
			if tracked {
				cli.Success(w, "grove tracks %s", path)
				return nil
			}
			cli.Warning(w, "grove does not track %s", path)
			cli.Faint(w, "Run 'grove adopt %s' to bootstrap it.", path)
			return nil
		default:
			return fmt.Errorf("unknown herdr action %q", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(herdrEventCmd)
	rootCmd.AddCommand(herdrActionCmd)
}
