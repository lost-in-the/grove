package version

// Version information set by build flags or defaults
var (
	// Version is the semantic version of the application
	Version = "0.1.0-dev"
	// Commit is the git commit hash
	Commit = "unknown"
	// BuildDate is the date the binary was built
	BuildDate = "unknown"
)

// GetVersion returns the full version string
func GetVersion() string {
	return Version
}

// GetFullVersion returns version with commit and build date
func GetFullVersion() string {
	return Version + " (commit: " + Commit + ", built: " + BuildDate + ")"
}
