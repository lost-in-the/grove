package hooks

import (
	"fmt"
	"log"

	"github.com/lost-in-the/grove/internal/config"
)

// Hook is a function that can be registered to run at specific lifecycle points
type Hook func(ctx *Context) error

// Context contains information passed to hooks
type Context struct {
	Worktree         string                 // Current worktree name
	PrevWorktree     string                 // Previous worktree name
	Config           *config.Config         // Grove configuration
	Data             map[string]interface{} // Additional data for plugins
	WorktreePath     string                 // Absolute path to current/new worktree
	PrevWorktreePath string                 // Absolute path to previous worktree
	MainPath         string                 // Absolute path to main worktree (project root)
}

// Registry manages hook registration and execution
type Registry struct {
	hooks map[string][]Hook
}

// NewRegistry creates a new hook registry
func NewRegistry() *Registry {
	return &Registry{
		hooks: make(map[string][]Hook),
	}
}

// Register adds a hook for a specific event
func (r *Registry) Register(event string, hook Hook) {
	r.hooks[event] = append(r.hooks[event], hook)
}

// Fire executes all hooks registered for an event
// If a hook returns an error, it logs the error but continues executing other hooks
func (r *Registry) Fire(event string, ctx *Context) error {
	hooks, ok := r.hooks[event]
	if !ok {
		return nil // No hooks registered for this event
	}

	var firstErr error
	for i, hook := range hooks {
		if err := hook(ctx); err != nil {
			log.Printf("hook %d for event '%s' failed: %v", i, event, err)
			if firstErr == nil {
				firstErr = err
			}
			// Continue executing other hooks
		}
	}

	return firstErr
}

// List of standard hook events
const (
	// EventPreCreate fires before creating a worktree
	EventPreCreate = "pre-create"
	// EventPostCreate fires after creating a worktree
	EventPostCreate = "post-create"
	// EventPreSwitch fires before switching to a worktree
	EventPreSwitch = "pre-switch"
	// EventPostSwitch fires after switching to a worktree
	EventPostSwitch = "post-switch"
	// EventPreRemove fires before removing a worktree
	EventPreRemove = "pre-remove"
	// EventPostRemove fires after removing a worktree
	EventPostRemove = "post-remove"
)

// GetEventName returns a human-readable name for an event
func GetEventName(event string) string {
	names := map[string]string{
		EventPreCreate:  "Pre-Create",
		EventPostCreate: "Post-Create",
		EventPreSwitch:  "Pre-Switch",
		EventPostSwitch: "Post-Switch",
		EventPreRemove:  "Pre-Remove",
		EventPostRemove: "Post-Remove",
	}

	if name, ok := names[event]; ok {
		return name
	}
	return event
}

// ValidateEvent checks if an event name is valid
func ValidateEvent(event string) error {
	validEvents := []string{
		EventPreCreate,
		EventPostCreate,
		EventPreSwitch,
		EventPostSwitch,
		EventPreRemove,
		EventPostRemove,
	}

	for _, valid := range validEvents {
		if event == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid event: %s", event)
}

// Global registry instance (can be replaced with DI in future)
var globalRegistry = NewRegistry()

// GlobalRegistry returns the global hook registry for plugin registration.
func GlobalRegistry() *Registry {
	return globalRegistry
}

// Register adds a hook to the global registry
func Register(event string, hook Hook) {
	globalRegistry.Register(event, hook)
}

// Fire executes hooks in the global registry
func Fire(event string, ctx *Context) error {
	return globalRegistry.Fire(event, ctx)
}

// HasHooks reports whether any hooks are registered for the given event.
func HasHooks(event string) bool {
	return globalRegistry.HasHooks(event)
}

// HasHooks reports whether any hooks are registered for the given event.
func (r *Registry) HasHooks(event string) bool {
	return len(r.hooks[event]) > 0
}
