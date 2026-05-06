package detect

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// composeFiles are the conventional compose file names checked in order.
var composeFiles = []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}

// infraServiceNames are services we skip when guessing the "app" service.
// These typically host stateful infra rather than application code.
var infraServiceNames = map[string]bool{
	"db":            true,
	"database":      true,
	"postgres":      true,
	"postgresql":    true,
	"mysql":         true,
	"mariadb":       true,
	"redis":         true,
	"memcached":     true,
	"cache":         true,
	"elasticsearch": true,
	"opensearch":    true,
	"kafka":         true,
	"rabbitmq":      true,
	"mailcatcher":   true,
	"mailhog":       true,
	"selenium":      true,
	"chrome":        true,
	"chromedriver":  true,
	"minio":         true,
}

// appServicePriorityNames are canonical names for the "main" application
// service. When a compose file declares one of these alongside other
// non-infra services, we prefer it. Order in the slice is preference order:
// `web` is the most common Rails convention, `app` covers Django/Node, etc.
//
// This exists because the previous heuristic ("first non-infra in declaration
// order") got confused by stacks like Rails+webpack: `webpack` declared
// before `web` would win, even though `bundle install` belongs in `web`.
var appServicePriorityNames = []string{
	"web",
	"app",
	"rails",
	"backend",
	"api",
	"server",
	"application",
}

// FindComposeFile returns the path to the first compose file present in dir,
// or empty string if none.
func FindComposeFile(dir string) string {
	for _, name := range composeFiles {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// HasDocker reports whether the directory looks like a Docker-based project
// (compose file or Dockerfile present).
func HasDocker(dir string) bool {
	if FindComposeFile(dir) != "" {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err == nil {
		return true
	}
	return false
}

// InferAppService picks the most likely application service from a compose
// file, returning ("", false) if it can't make a confident guess.
//
// Heuristic:
//  1. Single service → return it.
//  2. Multiple services → return first non-infra service.
//  3. No services or all infra → ("", false).
//
// Callers that also need the full service list should call readComposeServices
// directly + pickAppService to avoid reading the file twice.
func InferAppService(composePath string) (string, bool) {
	services, err := readComposeServices(composePath)
	if err != nil {
		return "", false
	}
	return pickAppService(services)
}

// pickAppService applies the inference heuristic to a service list.
//
// Order:
//  1. Single service → return it.
//  2. Service name matches one of appServicePriorityNames (case-insensitive)
//     → return it. This catches the common case where a Rails app has a
//     `web` service alongside `webpack` / `worker` / `mailer`.
//  3. First non-infra service in declaration order.
//  4. No services or all infra → ("", false).
func pickAppService(services []string) (string, bool) {
	if len(services) == 0 {
		return "", false
	}
	if len(services) == 1 {
		return services[0], true
	}

	// Build a set for O(1) lookup of the declared services.
	declared := make(map[string]string, len(services))
	for _, s := range services {
		declared[strings.ToLower(s)] = s
	}

	// First pass: prefer canonical app names if any are declared.
	for _, name := range appServicePriorityNames {
		if original, ok := declared[name]; ok {
			return original, true
		}
	}

	// Second pass: first non-infra in declaration order.
	for _, s := range services {
		if !infraServiceNames[strings.ToLower(s)] {
			return s, true
		}
	}
	return "", false
}

// readComposeServices returns the top-level service names from a compose file.
// We do a structural read rather than full YAML parsing to avoid the
// dependency. Supported layouts:
//
//   - Top-level `services:` block (no `services:` nested under another key
//     like `profiles:` is recognized — by design).
//   - Mapping-style child keys, indented uniformly with spaces or tabs.
//
// Mixed-indent files (some children with spaces, some with tabs at the same
// nesting depth) and YAML list-form services are not understood and produce
// an empty list — callers fall back to "service unknown" and prompt or skip.
//
// Heuristic limitations (intentional, not bugs):
//
//   - YAML anchors (`&name`) and aliases (`*name`) are treated as opaque
//     text. A service definition reached via an alias won't be expanded.
//   - Merge keys (`<<: *base`) are ignored.
//   - Multi-document YAML (`---` separators) is not recognized — only the
//     first document's top-level `services:` is read.
//   - Quoted service keys (`"web":`) and folded/flow-mapping forms
//     (`services: {web: {...}}`) are not matched by the line regex.
//
// This is acceptable because the result is only used to *suggest* an app
// service name during init/doctor; an empty result triggers a manual prompt
// rather than a wrong inference.
//
// Service-name lines look like `  app:` (key, optional whitespace, optional
// trailing comment); deeper keys (`    image: ruby`) are ignored because
// they're at a different indent than the first child key.
var serviceLineRe = regexp.MustCompile(`^([ \t]+)([A-Za-z0-9_.-]+):\s*(?:#.*)?$`)

func readComposeServices(composePath string) ([]string, error) {
	f, err := os.Open(composePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		inServices bool
		baseIndent = "" // exact indent prefix of service name lines under `services:`
		services   []string
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}

		// Top-level keys start at column 0.
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			if strings.HasPrefix(trim, "services:") {
				inServices = true
				baseIndent = ""
			} else {
				inServices = false
			}
			continue
		}

		if !inServices {
			continue
		}

		m := serviceLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		indent := m[1]
		if baseIndent == "" {
			baseIndent = indent
		}
		// Compare full prefix (preserves tab vs space distinction) so we
		// never confuse a 2-space key under `services:` with a 2-tab nested
		// key under a different parent.
		if indent == baseIndent {
			services = append(services, m[2])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return services, nil
}

// init.go's renderer leans on this list to make a confident inference. When
// it returns empty for a syntactically valid compose file, the caller falls
// back to a manual-setup hint rather than invent a service name. Don't add
// silent rescue heuristics here — better to ask the user.
