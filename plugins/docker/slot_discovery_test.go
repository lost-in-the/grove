package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlotDiscoveryTemplateDirectory(t *testing.T) {
	for _, absolute := range []bool{false, true} {
		t.Run(map[bool]string{false: "relative", true: "absolute"}[absolute], func(t *testing.T) {
			root := t.TempDir()
			cfg := newTestAgentConfigWithSlots(t, "sample", root)
			templateDir := filepath.Join(root, "agent-stacks")
			if absolute {
				templateDir = t.TempDir()
				cfg.Plugins.Docker.External.Agent.TemplatePath = filepath.Join(templateDir, "template.yml")
			}
			cfg.Plugins.Docker.External.Agent.TemplateOverlays = []string{"other/overlay.yml"}
			s := newAgentExternalStrategy(cfg)
			if err := os.MkdirAll(filepath.Dir(s.slots.slotsFile), 0755); err != nil {
				t.Fatal(err)
			}
			writeFakeDocker(t,
				"foreign-agent-1|"+templateDir+"|exited",
				"foreign-agent-2|"+root+"|running",
				"unrelated-agent-3|"+filepath.Join(root, "other")+"|running")
			slots, err := ListStackSlots(cfg)
			if err != nil || len(slots) != 2 {
				t.Fatalf("slots = %+v, err = %v", slots, err)
			}
			if slots[0].Running || !slots[0].Foreign || !slots[1].Running {
				t.Fatalf("incorrect attribution: %+v", slots)
			}
			if holder, ok := s.foreignSlotHolder(root, 1, "sample-agent-1"); !ok || holder != "foreign-agent-1" {
				t.Fatalf("holder = %q, %v", holder, ok)
			}
			if err := s.Up(filepath.Join(root, "sample-feature"), true); err != nil {
				t.Fatal(err)
			}
			slot, err := s.slots.FindSlot("sample-feature")
			if err != nil || slot != 3 {
				t.Fatalf("allocated %d, err %v; want 3", slot, err)
			}
		})
	}
}

func TestSlotDiscoveryUnknownOrAmbiguous(t *testing.T) {
	for _, scenario := range []string{"unreachable", "foreign-first", "own-first"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			cfg := newTestAgentConfigWithSlots(t, "sample", root)
			s := newAgentExternalStrategy(cfg)
			if err := os.MkdirAll(filepath.Dir(s.slots.slotsFile), 0755); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "unreachable":
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "docker"), []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			case "foreign-first":
				writeFakeDocker(t, "foreign-agent-1|"+root+"|running", "sample-agent-1|"+root+"|exited")
			case "own-first":
				writeFakeDocker(t, "sample-agent-1|"+root+"|exited", "foreign-agent-1|"+root+"|running")
			}
			if slots, err := ListStackSlots(cfg); err == nil || len(slots) != 0 {
				t.Errorf("want discovery error, got %+v, %v", slots, err)
			}
			err := s.Up(filepath.Join(root, "sample-feature"), true)
			if err == nil || !strings.Contains(err.Error(), "occupancy") {
				t.Errorf("want occupancy error, got %v", err)
			}
			slots, err := s.slots.ListActive()
			if err != nil || len(slots) != 0 {
				t.Errorf("discovery failure allocated a slot: %+v, %v", slots, err)
			}
		})
	}
}
