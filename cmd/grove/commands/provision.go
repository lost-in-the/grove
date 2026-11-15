package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/worktree"
)

// provisioning is what a create command learned from bootstrap about the
// parts of a new worktree that are not hooks: configured files and state.
// The zero value means "nothing configured, nothing failed".
type provisioning struct {
	files       []worktree.FileResult
	fileErr     *worktree.FileSetupError
	stateFailed *worktree.StateRegistrationError
}

// newProvisioning splits a Bootstrap error into its file and state parts.
// Other errors (a required hook) are the caller's to handle.
func newProvisioning(res worktree.BootstrapResult, err error) provisioning {
	p := provisioning{files: res.Files}
	var fe *worktree.FileSetupError
	if errors.As(err, &fe) {
		p.fileErr = fe
	}
	var se *worktree.StateRegistrationError
	if errors.As(err, &se) {
		p.stateFailed = se
	}
	return p
}

// complete is true when every configured entry was copied or linked and the
// worktree is registered in state.
func (p provisioning) complete() bool { return p.fileErr == nil && p.stateFailed == nil }

// jsonFields returns the "provisioned" and "file_setup" values for a create
// result object.
func (p provisioning) jsonFields() (*bool, *output.FileSetupResult) {
	fs := &output.FileSetupResult{
		Copied: []string{},
		Linked: []string{},
		Failed: []output.FileSetupFailure{},
	}
	for _, r := range p.files {
		switch r.Status {
		case worktree.FileCopied:
			fs.Copied = append(fs.Copied, r.Path)
		case worktree.FileLinked:
			fs.Linked = append(fs.Linked, r.Path)
		case worktree.FileFailed:
			fs.Failed = append(fs.Failed, output.FileSetupFailure{Field: string(r.Field), Path: r.Path, Error: r.Err.Error()})
		}
	}
	ok := p.complete()
	return &ok, fs
}

// report prints the failures to stderr, in every output mode (stdout carries
// the JSON). Prints nothing when provisioning is complete.
func (p provisioning) report(name, worktreePath string) {
	p.reportWith(name, worktreePath, "The worktree was created")
}

// reportWith is report with a custom opening for the remedy line.
func (p provisioning) reportWith(name, worktreePath, lead string) {
	w := cli.NewStderr()
	if p.fileErr != nil {
		for _, line := range p.fileErr.Lines(name, lead) {
			_, _ = fmt.Fprintln(w, line)
		}
	}
	if p.stateFailed != nil {
		_, _ = fmt.Fprintf(w, "Not registered: %v\n", p.stateFailed.Err)
		_, _ = fmt.Fprintf(w, "%s, but grove does not know it. Run `grove adopt %s`.\n", lead, worktreePath)
	}
}

// exitIfIncomplete ends the command with exitcode.CreatedNotProvisioned. Call
// it last, after the result has been printed: the worktree is kept.
func (p provisioning) exitIfIncomplete() {
	if !p.complete() {
		os.Exit(exitcode.CreatedNotProvisioned)
	}
}
