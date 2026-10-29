package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPluginHostStateCrossHostAbsentOrphanCleanup(t *testing.T) {
	var ca, cb []string
	a := fakePiNpm(t, &ca)
	source := hostStatePiSource(t)
	applyPluginRequest(t, a, Request{Action: "add", Source: source, Targets: []string{"pi"}})
	copyIntent := func(from, to *Service) {
		t.Helper()
		raw, err := os.ReadFile(from.ConfigPath)
		if err != nil {
			t.Fatal(err)
		}
		writePluginFile(t, filepath.Dir(to.ConfigPath), filepath.Base(to.ConfigPath), string(raw))
	}
	b := fakePiNpm(t, &cb)
	copyIntent(a, b)
	applyPluginRequest(t, b, Request{Action: "sync"})
	applyPluginRequest(t, b, Request{Action: "disable", Name: "demo", Targets: []string{"pi"}})
	copyIntent(b, a)
	applyPluginRequest(t, b, Request{Action: "sync"})
	repairHost(t, a)
	applyPluginRequest(t, a, Request{Action: "sync"})
	if readState(t, a).state.Bindings["demo"]["pi"].Applied == nil {
		t.Fatal("deselection lost retained receipt")
	}
	repairHost(t, b)
	applyPluginRequest(t, b, Request{Action: "enable", Name: "demo", Targets: []string{"pi"}})
	applyPluginRequest(t, b, Request{Action: "sync"})
	applyPluginRequest(t, b, Request{Action: "remove", Name: "demo"})
	copyIntent(b, a)
	repairHost(t, a)
	same := unchanged(t, a.ConfigPath)
	before := len(ca)
	req := Request{Action: "sync"}
	p, err := a.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "forget" {
		t.Fatalf("absent orphan did not plan metadata cleanup: %+v", p)
	}
	if _, err := a.Apply(context.Background(), req, p.Revision); err != nil {
		t.Fatal(err)
	}
	same()
	d := readState(t, a)
	if len(d.state.Bindings) != 0 || len(d.packages) != 0 {
		t.Fatalf("orphan local state survived apply: %+v", d.state)
	}
	applyPluginRequest(t, a, req)
	same()
	if len(ca) != before {
		t.Fatal("metadata cleanup or second sync ran native commands")
	}
}

func TestPluginHostStateAbsentForgetAcrossAdapters(t *testing.T) {
	for _, target := range []string{"pi", "opencode", "cursor", "antigravity", "claude"} {
		t.Run(target, func(t *testing.T) {
			var commands []string
			var s *Service
			if target == "claude" {
				s = (&fakeAgents{}).service(t)
			} else {
				s = fakePiNpm(t, &commands)
				run := s.Run
				s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
					if bin == "opencode" && args[0] == "--version" {
						return []byte("1.9.0"), nil
					}
					return run(ctx, dir, env, bin, args...)
				}
			}
			source := fixture(t)
			writePluginFile(t, source, ".cursor-plugin/plugin.json", `{"name":"demo"}`)
			writePluginFile(t, source, "plugin.json", `{"name":"demo"}`)
			writePluginFile(t, source, "package.json", `{"name":"demo","keywords":["pi-package"],"dependencies":{"@opencode-ai/plugin":"1"},"main":"index.js"}`)
			writePluginFile(t, source, "index.js", "export default {}")
			writePluginFile(t, source, "extensions/main.ts", "export default () => {}")
			applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{target}})
			applyPluginRequest(t, s, Request{Action: "disable", Name: "demo", Targets: []string{target}})
			applyPluginRequest(t, s, Request{Action: "sync"})
			d := readState(t, s)
			delete(d.intent, "demo")
			if err := s.save(d); err != nil {
				t.Fatal(err)
			}
			same := unchanged(t, s.ConfigPath)
			applyPluginRequest(t, s, Request{Action: "sync"})
			same()
			if len(readState(t, s).state.Bindings) != 0 {
				t.Fatal("metadata-only forget retained local receipt")
			}
			stable := unchanged(t, s.ConfigPath, s.statePath())
			applyPluginRequest(t, s, Request{Action: "sync"})
			stable()
		})
	}
}

func TestPluginHostStateAbsentForgetClearsPendingAndLegacy(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        pi: {id: 'npm:old'}\n")
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if args[0] == "install" && args[1] != "--help" {
			return nil, errors.New("install failed before native registration")
		}
		return run(ctx, dir, env, bin, args...)
	}
	applyFailure(t, s, Request{Action: "add", Name: "demo", Source: "npm:new", Targets: []string{"pi"}})
	if b := hostPackages(t, s)["demo"].Bindings["pi"]; b.Pending != "install" {
		t.Fatal("fixture did not expose failed work")
	}
	s.Run = run
	// Another machine removed the shared declaration; retain this host's failed work.
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  version: 2\n  packages: {}\n")
	applyPluginRequest(t, s, Request{Action: "sync"})
	if changes := previewChanges(t, s, Request{Action: "sync"}); len(changes) != 0 {
		t.Fatalf("forget left executable pending or legacy work: %+v", changes)
	}

	same := unchanged(t, s.ConfigPath, s.statePath())
	applyPluginRequest(t, s, Request{Action: "sync"})
	same()
}

func TestPluginHostStateAbsentReplacementDoesNotBlockHealthyTarget(t *testing.T) {
	var ca, cb []string
	a := fakePiNpm(t, &ca)
	source := hostStatePiSource(t)
	applyPluginRequest(t, a, Request{Action: "add", Source: source, Targets: []string{"pi"}})
	applyPluginRequest(t, a, Request{Action: "disable", Name: "demo", Targets: []string{"pi"}})
	applyPluginRequest(t, a, Request{Action: "sync"})
	oldID := readState(t, a).packages["demo"].Bindings["pi"].ID
	raw, _ := os.ReadFile(a.ConfigPath)
	b := fakePiNpm(t, &cb)
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(raw))
	applyPluginRequest(t, b, Request{Action: "remove", Name: "demo", Targets: []string{"pi"}})
	applyPluginRequest(t, b, Request{Action: "add", Name: "demo", Source: hostStatePiSource(t), Targets: []string{"pi"}})
	healthy := hostStatePiSource(t)
	writePluginFile(t, healthy, "package.json", `{"name":"healthy","keywords":["pi-package"]}`)
	applyPluginRequest(t, b, Request{Action: "add", Source: healthy, Targets: []string{"pi"}})
	raw, _ = os.ReadFile(b.ConfigPath)
	writePluginFile(t, filepath.Dir(a.ConfigPath), filepath.Base(a.ConfigPath), string(raw))
	repairHost(t, a)
	same := unchanged(t, a.ConfigPath)
	before := len(ca)
	p, err := a.Preview(context.Background(), Request{Action: "sync"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Blocked || len(p.Changes) != 3 || p.Changes[0].Action != "forget" {
		t.Fatalf("metadata cleanup blocked replacement/healthy target: %+v", p.Changes)
	}
	if _, err := a.Apply(context.Background(), Request{Action: "sync"}, p.Revision); err != nil {
		t.Fatal(err)
	}
	same()
	d := readState(t, a)
	local := d.state.Bindings["demo"]["pi"]
	if local.Applied == nil || local.Applied.Binding.ID == oldID || local.Pending != nil || local.Legacy != nil || d.state.Bindings["healthy"]["pi"].Applied == nil {
		t.Fatal("replacement or healthy receipt missing")
	}
	if len(ca)-before != 2 {
		t.Fatalf("forget called native client: %v", ca[before:])
	}
	stable := unchanged(t, a.ConfigPath, a.statePath())
	applyPluginRequest(t, a, Request{Action: "sync"})
	stable()
	if len(ca)-before != 2 {
		t.Fatal("second sync repeated native work")
	}
}
