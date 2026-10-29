package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
)

// This fixture is produced by Preview/Apply on upstream e64fac67, using only
// fake native clients and scratch HOME/config/state. No hand-written native IDs.
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
func seedLocalBinding(t *testing.T, s *Service, name, target string, b Binding) {
	t.Helper()
	d, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	d.putLocal(name, target, localBinding{Applied: &receipt{Intent: d.intent[name].Bindings[target].fingerprint(), Store: s.storeIdentity(target), Binding: b}})
	if err := s.saveState(d); err != nil {
		t.Fatal(err)
	}
}

func hostStatePiSource(t *testing.T) string {
	t.Helper()
	source := t.TempDir()
	writePluginFile(t, source, "package.json", `{"name":"demo","keywords":["pi-package"]}`)
	writePluginFile(t, source, "extensions/demo.ts", "export default () => {}")
	return source
}
func readState(t *testing.T, s *Service) *document {
	t.Helper()
	d, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func applyFailure(t *testing.T, s *Service, r Request) *Result {
	t.Helper()
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blocked {
		t.Fatalf("blocked instead of attempted: %+v", p.Changes)
	}
	result, err := s.Apply(context.Background(), r, p.Revision)
	if err == nil {
		t.Fatalf("expected failure: %+v", result)
	}
	return result
}

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
	if b := readState(t, b).packages["router"].Bindings["omp"]; b.ID != "demo@umbrel" {
		t.Fatal(b)
	}
}

func TestPluginHostStateNpmRecipeAndLegacyAmbiguity(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	applyPluginRequest(t, s, Request{Action: "add", Source: "npm:demo@1.2.3", Targets: []string{"pi"}})
	raw, _ := os.ReadFile(s.ConfigPath)
	if strings.Contains(string(raw), "id:") || readState(t, s).intent["demo"].Bindings["pi"].Source != "npm:demo@1.2.3" {
		t.Fatalf("npm recipe: %s", raw)
	}
	b := fakePiNpm(t, &commands)
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(raw))
	applyPluginRequest(t, b, Request{Action: "sync"})
	if got := readState(t, b).packages["demo"].Bindings["pi"]; got.ID != "npm:demo@1.2.3" || got.Source != "" {
		t.Fatal(got)
	}
	legacy := []byte("plugins:\n  packages:\n    demo:\n      bindings:\n        pi:\n          id: npm:demo@1.2.3\n          pending: install\n")
	c := fakePiNpm(t, &commands)
	writePluginFile(t, filepath.Dir(c.ConfigPath), filepath.Base(c.ConfigPath), string(legacy))
	same := unchanged(t, c.ConfigPath)
	p, err := c.Preview(context.Background(), Request{Action: "sync"})
	if err != nil || p.Changes[0].Action != "skip" {
		t.Fatalf("ambiguous npm auto-installed: %+v %v", p, err)
	}
	applyPluginRequest(t, c, Request{Action: "sync"})
	same()
	applyPluginRequest(t, c, Request{Action: "add", Source: "npm:demo@1.2.3", Targets: []string{"pi"}})
	if readState(t, c).intent["demo"].Bindings["pi"].Source != "npm:demo@1.2.3" {
		t.Fatal("explicit re-add lost recipe")
	}
}

func TestPluginHostStateStaleApprovalAndCorruption(t *testing.T) {
	for _, change := range []string{"local", "account", "shared", "missing", "unknown", "corrupt"} {
		t.Run(change, func(t *testing.T) {
			var commands []string
			s := fakePiNpm(t, &commands)
			s.Accounts = map[string]Account{"work": {Agent: "pi", Dir: t.TempDir()}}
			applyPluginRequest(t, s, Request{Action: "add", Source: hostStatePiSource(t), Targets: []string{"work"}})
			req := Request{Action: "sync"}
			p, err := s.Preview(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "local":
				raw, _ := os.ReadFile(s.statePath())
				if err := os.WriteFile(s.statePath(), append(raw, ' '), 0600); err != nil {
					t.Fatal(err)
				}
			case "account":
				s.Accounts["work"] = Account{Agent: "pi", Dir: t.TempDir()}
			case "shared":
				raw, _ := os.ReadFile(s.ConfigPath)
				_ = os.WriteFile(s.ConfigPath, append(raw, '\n'), 0600)
			case "missing":
				_ = os.Remove(s.ConfigPath)
			case "unknown":
				_ = os.WriteFile(s.statePath(), []byte(`{"version":2,"bindings":{}}`), 0600)
			case "corrupt":
				_ = os.WriteFile(s.statePath(), []byte(`{broken`), 0600)
			}
			before := len(commands)
			if _, err := s.Apply(context.Background(), req, p.Revision); err == nil {
				t.Fatal("stale or corrupt state accepted")
			}
			if len(commands) != before {
				t.Fatal("stale state ran native command")
			}
		})
	}
}

func TestPluginHostStateMidApplyIntentEditStopsRemainingTargets(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	s.Accounts = map[string]Account{"work": {Agent: "pi", Dir: t.TempDir()}}
	source := hostStatePiSource(t)
	req := Request{Action: "add", Source: source, Targets: []string{"pi", "work"}}
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		raw, err := run(ctx, dir, env, bin, args...)
		if args[0] == "install" && args[1] != "--help" {
			cfg, _ := os.ReadFile(s.ConfigPath)
			_ = os.WriteFile(s.ConfigPath, append(cfg, []byte("# external intent edit\n")...), 0600)
		}
		return raw, err
	}
	result := applyFailure(t, s, req)
	if len(result.Results) != 1 || result.Results[0].Status != "installed" || len(commands) != 1 {
		t.Fatalf("continued after external edit: %+v %q", result, commands)
	}
	if local := readState(t, s).state.Bindings["demo"]["pi"]; local.Applied == nil || local.Pending != nil {
		t.Fatal("completed result not retained")
	}
}

func TestPluginHostStateLockIsLocal(t *testing.T) {
	s := &Service{ConfigPath: filepath.Join(t.TempDir(), "missing", "config.yaml"), StateDir: t.TempDir()}
	unlock, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := s.Lock(); err == nil {
		t.Fatal("same local state did not contend")
	}
	other := *s
	other.StateDir = t.TempDir()
	release, err := other.Lock()
	if err != nil {
		t.Fatal(err)
	}
	release()
	if _, err := os.Stat(filepath.Dir(s.ConfigPath)); !os.IsNotExist(err) {
		t.Fatal("local lock touched shared config directory")
	}
}

func TestPluginHostStateProducerLegacyProjection(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	s.Accounts = map[string]Account{"pi-work": {Agent: "pi", Dir: t.TempDir()}}
	raw, err := os.ReadFile("testdata/legacy-host-state/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	if err := copyTree("testdata/legacy-host-state/source", source); err != nil {
		t.Fatal(err)
	}
	// The fixture is foreign: shared native IDs, pending and private records must
	// not appear as this host's receipts, even when its snapshot was copied here.
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), string(raw))
	d := readState(t, s)
	if b := d.intent["demo"].Bindings["pi-work"]; b.Source != "__ALTERNATE__" {
		t.Fatalf("lost per-target distribution: %+v", b)
	}
	if b := d.intent["demo"].Bindings["opencode"]; b.Sync == nil || *b.Sync || b.SourceRef != "main" {
		t.Fatalf("lost selection/ref: %+v", b)
	}
	if b := d.intent["native"].Bindings["pi"]; !b.Imported {
		t.Fatal("native import inherited source")
	}
	for _, p := range d.packages {
		for _, b := range p.Bindings {
			if b.ID != "" || b.Pending != "" || b.PiRegistration != "" || b.Digest != "" {
				t.Fatalf("foreign receipt escaped: %+v", b)
			}
		}
	}
	same := unchanged(t, s.ConfigPath)
	if _, err := s.Packages(); err != nil {
		t.Fatal(err)
	}
	same()
	if _, err := os.Stat(s.StateDir); !os.IsNotExist(err) {
		t.Fatal("read created state")
	}
	// A local legacy source is adopted only with scoped inventory, exact local
	// identity, owned snapshot markers, and matching reviewed content.
	legacy := d.legacy["demo"].Bindings["pi"]
	legacy.ID = filepath.Join(s.snapshotPath(legacy, "pi"), "content", "nested", "pkg")
	snapshot := s.snapshotPath(legacy, "pi")
	if err := copyTree(source, filepath.Join(snapshot, "content")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".skillshare-owner", ".skillshare-digest"} {
		body, err := os.ReadFile(filepath.Join("testdata/legacy-host-state", name))
		if err != nil {
			t.Fatal(err)
		}
		writePluginFile(t, snapshot, name, string(body))
	}
	d.legacy = map[string]Package{"demo": {Bindings: map[string]Binding{"pi": legacy}}}
	host := func(string) Host { return Host{Installed: []Installed{{ID: legacy.ID}}} }
	s.adoptLegacy(d, host)
	if got := d.packages["demo"].Bindings["pi"]; got.ID != legacy.ID || got.Digest != legacy.Digest || got.Pending != "" {
		t.Fatalf("local proof rejected: %+v", got)
	}
	// A foreign Pi ID cannot acquire local ownership just because markers exist.
	foreign := legacy
	foreign.ID = "/foreign/content/nested/pkg"
	d.state.Bindings = map[string]map[string]localBinding{}
	d.legacy["demo"] = Package{Bindings: map[string]Binding{"pi": foreign}}
	s.adoptLegacy(d, func(string) Host { return Host{Installed: []Installed{{ID: foreign.ID}}} })
	if got := d.packages["demo"].Bindings["pi"]; got.ID != "" {
		t.Fatal("foreign native ID adopted from copied markers")
	}
}

func TestPluginHostStateDetachedAccountRetainsReceipt(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	s.Accounts = map[string]Account{"work": {Agent: "pi", Dir: t.TempDir()}}
	applyPluginRequest(t, s, Request{Action: "add", Source: hostStatePiSource(t), Targets: []string{"work"}})
	d := readState(t, s)
	pack := d.intent["demo"]
	delete(pack.Bindings, "work")
	d.intent["demo"] = pack
	if err := s.save(d); err != nil {
		t.Fatal(err)
	}
	s.Accounts = nil
	inv, err := s.Packages()
	if err != nil {
		t.Fatal(err)
	}
	b := inv.Packages["demo"].Bindings["work"]
	if !b.Cleanup || !b.ContextChanged || b.ID == "" {
		t.Fatalf("detached account lost cleanup evidence: %+v", b)
	}
	p, err := s.Preview(context.Background(), Request{Action: "sync"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Blocked {
		t.Fatal("detached account could execute native cleanup")
	}
	request := Request{Action: "update", All: true}
	p, err = s.Preview(context.Background(), request)
	if err != nil || p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "skip" || !p.Changes[0].batchError {
		t.Fatalf("--all did not isolate the detached account: %+v %v", p, err)
	}
	same := unchanged(t, s.ConfigPath, s.statePath())
	if _, err := s.Apply(context.Background(), request, p.Revision); err == nil {
		t.Fatal("--all did not report the detached account failure")
	}
	same()
}

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
	if got := readState(t, s).state.Bindings["demo"]["pi"]; got.Applied == nil || got.Pending != nil {
		t.Fatalf("verified pending receipt lost: %+v", got)
	}
	applyPluginRequest(t, s, Request{Action: "sync"})
	if len(commands) != 1 {
		t.Fatal("replayed verified install", commands)
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
			if got := readState(t, other).packages["demo"].Bindings["claude"]; got.Source != source || got.Digest == "" || got.ID == test.id {
				t.Fatalf("new host lost review: %+v", got)
			}
			if got := readState(t, other).packages["demo"].Version; got != "1.0.0" {
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

func TestPluginHostStateUpdateLeavesSharedIntentUntouched(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	source := hostStatePiSource(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"pi"}})
	old := readState(t, s).packages["demo"].Bindings["pi"]
	same := unchanged(t, s.ConfigPath)
	writePluginFile(t, source, "extensions/demo.ts", "export default () => { return 1 }")
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo"})
	same()
	if b := readState(t, s).packages["demo"].Bindings["pi"]; b.Digest == old.Digest || b.Pending != "" {
		t.Fatal("local update lost receipt")
	}
}

func TestPluginHostStateIntentEditBetweenNativeSteps(t *testing.T) {
	native := &fakeAgents{}
	s := native.service(t)
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		out, err := run(ctx, dir, env, bin, args...)
		if len(args) > 2 && args[0] == "plugin" && args[1] == "marketplace" && args[2] == "add" {
			raw, _ := os.ReadFile(s.ConfigPath)
			_ = os.WriteFile(s.ConfigPath, append(raw, []byte("# external intent edit\n")...), 0600)
		}
		return out, err
	}
	r := Request{Action: "add", Source: fixture(t), Targets: []string{"claude"}}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Apply(context.Background(), r, p.Revision)
	if err == nil || slices.ContainsFunc(native.commands, func(c string) bool { return strings.Contains(c, "plugin install") }) {
		t.Fatalf("continued native install after changed intent: %q %v", native.commands, err)
	}
}

func TestPluginDeclarationSchemas(t *testing.T) {
	for _, file := range []string{"config.schema.json", "project-config.schema.json"} {
		t.Run(file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", file))
			if err != nil {
				t.Fatal(err)
			}
			var schema struct {
				Properties map[string]json.RawMessage `json:"properties"`
			}
			if err := json.Unmarshal(raw, &schema); err != nil {
				t.Fatal(err)
			}
			var plugins jsonschema.Schema
			if err := json.Unmarshal(schema.Properties["plugins"], &plugins); err != nil {
				t.Fatal(err)
			}
			resolved, err := plugins.Resolve(nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, sample := range []struct {
				raw   string
				valid bool
			}{
				{`{"version":2,"packages":{"demo":{"source":"https://example.invalid/demo.git","bindings":{"pi":{},"omp":{"imported":true}}}}}`, true},
				{`{"packages":{"demo":{"bindings":{"pi":{"id":"npm:demo"}}}}}`, true},
				{`{"version":2,"packages":{"demo":{"bindings":{"pi":{"id":"foreign"}}}}}`, false},
			} {
				var value any
				_ = json.Unmarshal([]byte(sample.raw), &value)
				if err := resolved.Validate(value); (err == nil) != sample.valid {
					t.Fatalf("validation %s: %v", sample.raw, err)
				}
			}
		})
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

func TestPluginHostStateSourceRefIsIntentButCommitsAreLocal(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	source := hostStatePiSource(t)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = source
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	git("init", "-b", "main")
	git("add", ".")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "Fixture")
	git("branch", "next")
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	writePluginFile(t, filepath.Dir(cfg), filepath.Base(cfg), "[url \"file://"+source+"\"]\n  insteadOf = https://fixture.invalid/state.git\n")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	applyPluginRequest(t, s, Request{Action: "add", Source: "https://fixture.invalid/state.git", SourceRef: "main", Targets: []string{"pi"}})
	aCommit := readState(t, s).packages["demo"].Bindings["pi"].Commit
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo", SourceRef: "next"})
	raw, _ := os.ReadFile(s.ConfigPath)
	if readState(t, s).intent["demo"].Bindings["pi"].SourceRef != "next" || strings.Contains(string(raw), "commit:") {
		t.Fatal("ref and receipt ownership confused")
	}
	same := unchanged(t, s.ConfigPath)
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo", SourceRef: "next"})
	same()
	git("checkout", "next")
	writePluginFile(t, source, "extensions/demo.ts", "export default () => 2")
	git("add", ".")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "Second")
	b := fakePiNpm(t, &commands)
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(raw))
	applyPluginRequest(t, b, Request{Action: "sync"})
	if got := readState(t, b).packages["demo"].Bindings["pi"].Commit; got == aCommit || got == "" {
		t.Fatal("host B copied reviewed commit from A")
	}
	if got := readState(t, s).state.Bindings["demo"]["pi"].Applied.Binding.Commit; got != aCommit {
		t.Fatal("B update changed A receipt")
	}
}

func TestPluginHostStateNativeErrorKeepsCause(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	cause := errors.New("disk full")
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if args[0] == "install" && args[1] != "--help" {
			return nil, cause
		}
		return run(ctx, dir, env, bin, args...)
	}
	r := Request{Action: "add", Source: "npm:demo", Targets: []string{"pi"}}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Apply(context.Background(), r, p.Revision)
	if !errors.Is(err, cause) || strings.Contains(err.Error(), "project packages require trust") {
		t.Fatalf("misleading native failure: %v", err)
	}
}

func TestPluginHostStateResultWriteFailureStopsNativeWork(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	s.Accounts = map[string]Account{"work": {Agent: "pi", Dir: t.TempDir()}}
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		out, err := run(ctx, dir, env, bin, args...)
		if args[0] == "install" && args[1] != "--help" {
			if err := os.Rename(s.statePath(), s.statePath()+".pending"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(s.statePath(), 0700); err != nil {
				t.Fatal(err)
			}
		}
		return out, err
	}
	result := applyFailure(t, s, Request{Action: "add", Source: hostStatePiSource(t), Targets: []string{"pi", "work"}})
	if len(commands) != 1 || len(result.Results) != 1 || !strings.Contains(result.Results[0].Message, "recording its result failed") {
		t.Fatalf("continued after result write failure: %+v %q", result, commands)
	}
	if err := os.Remove(s.statePath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(s.statePath()+".pending", s.statePath()); err != nil {
		t.Fatal(err)
	}
	if got := readState(t, s).state.Bindings["demo"]["pi"]; got.Pending == nil {
		t.Fatal("durable pending lost")
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
	b := readState(t, s).packages["demo"].Bindings["claude"]
	if b.Source != other || b.ID == "demo@team" || len(native.installed["claude"]) != 1 || native.installed["claude"][0].ID != b.ID {
		t.Fatalf("adopted removed native identity: %+v %+v", b, native.installed)
	}
}

// Ref-only drift is a computed view; the selected declaration and applied native
// identity remain separate even when refs have equal content or empty names.
func FuzzPluginHostStateRefProjection(f *testing.F) {
	f.Setenv("HOME", f.TempDir())
	f.Setenv("PI_CODING_AGENT_DIR", f.TempDir())
	f.Add("main", "next", "native-version", uint8(0))
	f.Add("", "", "", uint8(1))
	f.Add("next", "", "pending", uint8(2))
	s := &Service{ConfigPath: filepath.Join(f.TempDir(), "config.yaml"), StateDir: f.TempDir()}
	f.Fuzz(func(t *testing.T, appliedRef, desiredRef, metadata string, mode uint8) {
		selected := mode%3 != 1
		var sync *bool
		if mode%3 != 0 {
			sync = &selected
		}
		source := "https://fixture.invalid/source.git"
		decl := declaration{Source: source, SourceRef: desiredRef, Plugin: "demo", Sync: sync}
		b := Binding{Source: source, SourceRef: appliedRef, Plugin: "demo", Version: metadata, Digest: hash([]byte(metadata)), Commit: hash([]byte(metadata))[:40]}
		b.ID = filepath.Join(s.snapshotPath(b, "pi"), "content")
		switch mode % 5 {
		case 1:
			decl.Source += "-replacement"
		case 2:
			decl = declaration{Imported: true, Sync: sync}
		case 3:
			decl.Entry = "nested"
		case 4:
			decl.Plugin = "other"
		}
		d := &document{intent: map[string]packageDeclaration{"demo": {Bindings: map[string]declaration{"pi": decl}}}, state: localState{Version: 1, Bindings: map[string]map[string]localBinding{"demo": {"pi": {Applied: &receipt{Intent: declareBinding(b).fingerprint(), Store: s.storeIdentity("pi"), Binding: b}}}}}}
		s.compose(d)
		got := d.packages["demo"].Bindings["pi"]
		raw, _ := json.Marshal(got)
		var view struct {
			RefChanged bool `json:"refChanged"`
		}
		if err := json.Unmarshal(raw, &view); err != nil {
			t.Fatal(err)
		}
		want := selected && mode%5 == 0 && appliedRef != desiredRef
		if view.RefChanged != want || got.ID != b.ID || got.SourceRef != appliedRef {
			t.Fatalf("projection violated applied identity / desired ref: drift=%v want=%v", view.RefChanged, want)
		}
	})
}

// Local metadata can be arbitrary JSON strings. Neither saving it nor projecting
// it through Packages may rewrite shared bytes; an explicit save retains intent.
func FuzzPluginHostStateSharedIntentIsolation(f *testing.F) {
	f.Setenv("HOME", f.TempDir())
	f.Setenv("PI_CODING_AGENT_DIR", f.TempDir())
	f.Add("branch", "native-path", uint8(0))
	f.Add("", "\x00pending\n", uint8(1))
	f.Fuzz(func(t *testing.T, ref, metadata string, selection uint8) {
		// Bound materialized strings and files, without excluding any input byte class.
		if len(ref) > 4096 {
			ref = ref[:4096]
		}
		if len(metadata) > 4096 {
			metadata = metadata[:4096]
		}
		selected := selection%3 != 1
		decl := declaration{Source: "https://fixture.invalid/source.git", SourceRef: ref, Plugin: "demo"}
		if selection%3 != 0 {
			decl.Sync = &selected
		}
		root := t.TempDir()
		s := &Service{ConfigPath: filepath.Join(root, "config.yaml"), StateDir: filepath.Join(root, "state")}
		intent := map[string]packageDeclaration{"demo": {Bindings: map[string]declaration{"pi": decl}}}
		before, err := yaml.Marshal(intent)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := yaml.Marshal(map[string]any{"unrelated": "kept", "plugins": map[string]any{"version": 2, "packages": intent}})
		if err != nil {
			t.Fatal(err)
		}
		writePluginFile(t, root, "config.yaml", string(raw))
		d := readState(t, s)
		b := Binding{Source: decl.Source, SourceRef: ref, Plugin: "demo", Version: metadata, Digest: hash([]byte(metadata)), Commit: hash([]byte(metadata))[:40]}
		b.ID = filepath.Join(s.snapshotPath(b, "pi"), "content")
		r := &receipt{Intent: declareBinding(b).fingerprint(), Store: s.storeIdentity("pi"), Binding: b}
		local := localBinding{Applied: r}
		switch selection / 3 % 3 {
		case 1:
			r.Action = "install"
			local = localBinding{Pending: r}
		case 2:
			local = localBinding{Legacy: r}
		}
		d.putLocal("demo", "pi", local)
		same := unchanged(t, s.ConfigPath)
		if err := s.saveState(d); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Packages(); err != nil {
			t.Fatal(err)
		}
		same()
		// Exercise the declaration writer with composed runtime metadata present.
		d = readState(t, s)
		if err := s.save(d); err != nil {
			t.Fatal(err)
		}
		after, err := yaml.Marshal(readState(t, s).intent)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatal("local metadata changed shared declaration roundtrip")
		}
	})
}

func TestPluginHostStateRetriesReviewedUpdateAfterRollback(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	source := hostStatePiSource(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"pi"}})
	old := readState(t, s).state.Bindings["demo"]["pi"].Applied.Binding
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
	local := readState(t, s).state.Bindings["demo"]["pi"]
	actual, err := treeDigest(content)
	if !failedAfterMaterialization || err != nil || actual != old.Digest || local.Applied == nil || local.Applied.Binding.Digest != old.Digest || local.Pending == nil || local.Pending.Action != "update" || local.Pending.Binding.Digest != updated.Digest {
		t.Fatalf("failure did not leave a rolled-back snapshot and reviewed pending update: %+v digest=%s err=%v", local, actual, err)
	}
	applyPluginRequest(t, s, Request{Action: "sync", Name: "demo"})
	local = readState(t, s).state.Bindings["demo"]["pi"]
	if local.Pending != nil || local.Applied == nil || local.Applied.Binding.Digest != updated.Digest || !s.ownedSnapshot(local.Applied.Binding, "pi") {
		t.Fatalf("retry lost reviewed update provenance: %+v", local)
	}
	same()
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
			writePluginFile(t, filepath.Dir(b.ConfigPath), ".pi/agent/settings.json", `{"packages":["npm:other"]}`)
			applyPluginRequest(t, b, Request{Action: "import", From: "pi", Plugin: "npm:other", Name: "demo"})
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
