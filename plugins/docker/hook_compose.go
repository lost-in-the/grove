package docker

import (
	"errors"
	"fmt"

	"github.com/lost-in-the/grove/internal/hooks"
)

// composeHandler returns a hooks.ActionHandler bound to the plugin's
// configured strategy. Called by Plugin.Init during startup.
//
// Hook order: helpers.go fires plugin Go hooks (which Up containers when
// auto_start is on) BEFORE config-driven hooks. So by the time this handler
// runs on post_create, the local-strategy stack is up. External / agent
// strategies expect the user to have started their stack with `grove up`.
func (p *Plugin) composeHandler(action *hooks.HookAction, ctx *hooks.ExecutionContext, vars *hooks.Variables) error {
	if !p.enabled || p.strategy == nil {
		return fmt.Errorf("docker:compose hook: docker plugin not enabled")
	}
	service := vars.Interpolate(action.Service)
	if service == "" {
		return fmt.Errorf("docker:compose hook: 'service' is required")
	}
	// Rewrite the command to reference GROVE_HOOK_* env vars rather than splice
	// values into the `bash -cil` string; the values ride along as container
	// environment (hookEnv), so an untrusted {{.branch}} from `grove fetch pr/N`
	// cannot inject a command (B13).
	command := vars.InterpolateShell(action.Command)
	if command == "" {
		return fmt.Errorf("docker:compose hook: 'command' is required")
	}
	hookEnv := vars.ShellEnv()

	wtPath := ctx.NewPath
	if wtPath == "" {
		// Switch events use the worktree being switched to; create has only
		// NewPath. Fall back to MainPath as a last resort so docker compose
		// can still find the project root.
		wtPath = ctx.MainPath
	}

	mode := action.Mode
	if mode == "" {
		mode = "run"
	}

	w := ctx.Out()

	switch mode {
	case "run":
		// `compose run --rm` brings up service deps on demand.
		if err := p.strategy.Run(wtPath, service, command, hookEnv); err != nil {
			if errors.Is(err, ErrNoComposeFile) {
				return fmt.Errorf("docker:compose hook: no compose file found at %s — did you mean type = \"docker:exec\"?", wtPath)
			}
			return fmt.Errorf("docker:compose run %s: %w", service, err)
		}
	case "exec":
		// exec requires a running container. Plugin Go hooks fire before
		// config hooks (helpers.go runPostCreateHooks), so a stack with
		// auto_start = true will already be up. We do NOT call Up() here —
		// silent side effects in a hook are surprising.
		if err := p.strategy.Exec(wtPath, service, command, hookEnv); err != nil {
			if errors.Is(err, ErrNoComposeFile) {
				return fmt.Errorf("docker:compose hook: no compose file found at %s — did you mean type = \"docker:exec\"?", wtPath)
			}
			return fmt.Errorf("docker:compose exec %s: %w (is the stack up? try mode = \"run\" or `grove up`)", service, err)
		}
	default:
		return fmt.Errorf("docker:compose hook: invalid mode %q (want \"run\" or \"exec\")", mode)
	}

	_, _ = fmt.Fprintf(w, "✓ Ran in %s container (service: %s): %s\n", mode, service, command)
	return nil
}
