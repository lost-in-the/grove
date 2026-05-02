package commands

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/detect"
)

// fakePrompter is a scripted cli.Prompter used by tests that need to drive
// the interactive code paths without raw-TTY stdin. ChooseIndex pops from
// chooseQueue; Confirm pops from confirmQueue. Calls beyond the queue
// length panic, so each test case is forced to declare exactly the prompts
// it expects.
type fakePrompter struct {
	interactive  bool
	chooseQueue  []chooseResult
	confirmQueue []confirmResult
	chooseLog    []chooseCall
	confirmLog   []confirmCall
}

type chooseResult struct {
	idx int
	err error
}

type confirmResult struct {
	ok  bool
	err error
}

type chooseCall struct {
	title   string
	options []string
}

type confirmCall struct {
	question   string
	defaultYes bool
}

func (p *fakePrompter) IsInteractive() bool { return p.interactive }

func (p *fakePrompter) ChooseIndex(title string, options []string) (int, error) {
	p.chooseLog = append(p.chooseLog, chooseCall{title: title, options: append([]string{}, options...)})
	if len(p.chooseQueue) == 0 {
		panic(fmt.Sprintf("fakePrompter.ChooseIndex called more times than scripted (title=%q)", title))
	}
	r := p.chooseQueue[0]
	p.chooseQueue = p.chooseQueue[1:]
	return r.idx, r.err
}

func (p *fakePrompter) Confirm(question string, defaultYes bool) (bool, error) {
	p.confirmLog = append(p.confirmLog, confirmCall{question: question, defaultYes: defaultYes})
	if len(p.confirmQueue) == 0 {
		panic(fmt.Sprintf("fakePrompter.Confirm called more times than scripted (question=%q)", question))
	}
	r := p.confirmQueue[0]
	p.confirmQueue = p.confirmQueue[1:]
	return r.ok, r.err
}

func TestGenerateHooksToml_HostCommand(t *testing.T) {
	p := &detect.ProjectProfile{
		Type:     "rails",
		Commands: []string{"bundle install --quiet"},
	}
	got := generateHooksToml(p)
	if !strings.Contains(got, `type = "command"`) {
		t.Error("expected command-type hook for plain Rails")
	}
	if strings.Contains(got, `type = "docker:compose"`) {
		t.Error("expected NO compose-type hook when Docker not detected")
	}
}

func TestGenerateHooksToml_ContainerCommandRendersAsCompose(t *testing.T) {
	p := &detect.ProjectProfile{
		Type:                  "rails",
		HasDocker:             true,
		DockerService:         "web",
		DockerServiceInferred: true,
		ContainerCommands: []detect.ContainerCommand{
			{Service: "web", Command: "bundle install --quiet"},
		},
	}
	got := generateHooksToml(p)

	if !strings.Contains(got, `type = "docker:compose"`) {
		t.Errorf("expected compose-type hook, got:\n%s", got)
	}
	if !strings.Contains(got, `service = "web"`) {
		t.Error("expected service=web in rendered hook")
	}
	if !strings.Contains(got, `command = "bundle install --quiet"`) {
		t.Error("expected bundle install command in rendered hook")
	}
	if !strings.Contains(got, `mode = "run"`) {
		t.Error("expected mode=run default")
	}
	if !strings.Contains(got, "service name was inferred") {
		t.Error("expected inferred-service comment when DockerServiceInferred=true")
	}
}

func TestGenerateHooksToml_NoInferredCommentWhenSingleService(t *testing.T) {
	p := &detect.ProjectProfile{
		Type:                  "rails",
		HasDocker:             true,
		DockerService:         "app",
		DockerServiceInferred: false,
		ContainerCommands: []detect.ContainerCommand{
			{Service: "app", Command: "bundle install"},
		},
	}
	got := generateHooksToml(p)
	if strings.Contains(got, "service name was inferred") {
		t.Errorf("should not emit inferred comment when single-service compose. Got:\n%s", got)
	}
}

func TestGenerateHooksToml_MixedHostAndContainer(t *testing.T) {
	p := &detect.ProjectProfile{
		Type:      "mixed",
		HasDocker: true,
		Commands:  []string{"echo host"},
		ContainerCommands: []detect.ContainerCommand{
			{Service: "app", Command: "echo container"},
		},
	}
	got := generateHooksToml(p)
	if !strings.Contains(got, `type = "command"`) {
		t.Error("expected host command type")
	}
	if !strings.Contains(got, `type = "docker:compose"`) {
		t.Error("expected compose type")
	}
}

// TestResolveInitMode_FlagPrecedence covers the non-interactive precedence
// rules of resolveInitMode. The interactive prompt path is not exercised
// because cli.IsInteractive reads os.Stdin.Fd() directly; in `go test`
// stdin is not a TTY, so IsInteractive() is false and the prompt branch
// is unreachable from a test process. That branch is covered manually.
func TestResolveInitMode_FlagPrecedence(t *testing.T) {
	// Save and restore package-level flag globals so cases don't leak.
	origAuto, origWalkthrough, origYes := initAuto, initWalkthrough, initYes
	t.Cleanup(func() {
		initAuto, initWalkthrough, initYes = origAuto, origWalkthrough, origYes
	})

	cases := []struct {
		name            string
		auto            bool
		walkthrough     bool
		yes             bool
		wantMode        string
		wantSkipConfirm bool
	}{
		{
			name:            "walkthrough flag wins over auto and yes",
			walkthrough:     true,
			auto:            true,
			yes:             true,
			wantMode:        initModeWalkthrough,
			wantSkipConfirm: true,
		},
		{
			name:            "walkthrough alone keeps confirm",
			walkthrough:     true,
			wantMode:        initModeWalkthrough,
			wantSkipConfirm: false,
		},
		{
			name:            "auto with yes skips confirm",
			auto:            true,
			yes:             true,
			wantMode:        initModeAuto,
			wantSkipConfirm: true,
		},
		{
			name:            "auto alone keeps confirm",
			auto:            true,
			wantMode:        initModeAuto,
			wantSkipConfirm: false,
		},
		{
			name:            "yes alone implies auto and skips confirm",
			yes:             true,
			wantMode:        initModeAuto,
			wantSkipConfirm: true,
		},
		{
			// Non-interactive prompter triggers the non-TTY default branch.
			name:            "no flags in non-TTY defaults to auto+skipconfirm",
			wantMode:        initModeAuto,
			wantSkipConfirm: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			initAuto, initWalkthrough, initYes = c.auto, c.walkthrough, c.yes
			p := &fakePrompter{interactive: false}
			got := resolveInitMode(p)
			if got.Mode != c.wantMode {
				t.Errorf("Mode = %q, want %q", got.Mode, c.wantMode)
			}
			if got.SkipConfirm != c.wantSkipConfirm {
				t.Errorf("SkipConfirm = %v, want %v", got.SkipConfirm, c.wantSkipConfirm)
			}
			if len(p.chooseLog) != 0 {
				t.Errorf("non-TTY/flag path should not call ChooseIndex; got %d calls", len(p.chooseLog))
			}
		})
	}
}

// TestResolveInitMode_Interactive covers the prompt branch that was
// previously unreachable in tests because cli.IsInteractive() reads
// os.Stdin.Fd() directly. With the cli.Prompter seam, we can drive it.
func TestResolveInitMode_Interactive(t *testing.T) {
	// Save and restore flag globals so the cases land on the prompt branch.
	origAuto, origWalkthrough, origYes := initAuto, initWalkthrough, initYes
	t.Cleanup(func() {
		initAuto, initWalkthrough, initYes = origAuto, origWalkthrough, origYes
	})
	initAuto, initWalkthrough, initYes = false, false, false

	cases := []struct {
		name            string
		choose          chooseResult
		wantMode        string
		wantSkipConfirm bool
	}{
		{name: "user picks auto", choose: chooseResult{idx: 0}, wantMode: initModeAuto},
		{name: "user picks walkthrough", choose: chooseResult{idx: 1}, wantMode: initModeWalkthrough},
		{name: "user picks skip", choose: chooseResult{idx: 2}, wantMode: initModeSkip},
		{
			// Cancel / unknown index falls through the switch default to skip.
			name: "out-of-range index falls back to skip", choose: chooseResult{idx: 99}, wantMode: initModeSkip,
		},
		{
			// Errors (Ctrl+C, EOF) → safest path is skip.
			name: "ChooseIndex error returns skip", choose: chooseResult{err: errors.New("canceled")}, wantMode: initModeSkip,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &fakePrompter{
				interactive: true,
				chooseQueue: []chooseResult{c.choose},
			}
			got := resolveInitMode(p)
			if got.Mode != c.wantMode {
				t.Errorf("Mode = %q, want %q", got.Mode, c.wantMode)
			}
			if got.SkipConfirm != c.wantSkipConfirm {
				t.Errorf("SkipConfirm = %v, want %v", got.SkipConfirm, c.wantSkipConfirm)
			}
			if len(p.chooseLog) != 1 {
				t.Fatalf("expected exactly 1 ChooseIndex call, got %d", len(p.chooseLog))
			}
			// Sanity: the prompt copy still offers three options in stable order.
			if n := len(p.chooseLog[0].options); n != 3 {
				t.Errorf("expected 3 options in prompt, got %d", n)
			}
		})
	}
}

func TestFilterProfileForExternalDocker(t *testing.T) {
	cases := []struct {
		name              string
		profile           detect.ProjectProfile
		ext               config.ExternalComposeConfig
		wantSymlinks      []string
		wantCopy          []string
		wantCommands      []string
		wantContainerCmds []detect.ContainerCommand
	}{
		{
			// Regression: original case from #38 e7ea728.
			name: "strips container bundle install when vendor/bundle symlinked",
			profile: detect.ProjectProfile{
				HasDocker: true,
				ContainerCommands: []detect.ContainerCommand{
					{Service: "app", Command: "bundle install --quiet"},
					{Service: "app", Command: "rails db:setup"},
				},
				Commands: []string{"npm install"},
				Symlinks: []string{"vendor/bundle"},
			},
			ext:               config.ExternalComposeConfig{SymlinkDirs: []string{"vendor/bundle", "node_modules"}},
			wantSymlinks:      []string{},
			wantCopy:          nil,
			wantCommands:      []string{},
			wantContainerCmds: []detect.ContainerCommand{{Service: "app", Command: "rails db:setup"}},
		},
		{
			name: "empty ext is a no-op",
			profile: detect.ProjectProfile{
				Symlinks:          []string{"node_modules"},
				Copy:              []string{".env"},
				Commands:          []string{"npm install"},
				ContainerCommands: []detect.ContainerCommand{{Service: "app", Command: "bundle install"}},
			},
			ext:               config.ExternalComposeConfig{},
			wantSymlinks:      []string{"node_modules"},
			wantCopy:          []string{".env"},
			wantCommands:      []string{"npm install"},
			wantContainerCmds: []detect.ContainerCommand{{Service: "app", Command: "bundle install"}},
		},
		{
			// Same path appears in both CopyFiles and SymlinkFiles (e.g., user
			// migrated config). Filtering must dedupe transparently — the file
			// is removed once and there are no panics.
			name: "overlapping CopyFiles and SymlinkFiles both contribute to copy filter",
			profile: detect.ProjectProfile{
				Copy: []string{".env", "config/master.key"},
			},
			ext: config.ExternalComposeConfig{
				CopyFiles:    []string{".env"},
				SymlinkFiles: []string{".env"}, // overlap
			},
			wantCopy:     []string{"config/master.key"},
			wantSymlinks: nil,
		},
		{
			// Mirrors the bundle/npm coverage at shouldSkipCommand: pip+.venv.
			name: "pip install stripped when .venv symlinked",
			profile: detect.ProjectProfile{
				Commands:          []string{"pip install -r requirements.txt", "django-admin migrate"},
				ContainerCommands: []detect.ContainerCommand{{Service: "web", Command: "pip install -r requirements.txt"}},
			},
			ext:               config.ExternalComposeConfig{SymlinkDirs: []string{".venv"}},
			wantCommands:      []string{"django-admin migrate"},
			wantContainerCmds: []detect.ContainerCommand{},
			wantSymlinks:      nil,
			wantCopy:          nil,
		},
		{
			// Nil ContainerCommands must not panic; field stays nil/empty.
			name: "nil ContainerCommands is safe",
			profile: detect.ProjectProfile{
				Symlinks: []string{"node_modules"},
				Commands: []string{"npm install"},
				// ContainerCommands intentionally nil.
			},
			ext:               config.ExternalComposeConfig{SymlinkDirs: []string{"node_modules"}},
			wantSymlinks:      []string{},
			wantCommands:      []string{},
			wantContainerCmds: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := c.profile // copy so the table entry isn't mutated across runs
			filterProfileForExternalDocker(&p, &c.ext)

			if !reflect.DeepEqual(p.Symlinks, c.wantSymlinks) {
				t.Errorf("Symlinks = %#v, want %#v", p.Symlinks, c.wantSymlinks)
			}
			if !reflect.DeepEqual(p.Copy, c.wantCopy) {
				t.Errorf("Copy = %#v, want %#v", p.Copy, c.wantCopy)
			}
			if !reflect.DeepEqual(p.Commands, c.wantCommands) {
				t.Errorf("Commands = %#v, want %#v", p.Commands, c.wantCommands)
			}
			if !reflect.DeepEqual(p.ContainerCommands, c.wantContainerCmds) {
				t.Errorf("ContainerCommands = %#v, want %#v", p.ContainerCommands, c.wantContainerCmds)
			}
		})
	}
}

// TestWalkthroughProfile exercises the per-item routing loop. The body was
// previously unreachable in tests because cli.ChooseIndex / cli.Confirm
// read raw-TTY stdin. With the cli.Prompter seam, we can drive each
// branch deterministically.
func TestWalkthroughProfile(t *testing.T) {
	t.Run("host route keeps command on host", func(t *testing.T) {
		p := &fakePrompter{
			chooseQueue: []chooseResult{{idx: 0}}, // host
		}
		in := &detect.ProjectProfile{
			Commands:      []string{"bundle install"},
			HasDocker:     true,
			DockerService: "web",
		}
		got := walkthroughProfile(in, p)
		if !reflect.DeepEqual(got.Commands, []string{"bundle install"}) {
			t.Errorf("Commands = %#v, want [bundle install]", got.Commands)
		}
		if len(got.ContainerCommands) != 0 {
			t.Errorf("expected no ContainerCommands, got %#v", got.ContainerCommands)
		}
	})

	t.Run("container route moves command to ContainerCommands with declared service", func(t *testing.T) {
		p := &fakePrompter{
			chooseQueue: []chooseResult{{idx: 1}}, // container
		}
		in := &detect.ProjectProfile{
			Commands:      []string{"bundle install"},
			HasDocker:     true,
			DockerService: "web",
		}
		got := walkthroughProfile(in, p)
		if len(got.Commands) != 0 {
			t.Errorf("expected Commands empty after container routing, got %#v", got.Commands)
		}
		want := []detect.ContainerCommand{{Service: "web", Command: "bundle install"}}
		if !reflect.DeepEqual(got.ContainerCommands, want) {
			t.Errorf("ContainerCommands = %#v, want %#v", got.ContainerCommands, want)
		}
	})

	t.Run("container route falls back to service \"app\" when none declared", func(t *testing.T) {
		p := &fakePrompter{
			chooseQueue: []chooseResult{{idx: 1}}, // container
		}
		in := &detect.ProjectProfile{
			Commands:      []string{"npm install"},
			HasDocker:     true,
			DockerService: "", // unknown
		}
		got := walkthroughProfile(in, p)
		want := []detect.ContainerCommand{{Service: "app", Command: "npm install"}}
		if !reflect.DeepEqual(got.ContainerCommands, want) {
			t.Errorf("ContainerCommands = %#v, want %#v (service should default to \"app\")", got.ContainerCommands, want)
		}
	})

	t.Run("skip route drops command", func(t *testing.T) {
		// hasDocker=true → 3 options [host, container, skip] at index 2.
		p := &fakePrompter{
			chooseQueue: []chooseResult{{idx: 2}},
		}
		in := &detect.ProjectProfile{
			Commands:      []string{"bundle install"},
			HasDocker:     true,
			DockerService: "web",
		}
		got := walkthroughProfile(in, p)
		if len(got.Commands) != 0 || len(got.ContainerCommands) != 0 {
			t.Errorf("skip should drop command; got Commands=%#v Container=%#v", got.Commands, got.ContainerCommands)
		}
	})

	t.Run("ChooseIndex error defaults to skip", func(t *testing.T) {
		p := &fakePrompter{
			chooseQueue: []chooseResult{{err: errors.New("canceled")}},
		}
		in := &detect.ProjectProfile{
			Commands:      []string{"bundle install"},
			HasDocker:     true,
			DockerService: "web",
		}
		got := walkthroughProfile(in, p)
		if len(got.Commands) != 0 || len(got.ContainerCommands) != 0 {
			t.Errorf("error should drop command; got Commands=%#v Container=%#v", got.Commands, got.ContainerCommands)
		}
	})

	t.Run("container command demoted to host", func(t *testing.T) {
		p := &fakePrompter{
			chooseQueue: []chooseResult{{idx: 0}}, // host (index 0 of [host, container, skip])
		}
		in := &detect.ProjectProfile{
			ContainerCommands: []detect.ContainerCommand{{Service: "web", Command: "rails db:setup"}},
		}
		got := walkthroughProfile(in, p)
		if !reflect.DeepEqual(got.Commands, []string{"rails db:setup"}) {
			t.Errorf("expected demoted command on Commands, got %#v", got.Commands)
		}
		if len(got.ContainerCommands) != 0 {
			t.Errorf("expected ContainerCommands cleared after demotion, got %#v", got.ContainerCommands)
		}
	})

	t.Run("container command kept", func(t *testing.T) {
		p := &fakePrompter{
			chooseQueue: []chooseResult{{idx: 1}}, // container
		}
		in := &detect.ProjectProfile{
			ContainerCommands: []detect.ContainerCommand{{Service: "web", Command: "rails db:setup"}},
		}
		got := walkthroughProfile(in, p)
		want := []detect.ContainerCommand{{Service: "web", Command: "rails db:setup"}}
		if !reflect.DeepEqual(got.ContainerCommands, want) {
			t.Errorf("ContainerCommands = %#v, want %#v", got.ContainerCommands, want)
		}
		if len(got.Commands) != 0 {
			t.Errorf("expected Commands empty, got %#v", got.Commands)
		}
	})

	t.Run("copy and symlink prompts filter via Confirm", func(t *testing.T) {
		p := &fakePrompter{
			confirmQueue: []confirmResult{
				{ok: true},  // keep .env
				{ok: false}, // drop config/master.key
				{ok: true},  // keep node_modules
			},
		}
		in := &detect.ProjectProfile{
			Copy:     []string{".env", "config/master.key"},
			Symlinks: []string{"node_modules"},
		}
		got := walkthroughProfile(in, p)
		if !reflect.DeepEqual(got.Copy, []string{".env"}) {
			t.Errorf("Copy = %#v, want [.env]", got.Copy)
		}
		if !reflect.DeepEqual(got.Symlinks, []string{"node_modules"}) {
			t.Errorf("Symlinks = %#v, want [node_modules]", got.Symlinks)
		}
	})

	t.Run("empty profile is a no-op and never prompts", func(t *testing.T) {
		p := &fakePrompter{} // empty queues; any call panics
		in := &detect.ProjectProfile{}
		got := walkthroughProfile(in, p)
		if len(got.Commands) != 0 || len(got.ContainerCommands) != 0 || len(got.Copy) != 0 || len(got.Symlinks) != 0 {
			t.Errorf("empty profile should stay empty, got %#v", got)
		}
		if len(p.chooseLog) != 0 || len(p.confirmLog) != 0 {
			t.Errorf("expected no prompts, got chooseLog=%d confirmLog=%d", len(p.chooseLog), len(p.confirmLog))
		}
	})

	t.Run("input profile is not mutated", func(t *testing.T) {
		// Container route on a host command must not write through to the
		// input slices (regression check on the fresh-allocation logic).
		p := &fakePrompter{
			chooseQueue: []chooseResult{{idx: 1}}, // container
		}
		origCmds := []string{"bundle install"}
		origContainer := []detect.ContainerCommand{{Service: "web", Command: "rails db:setup"}}
		in := &detect.ProjectProfile{
			Commands:          append([]string{}, origCmds...),
			ContainerCommands: append([]detect.ContainerCommand{}, origContainer...),
			HasDocker:         true,
			DockerService:     "web",
		}
		// Need a second prompt for the second loop (ContainerCommands).
		p.chooseQueue = append(p.chooseQueue, chooseResult{idx: 1}) // keep container

		_ = walkthroughProfile(in, p)
		if !reflect.DeepEqual(in.Commands, origCmds) {
			t.Errorf("input Commands mutated: got %#v, want %#v", in.Commands, origCmds)
		}
		if !reflect.DeepEqual(in.ContainerCommands, origContainer) {
			t.Errorf("input ContainerCommands mutated: got %#v, want %#v", in.ContainerCommands, origContainer)
		}
	})
}

// Compile-time check that fakePrompter satisfies the cli.Prompter interface.
var _ cli.Prompter = (*fakePrompter)(nil)
