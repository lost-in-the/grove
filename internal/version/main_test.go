package version

import (
	"os"
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

func TestMain(m *testing.M) {
	os.Exit(testhelper.RunIsolated(m))
}
