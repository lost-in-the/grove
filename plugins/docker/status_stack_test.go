package docker

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lost-in-the/grove/internal/plugins"
)

// sharedStackFixture is an external setup with the stack on main and a second
// worktree, feature, that the stack is not on.
func sharedStackFixture(t *testing.T, envBody func(main string) string) (s *externalStrategy, main, feature string) {
	t.Helper()
	root := t.TempDir()
	main = filepath.Join(root, "myapp")
	feature = filepath.Join(root, "myapp-feature")
	cfg := agentEnabledCfg(t, t.TempDir(), envBody(main), "")
	return newExternalStrategy(cfg), main, feature
}

func onMain(main string) string { return "APP_DIR=" + main + "\n" }

func TestExternalStatuses_StackOnThisWorktreeRunning(t *testing.T) {
	s, main, feature := sharedStackFixture(t, onMain)
	stubProbe(t, []ServiceStatus{{Name: "app", Status: ServiceRunning}}, nil)

	got := externalStatuses(s, []string{main, feature})[main]

	want := &plugins.SharedStack{OnPath: main, Here: true, State: plugins.StackRunning, Running: 1}
	if !reflect.DeepEqual(got.Stack, want) {
		t.Errorf("Stack = %+v, want %+v", got.Stack, want)
	}
	if got.Short != "up" || got.Detail != "1 service(s) running, the stack is on this worktree" {
		t.Errorf("Short/Detail changed: %q / %q", got.Short, got.Detail)
	}
}

func TestExternalStatuses_StackOnAnotherWorktreeSaysWhereAndStaysOutOfLists(t *testing.T) {
	s, main, feature := sharedStackFixture(t, onMain)
	stubProbe(t, []ServiceStatus{{Name: "app", Status: ServiceRunning}}, nil)

	got, ok := externalStatuses(s, []string{main, feature})[feature]

	if !ok {
		t.Fatal("no entry for a worktree the stack is not on; the dashboard cannot say where the stack is")
	}
	if want := (&plugins.SharedStack{OnPath: main}); !reflect.DeepEqual(got.Stack, want) {
		t.Errorf("Stack = %+v, want %+v", got.Stack, want)
	}
	if got.Short != "" || got.Detail != "" {
		t.Errorf("Short/Detail = %q / %q, want both empty so grove ls shows nothing new", got.Short, got.Detail)
	}
}

func TestExternalStatuses_StackOnAWorktreeNotListed(t *testing.T) {
	s, _, feature := sharedStackFixture(t, func(string) string { return "APP_DIR=./myapp-other\n" })
	stubProbe(t, nil, errors.New("must not probe when the stack is on none of the listed worktrees"))

	got := externalStatuses(s, []string{feature})[feature]

	if got.Stack == nil || got.Stack.Here || filepath.Base(got.Stack.OnPath) != "myapp-other" {
		t.Errorf("Stack = %+v, want OnPath ending in myapp-other, not here", got.Stack)
	}
}

func TestExternalStatuses_StackNotOnAnyWorktree(t *testing.T) {
	s, main, feature := sharedStackFixture(t, func(string) string { return "" })
	stubProbe(t, nil, errors.New("must not probe when the stack is on no worktree"))

	statuses := externalStatuses(s, []string{main, feature})
	if len(statuses) != 2 {
		t.Fatalf("got %d entries, want one per worktree saying the stack is not on any", len(statuses))
	}
	for path, got := range statuses {
		if want := (&plugins.SharedStack{}); !reflect.DeepEqual(got.Stack, want) {
			t.Errorf("%s: Stack = %+v, want %+v (not on any worktree)", filepath.Base(path), got.Stack, want)
		}
		if got.Short != "" || got.Detail != "" {
			t.Errorf("%s: Short/Detail = %q / %q, want both empty", filepath.Base(path), got.Short, got.Detail)
		}
	}
}

func TestExternalStatuses_StackUnknownAndDegraded(t *testing.T) {
	for _, tc := range []struct {
		name     string
		statuses []ServiceStatus
		err      error
		want     plugins.SharedStack
	}{
		{"unknown", nil, errors.New("timeout"), plugins.SharedStack{Here: true, State: plugins.StackUnknown}},
		{"degraded", []ServiceStatus{{Name: "app", Status: ServiceExitedError}}, nil,
			plugins.SharedStack{Here: true, State: plugins.StackDegraded, Blockers: []string{"app"}}},
		{"stopped", []ServiceStatus{{Name: "app", Status: ServiceExitedClean}}, nil,
			plugins.SharedStack{Here: true, State: plugins.StackStopped}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, main, _ := sharedStackFixture(t, onMain)
			stubProbe(t, tc.statuses, tc.err)

			got := externalStatuses(s, []string{main})[main]

			tc.want.OnPath = main
			if !reflect.DeepEqual(got.Stack, &tc.want) {
				t.Errorf("Stack = %+v, want %+v", got.Stack, &tc.want)
			}
		})
	}
}

func TestExternalStatuses_AgentSlotEntryHasNoSharedStack(t *testing.T) {
	root := t.TempDir()
	slotted := filepath.Join(root, "myapp-feature")
	cfg := agentEnabledCfg(t, t.TempDir(), "APP_DIR="+filepath.Join(root, "myapp")+"\n", `[{"slot":1,"worktree":"myapp-feature"}]`)

	got := externalStatuses(newExternalStrategy(cfg), []string{slotted})[slotted]

	if got.Short != "#1" || got.Stack != nil {
		t.Errorf("slotted worktree = %+v, want the agent entry with no shared stack", got)
	}
}
