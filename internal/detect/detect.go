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
	Commands []string // setup commands (e.g., "bundle install --quiet")
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

	seen := make(map[string]bool) // dedup copy/symlink/command entries
	var matchedTypes []string

	for _, rule := range rules {
		if !markerExists(dir, rule.Marker) {
			continue
		}

		matchedTypes = append(matchedTypes, rule.Type)

		for _, f := range rule.Copy {
			if !seen["copy:"+f] {
				seen["copy:"+f] = true
				profile.Copy = append(profile.Copy, f)
			}
		}
		for _, s := range rule.Symlinks {
			if !seen["sym:"+s] {
				seen["sym:"+s] = true
				profile.Symlinks = append(profile.Symlinks, s)
			}
		}
		for _, c := range rule.Commands {
			if !seen["cmd:"+c] {
				seen["cmd:"+c] = true
				profile.Commands = append(profile.Commands, c)
			}
		}
	}

	// Always include .env and .env.local if present but not already listed
	for _, envFile := range []string{".env", ".env.local"} {
		if !seen["copy:"+envFile] {
			if _, err := os.Stat(filepath.Join(dir, envFile)); err == nil {
				profile.Copy = append(profile.Copy, envFile)
			}
		}
	}

	switch len(matchedTypes) {
	case 0:
		profile.Type = "unknown"
	case 1:
		profile.Type = matchedTypes[0]
	default:
		// Dedup types
		unique := dedupStrings(matchedTypes)
		if len(unique) == 1 {
			profile.Type = unique[0]
		} else {
			profile.Type = "mixed"
			profile.Types = unique
		}
	}

	return profile
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
