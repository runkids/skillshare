package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPluginHostStateRecoversVerifiedPendingInstall(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	applyPluginRequest(t, s, Request{Action: "add", Source: hostStatePiSource(t), Targets: []string{"pi"}})
	d := readState(t, s)
	local := d.state.Bindings["demo"]["pi"]
	// Crash after native success, before the final local receipt save.
	local.Pending = local.Applied
	local.Pending.Action = "install"
	local.Applied = nil
	d.putLocal("demo", "pi", local)
	if err := s.saveState(d); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, Request{Action: "sync"})
	if got := hostPackages(t, s)["demo"].Bindings["pi"]; got.ID == "" || got.Pending != "" {
		t.Fatalf("verified pending receipt lost: %+v", got)
	}
	applyPluginRequest(t, s, Request{Action: "sync"})
	if len(commands) != 1 {
		t.Fatal("replayed verified install", commands)
	}
}

func TestPluginHostStateUpdateLeavesSharedIntentUntouched(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	source := hostStatePiSource(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"pi"}})
	old := hostPackages(t, s)["demo"].Bindings["pi"]
	same := unchanged(t, s.ConfigPath)
	writePluginFile(t, source, "extensions/demo.ts", "export default () => { return 1 }")
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo"})
	same()
	if b := hostPackages(t, s)["demo"].Bindings["pi"]; b.Digest == old.Digest || b.Pending != "" {
		t.Fatal("local update lost receipt")
	}
}

func TestPluginHostStateOrphanSelectionDoesNotRecreateIntent(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	applyPluginRequest(t, s, Request{Action: "add", Source: hostStatePiSource(t), Targets: []string{"pi"}})
	d := readState(t, s)
	delete(d.intent, "demo")
	if err := s.save(d); err != nil {
		t.Fatal(err)
	}
	same := unchanged(t, s.ConfigPath)
	applyPluginRequest(t, s, Request{Action: "disable", Name: "demo", Targets: []string{"pi"}})
	same()
	if _, err := s.Packages(); err != nil {
		t.Fatal("selection wrote invalid intent", err)
	}
}

func TestPluginHostStateReplacementCannotAdoptInstallationBeingRemoved(t *testing.T) {
	native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true, Enabled: true}}}}
	s := native.service(t)
	source := fixture(t)
	writePluginFile(t, source, ".claude-plugin/marketplace.json", `{"name":"team","plugins":[{"name":"demo","source":"."}]}`)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"claude"}})
	other := fixture(t)
	writePluginFile(t, other, ".claude-plugin/marketplace.json", `{"name":"team","plugins":[{"name":"demo","source":"."}]}`)
	d := readState(t, s)
	pack := d.intent["demo"]
	decl := pack.Bindings["claude"]
	decl.Source = other
	pack.Bindings["claude"] = decl
	d.intent["demo"] = pack
	if err := s.save(d); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, Request{Action: "sync"})
	b := hostPackages(t, s)["demo"].Bindings["claude"]
	if b.Source != other || b.ID == "demo@team" || len(native.installed["claude"]) != 1 || native.installed["claude"][0].ID != b.ID {
		t.Fatalf("adopted removed native identity: %+v %+v", b, native.installed)
	}
}

// Ref-only drift is a computed view; the selected declaration and applied native
// identity remain separate even when refs have equal content or empty names.

func TestPluginHostStateRetriesReviewedUpdateAfterRollback(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	source := hostStatePiSource(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"pi"}})
	old := hostPackages(t, s)["demo"].Bindings["pi"]
	writePluginFile(t, source, "extensions/demo.ts", "export default () => { console.log('v2') }")
	request := Request{Action: "update", Name: "demo", Targets: []string{"pi"}}
	plan, err := s.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	updated := plan.Changes[0].Binding
	content := filepath.Join(s.snapshotPath(updated, "pi"), "content")
	run := s.Run
	failedAfterMaterialization := false
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if args[0] == "--version" {
			if actual, err := treeDigest(content); err == nil && actual == updated.Digest && actual != old.Digest {
				failedAfterMaterialization = true
				return nil, errors.New("post-materialization verification failure")
			}
		}
		return run(ctx, dir, env, bin, args...)
	}
	same := unchanged(t, s.ConfigPath)
	applyFailure(t, s, request)
	s.Run = run
	local := hostPackages(t, s)["demo"].Bindings["pi"]
	actual, err := treeDigest(content)
	if !failedAfterMaterialization || err != nil || actual != old.Digest || local.Digest != old.Digest || local.Pending != "update" {
		t.Fatalf("failure did not leave a rolled-back snapshot and reviewed pending update: %+v digest=%s err=%v", local, actual, err)
	}
	applyPluginRequest(t, s, Request{Action: "sync", Name: "demo"})
	local = hostPackages(t, s)["demo"].Bindings["pi"]
	if local.Pending != "" || local.Digest != updated.Digest || !s.ownedSnapshot(local, "pi") {
		t.Fatalf("retry lost reviewed update provenance: %+v", local)
	}
	same()
}

func TestPluginHostStateOrphanAndReplacementRetry(t *testing.T) {
	for _, mode := range []string{"orphan", "replacement"} {
		t.Run(mode, func(t *testing.T) {
			var ca, cb []string
			a := fakePiNpm(t, &ca)
			oldSource := hostStatePiSource(t)
			applyPluginRequest(t, a, Request{Action: "add", Source: oldSource, Targets: []string{"pi"}})
			raw, _ := os.ReadFile(a.ConfigPath)
			b := fakePiNpm(t, &cb)
			writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(raw))
			applyPluginRequest(t, b, Request{Action: "sync"})
			original := hostPackages(t, b)["demo"].Bindings["pi"]
			repairHost(t, a)
			removal := Request{Action: "remove", Name: "demo"}
			if mode == "replacement" {
				removal.Targets = []string{"pi"}
			}
			applyPluginRequest(t, a, removal)
			next := ""
			req := Request{Action: "sync"}
			if mode == "replacement" {
				var source string
				source, next = repairGitSource(t, "replacement")
				applyPluginRequest(t, a, Request{Action: "add", Name: "demo", Source: source, SourceRef: "main", Targets: []string{"pi"}})
				req = Request{Action: "update", Name: "demo", SourceRef: "next"}
			}
			raw, _ = os.ReadFile(a.ConfigPath)
			writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(raw))
			repairHost(t, b)
			p, err := b.Preview(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "replacement" && (len(p.Changes) != 2 || p.Changes[1].Binding.Commit != next || p.Changes[1].Binding.SourceRef != "next") {
				t.Fatalf("replacement reviewed wrong ref: %+v", p.Changes)
			}
			run := b.Run
			failRemove, failInstall := true, true
			b.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
				if len(args) > 1 && args[1] != "--help" && (args[0] == "remove" && failRemove || args[0] == "install" && failInstall) {
					return nil, errors.New("native failure")
				}
				return run(ctx, dir, env, bin, args...)
			}
			same := unchanged(t, b.ConfigPath)
			applyFailure(t, b, req)
			if mode == "orphan" {
				same()
			} else {
				same = unchanged(t, b.ConfigPath)
			} // Explicit ref update changes intent once; retries never do.
			if got := hostPackages(t, b)["demo"].Bindings["pi"]; got.ID != original.ID || !got.Cleanup {
				t.Fatal("failed cleanup lost original evidence")
			}
			failRemove = false
			if mode == "replacement" {
				applyFailure(t, b, Request{Action: "sync"})
				retry := previewChanges(t, b, Request{Action: "sync"})
				if len(retry) != 1 || retry[0].Action != "install" || retry[0].Binding.ID == original.ID || retry[0].Binding.Commit != next {
					t.Fatal("failed replacement lost requested intent")
				}
				same()
			}
			failInstall = false
			applyPluginRequest(t, b, Request{Action: "sync"})
			same()
			d := readState(t, b)
			if mode == "orphan" && len(d.packages) != 0 {
				t.Fatal("orphan receipt survived cleanup")
			}
			if mode == "replacement" {
				got := d.packages["demo"].Bindings["pi"]
				if got.ID == original.ID || got.Cleanup || got.Pending != "" || got.Commit != next || got.SourceRef != "next" {
					t.Fatalf("retry installed wrong ref or retained stale state: %+v", got)
				}
			}
		})
	}
}

func TestPluginHostStateNativeOnlyReimportDropsPreviousSource(t *testing.T) {
	var ac, bc []string
	a := fakePiNpm(t, &ac)
	source, _ := repairGitSource(t, "native-only")
	applyPluginRequest(t, a, Request{Action: "add", Source: source, SourceRef: "main", Targets: []string{"pi"}})
	original := hostPackages(t, a)["demo"].Bindings["pi"]
	shared, _ := os.ReadFile(a.ConfigPath)
	b := fakePiNpm(t, &bc)
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(shared))
	nativeDir := hostStatePiSource(t)
	nativeFile, _ := b.piSettingsPath("pi")
	relative, err := filepath.Rel(filepath.Dir(nativeFile), nativeDir)
	if err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, filepath.Dir(nativeFile), filepath.Base(nativeFile), `{"packages":[`+strconv.Quote(relative)+`]}`)
	// B has no source receipt; removal forgets intent while leaving its native item.
	applyPluginRequest(t, b, Request{Action: "remove", Name: "demo"})
	applyPluginRequest(t, b, Request{Action: "import", Name: "demo", From: "pi", Plugin: nativeDir})
	if decl := readState(t, b).intent["demo"].Bindings["pi"]; !decl.Imported || decl.Source != "" {
		t.Fatal("B did not produce native-only intent")
	}
	shared, _ = os.ReadFile(b.ConfigPath)
	writePluginFile(t, filepath.Dir(a.ConfigPath), filepath.Base(a.ConfigPath), string(shared))
	repairHost(t, a)
	if old := hostPackages(t, a)["demo"].Bindings["pi"]; !old.Cleanup || old.Source != source {
		t.Fatal("A lacks original source cleanup evidence")
	}
	nativeA, _ := a.piSettingsPath("pi")
	otherNative := hostStatePiSource(t)
	oldRelative, err := filepath.Rel(filepath.Dir(nativeA), original.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherRelative, err := filepath.Rel(filepath.Dir(nativeA), otherNative)
	if err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, filepath.Dir(nativeA), filepath.Base(nativeA), `{"packages":[`+strconv.Quote(oldRelative)+`,`+strconv.Quote(otherRelative)+`]}`)
	same := unchanged(t, a.ConfigPath, a.statePath(), nativeA)
	if _, err := a.Preview(context.Background(), Request{Action: "import", Name: "demo", From: "pi", Plugin: otherNative}); err == nil || !strings.Contains(err.Error(), "different plugin") {
		t.Fatalf("cleanup allowed a different native ID: %v", err)
	}
	same()
	same = unchanged(t, a.ConfigPath, nativeA)
	applyPluginRequest(t, a, Request{Action: "import", Name: "demo", From: "pi", Plugin: original.ID})
	got := hostPackages(t, a)["demo"].Bindings["pi"]
	if got.Source != "" || got.SourceRef != "" || got.Commit != "" || got.Digest != "" || got.Cleanup || got.ID != original.ID {
		t.Errorf("native-only reimport retained old source metadata: %+v", got)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := a.Preview(context.Background(), Request{Action: "update", Name: "demo", Targets: []string{"pi"}}); err != nil {
		t.Errorf("native-only update reacquired previous Git source: %v", err)
	}
	same()
}

// A shared recipe can change before this host reconciles its applied receipt.
