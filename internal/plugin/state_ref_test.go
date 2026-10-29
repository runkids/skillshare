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
	aCommit := hostPackages(t, s)["demo"].Bindings["pi"].Commit
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
	if got := hostPackages(t, b)["demo"].Bindings["pi"].Commit; got == aCommit || got == "" {
		t.Fatal("host B copied reviewed commit from A")
	}
	if got := hostPackages(t, s)["demo"].Bindings["pi"].Commit; got != aCommit {
		t.Fatal("B update changed A receipt")
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
	if binding.SourceRef != "next" || binding.Commit != next {
		t.Fatal("installed receipt disagrees with requested ref")
	}
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
	old := hostPackages(t, a)["demo"].Bindings["pi"]
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
	if got := hostPackages(t, a)["demo"].Bindings["pi"]; got.SourceRef != "main" || got.ID != old.ID {
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
	got := hostPackages(t, a)["demo"].Bindings["pi"]
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
	if hostPackages(t, a)["demo"].Bindings["pi"].SourceRef != "" {
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
				if got := hostPackages(t, s)["demo"].Bindings["pi"]; got.SourceRef != "next" {
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
			applied := hostPackages(t, b)["demo"].Bindings["pi"]
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
			if got := hostPackages(t, b)["demo"].Bindings["pi"]; got.Source != applied.Source || got.SourceRef != applied.SourceRef || got.Digest != applied.Digest {
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
	installed := hostPackages(t, b)["demo"].Bindings["claude"]
	if installed.Plugin != "second" || installed.Digest != approved.Digest {
		t.Fatalf("wrong replacement installed: %+v", installed)
	}
}
