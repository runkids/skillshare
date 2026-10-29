package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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
	for _, local := range []bool{false, true} {
		t.Run(map[bool]string{false: "foreign", true: "local"}[local], func(t *testing.T) {
			var commands []string
			s := fakePiNpm(t, &commands)
			s.Accounts = map[string]Account{"pi-work": {Agent: "pi", Dir: t.TempDir()}}
			raw, err := os.ReadFile("testdata/legacy-host-state/config.yaml")
			if err != nil {
				t.Fatal(err)
			}
			// Decode the producer's public v1 YAML, not today's private receipt layout.
			var legacy struct {
				Plugins struct {
					Packages map[string]Package `yaml:"packages"`
				} `yaml:"plugins"`
			}
			if err := yaml.Unmarshal(raw, &legacy); err != nil {
				t.Fatal(err)
			}
			pack := legacy.Plugins.Packages["demo"]
			binding := pack.Bindings["pi"]
			snapshot := s.snapshotPath(binding, "pi")
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
			binding.ID = "/foreign/content/nested/pkg"
			if local {
				binding.ID = filepath.Join(snapshot, "content", "nested", "pkg")
			}
			pack.Bindings["pi"] = binding
			legacy.Plugins.Packages["demo"] = pack
			raw, err = yaml.Marshal(legacy)
			if err != nil {
				t.Fatal(err)
			}
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), string(raw))
			file, _ := s.piSettingsPath("pi")
			writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"packages":["`+binding.ID+`"]}`)
			same := unchanged(t, s.ConfigPath)
			quick := hostPackages(t, s)
			if b := quick["demo"].Bindings["pi-work"]; b.Source != "__ALTERNATE__" {
				t.Fatalf("lost per-target distribution: %+v", b)
			}
			if b := quick["demo"].Bindings["opencode"]; b.Sync == nil || *b.Sync || b.SourceRef != "main" {
				t.Fatalf("lost selection/ref: %+v", b)
			}
			if b := quick["native"].Bindings["pi"]; !b.Imported {
				t.Fatal("native import inherited source")
			}
			for _, pack := range quick {
				for _, b := range pack.Bindings {
					if b.ID != "" || b.Pending != "" || b.PiRegistration != "" || b.Digest != "" {
						t.Fatalf("unproved receipt escaped: %+v", b)
					}
				}
			}
			full, err := s.Inventory(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			got := full.Packages["demo"].Bindings["pi"]
			if local {
				if got.ID != binding.ID || got.Digest != binding.Digest || got.Pending != "" {
					t.Fatalf("local proof rejected: %+v", got)
				}
			} else if got.ID != "" {
				t.Fatal("foreign ID adopted from copied snapshot markers")
			}
			same()
		})
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
			if mode == "replaced" {
				if len(changes) != 1 || changes[0].Action != "noop" || changes[0].ID != "demo@team" || !changes[0].Binding.Imported {
					t.Fatalf("replaced recipe changed native-only legacy ownership: %+v", changes)
				}
				before := len(native.commands)
				applyPluginRequest(t, s, Request{Action: "sync"})
				if len(native.commands) != before || len(native.installed["claude"]) != 1 {
					t.Fatal("recipe movement changed native installation")
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
