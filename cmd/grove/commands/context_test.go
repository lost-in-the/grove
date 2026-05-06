package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/grove"
)

func TestEmitDriftNotice_PrintsAdoptHint(t *testing.T) {
	var buf bytes.Buffer
	w := cli.NewWriter(&buf, false)

	emitDriftNotice(w, "drifted-wt", grove.ReasonDriftedWorktree)

	out := buf.String()
	if !strings.Contains(out, "grove adopt") {
		t.Errorf("expected 'grove adopt' hint, got: %s", out)
	}
	if !strings.Contains(out, "drifted-wt") {
		t.Errorf("expected worktree name in notice, got: %s", out)
	}
}

func TestEmitDriftNotice_SilentWhenRegistered(t *testing.T) {
	var buf bytes.Buffer
	w := cli.NewWriter(&buf, false)

	emitDriftNotice(w, "ok-wt", grove.ReasonRegistered)

	if buf.Len() != 0 {
		t.Errorf("expected no output for registered worktree, got: %s", buf.String())
	}
}
