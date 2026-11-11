package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Keep native filesystem effects real: the removal adapter must free the link
// before the install runner can register a marketplace or create a replacement.
func ompSourceChangeFixture(t *testing.T, project bool, module string) (*Service, string, string, Binding) {
	t.Helper()
	agents := &fakeAgents{version: "1.0.0"}
	s := agents.service(t)
	for _, key := range []string{"XDG_DATA_HOME", "PI_CONFIG_DIR", "PI_CODING_AGENT_DIR", "OMP_PROFILE", "PI_PROFILE"} {
		t.Setenv(key, "")
	}
	root := filepath.Join(os.Getenv("HOME"), ".omp", "plugins")
	if project {
		s.ProjectRoot = filepath.Join(os.Getenv("HOME"), "project")
		s.ConfigPath = filepath.Join(s.ProjectRoot, ".skillshare", "config.yaml")
		root = filepath.Join(s.ProjectRoot, ".omp", "plugins")
	}
	source := ompReplacementSource(t, module)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"omp"}})
	old := hostPackages(t, s)["demo"].Bindings["omp"]
	cacheRoot := filepath.Join(os.Getenv("HOME"), ".omp", "plugins", "cache", "plugins")
	install := func(id, snapshot string) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(snapshot, "content", "package.json"))
		if err != nil {
			t.Fatal(err)
		}
		var pkg struct{ Name, Version string }
		if err := json.Unmarshal(raw, &pkg); err != nil {
			t.Fatal(err)
		}
		cache := ompCachePath(cacheRoot, id, pkg.Version)
		if err := os.CopyFS(cache, os.DirFS(filepath.Join(snapshot, "content"))); err != nil {
			t.Fatal(err)
		}
		scope := "user"
		if project {
			scope = "project"
		}
		// Native install adds its entries and keeps the other plugins and settings.
		reg := map[string]any{"version": 2, "plugins": map[string]any{}}
		if data, err := os.ReadFile(filepath.Join(root, "installed_plugins.json")); err == nil {
			if err := json.Unmarshal(data, &reg); err != nil {
				t.Fatal(err)
			}
		}
		reg["plugins"].(map[string]any)[id] = []any{map[string]any{"scope": scope, "installPath": cache, "version": pkg.Version}}
		data, _ := json.Marshal(reg)
		writeFile(t, root, "installed_plugins.json", string(data))
		lock := map[string]any{"settings": map[string]any{"unrelated": "KEEP"}, "plugins": map[string]any{}}
		if data, err := os.ReadFile(filepath.Join(root, "omp-plugins.lock.json")); err == nil {
			if err := json.Unmarshal(data, &lock); err != nil {
				t.Fatal(err)
			}
		}
		lock["plugins"].(map[string]any)[pkg.Name] = map[string]any{"enabled": true}
		data, _ = json.Marshal(lock)
		writeFile(t, root, "omp-plugins.lock.json", string(data))
		link := filepath.Join(root, "node_modules", filepath.FromSlash(pkg.Name))
		if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(cache, link); err != nil {
			t.Fatal(err)
		}
	}
	install(old.ID, s.snapshotPath(old, "omp"))
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		command := strings.Join(args, " ")
		if bin == "omp" && command == "plugin list --json" {
			reg := readJSON(t, filepath.Join(root, "installed_plugins.json"))
			agents.installed["omp"] = nil
			for _, id := range slices.Sorted(maps.Keys(reg["plugins"].(map[string]any))) {
				scope := "user"
				if project {
					scope = "project"
				}
				agents.installed["omp"] = append(agents.installed["omp"], Installed{ID: id, Version: "1.0.0", Scope: scope, Installed: true, Enabled: true})
			}
		}
		mutating := bin == "omp" && !slices.Contains(args, "--help") && (strings.HasPrefix(command, "plugin marketplace add ") || strings.HasPrefix(command, "plugin install "))
		if mutating {
			snapshot := args[len(args)-1]
			if args[1] == "install" {
				_, market, _ := strings.Cut(args[2], "@")
				snapshot = agents.markets["omp"][market]
			}
			var pkg struct{ Name string }
			raw, _ := os.ReadFile(filepath.Join(snapshot, "content", "package.json"))
			if json.Unmarshal(raw, &pkg) == nil && pkg.Name != "" {
				if _, err := os.Lstat(filepath.Join(root, "node_modules", filepath.FromSlash(pkg.Name))); !os.IsNotExist(err) {
					return nil, fmt.Errorf("old runtime link still present before native mutation")
				}
			}
		}
		out, err := run(ctx, dir, env, bin, args...)
		if err == nil && mutating && args[1] == "install" {
			_, market, _ := strings.Cut(args[2], "@")
			install(args[2], agents.markets["omp"][market])
		}
		return out, err
	}
	return s, source, root, old
}

func ompReplacementSource(t *testing.T, module string) string {
	t.Helper()
	source := fixture(t)
	writeFile(t, source, "package.json", fmt.Sprintf(`{"name":%q,"version":"1.0.0"}`, module))
	return source
}

func receiveOMPSource(t *testing.T, s *Service, source string) {
	t.Helper()
	d := readState(t, s)
	pack := d.intent["demo"]
	decl := pack.Bindings["omp"]
	decl.Source = source
	pack.Source, pack.Bindings["omp"] = source, decl
	d.intent["demo"] = pack
	if err := s.save(d); err != nil {
		t.Fatal(err)
	}
}

func TestOMPSourceChangeReconcilesRuntimeLink(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, action := range []string{"sync", "update"} {
			for _, modules := range [][2]string{{"demo-runtime", "demo-runtime"}, {"@scope/demo-runtime", "@scope/demo-runtime"}, {"demo-runtime", "@other/replacement"}} {
				module, newModule := modules[0], modules[1]
				t.Run(fmt.Sprintf("project=%t/%s/%s-to-%s", project, action, module, newModule), func(t *testing.T) {
					s, _, root, old := ompSourceChangeFixture(t, project, module)
					source := ompReplacementSource(t, newModule)
					receiveOMPSource(t, s, source)
					r := Request{Action: action, Name: "demo"}
					sameConfig := unchanged(t, s.ConfigPath)
					sameNative := unchanged(t, s.statePath(), filepath.Join(root, "installed_plugins.json"), filepath.Join(root, "omp-plugins.lock.json"))
					p, err := s.Preview(context.Background(), r)
					if err != nil || p.Blocked || len(p.Changes) != 2 || p.Changes[0].Action != "remove" || p.Changes[1].Action != "install" {
						t.Fatalf("source replacement blocked instead of ordered cleanup/install: %+v %v", p, err)
					}
					sameNative()
					again, err := s.Preview(context.Background(), r)
					if err != nil || again.Revision != p.Revision {
						t.Fatalf("unchanged preview was unstable: %+v %v", again, err)
					}
					if result, err := s.Apply(context.Background(), r, p.Revision); err != nil {
						t.Fatalf("replacement failed: %+v %v", result, err)
					}
					sameConfig()
					b := hostPackages(t, s)["demo"].Bindings["omp"]
					if b.Source != source || b.ID == old.ID || b.Cleanup || b.Pending != "" {
						t.Fatalf("replacement receipt lost: %+v", b)
					}
					link, err := os.Readlink(filepath.Join(root, "node_modules", filepath.FromSlash(newModule)))
					if err != nil || link == ompCachePath(filepath.Join(os.Getenv("HOME"), ".omp/plugins/cache/plugins"), old.ID, old.Version) {
						t.Fatalf("runtime still uses old cache: %s %v", link, err)
					}
					oldCache := ompCachePath(filepath.Join(os.Getenv("HOME"), ".omp/plugins/cache/plugins"), old.ID, old.Version)
					if _, err := os.Stat(filepath.Join(oldCache, "package.json")); err != nil {
						t.Fatal("cleanup removed shared cache", err)
					}
					lock := readJSON(t, filepath.Join(root, "omp-plugins.lock.json"))
					if lock["settings"].(map[string]any)["unrelated"] != "KEEP" {
						t.Fatal("cleanup lost unrelated settings")
					}
				})
			}
		}
	}
}

// Issue #833: an OMP update removes only this scope's registration and installs
// the new commit under a fresh cache identity, never upgrading shared cache in place.
// OMP installs from the catalog cached when its marketplace was added, so a
// refresh must register a new identity even when the new version's cache
// folder is still free; reusing the old one would install the old version.
func TestOMPUpdateRefreshesIntoFreshCache(t *testing.T) {
	for _, edit := range []string{"content", "version"} {
		t.Run(edit, func(t *testing.T) {
			s, source, root, old := ompSourceChangeFixture(t, false, "demo-runtime")
			if edit == "content" {
				writeFile(t, source, "skills/demo/SKILL.md", "---\nname: demo\ndescription: Demo\n---\nUpdated")
			} else {
				writeFile(t, source, "package.json", `{"name":"demo-runtime","version":"2.0.0"}`)
				writeFile(t, source, ".claude-plugin/plugin.json", `{"name":"demo","version":"2.0.0"}`)
			}
			r := Request{Action: "update", Name: "demo"}
			p, err := s.Preview(context.Background(), r)
			if err != nil || p.Blocked || len(p.Changes) != 2 || p.Changes[0].Action != "remove" || p.Changes[1].Action != "install" {
				t.Fatalf("update did not refresh through removal and install: %+v %v", p, err)
			}
			if p.Changes[1].ID == old.ID {
				t.Fatalf("refresh reuses the old marketplace: %+v", p.Changes[1])
			}
			if result, err := s.Apply(context.Background(), r, p.Revision); err != nil {
				t.Fatalf("refresh failed: %+v %v", result, err)
			}
			b := hostPackages(t, s)["demo"].Bindings["omp"]
			if b.ID == old.ID || b.Digest == old.Digest || b.Pending != "" {
				t.Fatalf("refresh receipt not updated: %+v", b)
			}
			oldCache := ompCachePath(filepath.Join(os.Getenv("HOME"), ".omp/plugins/cache/plugins"), old.ID, old.Version)
			if _, err := os.Stat(filepath.Join(oldCache, "package.json")); err != nil {
				t.Fatal("refresh removed shared cache", err)
			}
			if lock := readJSON(t, filepath.Join(root, "omp-plugins.lock.json")); lock["settings"].(map[string]any)["unrelated"] != "KEEP" {
				t.Fatal("refresh lost unrelated settings")
			}
		})
	}
}

// One update --all plan refreshes several OMP packages that share the native
// registry and lock. An earlier refresh rewrites those files, which must not
// stop the next package's removal.
func TestOMPUpdateAllRefreshesEveryPackage(t *testing.T) {
	for _, ownEdit := range []bool{false, true} {
		t.Run(fmt.Sprintf("own-entry-edited=%t", ownEdit), func(t *testing.T) {
			s, source, root, old := ompSourceChangeFixture(t, false, "demo-runtime")
			other := ompReplacementSource(t, "other-runtime")
			writeFile(t, other, ".claude-plugin/plugin.json", `{"name":"other","version":"1.0.0"}`)
			writeFile(t, other, ".codex-plugin/plugin.json", `{"name":"other","version":"1.0.0","skills":"./skills"}`)
			applyPluginRequest(t, s, Request{Action: "add", Source: other, Targets: []string{"omp"}})
			before := hostPackages(t, s)["other"].Bindings["omp"]
			for _, src := range []string{source, other} {
				writeFile(t, src, "skills/demo/SKILL.md", "---\nname: demo\ndescription: Demo\n---\nUpdated")
			}
			if ownEdit {
				// Something outside Skillshare changes the second package's own
				// runtime selection while the first one installs.
				run := s.Run
				s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
					out, err := run(ctx, dir, env, bin, args...)
					if err == nil && bin == "omp" && len(args) > 2 && args[1] == "install" && strings.HasPrefix(args[2], "demo@") {
						lock := readJSON(t, filepath.Join(root, "omp-plugins.lock.json"))
						lock["plugins"].(map[string]any)["other-runtime"] = map[string]any{"enabled": false}
						data, _ := json.Marshal(lock)
						writeFile(t, root, "omp-plugins.lock.json", string(data))
					}
					return out, err
				}
			}
			r := Request{Action: "update", All: true}
			p, err := s.Preview(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Apply(context.Background(), r, p.Revision)
			packages := hostPackages(t, s)
			refreshed := packages["other"].Bindings["omp"].ID != before.ID
			if packages["demo"].Bindings["omp"].ID == old.ID || refreshed == ownEdit || (err != nil) != ownEdit {
				t.Fatalf("other refreshed=%t err=%v: %+v", refreshed, err, packages)
			}
		})
	}
}

func TestOMPSourceChangeAfterPartialCleanup(t *testing.T) {
	for _, phase := range []string{"registry pruned", "link removed", "registry and lock pruned"} {
		t.Run(phase, func(t *testing.T) {
			s, _, root, _ := ompSourceChangeFixture(t, false, "demo-runtime")
			receiveOMPSource(t, s, ompReplacementSource(t, "demo-runtime"))
			if phase == "link removed" {
				if err := os.Remove(filepath.Join(root, "node_modules", "demo-runtime")); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFile(t, root, "installed_plugins.json", `{"version":2,"plugins":{}}`)
				if phase == "registry and lock pruned" {
					writeFile(t, root, "omp-plugins.lock.json", `{"plugins":{},"settings":{"unrelated":"KEEP"}}`)
				}
			}
			same := unchanged(t, s.ConfigPath)
			applyPluginRequest(t, s, Request{Action: "sync", Name: "demo"})
			same()
			if b := hostPackages(t, s)["demo"].Bindings["omp"]; b.Cleanup || b.Pending != "" {
				t.Fatalf("partial cleanup did not finish replacement: %+v", b)
			}
		})
	}
}

func TestOMPSourceChangeRejectsMutationDuringApply(t *testing.T) {
	for _, stage := range []string{"removal", "install"} {
		t.Run(stage, func(t *testing.T) {
			s, _, root, _ := ompSourceChangeFixture(t, false, "demo-runtime")
			receiveOMPSource(t, s, ompReplacementSource(t, "demo-runtime"))
			r := Request{Action: "sync", Name: "demo"}
			p, err := s.Preview(context.Background(), r)
			if err != nil || p.Blocked {
				t.Fatalf("replacement not ready: %+v %v", p, err)
			}
			link := filepath.Join(root, "node_modules", "demo-runtime")
			run := s.Run
			installs := 0
			foreign := t.TempDir()
			s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
				command := strings.Join(args, " ")
				if strings.HasPrefix(command, "plugin install ") && !slices.Contains(args, "--help") {
					installs++
				}
				out, err := run(ctx, dir, env, bin, args...)
				if stage == "install" && strings.HasPrefix(command, "plugin marketplace add ") && err == nil {
					if err := os.Symlink(foreign, link); err != nil {
						t.Fatal(err)
					}
				}
				return out, err
			}
			if stage == "removal" {
				// Model an edit after Apply's fresh preview, before removal's CAS check.
				writeFile(t, root, "omp-plugins.lock.json", `{"plugins":{"demo-runtime":{}},"settings":{"concurrent":"KEEP"}}`)
			}
			result, err := s.applyPlan(context.Background(), r, p)
			if err == nil || installs != 0 {
				t.Fatalf("native install ran after observed mutation: %+v %v, installs=%d", result, err, installs)
			}
			if stage == "install" {
				if got, err := os.Readlink(link); err != nil || got != foreign {
					t.Fatalf("foreign link overwritten: %s %v", got, err)
				}
			}
		})
	}
}

func TestOMPSourceChangeRejectsUnownedRuntime(t *testing.T) {
	for _, corrupt := range []string{"foreign link", "directory", "linked parent", "npm owner", "other owner", "occupied new module"} {
		t.Run(corrupt, func(t *testing.T) {
			s, _, root, _ := ompSourceChangeFixture(t, false, "demo-runtime")
			receiveOMPSource(t, s, ompReplacementSource(t, "demo-runtime"))
			link := filepath.Join(root, "node_modules", "demo-runtime")
			switch corrupt {
			case "occupied new module":
				receiveOMPSource(t, s, ompReplacementSource(t, "different-runtime"))
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "node_modules", "different-runtime")); err != nil {
					t.Fatal(err)
				}
			case "foreign link", "directory":
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
				if corrupt == "foreign link" {
					if err := os.Symlink(t.TempDir(), link); err != nil {
						t.Fatal(err)
					}
				} else {
					writeFile(t, link, "keep", "KEEP")
				}
			case "linked parent":
				parent := filepath.Join(root, "node_modules")
				if err := os.Rename(parent, parent+"-retained"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(parent+"-retained", parent); err != nil {
					t.Fatal(err)
				}
			case "npm owner":
				writeFile(t, root, "package.json", `{"dependencies":{"demo-runtime":"1.0.0"}}`)
			case "other owner":
				reg := readJSON(t, filepath.Join(root, "installed_plugins.json"))
				cache := filepath.Join(t.TempDir(), "cache")
				writeFile(t, cache, "package.json", `{"name":"demo-runtime","version":"1.0.0"}`)
				reg["plugins"].(map[string]any)["other@team"] = []any{map[string]any{"scope": "user", "installPath": cache, "version": "1.0.0"}}
				raw, _ := json.Marshal(reg)
				writeFile(t, root, "installed_plugins.json", string(raw))
			}
			same := unchanged(t, s.ConfigPath, s.statePath(), filepath.Join(root, "installed_plugins.json"), filepath.Join(root, "omp-plugins.lock.json"))
			r := Request{Action: "sync", Name: "demo"}
			p, err := s.Preview(context.Background(), r)
			if err != nil || !p.Blocked {
				t.Fatalf("unsafe replacement allowed: %+v %v", p, err)
			}
			if _, err := s.Apply(context.Background(), r, p.Revision); err == nil {
				t.Fatal("unsafe replacement applied")
			}
			same()
		})
	}
}

func TestOMPSourceChangeRejectsStaleProof(t *testing.T) {
	for _, mutation := range []string{"link", "metadata", "source"} {
		t.Run(mutation, func(t *testing.T) {
			s, _, root, _ := ompSourceChangeFixture(t, false, "demo-runtime")
			source := ompReplacementSource(t, "demo-runtime")
			receiveOMPSource(t, s, source)
			r := Request{Action: "sync", Name: "demo"}
			p, err := s.Preview(context.Background(), r)
			if err != nil || p.Blocked {
				t.Fatalf("replacement not ready: %+v %v", p, err)
			}
			switch mutation {
			case "link":
				link := filepath.Join(root, "node_modules", "demo-runtime")
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), link); err != nil {
					t.Fatal(err)
				}
			case "metadata":
				writeFile(t, root, "omp-plugins.lock.json", `{"plugins":{"demo-runtime":{"enabled":false}},"settings":{"new":"KEEP"}}`)
			case "source":
				writeFile(t, source, "new.txt", "changed after review")
			}
			same := unchanged(t, s.ConfigPath, s.statePath(), filepath.Join(root, "installed_plugins.json"), filepath.Join(root, "omp-plugins.lock.json"))
			if _, err := s.Apply(context.Background(), r, p.Revision); err == nil || !strings.Contains(err.Error(), "changed since preview") {
				t.Fatalf("stale proof was accepted: %v", err)
			}
			same()
		})
	}
}

func TestOMPSourceChangeRetriesFailedInstall(t *testing.T) {
	s, _, root, old := ompSourceChangeFixture(t, false, "demo-runtime")
	source := ompReplacementSource(t, "demo-runtime")
	receiveOMPSource(t, s, source)
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if len(args) > 2 && args[1] == "install" && !slices.Contains(args, "--help") {
			return nil, errors.New("fixture install failed")
		}
		return run(ctx, dir, env, bin, args...)
	}
	same := unchanged(t, s.ConfigPath)
	result := applyFailure(t, s, Request{Action: "sync", Name: "demo"})
	if len(result.Results) != 2 || result.Results[0].Status != "removed" || result.Results[1].Status != "failed" {
		t.Fatalf("cleanup did not precede failed install: %+v", result)
	}
	if _, err := os.Lstat(filepath.Join(root, "node_modules", "demo-runtime")); !os.IsNotExist(err) {
		t.Fatal("old runtime link survived cleanup", err)
	}
	local := hostPackages(t, s)["demo"].Bindings["omp"]
	if local.ID == old.ID || local.Source != source || local.Pending != "install" {
		t.Fatalf("replacement retry evidence lost: %+v", local)
	}
	s.Run = run
	applyPluginRequest(t, s, Request{Action: "sync", Name: "demo"})
	same()
	local = hostPackages(t, s)["demo"].Bindings["omp"]
	if local.Pending != "" || local.Cleanup || local.Source != source {
		t.Fatalf("retry did not finish: %+v", local)
	}
}

func TestOMPStaleCatalogRecovery(t *testing.T) {
	s, _, root, _ := ompSourceChangeFixture(t, false, "demo-runtime")
	source := ompReplacementSource(t, "demo-runtime")
	setVersion := func(version string) {
		writeFile(t, source, "package.json", `{"name":"demo-runtime","version":"`+version+`"}`)
		writeFile(t, source, ".claude-plugin/plugin.json", `{"name":"demo","version":"`+version+`"}`)
	}
	setVersion("2.9.1")
	receiveOMPSource(t, s, source)
	cacheDir := t.TempDir()
	writeFile(t, cacheDir, "catalog.json", `{"version":"2.6.0"}`)
	run := s.Run
	failInstall, failRefresh, adds, installs := true, true, 0, 0
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if bin == "omp" && len(args) > 2 && args[1] == "marketplace" && args[2] == "add" {
			adds++
			if adds > 1 {
				raw, err := os.ReadFile(filepath.Join(source, "package.json"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(cacheDir, "catalog.json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		if bin == "omp" && len(args) > 2 && args[1] == "marketplace" && args[2] == "update" {
			if failRefresh {
				return nil, errors.New("catalog refresh failed")
			}
			raw, err := os.ReadFile(filepath.Join(source, "package.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(cacheDir, "catalog.json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
		if bin == "omp" && len(args) > 2 && args[1] == "install" && !slices.Contains(args, "--help") {
			if failInstall {
				return nil, errors.New("initial install interrupted")
			}
			installs++
			var cached struct{ Version string }
			raw, err := os.ReadFile(filepath.Join(cacheDir, "catalog.json"))
			if err != nil || json.Unmarshal(raw, &cached) != nil || cached.Version != "2.9.1" && cached.Version != "2.9.2" {
				t.Fatalf("native install used stale catalog: %s %v", raw, err)
			}
		}
		return run(ctx, dir, env, bin, args...)
	}
	r := Request{Action: "sync", Name: "demo"}
	applyFailure(t, s, r)
	failInstall = false
	applyFailure(t, s, r)
	if installs != 0 {
		t.Fatal("installed after failed catalog refresh")
	}
	addsBeforeRetry := adds
	failRefresh = false
	applyPluginRequest(t, s, r)
	if adds != addsBeforeRetry {
		t.Fatal("retry added another marketplace")
	}
	b := hostPackages(t, s)["demo"].Bindings["omp"]
	reg := readJSON(t, filepath.Join(root, "installed_plugins.json"))
	entry := reg["plugins"].(map[string]any)[b.ID].([]any)[0].(map[string]any)
	if b.Version != "2.9.1" || entry["version"] != "2.9.1" || entry["installPath"] != ompCachePath(filepath.Join(os.Getenv("HOME"), ".omp/plugins/cache/plugins"), b.ID, "2.9.1") {
		t.Fatalf("wrong reviewed/native version: %+v %+v", b, entry)
	}
	setVersion("2.9.2")
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo"})
	b = hostPackages(t, s)["demo"].Bindings["omp"]
	reg = readJSON(t, filepath.Join(root, "installed_plugins.json"))
	entry = reg["plugins"].(map[string]any)[b.ID].([]any)[0].(map[string]any)
	if b.Version != "2.9.2" || entry["version"] != "2.9.2" {
		t.Fatalf("update lost native version: %+v %+v", b, entry)
	}
}

func TestOMPMismatchedNativeVersionRemoval(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"normal", "interrupted", "pending-only"} {
			t.Run(fmt.Sprintf("project=%t/mode=%s", project, mode), func(t *testing.T) {
				s, _, root, old := ompSourceChangeFixture(t, project, "demo-runtime")
				cacheRoot := filepath.Join(os.Getenv("HOME"), ".omp/plugins/cache/plugins")
				oldCache := ompCachePath(cacheRoot, old.ID, old.Version)
				nativeCache := ompCachePath(cacheRoot, old.ID, "0.9.0")
				if err := os.MkdirAll(filepath.Dir(nativeCache), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(oldCache, nativeCache); err != nil {
					t.Fatal(err)
				}
				link := filepath.Join(root, "node_modules", "demo-runtime")
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(nativeCache, link); err != nil {
					t.Fatal(err)
				}
				reg := readJSON(t, filepath.Join(root, "installed_plugins.json"))
				entry := reg["plugins"].(map[string]any)[old.ID].([]any)[0].(map[string]any)
				entry["version"], entry["installPath"] = "0.9.0", nativeCache
				raw, _ := json.Marshal(reg)
				writeFile(t, root, "installed_plugins.json", string(raw))
				if mode == "pending-only" {
					d := readState(t, s)
					local := d.state.Bindings["demo"]["omp"]
					local.Pending, local.Applied = local.Applied, nil
					local.Pending.Action = "install"
					d.putLocal("demo", "omp", local)
					if err := s.saveState(d); err != nil {
						t.Fatal(err)
					}
				}
				receiveOMPSource(t, s, ompReplacementSource(t, "demo-runtime"))
				r := Request{Action: "sync", Name: "demo"}
				p, err := s.Preview(context.Background(), r)
				if err != nil || p.Blocked || len(p.Changes) != 2 {
					t.Fatalf("preview: %+v %v", p, err)
				}
				if mode != "normal" {
					release, err := ompAcquireLock(filepath.Join(root, "installed_plugins.json"))
					if err != nil {
						t.Fatal(err)
					}
					_, applyErr := s.Apply(context.Background(), r, p.Revision)
					if err := release(); err != nil {
						t.Fatal(err)
					}
					if applyErr == nil {
						t.Fatal("locked removal unexpectedly succeeded")
					}
					local := readState(t, s).state.Bindings["demo"]["omp"]
					if local.Pending == nil || local.Pending.Binding.Version != old.Version || local.Pending.OMPRemovalVersion != "0.9.0" {
						t.Fatalf("pending receipt mixed reviewed and native versions: %+v", local.Pending)
					}
					// Simulate registry-first progress after the pending receipt was written.
					writeFile(t, root, "installed_plugins.json", `{"version":2,"plugins":{}}`)
					p, err = s.Preview(context.Background(), r)
					if err != nil || p.Blocked || len(p.Changes) != 2 || p.Changes[0].Binding.Version != old.Version {
						t.Fatalf("retry preview lost reviewed identity: %+v %v", p, err)
					}
				}
				if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
					t.Fatalf("apply: %v", err)
				}
				if _, err := os.Stat(filepath.Join(nativeCache, "package.json")); err != nil {
					t.Fatalf("shared cache lost: %v", err)
				}
			})
		}
	}
}

func TestOMPSourceChangeKeepsOrdinaryAddBlocked(t *testing.T) {
	s, _, _, _ := ompSourceChangeFixture(t, false, "demo-runtime")
	source := ompReplacementSource(t, "demo-runtime")
	receiveOMPSource(t, s, source)
	for _, action := range []string{"sync", "add"} {
		r := Request{Action: action, Name: "demo", Targets: []string{"omp"}}
		if action == "add" {
			r.Source = source
		}
		p, err := s.Preview(context.Background(), r)
		if err != nil || p.Blocked != (action == "add") {
			t.Fatalf("cleanup proof escaped its replacement: %+v %v", p, err)
		}
	}
}

func TestOMPSourceChangeRetriesFailedCleanup(t *testing.T) {
	for _, pendingOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("pending-only=%t", pendingOnly), func(t *testing.T) {
			s, _, root, old := ompSourceChangeFixture(t, false, "demo-runtime")
			if pendingOnly {
				d := readState(t, s)
				local := d.state.Bindings["demo"]["omp"]
				local.Pending, local.Applied = local.Applied, nil
				local.Pending.Action = "install" // Crash after native installation.
				d.putLocal("demo", "omp", local)
				if err := s.saveState(d); err != nil {
					t.Fatal(err)
				}
			}
			source := ompReplacementSource(t, "demo-runtime")
			receiveOMPSource(t, s, source)
			release, err := ompAcquireLock(filepath.Join(root, "installed_plugins.json"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = release() })
			same := unchanged(t, s.ConfigPath)
			result := applyFailure(t, s, Request{Action: "sync", Name: "demo"})
			if len(result.Results) != 1 || result.Results[0].Status != "failed" {
				t.Fatalf("replacement was attempted after failed cleanup: %+v", result)
			}
			local := hostPackages(t, s)["demo"].Bindings["omp"]
			if local.ID != old.ID || !local.Cleanup {
				t.Fatalf("failed cleanup lost original evidence: %+v", local)
			}
			if err := release(); err != nil {
				t.Fatal(err)
			}
			applyPluginRequest(t, s, Request{Action: "sync", Name: "demo"})
			same()
			local = hostPackages(t, s)["demo"].Bindings["omp"]
			if local.ID == old.ID || local.Source != source || local.Cleanup || local.Pending != "" {
				t.Fatalf("retry did not replace old installation: %+v", local)
			}
		})
	}
}

func TestOMPSourceChangeBatchPreservesBlockedTarget(t *testing.T) {
	setup := func(t *testing.T) (*Service, string, Binding) {
		s, _, root, old := ompSourceChangeFixture(t, false, "demo-runtime")
		source := ompReplacementSource(t, "demo-runtime")
		receiveOMPSource(t, s, source)
		d := readState(t, s)
		pack := d.intent["demo"]
		pack.Bindings["claude"] = declaration{Source: source, Plugin: "demo"}
		d.intent["demo"] = pack
		if err := s.save(d); err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, "package.json", `{"dependencies":{"demo-runtime":"1.0.0"}}`)
		return s, root, old
	}
	// update --all, and updating one package without --target (issue #833), install
	// the valid target while OMP's conflicting replacement keeps its old
	// installation and receipt for a later retry.
	for _, r := range []Request{{Action: "update", All: true}, {Action: "update", Name: "demo"}} {
		t.Run(fmt.Sprintf("all=%v/name=%q", r.All, r.Name), func(t *testing.T) {
			s, root, old := setup(t)
			same := unchanged(t, s.ConfigPath, filepath.Join(root, "installed_plugins.json"), filepath.Join(root, "omp-plugins.lock.json"))
			plan, err := s.Preview(context.Background(), r)
			if err != nil || plan.Blocked || !slices.ContainsFunc(plan.Changes, func(c Change) bool { return c.Target == "omp" && c.Failed && c.Message != "" }) {
				t.Fatalf("failed target missing from preview: %+v %v", plan, err)
			}
			encoded, err := json.Marshal(plan)
			if err != nil || !strings.Contains(string(encoded), `"failed":true`) {
				t.Fatalf("failure missing from JSON: %s %v", encoded, err)
			}
			result := applyFailure(t, s, r)
			if len(result.Results) != 2 || result.Results[0].Target != "claude" || result.Results[0].Status != "installed" || result.Results[1].Target != "omp" || result.Results[1].Status != "failed" {
				t.Fatalf("update did not preserve blocked replacement and install valid sibling: %+v", result)
			}
			same()
			packaged := hostPackages(t, s)["demo"]
			if packaged.Bindings["omp"].ID != old.ID || !packaged.Bindings["omp"].Cleanup || packaged.Bindings["claude"].ID == "" {
				t.Fatalf("update lost target evidence: %+v", packaged)
			}
		})
	}
	t.Run("named target stays blocked", func(t *testing.T) {
		s, _, _ := setup(t)
		p, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo", Targets: []string{"omp"}})
		if err != nil || !p.Blocked {
			t.Fatalf("an update naming the blocked target should be blocked: %+v %v", p, err)
		}
	})
}
