package worktree

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

func writeMainFile(t *testing.T, mainPath, rel, content string) {
	t.Helper()
	full := filepath.Join(mainPath, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func resultFor(t *testing.T, results []FileResult, field FileField, path string) FileResult {
	t.Helper()
	for _, r := range results {
		if r.Field == field && r.Path == path {
			return r
		}
	}
	t.Fatalf("no result for %s %q in %+v", field, path, results)
	return FileResult{}
}

func TestSetupFiles_NilConfig(t *testing.T) {
	if got := SetupFiles(nil, "/tmp/new", "/tmp/main"); len(got) != 0 {
		t.Errorf("SetupFiles(nil, ...) = %+v, want no results", got)
	}
}

func TestSetupFiles_ReturnsOneResultPerEntry(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()
	writeMainFile(t, mainPath, "config/secret.key", "secret")
	writeMainFile(t, mainPath, "config/dev.key", "devkey")
	if err := os.MkdirAll(filepath.Join(mainPath, "vendor", "bundle"), 0o755); err != nil {
		t.Fatal(err)
	}

	ext := &config.ExternalComposeConfig{
		CopyFiles:    []string{"config/secret.key", "missing.key"},
		SymlinkFiles: []string{"config/dev.key", "missing-link.key"},
		SymlinkDirs:  []string{"vendor/bundle"},
	}
	results := SetupFiles(ext, newPath, mainPath)

	if len(results) != 5 {
		t.Fatalf("got %d results, want 5: %+v", len(results), results)
	}
	want := []struct {
		field  FileField
		path   string
		status FileStatus
	}{
		{FieldCopyFiles, "config/secret.key", FileCopied},
		{FieldCopyFiles, "missing.key", FileFailed},
		{FieldSymlinkFiles, "config/dev.key", FileLinked},
		{FieldSymlinkFiles, "missing-link.key", FileFailed},
		{FieldSymlinkDirs, "vendor/bundle", FileLinked},
	}
	for i, w := range want {
		r := results[i]
		if r.Field != w.field || r.Path != w.path || r.Status != w.status {
			t.Errorf("result[%d] = {%s %q %s}, want {%s %q %s}", i, r.Field, r.Path, r.Status, w.field, w.path, w.status)
		}
		if (r.Status == FileFailed) != (r.Err != nil) {
			t.Errorf("result[%d] status %s but Err = %v", i, r.Status, r.Err)
		}
	}
}

func TestSetupFiles_CopyFiles(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()
	writeMainFile(t, mainPath, "config/secret.key", "secret")

	ext := &config.ExternalComposeConfig{CopyFiles: []string{"config/secret.key"}}
	results := SetupFiles(ext, newPath, mainPath)
	if r := resultFor(t, results, FieldCopyFiles, "config/secret.key"); r.Status != FileCopied {
		t.Fatalf("status = %s (%v), want copied", r.Status, r.Err)
	}

	data, err := os.ReadFile(filepath.Join(newPath, "config", "secret.key"))
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(data) != "secret" {
		t.Errorf("Expected 'secret', got %q", string(data))
	}
	info, err := os.Lstat(filepath.Join(newPath, "config", "secret.key"))
	if err != nil {
		t.Fatalf("Lstat failed: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("Expected a regular file (copy), not a symlink")
	}
}

func TestSetupFiles_SymlinkFilesAndDirs(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()
	writeMainFile(t, mainPath, "config/dev.key", "devkey")
	if err := os.MkdirAll(filepath.Join(mainPath, "vendor", "bundle"), 0o755); err != nil {
		t.Fatal(err)
	}

	ext := &config.ExternalComposeConfig{
		SymlinkFiles: []string{"config/dev.key"},
		SymlinkDirs:  []string{"vendor/bundle"},
	}
	results := SetupFiles(ext, newPath, mainPath)
	for _, r := range results {
		if r.Status != FileLinked {
			t.Errorf("%s %q status = %s (%v), want linked", r.Field, r.Path, r.Status, r.Err)
		}
	}

	for _, rel := range []string{"config/dev.key", "vendor/bundle"} {
		info, err := os.Lstat(filepath.Join(newPath, rel))
		if err != nil {
			t.Fatalf("Lstat %s: %v", rel, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("Expected %s to be a symlink", rel)
		}
	}
	target, err := os.Readlink(filepath.Join(newPath, "config", "dev.key"))
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}
	if target != filepath.Join(mainPath, "config", "dev.key") {
		t.Errorf("Symlink target = %q, want %q", target, filepath.Join(mainPath, "config", "dev.key"))
	}
}

func TestSetupFiles_MissingSource(t *testing.T) {
	ext := &config.ExternalComposeConfig{CopyFiles: []string{"nonexistent.key"}}
	results := SetupFiles(ext, t.TempDir(), t.TempDir())

	r := resultFor(t, results, FieldCopyFiles, "nonexistent.key")
	if r.Status != FileFailed {
		t.Fatalf("status = %s, want failed", r.Status)
	}
	if !errors.Is(r.Err, ErrSourceMissing) {
		t.Errorf("Err = %v, want ErrSourceMissing", r.Err)
	}
}

func TestSetupFiles_MissingSourceContinues(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()
	writeMainFile(t, mainPath, "config/valid.key", "valid")

	ext := &config.ExternalComposeConfig{CopyFiles: []string{"nonexistent.key", "config/valid.key"}}
	results := SetupFiles(ext, newPath, mainPath)

	if r := resultFor(t, results, FieldCopyFiles, "nonexistent.key"); r.Status != FileFailed {
		t.Errorf("missing entry status = %s, want failed", r.Status)
	}
	if r := resultFor(t, results, FieldCopyFiles, "config/valid.key"); r.Status != FileCopied {
		t.Errorf("valid entry status = %s, want copied", r.Status)
	}
	data, err := os.ReadFile(filepath.Join(newPath, "config", "valid.key"))
	if err != nil || string(data) != "valid" {
		t.Fatalf("valid file not copied after an earlier failure: %v %q", err, data)
	}
}

func TestSetupFiles_SymlinkMissingSource(t *testing.T) {
	ext := &config.ExternalComposeConfig{SymlinkFiles: []string{"nonexistent.key"}}
	results := SetupFiles(ext, t.TempDir(), t.TempDir())

	r := resultFor(t, results, FieldSymlinkFiles, "nonexistent.key")
	if r.Status != FileFailed || !errors.Is(r.Err, ErrSourceMissing) {
		t.Errorf("got status %s err %v, want failed with ErrSourceMissing", r.Status, r.Err)
	}
}

func TestSetupFiles_DirectoryEntryLeavesNoFile(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(mainPath, "config"), 0o755); err != nil {
		t.Fatal(err)
	}

	ext := &config.ExternalComposeConfig{CopyFiles: []string{"config"}}
	results := SetupFiles(ext, newPath, mainPath)

	if r := resultFor(t, results, FieldCopyFiles, "config"); r.Status != FileFailed {
		t.Fatalf("status = %s, want failed", r.Status)
	}
	if _, err := os.Lstat(filepath.Join(newPath, "config")); !os.IsNotExist(err) {
		t.Errorf("an entry was left at the destination (stat err = %v)", err)
	}
}

func TestSetupFiles_PathTraversal(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()

	t.Run("legitimate symlink_files entry succeeds", func(t *testing.T) {
		writeMainFile(t, mainPath, "config.yml", "ok")
		ext := &config.ExternalComposeConfig{SymlinkFiles: []string{"config.yml"}}
		if r := resultFor(t, SetupFiles(ext, newPath, mainPath), FieldSymlinkFiles, "config.yml"); r.Status != FileLinked {
			t.Errorf("legitimate entry status = %s (%v), want linked", r.Status, r.Err)
		}
	})

	cases := []struct {
		name  string
		field FileField
		ext   *config.ExternalComposeConfig
		path  string
	}{
		{"dotdot in copy_files", FieldCopyFiles, &config.ExternalComposeConfig{CopyFiles: []string{"../../etc/passwd"}}, "../../etc/passwd"},
		{"dotdot in symlink_files", FieldSymlinkFiles, &config.ExternalComposeConfig{SymlinkFiles: []string{"../../.ssh/id_rsa"}}, "../../.ssh/id_rsa"},
		{"dotdot in symlink_dirs", FieldSymlinkDirs, &config.ExternalComposeConfig{SymlinkDirs: []string{"../../../secret_dir"}}, "../../../secret_dir"},
	}
	for _, tc := range cases {
		t.Run(tc.name+" rejected", func(t *testing.T) {
			r := resultFor(t, SetupFiles(tc.ext, newPath, mainPath), tc.field, tc.path)
			if r.Status != FileFailed {
				t.Fatalf("status = %s, want failed", r.Status)
			}
			if !strings.Contains(r.Err.Error(), "escapes base directory") {
				t.Errorf("expected 'escapes base directory' in error, got %q", r.Err)
			}
		})
	}
}

func TestFileSetupError(t *testing.T) {
	ok := FileResult{Field: FieldCopyFiles, Path: "a", Status: FileCopied}
	if err := NewFileSetupError([]FileResult{ok}); err != nil {
		t.Fatalf("NewFileSetupError with no failures = %v, want nil", err)
	}

	failed := []FileResult{
		{Field: FieldCopyFiles, Path: "config/secret.key", Status: FileFailed, Err: ErrSourceMissing},
		{Field: FieldSymlinkDirs, Path: "vendor", Status: FileFailed, Err: ErrSourceMissing},
	}
	err := NewFileSetupError(append([]FileResult{ok}, failed...))
	if !errors.Is(err, ErrFileSetupIncomplete) {
		t.Fatalf("errors.Is(err, ErrFileSetupIncomplete) = false for %v", err)
	}
	var fe *FileSetupError
	if !errors.As(err, &fe) || len(fe.Failed) != 2 {
		t.Fatalf("errors.As: got %+v, want 2 failed entries", fe)
	}

	lines := fe.Lines("feature", "The worktree was created")
	want := []string{
		"Not copied: config/secret.key (missing in the main worktree)",
		"Not linked: vendor (missing in the main worktree)",
		"The worktree was created, but 2 files are missing. Fix the source, then run `grove doctor feature --fix`.",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("Lines = %q, want %q", lines, want)
	}
}
