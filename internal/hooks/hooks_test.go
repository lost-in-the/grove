package hooks

import (
	"fmt"
	"testing"
)

func TestHookRegistry(t *testing.T) {
	registry := NewRegistry()

	called := false
	testHook := func(ctx *Context) error {
		called = true
		return nil
	}

	// Test registration
	registry.Register("test-event", testHook)

	// Test firing
	err := registry.Fire("test-event", &Context{})
	if err != nil {
		t.Errorf("Fire() error = %v", err)
	}

	if !called {
		t.Error("Hook was not called")
	}
}

func TestMultipleHooks(t *testing.T) {
	registry := NewRegistry()

	callCount := 0
	hook1 := func(ctx *Context) error {
		callCount++
		return nil
	}
	hook2 := func(ctx *Context) error {
		callCount++
		return nil
	}

	registry.Register("test-event", hook1)
	registry.Register("test-event", hook2)

	err := registry.Fire("test-event", &Context{})
	if err != nil {
		t.Errorf("Fire() error = %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 hooks to be called, got %d", callCount)
	}
}

func TestHookFailure(t *testing.T) {
	registry := NewRegistry()

	hook1 := func(ctx *Context) error {
		return nil
	}
	hook2 := func(ctx *Context) error {
		return fmt.Errorf("hook failed")
	}
	hook3 := func(ctx *Context) error {
		return nil
	}

	registry.Register("test-event", hook1)
	registry.Register("test-event", hook2)
	registry.Register("test-event", hook3)

	// Fire should continue even if one hook fails
	err := registry.Fire("test-event", &Context{})

	// Should return error but continue executing other hooks
	if err == nil {
		t.Error("Expected error from failing hook")
	}
}

func TestNoHooksRegistered(t *testing.T) {
	registry := NewRegistry()

	// Should not error if no hooks are registered
	err := registry.Fire("nonexistent-event", &Context{})
	if err != nil {
		t.Errorf("Fire() error = %v, expected nil", err)
	}
}

func TestGetEventName(t *testing.T) {
	tests := []struct {
		event    string
		expected string
	}{
		{EventPreCreate, "Pre-Create"},
		{EventPostCreate, "Post-Create"},
		{EventPreSwitch, "Pre-Switch"},
		{EventPostSwitch, "Post-Switch"},
		{EventPreRemove, "Pre-Remove"},
		{EventPostRemove, "Post-Remove"},
		{"unknown-event", "unknown-event"}, // Unknown events return themselves
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			got := GetEventName(tt.event)
			if got != tt.expected {
				t.Errorf("GetEventName(%q) = %q, want %q", tt.event, got, tt.expected)
			}
		})
	}
}

func TestValidateEvent(t *testing.T) {
	tests := []struct {
		event   string
		wantErr bool
	}{
		{EventPreCreate, false},
		{EventPostCreate, false},
		{EventPreSwitch, false},
		{EventPostSwitch, false},
		{EventPreRemove, false},
		{EventPostRemove, false},
		{"invalid-event", true},
		{"", true},
		{"pre-create-typo", true},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			err := ValidateEvent(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEvent(%q) error = %v, wantErr %v", tt.event, err, tt.wantErr)
			}
		})
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Reset global registry for test isolation
	oldRegistry := globalRegistry
	globalRegistry = NewRegistry()
	defer func() { globalRegistry = oldRegistry }()

	called := false
	Register("test-global", func(ctx *Context) error {
		called = true
		return nil
	})

	err := Fire("test-global", &Context{Worktree: "test-wt"})
	if err != nil {
		t.Errorf("Fire() error = %v", err)
	}

	if !called {
		t.Error("Global hook was not called")
	}
}

func TestGlobalRegistryNoHooks(t *testing.T) {
	// Reset global registry for test isolation
	oldRegistry := globalRegistry
	globalRegistry = NewRegistry()
	defer func() { globalRegistry = oldRegistry }()

	// Should not error when no hooks registered
	err := Fire("nonexistent", &Context{})
	if err != nil {
		t.Errorf("Fire() error = %v, expected nil", err)
	}
}
