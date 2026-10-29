package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginHostStateTwoHosts(t *testing.T) {
	source := t.TempDir()
	writePluginFile(t, source, "package.json", `{"name":"demo","keywords":["pi-package"]}`)
	writePluginFile(t, source, "extensions/demo.ts", "export default () => {}")
	var commandsA, commandsB []string
	a := fakePiNpm(t, &commandsA)
	a.Accounts = map[string]Account{"pi-work": {Agent: "pi", Dir: filepath.Join(t.TempDir(), "account")}}
	applyPluginRequest(t, a, Request{Action: "add", Source: source, Targets: []string{"pi"}})
	raw, _ := os.ReadFile(a.ConfigPath)
	b := fakePiNpm(t, &commandsB)
	b.Accounts = map[string]Account{"pi-work": {Agent: "pi", Dir: filepath.Join(t.TempDir(), "account")}}
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(raw))
	// B has already read the shared declaration when A selects another target.
	if _, err := b.Preview(context.Background(), Request{Action: "sync"}); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, a, Request{Action: "add", Source: source, Targets: []string{"pi-work"}})
	updated, _ := os.ReadFile(a.ConfigPath)
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(updated))
	before, _ := os.Stat(b.ConfigPath)
	run := b.Run
	b.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if args[0] == "install" && len(args) > 1 && args[1] != "--help" {
			return nil, errors.New("local Pi failure")
		}
		return run(ctx, dir, env, bin, args...)
	}
	req := Request{Action: "sync"}
	plan, err := b.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Blocked {
		t.Fatalf("unexpected blocked plan: %+v", plan.Changes)
	}
	if _, err = b.Apply(context.Background(), req, plan.Revision); err == nil {
		t.Fatal("expected native failure")
	}
	after, _ := os.ReadFile(b.ConfigPath)
	info, _ := os.Stat(b.ConfigPath)
	if string(after) != string(updated) || !info.ModTime().Equal(before.ModTime()) {
		t.Fatalf("sync wrote shared intent:\n%s", after)
	}
	if strings.Contains(string(after), "pending:") {
		t.Fatal("exported pending state")
	}
}

// seedLocalBinding builds a local receipt for tests of existing native adapters.
// It is separate from legacy migration: the test already owns this local install.

func TestPluginHostStateSourcePathsAndReadOnly(t *testing.T) {
	for _, scope := range []string{"global", "project", "account"} {
		t.Run(scope, func(t *testing.T) {
			var commands []string
			s := fakePiNpm(t, &commands)
			// Catalog candidates must retain their nested target-specific path.
			source := t.TempDir()
			writePluginFile(t, source, ".claude-plugin/marketplace.json", `{"name":"team","plugins":[{"name":"demo","source":"./nested/pkg"}]}`)
			writePluginFile(t, source, "nested/pkg/package.json", `{"name":"demo","keywords":["pi-package"],"main":"entry.js","dependencies":{"@opencode-ai/plugin":"1"}}`)
			writePluginFile(t, source, "nested/pkg/extensions/main.ts", "export default () => {}")
			writePluginFile(t, source, "nested/pkg/entry.js", "export default {}")
			target := "pi"
			if scope == "project" {
				s.ProjectRoot = t.TempDir()
			}
			if scope == "account" {
				target = "pi-work"
				s.Accounts = map[string]Account{target: {Agent: "pi", Dir: t.TempDir()}}
			}
			run := s.Run
			s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
				if bin == "opencode" && args[0] == "--version" {
					return []byte("1.9.0"), nil
				}
				return run(ctx, dir, env, bin, args...)
			}
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  version: 2\n  packages:\n    demo:\n      source: "+source+"\n      plugin: demo\n      bindings:\n        "+target+": {}\n        opencode: {}\n")
			original, _ := os.ReadFile(s.ConfigPath)
			inv, err := s.Packages()
			if err != nil {
				t.Fatal(err)
			}
			if inv.Packages["demo"].Bindings[target].ID != "" {
				t.Fatal("invented local ID")
			}
			for _, action := range []string{"sync", "check"} {
				if _, err := s.Preview(context.Background(), Request{Action: action}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.Files("demo"); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(s.StateDir); !os.IsNotExist(err) {
				t.Fatalf("read created state: %v", err)
			}
			applyPluginRequest(t, s, Request{Action: "sync"})
			d := readState(t, s)
			pi := d.packages["demo"].Bindings[target]
			oc := d.packages["demo"].Bindings["opencode"]
			if !strings.HasSuffix(pi.ID, filepath.FromSlash("content/nested/pkg")) || !strings.HasSuffix(oc.ID, "/content/nested/pkg/entry.js") {
				t.Fatalf("lost candidate suffix: %s %s", pi.ID, oc.ID)
			}
			if !strings.HasPrefix(pi.ID, s.managedRoot()) {
				t.Fatal("foreign Pi ID")
			}
			same := unchanged(t, s.ConfigPath, s.statePath())
			applyPluginRequest(t, s, Request{Action: "sync"})
			_, _ = s.Inventory(context.Background())
			_, _ = s.Files("demo")
			same()
			got, _ := os.ReadFile(s.ConfigPath)
			if string(got) != string(original) {
				t.Fatal("sync normalized declarations")
			}
		})
	}
}

func TestPluginHostStateImportsAreLocal(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	aNative := &fakeAgents{installed: map[string][]Installed{"omp": {{ID: "demo@air", Name: "demo", Version: "1", Installed: true, Scope: "user"}}}}
	a := aNative.service(t)
	applyPluginRequest(t, a, Request{Action: "import", From: "omp", Plugin: "demo@air", Name: "router"})
	raw, _ := os.ReadFile(a.ConfigPath)
	bNative := &fakeAgents{}
	b := bNative.service(t)
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(raw))
	p, err := b.Preview(context.Background(), Request{Action: "sync"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "skip" || !strings.Contains(p.Changes[0].Message, "Import required") {
		t.Fatalf("foreign import: %+v", p)
	}
	inv, _ := b.Packages()
	if binding := inv.Packages["router"].Bindings["omp"]; !binding.Imported || binding.ID != "" {
		t.Fatalf("foreign local identity: %+v", binding)
	}
	applyPluginRequest(t, b, Request{Action: "sync"})
	if len(bNative.commands) != 0 {
		t.Fatal(bNative.commands)
	}
	before := unchanged(t, b.ConfigPath)
	bNative.installed["omp"] = []Installed{{ID: "demo@umbrel", Name: "demo", Version: "2", Installed: true, Scope: "user"}}
	applyPluginRequest(t, b, Request{Action: "import", From: "omp", Plugin: "demo@umbrel", Name: "router"})
	before()
	if b := hostPackages(t, b)["router"].Bindings["omp"]; b.ID != "demo@umbrel" {
		t.Fatal(b)
	}
}

func TestPluginHostStateSourceAdoptionKeepsRecipe(t *testing.T) {
	native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true, Enabled: true}}}}
	s := native.service(t)
	source := fixture(t)
	writePluginFile(t, source, ".claude-plugin/marketplace.json", `{"name":"team","plugins":[{"name":"demo","source":"."}]}`)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"claude"}})
	d := readState(t, s)
	if d.intent["demo"].Bindings["claude"].Source != source || d.packages["demo"].Bindings["claude"].Source != "" {
		t.Fatal("adoption lost recipe or invented snapshot provenance")
	}
	raw, _ := os.ReadFile(s.ConfigPath)
	for _, test := range []struct {
		name, id string
		catalog  bool
	}{
		{"same-market", "demo@team", true},
		{"other-market", "demo@other", true},
		{"standalone", "demo@other", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !test.catalog {
				if err := os.Remove(filepath.Join(source, ".claude-plugin/marketplace.json")); err != nil {
					t.Fatal(err)
				}
			}
			otherNative := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: test.id, Scope: "user", Installed: true, Enabled: true}}}}
			other := otherNative.service(t)
			writePluginFile(t, filepath.Dir(other.ConfigPath), filepath.Base(other.ConfigPath), string(raw))
			same := unchanged(t, other.ConfigPath)
			collision, err := other.Preview(context.Background(), Request{Action: "sync"})
			if err != nil || !collision.Blocked || len(collision.Changes) != 1 || collision.Changes[0].Action != "blocked" || !strings.Contains(collision.Changes[0].Message, "native") {
				t.Fatalf("same-name native plugin must block reviewed source installation: %+v %v", collision, err)
			}
			if _, err := other.Apply(context.Background(), Request{Action: "sync"}, collision.Revision); err == nil || len(otherNative.commands) != 0 {
				t.Fatalf("blocked collision mutated native installation: %v %v", otherNative.commands, err)
			}
			// The user removes the independent plugin in its native client before retrying.
			otherNative.installed["claude"] = nil
			applyPluginRequest(t, other, Request{Action: "sync"})
			if got := hostPackages(t, other)["demo"].Bindings["claude"]; got.Source != source || got.Digest == "" || got.ID == test.id {
				t.Fatalf("new host lost review: %+v", got)
			}
			if got := hostPackages(t, other)["demo"].Version; got != "1.0.0" {
				t.Fatalf("first source sync lost reviewed default version: %q", got)
			}
			for _, command := range otherNative.commands {
				if strings.Contains(command, "uninstall "+test.id) || strings.Contains(command, "remove "+test.id) || strings.Contains(command, "marketplace remove ") {
					t.Fatalf("removed an independent native installation: %s", command)
				}
			}
			commands := len(otherNative.commands)
			applyPluginRequest(t, other, Request{Action: "sync"})
			if len(otherNative.commands) != commands {
				t.Fatalf("second sync repeated native work: %v", otherNative.commands)
			}
			same()
		})
	}
}

func TestPluginHostStateFirstNativeImportPreservesSourceIntent(t *testing.T) {
	for _, selected := range []bool{true, false} {
		t.Run(map[bool]string{true: "selected", false: "deselected"}[selected], func(t *testing.T) {
			var aCalls, bCalls []string
			a := fakePiNpm(t, &aCalls)
			source := hostStatePiSource(t)
			applyPluginRequest(t, a, Request{Action: "add", Source: source, Targets: []string{"pi"}})
			if !selected {
				applyPluginRequest(t, a, Request{Action: "disable", Name: "demo", Targets: []string{"pi"}})
			}
			shared, _ := os.ReadFile(a.ConfigPath)
			b := fakePiNpm(t, &bCalls)
			writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(shared))
			writePluginFile(t, filepath.Dir(b.ConfigPath), ".pi/agent/settings.json", `{"packages":["npm:demo"]}`)
			applyPluginRequest(t, b, Request{Action: "import", From: "pi", Plugin: "npm:demo", Name: "demo"})
			changed, _ := os.ReadFile(b.ConfigPath)
			if string(changed) != string(shared) {
				t.Fatal("local import rewrote shared source recipe")
			}
			before := len(bCalls)
			applyPluginRequest(t, b, Request{Action: "sync"})
			if selected && len(bCalls) != before {
				t.Fatal("sync replaced local import")
			}
			writePluginFile(t, filepath.Dir(a.ConfigPath), filepath.Base(a.ConfigPath), string(changed))
			repairHost(t, a)
			p, err := a.Preview(context.Background(), Request{Action: "sync"})
			if err != nil {
				t.Fatal(err)
			}
			want := "noop"
			if !selected {
				want = "uninstall"
			}
			if len(p.Changes) != 1 || p.Changes[0].Action != want {
				t.Fatalf("local import disrupted other host: %+v", p.Changes)
			}
		})
	}
}

func TestSharedSourceLifecycleAcrossHostsAndScopes(t *testing.T) {
	for _, tc := range []struct {
		target, manifest string
		project          bool
	}{
		{"codex", ".codex-plugin/plugin.json", false},
		{"cursor", ".cursor-plugin/plugin.json", false},
		{"antigravity", "plugin.json", false},
		{"claude", ".claude-plugin/plugin.json", true},
		{"antigravity", "plugin.json", true},
	} {
		scope := "global"
		if tc.project {
			scope = "project"
		}
		t.Run(tc.target+"/"+scope, func(t *testing.T) {
			source := t.TempDir()
			writePluginFile(t, source, tc.manifest, `{"name":"demo","version":"1.0.0"}`)
			writePluginFile(t, source, "skills/demo/SKILL.md", "hello")
			a, b := newHostFixture(t, tc.project), newHostFixture(t, tc.project)
			a.apply(Request{Action: "add", Source: source, Targets: []string{tc.target}})
			b.receive(a)
			p := b.preview(Request{Action: "sync"})
			if p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "install" {
				t.Fatalf("new host did not plan its own install: %+v", p)
			}
			same := unchanged(t, b.service.ConfigPath)
			b.apply(Request{Action: "sync"})
			same()
			p = b.preview(Request{Action: "sync"})
			if len(p.Changes) != 1 || p.Changes[0].Action != "noop" {
				t.Fatalf("repeat sync was not quiet: %+v", p)
			}
			a.apply(Request{Action: "remove", Name: "demo"})
			b.receive(a)
			p = b.preview(Request{Action: "sync"})
			if len(p.Changes) != 1 || p.Changes[0].Action != "remove" {
				t.Fatalf("shared removal lost local cleanup: %+v", p)
			}
			b.apply(Request{Action: "sync"})
			if p = b.preview(Request{Action: "sync"}); len(p.Changes) != 0 {
				t.Fatalf("cleanup did not finish: %+v", p)
			}
		})
	}
}
