package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVariablesInterpolate(t *testing.T) {
	v := &Variables{
		Worktree:     "testing",
		WorktreeFull: "myproject-testing",
		Branch:       "feature/testing",
		Project:      "myproject",
		MainPath:     "/work/myproject",
		NewPath:      "/work/myproject-testing",
		PrevPath:     "/work/myproject-main",
		Port:         3000,
		User:         "alice",
		Timestamp:    1700000000,
		Date:         "2026-02-26",
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single var", "{{.worktree}}", "testing"},
		{"multiple vars", "{{.project}}/{{.worktree}}", "myproject/testing"},
		{"no vars passthrough", "hello world", "hello world"},
		{"unknown var left as-is", "{{.unknown}}", "{{.unknown}}"},
		{"empty string", "", ""},
		{"worktree", "wt={{.worktree}}", "wt=testing"},
		{"worktree_full", "full={{.worktree_full}}", "full=myproject-testing"},
		{"branch", "br={{.branch}}", "br=feature/testing"},
		{"project", "proj={{.project}}", "proj=myproject"},
		{"main_path", "main={{.main_path}}", "main=/work/myproject"},
		{"new_path", "new={{.new_path}}", "new=/work/myproject-testing"},
		{"prev_path", "prev={{.prev_path}}", "prev=/work/myproject-main"},
		{"port", "port={{.port}}", "port=3000"},
		{"user", "user={{.user}}", "user=alice"},
		{"timestamp", "ts={{.timestamp}}", "ts=1700000000"},
		{"date", "date={{.date}}", "date=2026-02-26"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.Interpolate(tt.input)
			if got != tt.want {
				t.Errorf("Interpolate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestReplaceAll(t *testing.T) {
	tests := []struct {
		name string
		s    string
		old  string
		new  string
		want string
	}{
		{"basic replacement", "hello world", "world", "go", "hello go"},
		{"empty old string no-op", "hello", "", "x", "hello"},
		{"multiple occurrences", "aaa", "a", "b", "bbb"},
		{"no match", "hello", "xyz", "abc", "hello"},
		{"replace entire string", "foo", "foo", "bar", "bar"},
		{"empty string", "", "x", "y", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceAll(tt.s, tt.old, tt.new)
			if got != tt.want {
				t.Errorf("replaceAll(%q, %q, %q) = %q, want %q", tt.s, tt.old, tt.new, got, tt.want)
			}
		})
	}
}

func TestIndexOf(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   int
	}{
		{"found at start", "hello world", "hello", 0},
		{"found in middle", "hello world", "world", 6},
		{"not found", "hello", "xyz", -1},
		{"empty string", "", "x", -1},
		{"substr longer than s", "hi", "hello", -1},
		{"exact match", "exact", "exact", 0},
		{"overlapping", "aaa", "aa", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := indexOf(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("indexOf(%q, %q) = %d, want %d", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestExecute(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		e := NewExecutorWithConfig(nil)
		var buf bytes.Buffer
		e.Output = &buf
		err := e.Execute(EventPostCreate, &ExecutionContext{})
		if err != nil {
			t.Errorf("Execute() with nil config = %v, want nil", err)
		}
	})

	t.Run("no actions returns nil", func(t *testing.T) {
		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		err := e.Execute(EventPostCreate, &ExecutionContext{})
		if err != nil {
			t.Errorf("Execute() with no actions = %v, want nil", err)
		}
	})

	t.Run("required action failure returns error", func(t *testing.T) {
		cfg := &HooksConfig{}
		cfg.Hooks.PostCreate = []HookAction{
			{Type: "unknown_type", Required: true, OnFailure: "warn", Timeout: 60, WorkingDir: "new"},
		}
		e := NewExecutorWithConfig(cfg)
		var buf bytes.Buffer
		e.Output = &buf
		err := e.Execute(EventPostCreate, &ExecutionContext{})
		if err == nil {
			t.Error("Execute() with required failing action = nil, want error")
		}
	})

	t.Run("OnFailure warn logs but returns nil", func(t *testing.T) {
		cfg := &HooksConfig{}
		cfg.Hooks.PostCreate = []HookAction{
			{Type: "unknown_type", OnFailure: "warn", Timeout: 60, WorkingDir: "new"},
		}
		e := NewExecutorWithConfig(cfg)
		var buf bytes.Buffer
		e.Output = &buf
		err := e.Execute(EventPostCreate, &ExecutionContext{})
		if err != nil {
			t.Errorf("Execute() with warn = %v, want nil", err)
		}
		if buf.Len() == 0 {
			t.Error("Execute() with warn should write warning to output")
		}
	})

	t.Run("OnFailure ignore silent", func(t *testing.T) {
		cfg := &HooksConfig{}
		cfg.Hooks.PostCreate = []HookAction{
			{Type: "unknown_type", OnFailure: "ignore", Timeout: 60, WorkingDir: "new"},
		}
		e := NewExecutorWithConfig(cfg)
		var buf bytes.Buffer
		e.Output = &buf
		err := e.Execute(EventPostCreate, &ExecutionContext{})
		if err != nil {
			t.Errorf("Execute() with ignore = %v, want nil", err)
		}
		if buf.Len() != 0 {
			t.Errorf("Execute() with ignore wrote output: %q", buf.String())
		}
	})

	t.Run("OnFailure fail returns error", func(t *testing.T) {
		cfg := &HooksConfig{}
		cfg.Hooks.PostCreate = []HookAction{
			{Type: "unknown_type", OnFailure: "fail", Timeout: 60, WorkingDir: "new"},
		}
		e := NewExecutorWithConfig(cfg)
		var buf bytes.Buffer
		e.Output = &buf
		err := e.Execute(EventPostCreate, &ExecutionContext{})
		if err == nil {
			t.Error("Execute() with OnFailure=fail = nil, want error")
		}
	})
}

func TestExecuteCopy(t *testing.T) {
	t.Run("file copy", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()
		srcContent := "hello from copy"
		if err := os.WriteFile(filepath.Join(mainDir, "source.txt"), []byte(srcContent), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		action := &HookAction{From: "source.txt", To: "dest.txt"}
		ctx := &ExecutionContext{MainPath: mainDir, NewPath: newDir}
		vars := &Variables{}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		if err := e.executeCopy(action, ctx, vars); err != nil {
			t.Fatalf("executeCopy() error = %v", err)
		}

		got, err := os.ReadFile(filepath.Join(newDir, "dest.txt"))
		if err != nil {
			t.Fatalf("ReadFile dest: %v", err)
		}
		if string(got) != srcContent {
			t.Errorf("dest content = %q, want %q", got, srcContent)
		}
	})

	t.Run("source missing returns error", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()
		action := &HookAction{From: "nonexistent.txt", To: "dest.txt"}
		ctx := &ExecutionContext{MainPath: mainDir, NewPath: newDir}
		vars := &Variables{}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		err := e.executeCopy(action, ctx, vars)
		if err == nil {
			t.Error("executeCopy() with missing source = nil, want error")
		}
	})

	t.Run("variable interpolation in paths", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()
		srcContent := "interpolated content"
		if err := os.WriteFile(filepath.Join(mainDir, "testing.txt"), []byte(srcContent), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		action := &HookAction{From: "{{.worktree}}.txt", To: "{{.worktree}}.txt"}
		ctx := &ExecutionContext{MainPath: mainDir, NewPath: newDir}
		vars := &Variables{Worktree: "testing"}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		if err := e.executeCopy(action, ctx, vars); err != nil {
			t.Fatalf("executeCopy() error = %v", err)
		}

		got, err := os.ReadFile(filepath.Join(newDir, "testing.txt"))
		if err != nil {
			t.Fatalf("ReadFile dest: %v", err)
		}
		if string(got) != srcContent {
			t.Errorf("dest content = %q, want %q", got, srcContent)
		}
	})
}

func TestExecuteSymlink(t *testing.T) {
	t.Run("creates symlink", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(mainDir, "source.txt"), []byte("source"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		action := &HookAction{From: "source.txt", To: "link.txt"}
		ctx := &ExecutionContext{MainPath: mainDir, NewPath: newDir}
		vars := &Variables{}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		if err := e.executeSymlink(action, ctx, vars); err != nil {
			t.Fatalf("executeSymlink() error = %v", err)
		}

		linkPath := filepath.Join(newDir, "link.txt")
		info, err := os.Lstat(linkPath)
		if err != nil {
			t.Fatalf("Lstat link: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Error("expected symlink, got regular file")
		}
	})

	t.Run("replaces existing file", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(mainDir, "source.txt"), []byte("source"), 0644); err != nil {
			t.Fatalf("WriteFile source: %v", err)
		}
		linkPath := filepath.Join(newDir, "link.txt")
		if err := os.WriteFile(linkPath, []byte("existing"), 0644); err != nil {
			t.Fatalf("WriteFile existing: %v", err)
		}

		action := &HookAction{From: "source.txt", To: "link.txt"}
		ctx := &ExecutionContext{MainPath: mainDir, NewPath: newDir}
		vars := &Variables{}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		if err := e.executeSymlink(action, ctx, vars); err != nil {
			t.Fatalf("executeSymlink() error = %v", err)
		}

		info, err := os.Lstat(linkPath)
		if err != nil {
			t.Fatalf("Lstat link: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Error("expected symlink after replacement, got regular file")
		}
	})

	t.Run("source missing returns error", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()
		action := &HookAction{From: "nonexistent.txt", To: "link.txt"}
		ctx := &ExecutionContext{MainPath: mainDir, NewPath: newDir}
		vars := &Variables{}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		err := e.executeSymlink(action, ctx, vars)
		if err == nil {
			t.Error("executeSymlink() with missing source = nil, want error")
		}
	})
}

func TestExecuteCommand(t *testing.T) {
	t.Run("simple command succeeds", func(t *testing.T) {
		dir := t.TempDir()
		action := &HookAction{Command: "echo hello", WorkingDir: "new", Timeout: 10}
		ctx := &ExecutionContext{NewPath: dir}
		vars := &Variables{}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		if err := e.executeCommand(action, ctx, vars); err != nil {
			t.Fatalf("executeCommand() error = %v", err)
		}
	})

	t.Run("WorkingDir main uses main path", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(mainDir, "main_marker.txt"), []byte("main"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		action := &HookAction{Command: "test -f main_marker.txt", WorkingDir: "main", Timeout: 10}
		ctx := &ExecutionContext{MainPath: mainDir, NewPath: newDir}
		vars := &Variables{}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		if err := e.executeCommand(action, ctx, vars); err != nil {
			t.Errorf("executeCommand() with main working dir = %v, want nil", err)
		}
	})

	t.Run("WorkingDir new uses new path", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(newDir, "new_marker.txt"), []byte("new"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		action := &HookAction{Command: "test -f new_marker.txt", WorkingDir: "new", Timeout: 10}
		ctx := &ExecutionContext{MainPath: mainDir, NewPath: newDir}
		vars := &Variables{}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		if err := e.executeCommand(action, ctx, vars); err != nil {
			t.Errorf("executeCommand() with new working dir = %v, want nil", err)
		}
	})

	t.Run("command failure returns error", func(t *testing.T) {
		dir := t.TempDir()
		action := &HookAction{Command: "exit 1", WorkingDir: "new", Timeout: 10}
		ctx := &ExecutionContext{NewPath: dir}
		vars := &Variables{}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		err := e.executeCommand(action, ctx, vars)
		if err == nil {
			t.Error("executeCommand() with failing command = nil, want error")
		}
	})
}

func TestExecuteTemplate(t *testing.T) {
	t.Run("reads template replaces vars writes output", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()
		tmplContent := "Project: {{.project}}, Worktree: {{.worktree}}"
		if err := os.WriteFile(filepath.Join(mainDir, "template.txt"), []byte(tmplContent), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		action := &HookAction{From: "template.txt", To: "output.txt"}
		ctx := &ExecutionContext{MainPath: mainDir, NewPath: newDir}
		vars := &Variables{Project: "myproject", Worktree: "testing"}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		if err := e.executeTemplate(action, ctx, vars); err != nil {
			t.Fatalf("executeTemplate() error = %v", err)
		}

		got, err := os.ReadFile(filepath.Join(newDir, "output.txt"))
		if err != nil {
			t.Fatalf("ReadFile output: %v", err)
		}
		want := "Project: myproject, Worktree: testing"
		if string(got) != want {
			t.Errorf("template output = %q, want %q", got, want)
		}
	})

	t.Run("action-specific vars applied", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(mainDir, "template.txt"), []byte("Custom: {{.custom_var}}"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		action := &HookAction{
			From: "template.txt",
			To:   "output.txt",
			Vars: map[string]string{"custom_var": "custom_value"},
		}
		ctx := &ExecutionContext{MainPath: mainDir, NewPath: newDir}
		vars := &Variables{}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		if err := e.executeTemplate(action, ctx, vars); err != nil {
			t.Fatalf("executeTemplate() error = %v", err)
		}

		got, err := os.ReadFile(filepath.Join(newDir, "output.txt"))
		if err != nil {
			t.Fatalf("ReadFile output: %v", err)
		}
		if string(got) != "Custom: custom_value" {
			t.Errorf("template output = %q, want %q", got, "Custom: custom_value")
		}
	})

	t.Run("source missing returns error", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()
		action := &HookAction{From: "nonexistent.txt", To: "output.txt"}
		ctx := &ExecutionContext{MainPath: mainDir, NewPath: newDir}
		vars := &Variables{}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		err := e.executeTemplate(action, ctx, vars)
		if err == nil {
			t.Error("executeTemplate() with missing source = nil, want error")
		}
	})

	t.Run("dest dir created if not exists", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(mainDir, "template.txt"), []byte("hello"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		action := &HookAction{From: "template.txt", To: "subdir/output.txt"}
		ctx := &ExecutionContext{MainPath: mainDir, NewPath: newDir}
		vars := &Variables{}

		e := NewExecutorWithConfig(&HooksConfig{})
		var buf bytes.Buffer
		e.Output = &buf
		if err := e.executeTemplate(action, ctx, vars); err != nil {
			t.Fatalf("executeTemplate() error = %v", err)
		}

		dstFile := filepath.Join(newDir, "subdir", "output.txt")
		if _, err := os.Stat(dstFile); err != nil {
			t.Errorf("expected output file at %s: %v", dstFile, err)
		}
	})
}

func TestBuildVariables(t *testing.T) {
	e := NewExecutorWithConfig(&HooksConfig{})
	ctx := &ExecutionContext{
		Event:        EventPostCreate,
		Worktree:     "testing",
		WorktreeFull: "myproject-testing",
		Branch:       "feature/testing",
		Project:      "myproject",
		MainPath:     "/work/myproject",
		NewPath:      "/work/myproject-testing",
		PrevPath:     "/work/myproject-main",
		Port:         8080,
	}

	vars := e.buildVariables(ctx)

	if vars.Worktree != ctx.Worktree {
		t.Errorf("Worktree = %q, want %q", vars.Worktree, ctx.Worktree)
	}
	if vars.WorktreeFull != ctx.WorktreeFull {
		t.Errorf("WorktreeFull = %q, want %q", vars.WorktreeFull, ctx.WorktreeFull)
	}
	if vars.Branch != ctx.Branch {
		t.Errorf("Branch = %q, want %q", vars.Branch, ctx.Branch)
	}
	if vars.Project != ctx.Project {
		t.Errorf("Project = %q, want %q", vars.Project, ctx.Project)
	}
	if vars.MainPath != ctx.MainPath {
		t.Errorf("MainPath = %q, want %q", vars.MainPath, ctx.MainPath)
	}
	if vars.NewPath != ctx.NewPath {
		t.Errorf("NewPath = %q, want %q", vars.NewPath, ctx.NewPath)
	}
	if vars.PrevPath != ctx.PrevPath {
		t.Errorf("PrevPath = %q, want %q", vars.PrevPath, ctx.PrevPath)
	}
	if vars.Port != ctx.Port {
		t.Errorf("Port = %d, want %d", vars.Port, ctx.Port)
	}
	if vars.User == "" {
		t.Error("User should be non-empty")
	}
	if vars.Timestamp == 0 {
		t.Error("Timestamp should be non-zero")
	}
	if vars.Date == "" {
		t.Error("Date should be non-empty")
	}
}

func TestDirOf(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"normal path", "/foo/bar/baz.txt", "/foo/bar"},
		{"single level", "/foo/bar", "/foo"},
		{"no slash returns empty", "bar", ""},
		{"root-level file returns empty", "/foo", ""},
		{"root slash returns empty", "/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dirOf(tt.path)
			if got != tt.want {
				t.Errorf("dirOf(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		basePath string
		want     string
	}{
		{"absolute path returned as-is", "/absolute/path", "/base", "/absolute/path"},
		{"relative joined with base", "relative/file", "/base", "/base/relative/file"},
		{"empty returns base", "", "/base", "/base"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePath(tt.path, tt.basePath)
			if got != tt.want {
				t.Errorf("resolvePath(%q, %q) = %q, want %q", tt.path, tt.basePath, got, tt.want)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	t.Run("copies content", func(t *testing.T) {
		dir := t.TempDir()
		srcContent := "file content to copy"
		src := filepath.Join(dir, "src.txt")
		if err := os.WriteFile(src, []byte(srcContent), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		dst := filepath.Join(dir, "dst.txt")
		if err := copyFile(src, dst); err != nil {
			t.Fatalf("copyFile() error = %v", err)
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(got) != srcContent {
			t.Errorf("copied content = %q, want %q", got, srcContent)
		}
	})

	t.Run("creates dest dir", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		if err := os.WriteFile(src, []byte("content"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		dst := filepath.Join(dir, "subdir", "nested", "dst.txt")
		if err := copyFile(src, dst); err != nil {
			t.Fatalf("copyFile() error = %v", err)
		}

		if _, err := os.Stat(dst); err != nil {
			t.Errorf("expected dst to exist: %v", err)
		}
	})
}

func TestCopyDir(t *testing.T) {
	t.Run("recursive copy", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := filepath.Join(t.TempDir(), "dst")

		if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		files := map[string]string{
			"file1.txt":     "content 1",
			"sub/file2.txt": "content 2",
		}
		for name, content := range files {
			path := filepath.Join(srcDir, filepath.FromSlash(name))
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatalf("WriteFile %s: %v", name, err)
			}
		}

		if err := copyDir(srcDir, dstDir); err != nil {
			t.Fatalf("copyDir() error = %v", err)
		}

		for name, want := range files {
			got, err := os.ReadFile(filepath.Join(dstDir, filepath.FromSlash(name)))
			if err != nil {
				t.Errorf("ReadFile %s: %v", name, err)
				continue
			}
			if string(got) != want {
				t.Errorf("file %s content = %q, want %q", name, got, want)
			}
		}
	})
}

func TestRunCommand(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		var stdout, stderr bytes.Buffer
		err := runCommand("echo hello", dir, 10*time.Second, &stdout, &stderr)
		if err != nil {
			t.Fatalf("runCommand() error = %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		dir := t.TempDir()
		var stdout, stderr bytes.Buffer
		err := runCommand("exit 1", dir, 10*time.Second, &stdout, &stderr)
		if err == nil {
			t.Error("runCommand() with failing command = nil, want error")
		}
	})

	t.Run("working directory respected", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("marker"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		var stdout, stderr bytes.Buffer
		err := runCommand("test -f marker.txt", dir, 10*time.Second, &stdout, &stderr)
		if err != nil {
			t.Errorf("runCommand() working dir test = %v, want nil", err)
		}
	})

	t.Run("output captured in writers", func(t *testing.T) {
		dir := t.TempDir()
		var stdout, stderr bytes.Buffer
		err := runCommand("echo captured_output", dir, 10*time.Second, &stdout, &stderr)
		if err != nil {
			t.Fatalf("runCommand() error = %v", err)
		}
		if stdout.Len() == 0 {
			t.Error("expected captured stdout output, got empty")
		}
	})
}
