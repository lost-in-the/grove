package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
)

func TestDoctorCmd_AllFlag(t *testing.T) {
	flag := doctorCmd.Flags().Lookup("all")
	if flag == nil {
		t.Fatal("doctorCmd missing --all flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--all should default to false, got %q", flag.DefValue)
	}
}

func TestDoctorCmd_AcceptsWorktreeArg(t *testing.T) {
	// Args = cobra.MaximumNArgs(1)
	if doctorCmd.Args == nil {
		t.Fatal("doctorCmd.Args is nil; should accept up to one positional arg")
	}
}

// TestClassifyEntry_CopyFiles asserts copy_files entries are classified as
// missing when absent, override when a symlink replaces the regular file,
// and ok when a regular file is present.
func TestClassifyEntry_CopyFiles(t *testing.T) {
	projectRoot := t.TempDir()
	worktree := t.TempDir()

	// project root has the source files (needed for Sources path resolution).
	mustWriteFile(t, filepath.Join(projectRoot, "config.local"), "tracked")

	tests := []struct {
		name       string
		setupWT    func(t *testing.T, wt string)
		wantStatus provisioningStatus
	}{
		{
			name:       "missing",
			setupWT:    func(t *testing.T, wt string) {},
			wantStatus: provisioningMissing,
		},
		{
			name: "ok regular file",
			setupWT: func(t *testing.T, wt string) {
				mustWriteFile(t, filepath.Join(wt, "config.local"), "local")
			},
			wantStatus: provisioningOK,
		},
		{
			name: "override symlink",
			setupWT: func(t *testing.T, wt string) {
				if err := os.Symlink(filepath.Join(projectRoot, "config.local"), filepath.Join(wt, "config.local")); err != nil {
					t.Fatal(err)
				}
			},
			wantStatus: provisioningOverride,
		},
		{
			name: "override directory",
			setupWT: func(t *testing.T, wt string) {
				if err := os.Mkdir(filepath.Join(wt, "config.local"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			wantStatus: provisioningOverride,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wt := filepath.Join(worktree, tt.name)
			if err := os.MkdirAll(wt, 0o755); err != nil {
				t.Fatal(err)
			}
			tt.setupWT(t, wt)
			got := classifyEntry("copy_files", projectRoot, wt, "config.local")
			if got.Status != tt.wantStatus {
				t.Errorf("status = %s, want %s (detail: %s)", got.Status, tt.wantStatus, got.Detail)
			}
		})
	}
}

func TestClassifyEntry_SymlinkFiles(t *testing.T) {
	projectRoot := t.TempDir()
	worktree := t.TempDir()
	mustWriteFile(t, filepath.Join(projectRoot, "shared.key"), "secret")

	tests := []struct {
		name       string
		setupWT    func(t *testing.T, wt string)
		wantStatus provisioningStatus
	}{
		{"missing", func(t *testing.T, wt string) {}, provisioningMissing},
		{
			"ok symlink",
			func(t *testing.T, wt string) {
				if err := os.Symlink(filepath.Join(projectRoot, "shared.key"), filepath.Join(wt, "shared.key")); err != nil {
					t.Fatal(err)
				}
			},
			provisioningOK,
		},
		{
			"override regular file",
			func(t *testing.T, wt string) {
				mustWriteFile(t, filepath.Join(wt, "shared.key"), "different")
			},
			provisioningOverride,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wt := filepath.Join(worktree, tt.name)
			if err := os.MkdirAll(wt, 0o755); err != nil {
				t.Fatal(err)
			}
			tt.setupWT(t, wt)
			got := classifyEntry("symlink_files", projectRoot, wt, "shared.key")
			if got.Status != tt.wantStatus {
				t.Errorf("status = %s, want %s (detail: %s)", got.Status, tt.wantStatus, got.Detail)
			}
		})
	}
}

// TestRepairMissing_OnlyTouchesMissing locks the contract that --fix never
// modifies override entries — only restores missing ones. This is the
// invariant that protects user customizations from being silently reverted.
func TestRepairMissing_OnlyTouchesMissing(t *testing.T) {
	projectRoot := t.TempDir()
	worktree := t.TempDir()

	mustWriteFile(t, filepath.Join(projectRoot, "absent.txt"), "from-main")
	mustWriteFile(t, filepath.Join(projectRoot, "kept.txt"), "from-main")

	// worktree has a user-customized "kept.txt" that should NOT be touched.
	customContent := "user-customized in this worktree"
	mustWriteFile(t, filepath.Join(worktree, "kept.txt"), customContent)
	// override case: copy_files entry replaced by a symlink — should stay.
	if err := os.Symlink(filepath.Join(projectRoot, "kept.txt"), filepath.Join(worktree, "kept-as-symlink.txt")); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(projectRoot, "kept-as-symlink.txt"), "from-main")

	results := []provisioningResult{
		{Field: "copy_files", Path: "absent.txt", Status: provisioningMissing, Sources: filepath.Join(projectRoot, "absent.txt")},
		{Field: "copy_files", Path: "kept.txt", Status: provisioningOverride, Sources: filepath.Join(projectRoot, "kept.txt"), Detail: "expected regular file, found symlink"},
		{Field: "copy_files", Path: "kept-as-symlink.txt", Status: provisioningOverride, Sources: filepath.Join(projectRoot, "kept-as-symlink.txt"), Detail: "expected regular file, found symlink"},
	}

	fixed, err := repairMissing(cli.NewStderr(), results, worktree)
	if err != nil {
		t.Fatalf("repairMissing: %v", err)
	}
	if fixed != 1 {
		t.Errorf("fixed = %d, want 1 (only the missing entry)", fixed)
	}

	// The missing entry was restored.
	got, err := os.ReadFile(filepath.Join(worktree, "absent.txt"))
	if err != nil {
		t.Fatalf("read restored: %v", err)
	}
	if string(got) != "from-main" {
		t.Errorf("restored content = %q, want %q", string(got), "from-main")
	}

	// The user's customized "kept.txt" was NOT overwritten.
	got, err = os.ReadFile(filepath.Join(worktree, "kept.txt"))
	if err != nil {
		t.Fatalf("read kept: %v", err)
	}
	if string(got) != customContent {
		t.Errorf("kept.txt was modified: got %q, want %q (override entries must be left alone)", string(got), customContent)
	}

	// The override symlink was NOT touched.
	info, err := os.Lstat(filepath.Join(worktree, "kept-as-symlink.txt"))
	if err != nil {
		t.Fatalf("lstat kept-as-symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("kept-as-symlink.txt was replaced — should have remained a symlink (override status)")
	}
}

func TestAuditWorktreeProvisioning_EmptyConfigReturnsNil(t *testing.T) {
	if got := auditWorktreeProvisioning(nil, "/x", "/y"); got != nil {
		t.Errorf("nil ext should return nil results, got %v", got)
	}
	if got := auditWorktreeProvisioning(&config.ExternalComposeConfig{}, "/x", "/y"); len(got) != 0 {
		t.Errorf("empty ext should return 0 results, got %d", len(got))
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
