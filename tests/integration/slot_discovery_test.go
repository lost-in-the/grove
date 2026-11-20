//go:build integration

package integration_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

func slotDiscoveryEnv() []string {
	var env []string
	for _, entry := range testhelper.GitEnv() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "AGENT_SLOT", "COMPOSE_PROJECT_NAME", "GROVE_AGENT_MODE":
			continue
		}
		env = append(env, entry)
	}
	return append(env, "GROVE_NONINTERACTIVE=1", "GROVE_TUI=0")
}

func slotDiscoveryConfig(t *testing.T, repo, template string) {
	t.Helper()
	testhelper.WriteFile(t, filepath.Join(repo, ".grove", "config.toml"), fmt.Sprintf(`
project_name = %q
[mux]
backend = "off"
[plugins.docker]
enabled = true
mode = "external"
[plugins.docker.external]
path = %q
env_var = "APP_DIR"
services = ["app"]
[plugins.docker.external.agent]
enabled = true
max_slots = 3
services = ["app"]
template_path = %q
`, fmt.Sprintf("grove-discovery-%x", sha256.Sum256([]byte(repo)))[:32], repo, template))
}

func TestSlotDiscoveryCLIRealCompose(t *testing.T) {
	requireDocker(t)
	binary := buildGroveBinary(t)
	for _, absolute := range []bool{false, true} {
		t.Run(fmt.Sprintf("absolute=%v", absolute), func(t *testing.T) {
			repo := testhelper.SetupRailsFixture(t)
			template := filepath.Join(repo, "stacks", "template.yml")
			testhelper.WriteFile(t, template, "services:\n  app:\n    image: alpine:3.22\n    command: [sleep, '300']\n")
			configured := "stacks/template.yml"
			if absolute {
				configured = template
			}
			slotDiscoveryConfig(t, repo, configured)
			// A unique namespace and private temp directory constrain every mutation.
			foreign := fmt.Sprintf("grove-foreign-%x", sha256.Sum256([]byte(repo)))[:32] + "-agent-1"
			compose := func(args ...string) {
				t.Helper()
				cmd := exec.Command("docker", append([]string{"compose", "-p", foreign, "-f", template}, args...)...)
				cmd.Dir = repo
				cmd.Env = slotDiscoveryEnv()
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("compose: %v: %s", err, out)
				}
			}
			t.Cleanup(func() { compose("down") })
			compose("up", "-d")
			ps := func(wantRunning bool) {
				t.Helper()
				cmd := exec.Command(binary, "ps", "--json")
				cmd.Dir = repo
				cmd.Env = slotDiscoveryEnv()
				out, err := cmd.Output()
				if err != nil {
					t.Fatalf("ps: %v", err)
				}
				var slots []struct {
					Slot    int
					Foreign bool
					Running bool
				}
				if err := json.Unmarshal(out, &slots); err != nil {
					t.Fatal(err)
				}
				if len(slots) != 1 || slots[0].Slot != 1 || !slots[0].Foreign || slots[0].Running != wantRunning {
					t.Fatalf("unexpected slots: %s", out)
				}
			}
			ps(true)
			compose("stop", "-t", "1")
			ps(false)
			// A fresh Grove process must skip the stopped foreign slot without exports.
			t.Cleanup(func() {
				cmd := exec.Command(binary, "down")
				cmd.Dir = repo
				cmd.Env = slotDiscoveryEnv()
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Errorf("cleanup: %v: %s", err, out)
				}
			})
			cmd := exec.Command(binary, "up", "--isolated")
			cmd.Dir = repo
			cmd.Env = slotDiscoveryEnv()
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("up: %v: %s", err, out)
			}
			cmd = exec.Command(binary, "ps", "--json")
			cmd.Dir = repo
			cmd.Env = slotDiscoveryEnv()
			out, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			var slots []struct {
				Slot    int
				Foreign bool
				Running bool
			}
			if err := json.Unmarshal(out, &slots); err != nil {
				t.Fatal(err)
			}
			if len(slots) != 2 || slots[1].Slot != 2 || slots[1].Foreign || !slots[1].Running {
				t.Fatalf("allocation: %s", out)
			}
		})
	}
}

func TestSlotDiscoveryCLIError(t *testing.T) {
	binary := buildGroveBinary(t)
	for _, configured := range []bool{false, true} {
		for _, jsonOutput := range []bool{false, true} {
			t.Run(fmt.Sprintf("configured=%v/json=%v", configured, jsonOutput), func(t *testing.T) {
				repo := testhelper.SetupRailsFixture(t)
				if configured {
					slotDiscoveryConfig(t, repo, "stacks/template.yml")
				} else {
					testhelper.WriteFile(t, filepath.Join(repo, ".grove", "config.toml"), "project_name = \"sample\"\n")
				}
				fake := t.TempDir()
				if err := os.WriteFile(filepath.Join(fake, "docker"), []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
					t.Fatal(err)
				}
				args := []string{"ps"}
				if jsonOutput {
					args = append(args, "--json")
				}
				cmd := exec.Command(binary, args...)
				cmd.Dir = repo
				cmd.Env = append(slotDiscoveryEnv(), "PATH="+fake+string(os.PathListSeparator)+os.Getenv("PATH"))
				out, err := cmd.CombinedOutput()
				if !configured && !jsonOutput {
					if err != nil || !strings.Contains(string(out), "Stacks not configured") {
						t.Fatalf("help: %s, %v", out, err)
					}
					return
				}
				if err == nil {
					t.Fatalf("expected nonzero exit: %s", out)
				}
				if jsonOutput {
					ee, ok := err.(*exec.ExitError)
					if !ok || ee.ExitCode() != 6 {
						t.Fatalf("exit = %v", err)
					}
					var failure struct {
						Error   bool
						Code    int
						Message string
					}
					if err := json.Unmarshal(out, &failure); err != nil {
						t.Fatalf("invalid JSON: %s", out)
					}
					if !failure.Error || failure.Code != 6 {
						t.Fatalf("error: %s", out)
					}
				} else if strings.Contains(string(out), "Stacks not configured") {
					t.Fatalf("discovery failure masked: %s", out)
				}
			})
		}
	}
}
