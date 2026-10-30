package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLegacyRemovalFromPreviousSerializerDoesNotRepeat(t *testing.T) {
	for _, phase := range []string{"hint", "applied"} {
		t.Run(phase, func(t *testing.T) {
			native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
			s := native.service(t)
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        claude: {id: demo@team, pending: remove}\n")
			if phase == "applied" {
				run := s.Run
				s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
					if len(args) > 1 && args[0] == "plugin" && args[1] == "uninstall" && !slices.Contains(args, "--help") {
						return nil, errors.New("temporary native failure")
					}
					return run(ctx, dir, env, bin, args...)
				}
				applyFailure(t, s, Request{Action: "sync"})
				s.Run = run
			} else if _, err := s.Packages(); err != nil {
				t.Fatal(err)
			}
			// The previous serializer emitted the same receipt without legacySeen.
			// Strip only that new field, preserving the produced hint/applied provenance.
			raw, err := os.ReadFile(s.statePath())
			if err != nil {
				t.Fatal(err)
			}
			var previous map[string]any
			if err := json.Unmarshal(raw, &previous); err != nil {
				t.Fatal(err)
			}
			for _, targets := range previous["bindings"].(map[string]any) {
				for _, binding := range targets.(map[string]any) {
					delete(binding.(map[string]any), "legacySeen")
				}
			}
			raw, err = json.Marshal(previous)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(s.statePath(), raw, 0600); err != nil {
				t.Fatal(err)
			}
			same := unchanged(t, s.ConfigPath)
			applyPluginRequest(t, s, Request{Action: "sync"})
			native.installed["claude"] = []Installed{{ID: "demo@team", Scope: "user", Installed: true}}
			native.commands = nil
			for range 2 {
				applyPluginRequest(t, s, Request{Action: "sync"})
			}
			same()
			if len(native.installed["claude"]) != 1 || len(native.commands) != 0 {
				t.Fatalf("old serializer state re-adopted manual install: %v", native.commands)
			}
		})
	}
}

func TestPluginPublicMessageContracts(t *testing.T) {
	t.Run("selection-outcome", func(t *testing.T) {
		native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
		s := native.service(t)
		applyPluginRequest(t, s, Request{Action: "import", From: "claude", Plugin: "demo@team"})
		req := Request{Action: "disable", Name: "demo", Targets: []string{"claude"}}
		p, err := s.Preview(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Changes) != 1 || p.Changes[0].MessageKey != "plugins.note.selection" {
			t.Fatalf("selection preview key: %+v", p)
		}
		result, err := s.Apply(context.Background(), req, p.Revision)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Results) != 1 || result.Results[0].MessageKey != "" || result.Results[0].Message != "" {
			t.Fatalf("selection outcome key: %+v", result)
		}
	})
	t.Run("imported-install-guidance", func(t *testing.T) {
		native := &fakeAgents{installed: map[string][]Installed{"copilot": {{ID: "demo", Name: "demo", Installed: true}}}}
		s := native.service(t)
		applyPluginRequest(t, s, Request{Action: "import", From: "copilot", Plugin: "demo"})
		native.installed["copilot"] = nil
		before := len(native.commands)
		req := Request{Action: "sync"}
		p, err := s.Preview(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if !p.Blocked || len(p.Changes) != 1 || p.Changes[0].MessageKey != "plugins.note.importInstallRequired" {
			t.Fatalf("import install guidance key: %+v", p)
		}
		if _, err := s.Apply(context.Background(), req, p.Revision); err == nil {
			t.Fatal("unreviewed source installed")
		}
		if len(native.commands) != before {
			t.Fatal("blocked reinstall mutated native state")
		}
	})
}

func TestMigratedNativeImportSurvivesRemoteSourceRecipe(t *testing.T) {
	for _, phase := range []string{"hint", "applied", "repeat-add"} {
		t.Run(phase, func(t *testing.T) {
			nativeA := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
			a := nativeA.service(t)
			writePluginFile(t, filepath.Dir(a.ConfigPath), filepath.Base(a.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        claude: {id: demo@team}\n")
			if phase == "hint" {
				if _, err := a.Packages(); err != nil {
					t.Fatal(err)
				}
			} else {
				applyPluginRequest(t, a, Request{Action: "sync"})
			}
			shared, err := os.ReadFile(a.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			nativeB := &fakeAgents{}
			b := nativeB.service(t)
			writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(shared))
			source := fixture(t)
			applyPluginRequest(t, b, Request{Action: "add", Name: "demo", Source: source, Targets: []string{"claude"}})
			shared, err = os.ReadFile(b.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			writePluginFile(t, filepath.Dir(a.ConfigPath), filepath.Base(a.ConfigPath), string(shared))
			repairHost(t, a)
			nativeA.commands = nil
			same := unchanged(t, a.ConfigPath)
			p, err := a.Preview(context.Background(), Request{Action: "sync"})
			if err != nil {
				t.Fatal(err)
			}
			if p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "noop" || p.Changes[0].ID != "demo@team" || !p.Changes[0].Binding.Imported || p.Changes[0].MessageKey != "plugins.note.importRecipeChanged" {
				t.Fatalf("migrated import replaced after remote recipe change: %+v", p)
			}
			applyPluginRequest(t, a, Request{Action: "sync"})
			same()
			if len(nativeA.commands) != 0 || len(nativeA.installed["claude"]) != 1 || nativeA.installed["claude"][0].ID != "demo@team" {
				t.Fatalf("changed migrated native import: %v %v", nativeA.commands, nativeA.installed)
			}
			if phase == "repeat-add" {
				req := Request{Action: "add", Name: "demo", Source: source, Targets: []string{"claude"}}
				add, err := a.Preview(context.Background(), req)
				if err != nil {
					t.Fatal(err)
				}
				if add.Blocked || len(add.Changes) != 1 || add.Changes[0].Action != "noop" || !add.Changes[0].Binding.Imported {
					t.Fatalf("matching add did not preserve import: %+v", add)
				}
				applyPluginRequest(t, a, req)
				repairHost(t, b)
				applyPluginRequest(t, b, Request{Action: "remove", Name: "demo"})
				applyPluginRequest(t, b, Request{Action: "add", Name: "demo", Source: fixture(t), Targets: []string{"claude"}})
				shared, err = os.ReadFile(b.ConfigPath)
				if err != nil {
					t.Fatal(err)
				}
				writePluginFile(t, filepath.Dir(a.ConfigPath), filepath.Base(a.ConfigPath), string(shared))
				repairHost(t, a)
				same = unchanged(t, a.ConfigPath)
				p, err = a.Preview(context.Background(), Request{Action: "sync"})
				if err != nil {
					t.Fatal(err)
				}
				if p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "noop" || !p.Changes[0].Binding.Imported {
					t.Fatalf("matching noop add lost imported ownership: %+v", p)
				}
				applyPluginRequest(t, a, Request{Action: "sync"})
				same()
				if len(nativeA.commands) != 0 || len(nativeA.installed["claude"]) != 1 || nativeA.installed["claude"][0].ID != "demo@team" {
					t.Fatalf("repeat add allowed native replacement: %v", nativeA.commands)
				}
			}

		})
	}
}
