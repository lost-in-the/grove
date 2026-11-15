package commands

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/lost-in-the/grove/internal/worktree"
)

func TestProvisioning_JSONFields(t *testing.T) {
	files := []worktree.FileResult{
		{Field: worktree.FieldCopyFiles, Path: "a.key", Status: worktree.FileCopied},
		{Field: worktree.FieldSymlinkDirs, Path: "vendor", Status: worktree.FileLinked},
		{Field: worktree.FieldCopyFiles, Path: "b.key", Status: worktree.FileFailed, Err: worktree.ErrSourceMissing},
	}
	p := newProvisioning(worktree.BootstrapResult{Files: files}, worktree.NewFileSetupError(files))

	ok, fs := p.jsonFields()
	if *ok {
		t.Error("provisioned = true with a failed entry")
	}
	data, err := json.Marshal(fs)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"copied":["a.key"],"linked":["vendor"],"failed":[{"field":"copy_files","path":"b.key","error":"source missing in the main worktree"}]}`
	if string(data) != want {
		t.Errorf("file_setup = %s\nwant        %s", data, want)
	}
}

func TestProvisioning_NothingConfiguredIsProvisionedWithEmptyLists(t *testing.T) {
	p := newProvisioning(worktree.BootstrapResult{}, nil)
	ok, fs := p.jsonFields()
	if !*ok {
		t.Error("provisioned = false with nothing configured")
	}
	data, _ := json.Marshal(fs)
	if string(data) != `{"copied":[],"linked":[],"failed":[]}` {
		t.Errorf("file_setup = %s, want empty lists (not null)", data)
	}
}

func TestProvisioning_StateFailureIsIncomplete(t *testing.T) {
	err := errors.Join(&worktree.StateRegistrationError{Err: errors.New("disk full")})
	p := newProvisioning(worktree.BootstrapResult{}, err)
	if p.complete() {
		t.Error("complete() = true after a state registration failure")
	}
	if ok, _ := p.jsonFields(); *ok {
		t.Error("provisioned = true after a state registration failure")
	}
}

func TestProvisioning_IgnoresHookErrors(t *testing.T) {
	err := errors.Join(worktree.ErrRequiredHookFailed)
	if !newProvisioning(worktree.BootstrapResult{}, err).complete() {
		t.Error("a hook failure alone must not read as incomplete provisioning")
	}
}
