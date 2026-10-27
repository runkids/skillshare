package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRecordOnlyVersionSurvivesUnavailableSource(t *testing.T) {
	for _, format := range []string{"record-only", "legacy"} {
		t.Run(format, func(t *testing.T) {
			var commands []string
			s := fakePiNpm(t, &commands)
			source := t.TempDir()
			writePluginFile(t, source, ".claude-plugin/plugin.json", `{"name":"demo","version":"1.2.3"}`)
			writePluginFile(t, source, "skills/demo/SKILL.md", "---\nname: demo\ndescription: demo\n---\n")
			if format == "record-only" {
				r := Request{Action: "add", Source: source}
				p, err := s.Preview(context.Background(), r)
				if err != nil || p.Blocked || len(p.Changes) != 1 || p.Changes[0].Binding.Version != "1.2.3" {
					t.Fatalf("source was not reviewed at1.2.3: %+v %v", p, err)
				}
				if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
					t.Fatal(err)
				}
			} else {
				// Shape emitted by the upstream e64fac67 targetless-add package writer.
				raw, err := yaml.Marshal(map[string]any{"plugins": map[string]any{"packages": map[string]Package{"demo": {Source: source, Plugin: "demo", Version: "1.2.3", Bindings: map[string]Binding{}}}}})
				if err != nil {
					t.Fatal(err)
				}
				writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), string(raw))
			}
			if err := os.RemoveAll(source); err != nil {
				t.Fatal(err)
			} // Task-owned source is now offline.
			assertVersion := func(phase string) {
				t.Helper()
				same := unchanged(t, s.ConfigPath, s.statePath())
				inv, err := s.Packages()
				if err != nil {
					t.Fatal(err)
				}
				if got := inv.Packages["demo"].Version; got != "1.2.3" {
					t.Errorf("%s package version=%q, want reviewed1.2.3 without source", phase, got)
				}
				same()
			}
			assertVersion("offline " + format)
			if format == "legacy" {
				other := t.TempDir()
				writePluginFile(t, other, ".claude-plugin/plugin.json", `{"name":"other","version":"2.0.0"}`)
				applyPluginRequest(t, s, Request{Action: "add", Source: other})
				raw, _ := os.ReadFile(s.ConfigPath)
				if !strings.Contains(string(raw), "version: 2") {
					t.Fatal("supported declaration edit did not migrate legacy syntax")
				}
				if strings.Contains(string(raw), "1.2.3") {
					t.Fatal("migration exported an observed version into shared intent")
				}
				assertVersion("after legacy migration")
			}
			if len(commands) != 0 {
				t.Fatal("targetless record/read invoked native commands")
			}
		})
	}
}

func TestPackageVersionIsPrivateAndTracksDefaultSource(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	source := t.TempDir()
	manifest := ".claude-plugin/plugin.json"
	writePluginFile(t, source, manifest, `{"name":"demo","version":"1.2.3"}`)
	r := Request{Action: "add", Source: source}
	applyPluginRequest(t, s, r)
	shared, err := os.ReadFile(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	other := *s
	other.StateDir = t.TempDir()
	inv, err := other.Packages()
	if err != nil || inv.Packages["demo"].Version != "" {
		t.Fatalf("new host inherited version: %+v %v", inv, err)
	}
	for _, field := range []string{"source", "source_ref", "plugin", "entry"} {
		var doc map[string]any
		if err := yaml.Unmarshal(shared, &doc); err != nil {
			t.Fatal(err)
		}
		packages := doc["plugins"].(map[string]any)["packages"].(map[string]any)
		packages["demo"].(map[string]any)[field] = "changed"
		changed, err := yaml.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(s.ConfigPath, changed, 0600); err != nil {
			t.Fatal(err)
		}
		inv, err := s.Packages()
		if err != nil || inv.Packages["demo"].Version != "" {
			t.Fatalf("changed %s retained version: %+v %v", field, inv, err)
		}
	}
	if err := os.WriteFile(s.ConfigPath, shared, 0600); err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, source, manifest, `{"name":"demo"}`)
	applyPluginRequest(t, s, r)
	after, err := os.ReadFile(s.ConfigPath)
	if err != nil || string(after) != string(shared) {
		t.Fatal("metadata refresh rewrote shared configuration", err)
	}
	inv, err = s.Packages()
	if err != nil || inv.Packages["demo"].Version != "" {
		t.Fatalf("empty refresh retained version: %+v %v", inv, err)
	}
	if len(commands) != 0 {
		t.Fatal("record/read invoked native commands")
	}
}

func TestEmptyRefreshOverridesLegacyVersion(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	source := t.TempDir()
	writePluginFile(t, source, ".claude-plugin/plugin.json", `{"name":"demo"}`)
	raw, err := yaml.Marshal(map[string]any{"plugins": map[string]any{"packages": map[string]Package{"demo": {Source: source, Plugin: "demo", Version: "1.2.3", Bindings: map[string]Binding{}}}}})
	if err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), string(raw))
	applyPluginRequest(t, s, Request{Action: "add", Source: source})
	after, err := os.ReadFile(s.ConfigPath)
	if err != nil || string(after) != string(raw) {
		t.Fatal("refresh rewrote legacy configuration", err)
	}
	inv, err := s.Packages()
	if err != nil || inv.Packages["demo"].Version != "" {
		t.Fatalf("legacy version survived empty refresh: %+v %v", inv, err)
	}
}
