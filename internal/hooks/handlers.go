package hooks

import (
	"sync"
)

// ActionHandler is the function signature for hook action handlers.
// Plugins register handlers for new action types via RegisterActionHandler.
//
// Stability: this signature is the stable plugin extension point as of
// v0.7.0. Adding fields to HookAction, ExecutionContext, or Variables is
// backwards-compatible; renaming or removing fields is not. Breaking
// changes will bump grove's minor version and be called out in CHANGELOG.
type ActionHandler func(action *HookAction, ctx *ExecutionContext, vars *Variables) error

// actionHandlerRegistry is the in-process registry of action handlers.
type actionHandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[string]ActionHandler
}

func newActionHandlerRegistry() *actionHandlerRegistry {
	return &actionHandlerRegistry{handlers: map[string]ActionHandler{}}
}

func (r *actionHandlerRegistry) register(typeName string, h ActionHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[typeName] = h
}

func (r *actionHandlerRegistry) lookup(typeName string) (ActionHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[typeName]
	return h, ok
}

func (r *actionHandlerRegistry) unregister(typeName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, typeName)
}

// globalActionHandlers is the process-wide registry plugins extend.
var globalActionHandlers = newActionHandlerRegistry()

// RegisterActionHandler installs a handler for the given action type.
// Plugins call this during their Init() to add custom hook types.
//
// Idempotent: re-registering the same type replaces the previous handler.
// This lets plugin Init() run repeatedly across tests without bookkeeping;
// the cost is silently masking conflicts between two plugins claiming the
// same name. Use namespaced type names (`pluginname:action`) to avoid
// collisions, e.g. "docker:compose", "docker:exec".
//
// Stability: this is the stable plugin extension point as of v0.7.0. See
// the ActionHandler type for the compatibility contract.
func RegisterActionHandler(typeName string, h ActionHandler) {
	globalActionHandlers.register(typeName, h)
}

// unregisterActionHandler removes a registered handler. Test-only helper.
func unregisterActionHandler(typeName string) {
	globalActionHandlers.unregister(typeName)
}

// LookupActionHandler returns a handler if one is registered for the given type.
func LookupActionHandler(typeName string) (ActionHandler, bool) {
	return globalActionHandlers.lookup(typeName)
}
