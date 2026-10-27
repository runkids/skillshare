package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Produce private metadata through supported requests, then review the same
// source at a new version while establishing or refreshing its default target.
func TestPackageVersionRefreshOnTargetAdd(t *testing.T) {
	for _, history := range []string{"fresh", "record-only", "removed-record-only"} {
		t.Run(history, func(t *testing.T) {
			var commands []string
			s := fakePiNpm(t, &commands)
			source := hostStatePiSource(t)
			manifest := func(version string) {
				writePluginFile(t, source, "package.json", `{"name":"demo","version":"`+version+`","keywords":["pi-package"]}`)
			}
			manifest("1.2.3")
			if history != "fresh" {
				applyPluginRequest(t, s, Request{Action: "add", Source: source})
				if got := readState(t, s).packages["demo"].Version; got != "1.2.3" {
					t.Fatalf("record-only producer version = %q", got)
				}
			}
			if history == "removed-record-only" {
				applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
				if _, exists := readState(t, s).state.Versions["demo"]; exists {
					t.Fatal("whole removal retained private version")
				}
				if _, exists := readState(t, s).packages["demo"]; exists {
					t.Fatal("whole removal retained package declaration")
				}
			}
			manifest("2.0.0")
			applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"pi"}})
			if got := readState(t, s).packages["demo"].Bindings["pi"].Version; got != "2.0.0" {
				t.Fatalf("installed receipt version = %q, want reviewed 2.0.0", got)
			}
			applyPluginRequest(t, s, Request{Action: "remove", Name: "demo", Targets: []string{"pi"}})
			inv, err := s.Packages()
			if err != nil {
				t.Fatal(err)
			}
			pack, exists := inv.Packages["demo"]
			if !exists || len(pack.Bindings) != 0 {
				t.Fatalf("default source not retained after target removal: %+v", inv.Packages)
			}
			if pack.Version != "2.0.0" {
				t.Errorf("default source version after target removal = %q, want reviewed 2.0.0", pack.Version)
			}
		})
	}
}

func TestRecordOnlyInitialConfigSaveFailureCanRetry(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires enforced Unix directory permissions")
	}
	var commands []string
	s := fakePiNpm(t, &commands)
	configDir := t.TempDir()
	s.ConfigPath = filepath.Join(configDir, "config.yaml")
	s.StateDir = t.TempDir()
	source := hostStatePiSource(t)
	writePluginFile(t, source, "package.json", `{"name":"demo","version":"1.2.3","keywords":["pi-package"]}`)
	r := Request{Action: "add", Source: source}
	p, err := s.Preview(context.Background(), r)
	if err != nil || p.Blocked {
		t.Fatalf("initial preview: %+v %v", p, err)
	}
	if err := os.Chmod(configDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(configDir, 0700) })
	if _, err := s.Apply(context.Background(), r, p.Revision); err == nil {
		t.Fatal("expected initial config save to fail in read-only directory")
	} else {
		t.Logf("initial config save failure: %v", err)
	}
	if _, err := os.Stat(s.ConfigPath); !os.IsNotExist(err) {
		t.Fatalf("failed initial save created config: %v", err)
	}
	if raw, err := os.ReadFile(s.statePath()); err == nil {
		var local struct {
			Bindings map[string]json.RawMessage `json:"bindings"`
		}
		if err := json.Unmarshal(raw, &local); err != nil {
			t.Fatal(err)
		}
		if len(local.Bindings) != 0 {
			t.Fatalf("targetless producer created binding receipts: %s", raw)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(commands) != 0 {
		t.Fatalf("targetless producer ran native commands: %q", commands)
	}
	if err := os.Chmod(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	p, err = s.Preview(context.Background(), r)
	if err != nil || p.Blocked {
		t.Fatalf("retry after repairing config directory: %+v %v", p, err)
	}
	if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
		t.Fatalf("retry apply: %v", err)
	}
	if got := readState(t, s).packages["demo"].Version; got != "1.2.3" {
		t.Fatalf("retried package version = %q", got)
	}
}

func TestAdoptedNativeVersionKeepsReviewedSourceVersion(t *testing.T) {
	native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Version: "native", Scope: "user", Installed: true, Enabled: true}}}}
	s := native.service(t)
	source := fixture(t)
	writePluginFile(t, source, ".claude-plugin/marketplace.json", `{"name":"team","plugins":[{"name":"demo","source":"."}]}`)
	r := Request{Action: "add", Source: source, Targets: []string{"claude"}}
	p, err := s.Preview(context.Background(), r)
	if err != nil || p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "import" {
		t.Fatalf("preview: %+v %v", p, err)
	}
	writePluginFile(t, source, ".codex-plugin/plugin.json", `{"name":"demo","version":"2.0.0","skills":"./skills"}`)
	writePluginFile(t, source, ".claude-plugin/plugin.json", `{"name":"demo","version":"2.0.0"}`)
	if _, err := s.Apply(context.Background(), r, p.Revision); err == nil {
		t.Fatal("source version changed without invalidating preview")
	}
	applyPluginRequest(t, s, r)
	pack := readState(t, s).packages["demo"]
	if pack.Version != "2.0.0" || pack.Bindings["claude"].Version != "native" {
		t.Fatalf("source and native versions: %+v", pack)
	}
}

func TestOtherTargetSourceDoesNotReplaceDefaultVersion(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	first, other := hostStatePiSource(t), hostStatePiSource(t)
	writePluginFile(t, first, "package.json", `{"name":"demo","version":"1.0.0","keywords":["pi-package"]}`)
	writePluginFile(t, other, "package.json", `{"name":"demo","version":"2.0.0","keywords":["pi-package"]}`)
	applyPluginRequest(t, s, Request{Action: "add", Source: first})
	applyPluginRequest(t, s, Request{Action: "add", Source: other, Targets: []string{"pi"}})
	if got := readState(t, s).packages["demo"].Version; got != "1.0.0" {
		t.Fatalf("other source replaced default version: %q", got)
	}
}

func TestPruneVersionOnlyAfterConfigSave(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	source := hostStatePiSource(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: source})
	d := readState(t, s)
	delete(d.intent, "demo")
	if err := os.WriteFile(s.ConfigPath, []byte("changed: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.saveAndPruneVersions(d); err == nil {
		t.Fatal("expected config compare-and-swap failure")
	}
	if _, exists := d.state.Versions["demo"]; !exists {
		t.Fatal("failed save pruned version")
	}
	local, err := os.ReadFile(s.statePath())
	if err != nil {
		t.Fatal(err)
	}
	var state localState
	if err := json.Unmarshal(local, &state); err != nil {
		t.Fatal(err)
	}
	if _, exists := state.Versions["demo"]; !exists {
		t.Fatal("failed save erased persisted version")
	}
}
