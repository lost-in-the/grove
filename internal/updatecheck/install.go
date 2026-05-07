package updatecheck

import (
	"os"
	"path/filepath"
	"strings"
)

// InstallMethod identifies how the running grove binary was installed.
type InstallMethod int

const (
	InstallUnknown InstallMethod = iota
	InstallBrew
	InstallGoInstall
	InstallBinary
)

// String renders the method as a human-readable name (used in test output and logs).
func (m InstallMethod) String() string {
	switch m {
	case InstallBrew:
		return "brew"
	case InstallGoInstall:
		return "go-install"
	case InstallBinary:
		return "binary"
	default:
		return "unknown"
	}
}

// DetectInstall inspects the running binary's path and returns the most likely
// install method.
func DetectInstall() InstallMethod {
	exe, err := os.Executable()
	if err != nil {
		return InstallUnknown
	}
	// Resolve symlinks — brew installs the binary as a symlink from /opt/homebrew/bin/grove
	// to the versioned Cellar path. Without this, brew installs misclassify as InstallBinary.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return detectInstallFromPath(exe)
}

// Path heuristics use forward-slash separators. Grove is currently Unix-only;
// Windows install detection would require backslash variants.
func detectInstallFromPath(path string) InstallMethod {
	if path == "" {
		return InstallUnknown
	}
	if strings.Contains(path, "/Cellar/grove/") {
		// covers /opt/homebrew/Cellar/grove/... and /usr/local/Cellar/grove/...
		return InstallBrew
	}
	if strings.Contains(path, "/go/bin/") {
		return InstallGoInstall
	}
	return InstallBinary
}

// UpdateCommand returns the recommended update command for a given install method.
func UpdateCommand(m InstallMethod) string {
	switch m {
	case InstallBrew:
		return "brew upgrade lost-in-the/tap/grove"
	case InstallGoInstall:
		return "go install github.com/lost-in-the/grove/cmd/grove@latest"
	default:
		return "Visit https://github.com/lost-in-the/grove/releases for the latest binary"
	}
}
