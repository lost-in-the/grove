package updatecheck

import (
	"os"
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
	return detectInstallFromPath(exe)
}

func detectInstallFromPath(path string) InstallMethod {
	if path == "" {
		return InstallUnknown
	}
	if strings.Contains(path, "/Cellar/grove/") {
		// covers /opt/homebrew/Cellar/grove/... and /usr/local/Cellar/grove/...
		return InstallBrew
	}
	if strings.HasSuffix(path, "/go/bin/grove") || strings.Contains(path, "/go/bin/") {
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
