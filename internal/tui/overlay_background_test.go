package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Each detailed row's folder sits in the left column. At 160 columns the
// delete overlay is centered well to its right, so every folder visible
// without the overlay must stay visible with it.
func TestOverlay_KeepsBackgroundRowsVisible(t *testing.T) {
	m := goldenModel(t, sizeUltraWide, withItems(10), withDetailed())

	before := ansi.Strip(m.renderDashboard())
	var visible []string
	for _, item := range makeTestItems(10) {
		if strings.Contains(before, "test-project-"+item.ShortName+"/") {
			visible = append(visible, item.ShortName)
		}
	}
	if len(visible) < 8 {
		t.Fatalf("only %d of 10 rows visible without an overlay; the test needs at least 8", len(visible))
	}

	withDeleteOverlay()(&m)
	after := ansi.Strip(m.viewString())
	for _, name := range visible {
		if !strings.Contains(after, "test-project-"+name+"/") {
			t.Errorf("row %q disappeared behind the delete overlay", name)
		}
	}
}
