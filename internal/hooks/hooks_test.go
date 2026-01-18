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
