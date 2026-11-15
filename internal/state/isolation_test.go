package state

import (
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

func TestHomeIsIsolated(t *testing.T) {
	testhelper.AssertHomeIsolated(t)
}
