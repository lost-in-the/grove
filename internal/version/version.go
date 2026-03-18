package version

import "runtime"

// Version information set by build flags or defaults
var (
	// Version is the semantic version of the application
	Version = "0.5.0"
	// Commit is the git commit hash
	Commit = "unknown"
	// BuildDate is the date the binary was built
	BuildDate = "unknown"
)

// GetVersion returns the version string with platform info.
func GetVersion() string {
	return "grove " + Version + " " + runtime.GOOS + "/" + runtime.GOARCH
}

// GetFullVersion returns version with commit and build date.
func GetFullVersion() string {
	return "grove " + Version + " (" + runtime.GOOS + "/" + runtime.GOARCH +
		", " + runtime.Version() + ", commit: " + Commit + ", built: " + BuildDate + ")"
}
