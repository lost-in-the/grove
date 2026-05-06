package detect

import (
	"os"
	"path/filepath"
)

// ProjectProfile describes the detected project type and recommended hooks
type ProjectProfile struct {
	Type     string   // "rails", "node", "go", "python", "docker", "mixed", "unknown"
	Types    []string // all detected types when mixed
	Copy     []string // files to copy (e.g., ".env", "config/master.key")
	Symlinks []string // directories to symlink (e.g., "node_modules")
	Commands []string // host setup commands (e.g., "bundle install --quiet")

	// ContainerCommands is populated when Docker is detected alongside a
	// language toolchain. Install/setup commands move here so callers render
	// them as compose-typed hooks instead of host commands.
	ContainerCommands []ContainerCommand

	// HasDocker is true when a Dockerfile or compose file is present.
	HasDocker bool

	// DockerService is the inferred app service name, "" if unknown.
	DockerService string

	// DockerServiceInferred is true when DockerService was guessed (vs picked
	// from a single-service compose file). Callers can warn the user.
	DockerServiceInferred bool

	// DockerComposeMissing is true when Docker is detected but no compose
	// file is found, OR the compose file has no plausible app service. The
	// init renderer uses this to emit a manual-setup comment instead of
	// generating broken docker:compose hooks.
	DockerComposeMissing bool
}

// ContainerCommand is a setup command targeted at a compose service.
type ContainerCommand struct {
	Service string
	Command string
}

// DetectionRule maps a marker file to project type and recommended actions
type DetectionRule struct {
	Marker   string   // file to look for
	Type     string   // project type name
	Copy     []string // files to copy
	Symlinks []string // directories to symlink
	Commands []string // setup commands to run
}

// DefaultRules returns the built-in detection rules, checked in order.
// Multiple rules can match, resulting in a "mixed" project type.
func DefaultRules() []DetectionRule {
	return []DetectionRule{
		{
			Marker:   "Gemfile",
			Type:     "rails",
			Copy:     []string{".env", "config/master.key", "config/credentials/*.yml.enc"},
			Symlinks: []string{"vendor/bundle"},
			Commands: []string{"bundle install --quiet"},
		},
		{
			Marker:   "package.json",
			Type:     "node",
			Copy:     []string{".env", ".env.local"},
			Symlinks: []string{"node_modules"},
			Commands: []string{"npm install"},
		},
		{
			Marker: "go.mod",
			Type:   "go",
			Copy:   []string{".env"},
		},
		{
			Marker:   "requirements.txt",
			Type:     "python",
			Copy:     []string{".env"},
			Symlinks: []string{".venv"},
			Commands: []string{"pip install -r requirements.txt"},
		},
		{
			Marker:   "pyproject.toml",
			Type:     "python",
			Copy:     []string{".env"},
			Symlinks: []string{".venv"},
			Commands: []string{"pip install -e ."},
		},
		{
			Marker: "docker-compose.yml",
			Type:   "docker",
			Copy:   []string{".env"},
		},
	}
}

// Detect scans the given directory for marker files and returns a ProjectProfile
func Detect(dir string) *ProjectProfile {
	return DetectWithRules(dir, DefaultRules())
}

// DetectWithRules scans using the provided rules
func DetectWithRules(dir string, rules []DetectionRule) *ProjectProfile {
	profile := &ProjectProfile{
		Type: "unknown",
	}

	seen := make(map[string]bool)
	var matchedTypes []string

	for _, rule := range rules {
		if !markerExists(dir, rule.Marker) {
			continue
		}
		matchedTypes = append(matchedTypes, rule.Type)
		mergeRuleIntoProfile(profile, rule, seen)
	}

	ensureDefaultEnvFiles(profile, dir, seen)
	profile.Type = resolveProjectType(matchedTypes)
	if profile.Type == "mixed" {
		profile.Types = dedupStrings(matchedTypes)
	}

	applyDockerAwareness(profile, dir)

	return profile
}

// applyDockerAwareness moves host setup commands into ContainerCommands when
// a compose-managed Docker stack is present. Issue #28: running bundle/npm
// install on the host fails (or no-ops into a volume-masked dir) for Docker-
// based dev stacks.
//
// Dockerfile-only projects (no compose file) are intentionally skipped: a
// `docker:compose` hook would error every grove new because there's no
// compose project to run against. Those projects keep host commands; the
// init renderer adds a manual-setup comment.
func applyDockerAwareness(profile *ProjectProfile, dir string) {
	profile.HasDocker = HasDocker(dir)
	if !profile.HasDocker || len(profile.Commands) == 0 {
		return
	}

	composePath := FindComposeFile(dir)
	if composePath == "" {
		// Dockerfile-only — flag for init renderer but don't reroute commands.
		profile.DockerComposeMissing = true
		return
	}

	services, _ := readComposeServices(composePath)
	service, ok := pickAppService(services)
	if !ok {
		// All services look like infrastructure — not safe to guess.
		// Keep host commands; renderer will note the situation.
		profile.DockerComposeMissing = true
		return
	}

	profile.DockerService = service
	profile.DockerServiceInferred = len(services) > 1

	moved := make([]ContainerCommand, 0, len(profile.Commands))
	for _, cmd := range profile.Commands {
		moved = append(moved, ContainerCommand{Service: service, Command: cmd})
	}
	profile.ContainerCommands = moved
	profile.Commands = nil
}

func mergeRuleIntoProfile(profile *ProjectProfile, rule DetectionRule, seen map[string]bool) {
	profile.Copy = appendUnique(profile.Copy, rule.Copy, "copy:", seen)
	profile.Symlinks = appendUnique(profile.Symlinks, rule.Symlinks, "sym:", seen)
	profile.Commands = appendUnique(profile.Commands, rule.Commands, "cmd:", seen)
}

func appendUnique(dest, src []string, prefix string, seen map[string]bool) []string {
	for _, s := range src {
		key := prefix + s
		if !seen[key] {
			seen[key] = true
			dest = append(dest, s)
		}
	}
	return dest
}

func ensureDefaultEnvFiles(profile *ProjectProfile, dir string, seen map[string]bool) {
	for _, envFile := range []string{".env", ".env.local"} {
		if seen["copy:"+envFile] {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, envFile)); err == nil {
			profile.Copy = append(profile.Copy, envFile)
		}
	}
}

func resolveProjectType(matchedTypes []string) string {
	if len(matchedTypes) == 0 {
		return "unknown"
	}
	unique := dedupStrings(matchedTypes)
	if len(unique) == 1 {
		return unique[0]
	}
	return "mixed"
}

// markerExists checks if a marker file or glob pattern exists in dir
func markerExists(dir, marker string) bool {
	// Try as a glob pattern first
	matches, err := filepath.Glob(filepath.Join(dir, marker))
	if err == nil && len(matches) > 0 {
		return true
	}
	// Direct file check
	_, err = os.Stat(filepath.Join(dir, marker))
	return err == nil
}

func dedupStrings(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
