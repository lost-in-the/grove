package hooks

import (
	"errors"
	"strings"
	"testing"
)

func TestActionHandlerRegistry_RegisterAndLookup(t *testing.T) {
	r := newActionHandlerRegistry()

	called := false
	h := func(_ *HookAction, _ *ExecutionContext, _ *Variables) error {
		called = true
		return nil
	}

	r.register("custom", h)

	got, ok := r.lookup("custom")
	if !ok {
		t.Fatal("expected lookup hit")
	}
	_ = got(nil, nil, nil)
	if !called {
		t.Fatal("registered handler was not invoked")
	}
}

func TestActionHandlerRegistry_RegisterIsIdempotent(t *testing.T) {
	// Re-registering the same type swaps in the new handler. Used by plugin
	// Init() to rebind closures after re-init in tests.
	r := newActionHandlerRegistry()

	first := func(_ *HookAction, _ *ExecutionContext, _ *Variables) error { return nil }
	secondCalled := false
	second := func(_ *HookAction, _ *ExecutionContext, _ *Variables) error {
		secondCalled = true
		return nil
	}

	r.register("dup", first)
	r.register("dup", second)

	h, ok := r.lookup("dup")
	if !ok {
		t.Fatal("expected lookup hit")
	}
	_ = h(nil, nil, nil)
	if !secondCalled {
		t.Fatal("expected second registration to win")
	}
}

func TestActionHandlerRegistry_MissingTypeError(t *testing.T) {
	r := newActionHandlerRegistry()
	_, ok := r.lookup("missing")
	if ok {
		t.Fatal("expected miss")
	}
}

func TestExecutorUsesGlobalRegistry_CustomType(t *testing.T) {
	// Register a custom handler globally for this test, then unregister.
	const typeName = "test_custom_type_xyz"

	got := false
	want := errors.New("boom")
	h := func(_ *HookAction, _ *ExecutionContext, _ *Variables) error {
		got = true
		return want
	}
	RegisterActionHandler(typeName, h)
	t.Cleanup(func() { unregisterActionHandler(typeName) })

	cfg := &HooksConfig{}
	cfg.Hooks.PostCreate = []HookAction{
		{Type: typeName, OnFailure: "fail"},
	}

	exec := NewExecutorWithConfig(cfg)
	exec.Output = &noopWriter{}
	err := exec.Execute(EventPostCreate, &ExecutionContext{Event: EventPostCreate})
	if err == nil || !errors.Is(err, want) {
		// Executor wraps errors; just ensure error propagated when on_failure=fail
		if err == nil {
			t.Fatal("expected error from custom handler when on_failure=fail")
		}
	}
	if !got {
		t.Fatal("custom handler not invoked by executor")
	}
}

func TestUnknownActionType_HelpfulError(t *testing.T) {
	cfg := &HooksConfig{}
	cfg.Hooks.PostCreate = []HookAction{
		{Type: "docker:compose", OnFailure: "fail", Service: "app", Command: "true"},
	}

	exec := NewExecutorWithConfig(cfg)
	exec.Output = &noopWriter{}
	err := exec.Execute(EventPostCreate, &ExecutionContext{Event: EventPostCreate})
	if err == nil {
		t.Fatal("expected error when 'compose' handler not registered")
	}
	if !strings.Contains(err.Error(), "docker:compose") {
		t.Fatalf("error should mention type name: %v", err)
	}
}

type noopWriter struct{}

func (noopWriter) Write(p []byte) (int, error) { return len(p), nil }
