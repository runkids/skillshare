package plugin

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPluginHostStateRebindChangedAccountBlocked(t *testing.T) {
	for _, tc := range []struct{ kind, change string }{
		{"source", "directory"}, {"source", "existing-id"}, {"source", "cli"},
		{"npm", "directory"}, {"npm", "cli"}, {"npm", "filtered"},
		{"import", "directory"}, {"import", "cli"}, {"import", "filtered"},
	} {
		t.Run(tc.kind+"/"+tc.change, func(t *testing.T) {
			var commands []string
			s := fakePiNpm(t, &commands)
			originalAccount := Account{Agent: "pi", Dir: t.TempDir()}
			s.Accounts = map[string]Account{"work": originalAccount}
			r := Request{Action: "add", Source: "npm:demo", Targets: []string{"work"}}
			if tc.kind == "source" || tc.kind == "npm" && tc.change == "filtered" {
				r.Source = hostStatePiSource(t)
			}
			if tc.kind == "import" {
				file, _ := s.piSettingsPath("work")
				writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"packages":["npm:demo"]}`)
				r = Request{Action: "import", From: "work", Plugin: "npm:demo", Name: "demo"}
			}
			applyPluginRequest(t, s, r)
			original := readState(t, s).state.Bindings["demo"]["work"].Applied.Store
			if tc.kind == "npm" && tc.change == "filtered" {
				d := readState(t, s)
				pack := d.intent["demo"]
				pack.Bindings["work"] = declaration{Source: "npm:demo"}
				d.intent["demo"] = pack
				if err := s.save(d); err != nil {
					t.Fatal(err)
				}
				r.Source = "npm:demo"
			}
			changed := originalAccount
			if tc.change == "cli" {
				changed.CLI = "pi-fork"
			} else {
				changed.Dir = t.TempDir()
			}
			s.Accounts["work"] = changed
			file, _ := s.piSettingsPath("work")
			if tc.kind == "import" || tc.change == "existing-id" {
				b := readState(t, s).packages["demo"].Bindings["work"]
				writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"packages":[`+strconv.Quote(b.ID)+`]}`)
			}
			if tc.change == "filtered" {
				writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"packages":[{"source":"npm:demo","extensions":["-private.ts"]}]}`)
			}
			same := unchanged(t, s.ConfigPath, s.statePath())
			before := len(commands)
			p, err := s.Preview(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			if !p.Blocked {
				// On dacbba6e this Apply proves the original applied store is overwritten.
				if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
					t.Fatal(err)
				}
				if got := readState(t, s).state.Bindings["demo"]["work"].Applied.Store; got != original {
					t.Fatal("changed account overwrote original applied store")
				}
				t.Fatal("changed account add/import was not blocked")
			}
			if !strings.Contains(p.Changes[0].Message, "Native account or scope changed") {
				t.Fatalf("context guard ran after registration preparation: %+v", p.Changes)
			}
			if _, err := s.Apply(context.Background(), r, p.Revision); err == nil {
				t.Fatal("applied blocked account change")
			}
			same()
			if len(commands) != before {
				t.Fatal("blocked account ran native command")
			}
			s.Accounts["work"] = originalAccount
			if tc.change == "filtered" && tc.kind == "npm" {
				applyPluginRequest(t, s, Request{Action: "sync", Name: "demo"})
			} else {
				applyPluginRequest(t, s, r)
			}
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
	if got := readState(t, &restarted).packages["demo"].Bindings["claude"].ID; got != "demo@team" {
		t.Fatalf("legacy ID lost during unrelated intent migration: %q", got)
	}
}

func TestPluginHostStateDisabledRefChangeUninstalls(t *testing.T) {
	var ca, cb []string
	a := fakePiNpm(t, &ca)
	source, _ := repairGitSource(t, "selected")
	applyPluginRequest(t, a, Request{Action: "add", Source: source, SourceRef: "main", Targets: []string{"pi"}})
	raw, _ := os.ReadFile(a.ConfigPath)
	b := fakePiNpm(t, &cb)
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(raw))
	applyPluginRequest(t, b, Request{Action: "sync"})
	repairHost(t, a)
	applyPluginRequest(t, a, Request{Action: "update", Name: "demo", SourceRef: "next"})
	applyPluginRequest(t, a, Request{Action: "disable", Name: "demo", Targets: []string{"pi"}})
	raw, _ = os.ReadFile(a.ConfigPath)
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(raw))
	repairHost(t, b)
	p, err := b.Preview(context.Background(), Request{Action: "sync"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Changes) != 1 || p.Changes[0].Action != "uninstall" {
		t.Fatalf("disabled ref change diverted uninstall: %+v", p.Changes)
	}
	same := unchanged(t, b.ConfigPath)
	applyPluginRequest(t, b, Request{Action: "sync"})
	same()
	before := len(cb)
	applyPluginRequest(t, b, Request{Action: "sync"})
	same()
	if len(cb) != before || len(b.host(context.Background(), "pi").Installed) != 0 {
		t.Fatal("deselected sync was not quiet")
	}
}

func repairHost(t *testing.T, s *Service) {
	t.Helper()
	t.Setenv("HOME", filepath.Dir(s.ConfigPath))
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(filepath.Dir(s.ConfigPath), ".pi/agent"))
}

func repairGitSource(t *testing.T, name string) (string, string) {
	source, next, _ := repairGitSourceRepo(t, name)
	return source, next
}
func repairGitSourceRepo(t *testing.T, name string) (string, string, string) {
	t.Helper()
	src := hostStatePiSource(t)
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = src
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git: %s %v", out, err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-b", "main")
	git("add", ".")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "one")
	git("checkout", "-b", "next")
	writePluginFile(t, src, "extensions/demo.ts", "export default () => 2")
	git("add", ".")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "two")
	url := "https://fixture.invalid/" + name + ".git"
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	writePluginFile(t, filepath.Dir(cfg), filepath.Base(cfg), "[url \"file://"+src+"\"]\n  insteadOf = "+url+"\n")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	return url, git("rev-parse", "next"), src
}

func TestReviewNewHostUpdateRef(t *testing.T) {
	var commands []string
	a := fakePiNpm(t, &commands)
	source, next := repairGitSource(t, "fresh")
	applyPluginRequest(t, a, Request{Action: "add", Source: source, SourceRef: "main", Targets: []string{"pi"}})
	raw, _ := os.ReadFile(a.ConfigPath)
	b := fakePiNpm(t, &commands)
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(raw))
	req := Request{Action: "update", Name: "demo", SourceRef: "next"}
	p, err := b.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Changes) != 1 || p.Changes[0].Binding.SourceRef != "next" || p.Changes[0].Binding.Commit != next {
		t.Fatalf("preview approved the wrong ref: %+v", p.Changes)
	}
	if _, err := b.Apply(context.Background(), req, p.Revision); err != nil {
		t.Fatal(err)
	}
	d := readState(t, b)
	binding := d.packages["demo"].Bindings["pi"]
	if binding.SourceRef != "next" || binding.Commit != next || d.state.Bindings["demo"]["pi"].Applied.Intent != d.intent["demo"].Bindings["pi"].fingerprint() {
		t.Fatal("installed receipt disagrees with requested ref")
	}
}

func TestPluginHostStateRepeatAddPreservesSelection(t *testing.T) {
	for _, kind := range []string{"source", "npm"} {
		for _, receipt := range []string{"present", "missing", "adopt"} {
			if kind == "source" && receipt == "adopt" {
				continue
			}
			t.Run(kind+"/"+receipt, func(t *testing.T) {
				var commands []string
				s := fakePiNpm(t, &commands)
				source := "npm:demo"
				if kind == "source" {
					source = hostStatePiSource(t)
				}
				req := Request{Action: "add", Source: source, Targets: []string{"pi"}}
				applyPluginRequest(t, s, req)
				applyPluginRequest(t, s, Request{Action: "disable", Name: "demo", Targets: []string{"pi"}})
				var other *Service
				var otherCommands []string
				if receipt == "present" {
					shared, _ := os.ReadFile(s.ConfigPath)
					other = fakePiNpm(t, &otherCommands)
					writePluginFile(t, filepath.Dir(other.ConfigPath), filepath.Base(other.ConfigPath), string(shared))
					p, err := other.Preview(context.Background(), Request{Action: "sync"})
					if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "noop" {
						t.Fatalf("disabled on second host: %+v %v", p, err)
					}
					repairHost(t, s)
				}
				if receipt != "present" {
					if err := os.Remove(s.statePath()); err != nil {
						t.Fatal(err)
					}
					if receipt == "missing" {
						file, _ := s.piSettingsPath("pi")
						writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"packages":[]}`)
					}
				}
				applyPluginRequest(t, s, req)
				if b := readState(t, s).intent["demo"].Bindings["pi"]; b.Sync == nil || *b.Sync {
					t.Fatal("repeat add changed shared selection")
				}
				if other != nil {
					shared, _ := os.ReadFile(s.ConfigPath)
					writePluginFile(t, filepath.Dir(other.ConfigPath), filepath.Base(other.ConfigPath), string(shared))
					repairHost(t, other)
					applyPluginRequest(t, other, Request{Action: "sync"})
					if len(otherCommands) != 0 {
						t.Fatal("repeat add made another host install a disabled target")
					}
				}
			})
		}
	}
	native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
	s := native.service(t)
	source := fixture(t)
	writePluginFile(t, source, ".claude-plugin/marketplace.json", `{"name":"team","plugins":[{"name":"demo","source":"."}]}`)
	req := Request{Action: "add", Source: source, Targets: []string{"claude"}}
	applyPluginRequest(t, s, req)
	applyPluginRequest(t, s, Request{Action: "disable", Name: "demo", Targets: []string{"claude"}})
	applyPluginRequest(t, s, req)
	if b := readState(t, s).intent["demo"].Bindings["claude"]; b.Sync == nil || *b.Sync {
		t.Fatal("adoption changed selection")
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
			original := readState(t, s).intent["demo"].Bindings["claude"].fingerprint()
			applyPluginRequest(t, s, Request{Action: "disable", Name: "demo", Targets: []string{"claude"}})
			d := readState(t, s)
			hint := d.state.Bindings["demo"]["claude"].Legacy
			if hint == nil || hint.Intent != original || d.packages["demo"].Bindings["claude"].ID != "" {
				t.Fatal("hint became executable or lost provenance")
			}
			if mode == "forget" {
				applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
				if len(readState(t, s).state.Bindings) != 0 {
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
			d = readState(t, s)
			same := unchanged(t, s.ConfigPath, s.statePath())
			s.adoptLegacy(d, func(target string) Host { return s.host(context.Background(), target) })
			same()
			local := d.state.Bindings["demo"]["claude"]
			if mode == "context" {
				if local.Applied != nil || local.Legacy == nil {
					t.Fatal("hint crossed stores")
				}
				return
			}
			if local.Applied == nil || local.Applied.Intent != original || local.Legacy != nil || !d.packages["demo"].Bindings["claude"].Cleanup {
				t.Fatal("proof used today's intent instead of original ownership")
			}
			applyPluginRequest(t, s, Request{Action: "sync"})
			if local := readState(t, s).state.Bindings["demo"]["claude"]; local.Legacy != nil {
				t.Fatal("verified promotion retained hint")
			}
		})
	}
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
			original := readState(t, b).state.Bindings["demo"]["pi"].Applied
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
			if got := readState(t, b).state.Bindings["demo"]["pi"]; got.Applied == nil || got.Applied.Intent != original.Intent || got.Applied.Binding.ID != original.Binding.ID {
				t.Fatal("failed cleanup lost original evidence")
			}
			failRemove = false
			if mode == "replacement" {
				applyFailure(t, b, Request{Action: "sync"})
				d := readState(t, b)
				local := d.state.Bindings["demo"]["pi"]
				if local.Applied != nil || local.Pending == nil || local.Pending.Binding.ID == original.Binding.ID || local.Pending.Binding.Commit != next || local.Pending.Intent != d.intent["demo"].Bindings["pi"].fingerprint() {
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
				if got.ID == original.Binding.ID || got.Cleanup || got.Pending != "" || got.Commit != next || got.SourceRef != "next" {
					t.Fatalf("retry installed wrong ref or retained stale state: %+v", got)
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

func TestPluginHostStateUnsupportedUpdateRefHasNoEffects(t *testing.T) {
	for _, kind := range []string{"import", "npm"} {
		t.Run(kind, func(t *testing.T) {
			var s *Service
			var count func() int
			if kind == "import" {
				native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
				s = native.service(t)
				native.market("claude")["team"] = t.TempDir()
				count = func() int { return len(native.commands) }
				applyPluginRequest(t, s, Request{Action: "import", From: "claude", Plugin: "demo@team", Name: "demo"})
			} else {
				var commands []string
				s = fakePiNpm(t, &commands)
				count = func() int { return len(commands) }
				applyPluginRequest(t, s, Request{Action: "add", Source: "npm:demo@next", Targets: []string{"pi"}})
			}
			same := unchanged(t, s.ConfigPath, s.statePath())
			before := count()
			r := Request{Action: "update", Name: "demo", SourceRef: "branch"}
			p, err := s.Preview(context.Background(), r)
			if err == nil {
				if !p.Blocked {
					_, _ = s.Apply(context.Background(), r, p.Revision)
					if _, err := s.load(); err != nil {
						t.Fatalf("unsupported ref wrote invalid declaration: %v", err)
					}
				}
				t.Fatal("unsupported source ref accepted")
			}
			same()
			if count() != before {
				t.Fatal("unsupported ref changed native installation")
			}
		})
	}
}

func TestPluginHostStateRefDriftIsLocalReconciliation(t *testing.T) {
	var ac, bc []string
	a := fakePiNpm(t, &ac)
	source, next := repairGitSource(t, "ref-drift")
	applyPluginRequest(t, a, Request{Action: "add", Source: source, SourceRef: "main", Targets: []string{"pi"}})
	run := a.Run
	a.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if filepath.Base(bin) != "pi" {
			return nil, errors.New("fixture target unavailable")
		}
		return run(ctx, dir, env, bin, args...)
	}
	old := readState(t, a).packages["demo"].Bindings["pi"]
	shared, _ := os.ReadFile(a.ConfigPath)
	b := fakePiNpm(t, &bc)
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(shared))
	applyPluginRequest(t, b, Request{Action: "update", Name: "demo", SourceRef: "next"})
	share := func() {
		t.Helper()
		raw, _ := os.ReadFile(b.ConfigPath)
		writePluginFile(t, filepath.Dir(a.ConfigPath), filepath.Base(a.ConfigPath), string(raw))
		repairHost(t, a)
	}
	share()
	assertDrift := func(want bool) {
		t.Helper()
		quick, err := a.Packages()
		if err != nil {
			t.Fatal(err)
		}
		full, err := a.Inventory(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for _, inv := range []*Inventory{quick, full} {
			if got := inv.Packages["demo"].Bindings["pi"].RefChanged; got != want {
				t.Fatalf("ref drift=%v want %v", got, want)
			}
		}
	}
	assertDrift(true)
	if got := readState(t, a).packages["demo"].Bindings["pi"]; got.SourceRef != "main" || got.ID != old.ID {
		t.Fatal("projection replaced applied identity")
	}
	same := unchanged(t, a.ConfigPath)
	entry := filepath.Join(old.ID, "extensions", "demo.ts")
	original, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte("local edit"), 0600); err != nil {
		t.Fatal(err)
	}
	applyFailure(t, a, Request{Action: "sync"})
	assertDrift(true)
	pendingRaw, _ := os.ReadFile(a.statePath())
	if strings.Contains(string(pendingRaw), "refChanged") {
		t.Fatal("persisted computed drift in pending receipt")
	}
	if err := os.WriteFile(entry, original, 0600); err != nil {
		t.Fatal(err)
	}

	applyPluginRequest(t, a, Request{Action: "sync"})
	same()
	assertDrift(false)
	got := readState(t, a).packages["demo"].Bindings["pi"]
	if got.SourceRef != "next" || got.Commit != next {
		t.Fatalf("wrong local ref: %+v", got)
	}
	raw, _ := os.ReadFile(a.statePath())
	if strings.Contains(string(raw), "refChanged") {
		t.Fatal("persisted computed ref drift")
	}
	// Restoring a default ref is also drift, even if default currently has the same commit.
	repairHost(t, b)
	applyPluginRequest(t, b, Request{Action: "remove", Name: "demo"})
	applyPluginRequest(t, b, Request{Action: "add", Source: source, Targets: []string{"pi"}})
	share()
	assertDrift(true)
	applyPluginRequest(t, a, Request{Action: "disable", Name: "demo"})
	assertDrift(false)
	applyPluginRequest(t, a, Request{Action: "sync"})
	assertDrift(false)
	applyPluginRequest(t, a, Request{Action: "enable", Name: "demo"})
	assertDrift(true)
	same = unchanged(t, a.ConfigPath)
	applyPluginRequest(t, a, Request{Action: "sync"})
	same()
	assertDrift(false)
	if readState(t, a).packages["demo"].Bindings["pi"].SourceRef != "" {
		t.Fatal("default ref not restored")
	}
}

func TestPluginHostStateExplicitRefValidatedBeforeWrites(t *testing.T) {
	for _, kind := range []string{"local", "missing-git-ref", "valid-git-ref"} {
		for _, phase := range []string{"disabled", "new-host", "replacement"} {
			t.Run(kind+"/"+phase, func(t *testing.T) {
				var calls []string
				s := fakePiNpm(t, &calls)
				source, initial, requested := hostStatePiSource(t), "", "branch"
				if kind != "local" {
					source, _ = repairGitSource(t, "validate-ref")
					initial = "main"
					requested = "absent"
					if kind == "valid-git-ref" {
						requested = "next"
					}
				}
				original, originalRef := source, initial
				if phase == "replacement" {
					original, originalRef = hostStatePiSource(t), ""
				}
				applyPluginRequest(t, s, Request{Action: "add", Source: original, SourceRef: originalRef, Targets: []string{"pi"}})
				if phase == "replacement" {
					// A different producer changes distribution; the old source is now unavailable.
					shared, _ := os.ReadFile(s.ConfigPath)
					other := fakePiNpm(t, &calls)
					writePluginFile(t, filepath.Dir(other.ConfigPath), filepath.Base(other.ConfigPath), string(shared))
					applyPluginRequest(t, other, Request{Action: "remove", Name: "demo"})
					applyPluginRequest(t, other, Request{Action: "add", Source: source, SourceRef: initial, Targets: []string{"pi"}})
					shared, _ = os.ReadFile(other.ConfigPath)
					writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), string(shared))
					repairHost(t, s)
					if err := os.RemoveAll(original); err != nil {
						t.Fatal(err)
					}

				} else {
					applyPluginRequest(t, s, Request{Action: "disable", Name: "demo"})
					if phase == "new-host" {
						shared, _ := os.ReadFile(s.ConfigPath)
						s = fakePiNpm(t, &calls)
						writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), string(shared))
					}
				}
				settings, _ := s.piSettingsPath("pi")
				same := unchanged(t, s.ConfigPath, s.statePath(), settings)
				count := len(calls)
				info, _ := os.Stat(s.ConfigPath)
				trace := filepath.Join(t.TempDir(), "git.trace")
				t.Setenv("GIT_TRACE", trace)
				req := Request{Action: "update", Name: "demo", SourceRef: requested}
				p, err := s.Preview(context.Background(), req)
				if kind != "valid-git-ref" {
					if err == nil {
						t.Fatalf("invalid explicit ref accepted: %+v", p)
					}
					if _, err := s.Apply(context.Background(), req, "reviewed"); err == nil {
						t.Fatal("apply accepted invalid explicit ref")
					}
					same()
					after, _ := os.Stat(s.ConfigPath)
					if !info.ModTime().Equal(after.ModTime()) {
						t.Fatal("refusal rewrote shared YAML")
					}
					if len(calls) != count {
						t.Fatal("refusal mutated native state")
					}
					return
				}
				if err != nil || p.Blocked {
					t.Fatalf("valid ref: %v %+v", err, p)
				}
				raw, _ := os.ReadFile(trace)
				if n := strings.Count(string(raw), " clone --depth 1 -- "); n != 1 {
					t.Fatalf("effective declaration acquired %d times in one preview", n)
				}
				if s.discoveries != nil {
					t.Fatal("preview cache escaped operation")
				}
				if _, err := s.Apply(context.Background(), req, p.Revision); err != nil {
					t.Fatal(err)
				}
				if phase != "replacement" && len(calls) != count {
					t.Fatal("disabled ref update ran native command")
				}
				applyPluginRequest(t, s, Request{Action: "enable", Name: "demo"})
				applyPluginRequest(t, s, Request{Action: "sync"})
				if got := readState(t, s).packages["demo"].Bindings["pi"]; got.SourceRef != "next" {
					t.Fatalf("enabled sync ignored validated ref: %+v", got)
				}
			})
		}
	}
}

func TestPluginHostStateRefDiscoverySharedWithinPreview(t *testing.T) {
	var calls []string
	s := fakePiNpm(t, &calls)
	source, _ := repairGitSource(t, "one-acquisition")
	s.Accounts = map[string]Account{"work": {Agent: "pi", Dir: t.TempDir()}}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, SourceRef: "main", Targets: []string{"pi", "work"}})
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	if _, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo", SourceRef: "next"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(trace)
	if n := strings.Count(string(raw), " clone --depth 1 -- "); n != 1 {
		t.Fatalf("acquisitions=%d, want one: %s", n, raw)
	}
	// Validate each effective target, and refuse all writes when just one is invalid.
	applyPluginRequest(t, s, Request{Action: "remove", Name: "demo", Targets: []string{"work"}})
	applyPluginRequest(t, s, Request{Action: "add", Name: "demo", Source: hostStatePiSource(t), Targets: []string{"work"}})

	same := unchanged(t, s.ConfigPath, s.statePath())
	before := len(calls)
	if _, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo", SourceRef: "next"}); err == nil {
		t.Fatal("invalid target override accepted")
	}
	same()
	if len(calls) != before {
		t.Fatal("partial native update during validation")
	}
}

func TestPluginHostStateDisabledRefKeepsCompatibleCandidate(t *testing.T) {
	for _, change := range []string{"renamed", "incompatible"} {
		t.Run(change, func(t *testing.T) {
			var calls []string
			s := fakePiNpm(t, &calls)
			source, _, repo := repairGitSourceRepo(t, "candidate-ref")
			applyPluginRequest(t, s, Request{Action: "add", Source: source, SourceRef: "main", Targets: []string{"pi"}})
			applyPluginRequest(t, s, Request{Action: "disable", Name: "demo"})

			body := `{"name":"other","keywords":["pi-package"]}`
			if change == "incompatible" {
				body = `{"name":"demo","main":"index.js","dependencies":{"@opencode-ai/plugin":"1"}}`
				if err := os.RemoveAll(filepath.Join(repo, "extensions")); err != nil {
					t.Fatal(err)
				}
				writePluginFile(t, repo, "index.js", "export default () => {}")
			}
			writePluginFile(t, repo, "package.json", body)
			for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "changed candidate"}} {
				cmd := exec.Command("git", args...)
				cmd.Dir = repo
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git: %s %v", out, err)
				}
			}
			same := unchanged(t, s.ConfigPath, s.statePath())
			before := len(calls)
			req := Request{Action: "update", Name: "demo", SourceRef: "next"}
			p, err := s.Preview(context.Background(), req)
			if err == nil && !p.Blocked {
				t.Fatalf("disabled update accepted incompatible candidate: %+v", p.Changes)
			}
			same()
			if len(calls) != before {
				t.Fatal("candidate validation ran native mutation")
			}
		})
	}
}

func TestPluginHostStateNativeOnlyReimportDropsPreviousSource(t *testing.T) {
	var ac, bc []string
	a := fakePiNpm(t, &ac)
	source, _ := repairGitSource(t, "native-only")
	applyPluginRequest(t, a, Request{Action: "add", Source: source, SourceRef: "main", Targets: []string{"pi"}})
	original := readState(t, a).packages["demo"].Bindings["pi"]
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
	if old := readState(t, a).packages["demo"].Bindings["pi"]; !old.Cleanup || old.Source != source {
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
	got := readState(t, a).packages["demo"].Bindings["pi"]
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
func TestPluginHostStateCheckUsesDesiredRecipe(t *testing.T) {
	for _, kind := range []string{"ref content", "ref same content", "missing ref", "source", "plugin", "entry", "native-only", "npm"} {
		t.Run(kind, func(t *testing.T) {
			var aCalls, bCalls []string
			a := fakePiNpm(t, &aCalls)
			source, _, repo := repairGitSourceRepo(t, "desired-check")
			applyPluginRequest(t, a, Request{Action: "add", Source: source, SourceRef: "main", Targets: []string{"pi"}})
			raw, _ := os.ReadFile(a.ConfigPath)
			b := fakePiNpm(t, &bCalls)
			writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(raw))
			applyPluginRequest(t, b, Request{Action: "sync"})
			applied := readState(t, b).packages["demo"].Bindings["pi"]
			desiredSource, ref, plugin, entry, imported := source, "main", "demo", "", false
			switch kind {
			case "ref content", "ref same content":
				if kind == "ref same content" {
					cmd := exec.Command("git", "branch", "same", "main")
					cmd.Dir = repo
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("branch: %s %v", out, err)
					}
					ref = "same"
				} else {
					ref = "next"
				}
				repairHost(t, a)
				applyPluginRequest(t, a, Request{Action: "update", Name: "demo", SourceRef: ref})
				raw, _ = os.ReadFile(a.ConfigPath)
			case "missing ref":
				ref = "absent"
			case "source":
				desiredSource, ref = hostStatePiSource(t), ""
				writeFile(t, desiredSource, "package.json", `{"name":"demo","version":"2.0.0","keywords":["pi-package"]}`)
			case "plugin":
				plugin = "absent"
			case "entry":
				entry = "new-entry.js"
			case "native-only":
				imported = true
			case "npm":
				desiredSource, ref, plugin = "npm:demo", "", ""
			}
			if kind != "ref content" && kind != "ref same content" {
				// This is the shared file received from another host, including explicit target overrides.
				text := "plugins:\n  version: 2\n  packages:\n    demo:\n      bindings:\n        pi:\n"
				if imported {
					text += "          imported: true\n"
				} else {
					text += "          source: " + strconv.Quote(desiredSource) + "\n          source_ref: " + strconv.Quote(ref) + "\n          plugin: " + strconv.Quote(plugin) + "\n          entry: " + strconv.Quote(entry) + "\n"
				}
				raw = []byte(text)
			}
			repairHost(t, b)
			writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(raw))
			if kind == "native-only" || kind == "npm" {
				if err := os.RemoveAll(repo); err != nil {
					t.Fatal(err)
				}
			}

			same := unchanged(t, b.ConfigPath, b.statePath())
			calls := len(bCalls)
			plan, err := b.Preview(context.Background(), Request{Action: "check", Name: "demo"})
			if kind == "missing ref" || kind == "plugin" {
				if err == nil && (len(plan.Changes) != 1 || plan.Changes[0].Action != "blocked") {
					t.Fatalf("invalid desired recipe silently used applied source: %+v %v", plan, err)
				}
			} else if kind == "native-only" || kind == "npm" {
				if err != nil || len(plan.Changes) != 1 || plan.Changes[0].Action != "native-check" {
					t.Fatalf("cleanup target claimed applied source current: %+v %v", plan, err)
				}
			} else {
				if err != nil || len(plan.Changes) != 1 || plan.Changes[0].Action != "update-available" {
					t.Fatalf("check missed desired recipe: %+v %v", plan, err)
				}
				desired, discoverErr := DiscoverOptions(context.Background(), desiredSource, ref, entry)
				got := plan.Changes[0].Binding
				if discoverErr != nil || got.Digest != desired.Digest || got.Commit != desired.Commit || got.Source != desiredSource || got.SourceRef != ref || got.Plugin != plugin || got.Entry != entry {
					t.Fatalf("check did not expose desired approval: %+v %+v %v", got, desired, discoverErr)
				}
			}
			same()
			if len(bCalls) != calls {
				t.Fatalf("check ran native commands: %q", bCalls[calls:])
			}
			if got := readState(t, b).state.Bindings["demo"]["pi"].Applied.Binding; got.Source != applied.Source || got.SourceRef != applied.SourceRef || got.Digest != applied.Digest {
				t.Fatalf("check rewrote applied provenance: %+v", got)
			}
		})
	}
}

func TestPluginCheckedUpdateRejectsAnotherCandidateInTheSameSource(t *testing.T) {
	source := t.TempDir()
	writeFile(t, source, ".claude-plugin/marketplace.json", `{"name":"team","plugins":[{"name":"first","source":"./first"},{"name":"second","source":"./second"}]}`)
	for _, name := range []string{"first", "second"} {
		writeFile(t, source, name+"/.claude-plugin/plugin.json", `{"name":"`+name+`","version":"1.0.0"}`)
	}
	aNative := &fakeAgents{version: "1.0.0"}
	a := aNative.service(t)
	applyPluginRequest(t, a, Request{Action: "add", Name: "demo", Plugin: "first", Source: source, Targets: []string{"claude"}})
	shared, _ := os.ReadFile(a.ConfigPath)
	bNative := &fakeAgents{version: "1.0.0"}
	b := bNative.service(t)
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(shared))
	applyPluginRequest(t, b, Request{Action: "sync"})
	for _, name := range []string{"first", "second"} {
		writeFile(t, source, name+"/.claude-plugin/plugin.json", `{"name":"`+name+`","version":"2.0.0"}`)
	}
	aNative.version, bNative.version = "2.0.0", "2.0.0"
	checked, err := b.Preview(context.Background(), Request{Action: "check", Name: "demo"})
	if err != nil || len(checked.Changes) != 1 || checked.Changes[0].Action != "update-available" {
		t.Fatalf("check: %+v %v", checked, err)
	}
	observed := checked.Changes[0].Binding
	t.Setenv("HOME", filepath.Dir(a.ConfigPath))
	applyPluginRequest(t, a, Request{Action: "remove", Name: "demo"})
	applyPluginRequest(t, a, Request{Action: "add", Name: "demo", Plugin: "second", Source: source, Targets: []string{"claude"}})
	shared, _ = os.ReadFile(a.ConfigPath)
	t.Setenv("HOME", filepath.Dir(b.ConfigPath))
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(shared))
	plan, err := b.Preview(context.Background(), Request{Action: "update", Name: "demo"})
	if err != nil || len(plan.Changes) != 2 || plan.Changes[1].Action != "install" {
		t.Fatalf("replacement: %+v %v", plan, err)
	}
	replacement := plan.Changes[1].Binding
	if observed.Version != replacement.Version || observed.Commit != replacement.Commit || observed.Digest != replacement.Digest || observed.Plugin == replacement.Plugin {
		t.Fatalf("fixture did not isolate identity: checked=%+v replacement=%+v", observed, replacement)
	}
	same := unchanged(t, b.ConfigPath, b.statePath())
	commands := len(bNative.commands)
	_, _, err = b.updateLocked(context.Background(), UpdateItem{Name: "demo", Targets: []string{"claude"}, Seen: map[string]Seen{"claude": checkedObservation(t, observed)}}, func(*Plan) {})
	if err != ErrSourceMoved || len(bNative.commands) != commands {
		t.Fatalf("unreviewed candidate accepted: %v, commands=%q", err, bNative.commands[commands:])
	}
	same()
	checked, err = b.Preview(context.Background(), Request{Action: "check", Name: "demo"})
	if err != nil || len(checked.Changes) != 1 || checked.Changes[0].Action != "update-available" {
		t.Fatalf("fresh replacement check: %+v %v", checked, err)
	}
	approved := checked.Changes[0].Binding
	if approved.ID == replacement.ID {
		t.Fatal("fixture did not preserve applied ID as check baseline")
	}
	_, result, err := b.updateLocked(context.Background(), UpdateItem{Name: "demo", Targets: []string{"claude"}, Seen: map[string]Seen{"claude": checkedObservation(t, approved)}}, func(*Plan) {})
	if err != nil || len(result.Results) != 2 || result.Results[1].Status != "installed" {
		t.Fatalf("approved replacement rejected: %+v %v", result, err)
	}
	installed := readState(t, b).state.Bindings["demo"]["claude"].Applied.Binding
	if installed.Plugin != "second" || installed.Digest != approved.Digest {
		t.Fatalf("wrong replacement installed: %+v", installed)
	}
}
