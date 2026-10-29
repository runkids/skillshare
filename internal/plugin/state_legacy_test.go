package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if got := hostPackages(t, b)["demo"].Bindings["pi"]; got.ID != "npm:demo@1.2.3" || got.Source != "" {
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
	if _, err := os.Stat(s.statePath()); err != nil {
		t.Fatalf("legacy read did not capture hints: %v", err)
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
	d.state.Bindings = map[string]map[string]localBinding{}
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

func TestPluginHostStateUnavailableLegacySurvivesIntentEdit(t *testing.T) {
	native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
	s := native.service(t)
	run := s.Run
	s.Run = func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return nil, errors.New("temporarily unavailable")
	}
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        claude: {id: demo@team}\n    other:\n      bindings:\n        claude: {id: other@team}\n")
	applyPluginRequest(t, s, Request{Action: "disable", Name: "other", Targets: []string{"claude"}})
	// A new Service instance must recover after the client becomes available.
	restarted := *s
	restarted.Run = run
	applyPluginRequest(t, &restarted, Request{Action: "sync", Name: "demo"})
	if got := hostPackages(t, &restarted)["demo"].Bindings["claude"].ID; got != "demo@team" {
		t.Fatalf("legacy ID lost during unrelated intent migration: %q", got)
	}
}

func TestPluginHostStateLegacyHintKeepsOriginalOwnership(t *testing.T) {
	for _, mode := range []string{"removed", "replaced", "context", "stale", "forget"} {
		t.Run(mode, func(t *testing.T) {
			native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
			s := native.service(t)
			run := s.Run
			s.Run = func(context.Context, string, []string, string, ...string) ([]byte, error) {
				return nil, errors.New("unavailable")
			}
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        claude: {id: demo@team}\n")
			applyPluginRequest(t, s, Request{Action: "disable", Name: "demo", Targets: []string{"claude"}})
			d := readState(t, s)
			if hostPackages(t, s)["demo"].Bindings["claude"].ID != "" {
				t.Fatal("hint became executable or lost provenance")
			}
			if mode == "forget" {
				applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
				s.Run = run
				if changes := previewChanges(t, s, Request{Action: "sync"}); len(changes) != 0 {
					t.Fatal("hint-only deletion was not persisted")
				}
				return
			}
			if mode == "removed" {
				delete(d.intent, "demo")
			} else if mode == "replaced" {
				p := d.intent["demo"]
				p.Bindings["claude"] = declaration{Source: fixture(t), Plugin: "demo"}
				d.intent["demo"] = p
			}
			if err := s.save(d); err != nil {
				t.Fatal(err)
			}
			if mode == "stale" {
				p, err := s.Preview(context.Background(), Request{Action: "sync"})
				if err != nil {
					t.Fatal(err)
				}
				if err := s.saveState(d); err != nil {
					t.Fatal(err)
				}
				raw, _ := os.ReadFile(s.statePath())
				_ = os.WriteFile(s.statePath(), append(raw, ' '), 0600)
				if _, err := s.Apply(context.Background(), Request{Action: "sync"}, p.Revision); err == nil {
					t.Fatal("changed hint state did not invalidate approval")
				}
				return
			}
			if mode == "context" {
				t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			}
			s.Run = run
			changes := previewChanges(t, s, Request{Action: "sync"})
			if mode == "context" {
				for _, c := range changes {
					if c.ID != "" {
						t.Fatal("hint crossed stores")
					}
				}
				return
			}
			if len(changes) == 0 || changes[0].Action != "remove" || changes[0].ID != "demo@team" {
				t.Fatalf("lost original cleanup ownership: %+v", changes)
			}
			applyPluginRequest(t, s, Request{Action: "sync"})
			for _, c := range previewChanges(t, s, Request{Action: "sync"}) {
				if c.Action != "noop" {
					t.Fatalf("cleanup did not finish: %+v", c)
				}
			}

		})
	}
}

func TestPluginHostStateLegacyHintNeedsLocalProof(t *testing.T) {
	for _, mode := range []string{"foreign", "missing", "modified", "private", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			var commands []string
			s := fakePiNpm(t, &commands)
			s.Accounts = map[string]Account{"pi-work": {Agent: "pi", Dir: t.TempDir()}}
			raw, err := os.ReadFile("testdata/legacy-host-state/config.yaml")
			if err != nil {
				t.Fatal(err)
			}
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), string(raw))
			d := readState(t, s)
			s.captureLegacy(d)
			local := d.state.Bindings["demo"]["pi"]
			b := local.Legacy.Binding
			snapshot := s.snapshotPath(b, "pi")
			if mode != "foreign" {
				b.ID = filepath.Join(snapshot, "content", "nested", "pkg")
			}
			if mode != "missing" {
				if err := copyTree("testdata/legacy-host-state/source", filepath.Join(snapshot, "content")); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{".skillshare-owner", ".skillshare-digest"} {
					body, err := os.ReadFile(filepath.Join("testdata/legacy-host-state", name))
					if err != nil {
						t.Fatal(err)
					}
					writePluginFile(t, snapshot, name, string(body))
				}
			}
			if mode == "modified" {
				writePluginFile(t, snapshot, "content/nested/pkg/index.js", "modified")
			}
			if mode == "private" {
				b.PiRegistration = strings.Repeat("a", 64)
			}
			local.Legacy.Binding = b
			if mode == "malformed" {
				local.Legacy.Action = "install"
			}
			d.putLocal("demo", "pi", local)
			if err := s.saveState(d); err != nil {
				t.Fatal(err)
			}
			d, err = s.load()
			if mode == "malformed" {
				if err == nil {
					t.Fatal("executable hint accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			same := unchanged(t, s.ConfigPath, s.statePath())
			s.adoptLegacy(d, func(string) Host { return Host{Installed: []Installed{{ID: b.ID}}} })
			same()
			local = d.state.Bindings["demo"]["pi"]
			if local.Applied != nil || local.Legacy == nil || d.packages["demo"].Bindings["pi"].ID != "" {
				t.Fatal("unverified hint became executable")
			}
		})
	}
}

func TestPluginHostStateLegacyHintSaveFailurePreventsMigration(t *testing.T) {
	native := &fakeAgents{}
	s := native.service(t)
	s.Run = func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return nil, errors.New("unavailable")
	}
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        claude: {id: demo@team}\n")
	req := Request{Action: "disable", Name: "demo", Targets: []string{"claude"}}
	p, err := s.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	same := unchanged(t, s.ConfigPath)
	// The first preview now persists hints. Make the state path unwritable before apply.
	if err := os.Remove(s.statePath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(s.statePath(), 0700); err != nil {
		t.Fatal(err)
	}
	unlock, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := s.applyPlan(context.Background(), req, p); err == nil {
		t.Fatal("migration ignored failed hint save")
	}
	same()
}

func TestLegacyPendingRemovalRetriesAfterMigration(t *testing.T) {
	for _, whole := range []bool{false, true} {
		name := "target"
		if whole {
			name = "package"
		}
		t.Run(name, func(t *testing.T) {
			native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}, "codex": {{ID: "demo@team", Installed: true}}}}
			s := native.service(t)
			codex := ""
			if !whole {
				codex = "        codex: {id: demo@team}\n"
			}
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        claude: {id: demo@team, pending: remove}\n"+codex+"    other:\n      bindings:\n        claude: {id: other@team}\n")
			// An unrelated declaration edit upgrades the file before the retry.
			applyPluginRequest(t, s, Request{Action: "disable", Name: "other", Targets: []string{"claude"}})
			raw, err := os.ReadFile(s.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := s.Preview(context.Background(), Request{Action: "sync", Name: "demo"})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, c := range plan.Changes {
				if c.Target == "claude" && c.Action == "remove" {
					found = true
				}
			}
			if !found {
				t.Fatalf("pending removal was lost: %+v", plan.Changes)
			}
			applyPluginRequest(t, s, Request{Action: "sync", Name: "demo"})
			if len(native.installed["claude"]) != 0 {
				t.Fatal("removed plugin remains installed")
			}
			if !whole && len(native.installed["codex"]) != 1 {
				t.Fatal("removed another target")
			}
			after, _ := os.ReadFile(s.ConfigPath)
			if string(after) != string(raw) {
				t.Fatal("retry rewrote shared declarations")
			}
		})
	}
}

func TestLegacyReadPersistsEvidenceBeforeRemoteMigration(t *testing.T) {
	for _, read := range []string{"packages", "preview"} {
		t.Run(read, func(t *testing.T) {
			native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
			s := native.service(t)
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  packages:\n    demo:\n      bindings:\n        claude: {id: demo@team}\n")
			same := unchanged(t, s.ConfigPath)
			if read == "packages" {
				if _, err := s.Packages(); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := s.Preview(context.Background(), Request{Action: "sync"}); err != nil {
					t.Fatal(err)
				}
			}
			same()
			// Another machine migrates the shared file before this host applies anything.
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  version: 2\n  packages:\n    demo:\n      bindings:\n        claude: {imported: true}\n")
			restarted := *s
			p, err := restarted.Preview(context.Background(), Request{Action: "sync"})
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Changes) != 1 || p.Changes[0].Action != "noop" || p.Changes[0].ID != "demo@team" {
				t.Fatalf("read lost local migration evidence: %+v", p.Changes)
			}
		})
	}
}

func TestSharedImportRequiredPlanHasLocalizedGuidance(t *testing.T) {
	s := (&fakeAgents{}).service(t)
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  version: 2\n  packages:\n    demo:\n      bindings:\n        claude: {imported: true}\n")
	p, err := s.Preview(context.Background(), Request{Action: "sync"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Changes) != 1 || p.Changes[0].MessageKey != "plugins.note.localImportRequired" {
		t.Fatalf("unlocalized plan: %+v", p.Changes)
	}
}
