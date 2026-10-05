package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func ompRemovalFixture(t *testing.T, project bool) (*Service, Binding, string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, key := range []string{"XDG_DATA_HOME", "PI_CONFIG_DIR", "PI_CODING_AGENT_DIR", "OMP_PROFILE", "PI_PROFILE"} {
		t.Setenv(key, "")
	}
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	root := filepath.Join(home, ".omp", "plugins")
	scope := "user"
	if project {
		s.ProjectRoot = filepath.Join(home, "project")
		root = filepath.Join(s.ProjectRoot, ".omp", "plugins")
		scope = "project"
	}
	b := Binding{ID: "demo@team", Version: "1.0.0"}
	cache := ompCachePath(filepath.Join(home, ".omp", "plugins", "cache", "plugins"), b.ID, b.Version)
	writeFile(t, cache, "package.json", `{"name":"demo-runtime","version":"1.0.0"}`)
	writeFile(t, cache, "sentinel.txt", "other projects still need this")
	writeFile(t, root, "installed_plugins.json", `{"version":2,"opaque":9007199254740993,"plugins":{"demo@team":[{"scope":"`+scope+`","installPath":`+strconv.Quote(cache)+`,"version":"1.0.0"}]}}`)
	writeFile(t, root, "omp-plugins.lock.json", `{"version":1,"plugins":{"demo-runtime":{"version":"1.0.0","enabled":true},"other":{"enabled":false}},"settings":{"demo-runtime":{"token":"KEEP"},"other":{"opaque":9007199254740993}}}`)
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cache, filepath.Join(root, "node_modules", "demo-runtime")); err != nil {
		t.Fatal(err)
	}
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		switch strings.Join(args, " ") {
		case "--version":
			return []byte("omp/18.6.1"), nil
		case "plugin marketplace list":
			return []byte("No marketplaces configured\n"), nil
		case "plugin uninstall --help":
			return []byte("uninstall"), nil
		case "plugin list --json":
			raw, err := os.ReadFile(filepath.Join(root, "installed_plugins.json"))
			if err != nil {
				return nil, err
			}
			var reg struct {
				Plugins map[string]json.RawMessage `json:"plugins"`
			}
			if err := json.Unmarshal(raw, &reg); err != nil {
				return nil, err
			}
			list := []any{}
			for id, entries := range reg.Plugins {
				list = append(list, map[string]any{"id": id, "scope": scope, "entries": entries})
			}
			return json.Marshal(map[string]any{"npm": []any{}, "marketplace": list})
		default:
			t.Fatalf("unsafe native command: %v", args)
			return nil, nil
		}
	}
	applyPluginRequest(t, s, Request{Action: "import", From: "omp", Plugin: b.ID})
	return s, b, root, cache
}

func TestOMPRemoveRetainsCacheUsedByInvisibleProject(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "user", true: "project"}[project], func(t *testing.T) {
			s, _, root, cache := ompRemovalFixture(t, project)
			foreign := filepath.Join(t.TempDir(), ".omp", "plugins")
			writeFile(t, foreign, "installed_plugins.json", `{"version":2,"plugins":{"demo@team":[{"installPath":`+strconv.Quote(cache)+`}]}}`)
			if err := os.MkdirAll(filepath.Join(foreign, "node_modules"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(cache, filepath.Join(foreign, "node_modules", "demo-runtime")); err != nil {
				t.Fatal(err)
			}
			applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
			reg, err := os.ReadFile(filepath.Join(root, "installed_plugins.json"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(reg), "demo@team") || !strings.Contains(string(reg), "9007199254740993") {
				t.Fatalf("registry: %s", reg)
			}
			lock, err := os.ReadFile(filepath.Join(root, "omp-plugins.lock.json"))
			if err != nil {
				t.Fatal(err)
			}
			var cfg struct {
				Plugins  map[string]json.RawMessage
				Settings map[string]json.RawMessage
			}
			if err := json.Unmarshal(lock, &cfg); err != nil {
				t.Fatal(err)
			}
			if _, ok := cfg.Plugins["demo-runtime"]; ok || len(cfg.Plugins) != 1 || !strings.Contains(string(cfg.Settings["demo-runtime"]), "KEEP") {
				t.Fatalf("runtime config: %s", lock)
			}
			if _, err := os.Lstat(filepath.Join(root, "node_modules", "demo-runtime")); !os.IsNotExist(err) {
				t.Fatalf("runtime link survived: %v", err)
			}
			if data, err := os.ReadFile(filepath.Join(foreign, "node_modules", "demo-runtime", "sentinel.txt")); err != nil || string(data) != "other projects still need this" {
				t.Fatalf("foreign project damaged: %s %v", data, err)
			}
			if _, err := os.Stat(filepath.Join(cache, "package.json")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOMPRemovalRefusesForeignLinkAndStaleNativeFile(t *testing.T) {
	s, _, root, _ := ompRemovalFixture(t, false)
	r := Request{Action: "remove", Name: "demo"}
	p, err := s.Preview(context.Background(), r)
	if err != nil || p.Blocked {
		t.Fatalf("preview: %+v %v", p, err)
	}
	writeFile(t, root, "omp-plugins.lock.json", `{"plugins":{"demo-runtime":{"enabled":false}},"settings":{"concurrent":"KEEP"}}`)
	if _, err := s.Apply(context.Background(), r, p.Revision); err == nil {
		t.Fatal("stale plan applied")
	}
	link := filepath.Join(root, "node_modules", "demo-runtime")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	foreign := t.TempDir()
	if err := os.Symlink(foreign, link); err != nil {
		t.Fatal(err)
	}
	p, err = s.Preview(context.Background(), r)
	if err != nil || !p.Blocked {
		t.Fatalf("foreign link allowed: %+v %v", p, err)
	}
	if got, err := os.Readlink(link); err != nil || got != foreign {
		t.Fatalf("foreign link changed: %s %v", got, err)
	}
}

func TestOMPRemovalRefusesUnsafeMetadata(t *testing.T) {
	for name, corrupt := range map[string]func(*testing.T, string){
		"duplicate keys": func(t *testing.T, root string) {
			writeFile(t, root, "omp-plugins.lock.json", `{"plugins":{},"plugins":{"demo-runtime":{}}}`)
		},
		"invalid JSON": func(t *testing.T, root string) { writeFile(t, root, "omp-plugins.lock.json", `{broken`) },
		"unknown registry": func(t *testing.T, root string) {
			writeFile(t, root, "installed_plugins.json", `{"version":99,"plugins":{}}`)
		},
		"npm ownership": func(t *testing.T, root string) {
			writeFile(t, root, "package.json", `{"dependencies":{"demo-runtime":"1.0.0"}}`)
		},
		"real module directory": func(t *testing.T, root string) {
			link := filepath.Join(root, "node_modules", "demo-runtime")
			if err := os.Remove(link); err != nil {
				t.Fatal(err)
			}
			writeFile(t, link, "keep.txt", "KEEP")
		},
		"linked metadata": func(t *testing.T, root string) {
			file := filepath.Join(root, "omp-plugins.lock.json")
			if err := os.Remove(file); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(t.TempDir(), "config.json")
			writeFile(t, filepath.Dir(target), "config.json", `{"plugins":{}}`)
			if err := os.Symlink(target, file); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			s, _, root, cache := ompRemovalFixture(t, false)
			corrupt(t, root)
			before, err := os.ReadFile(filepath.Join(root, "installed_plugins.json"))
			if err != nil {
				t.Fatal(err)
			}
			p, err := s.Preview(context.Background(), Request{Action: "remove", Name: "demo"})
			if err != nil || !p.Blocked || p.Changes[0].MessageKey != "plugins.error.ompRemovalSafety" {
				t.Fatalf("unsafe removal: %+v %v", p, err)
			}
			after, err := os.ReadFile(filepath.Join(root, "installed_plugins.json"))
			if err != nil || string(before) != string(after) {
				t.Fatal("preview changed registration")
			}
			if _, err := os.Stat(filepath.Join(cache, "sentinel.txt")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOMPReinstallAllocatesFreshIdentityWithoutTouchingRetainedCache(t *testing.T) {
	agents := &fakeAgents{version: "1.0.0"}
	s := agents.service(t)
	source := fixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"omp"}})
	old := agents.installed["omp"][0].ID
	cache := ompCachePath(ompDefaultCacheRoot(), old, "1.0.0")
	writeFile(t, cache, "package.json", `{"name":"demo","version":"1.0.0"}`)
	writeFile(t, cache, "keep.txt", "another project uses this")
	agents.installed["omp"] = nil
	applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
	agents.commands = nil
	r := Request{Action: "add", Source: source, Targets: []string{"omp"}}
	p, err := s.Preview(context.Background(), r)
	if err != nil || p.Blocked || p.Changes[0].ID == old || p.Changes[0].MessageKey != "plugins.note.ompFreshCache" {
		t.Fatalf("reinstall: %+v %v", p, err)
	}
	again, err := s.Preview(context.Background(), r)
	if err != nil || again.Revision != p.Revision {
		t.Fatalf("unstable preview: %+v %v", again, err)
	}
	if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
		t.Fatal(err)
	}
	if len(agents.commands) != 2 || !strings.HasPrefix(agents.commands[1], "omp plugin install "+p.Changes[0].ID) {
		t.Fatalf("commands: %v", agents.commands)
	}
	if data, err := os.ReadFile(filepath.Join(cache, "keep.txt")); err != nil || string(data) != "another project uses this" {
		t.Fatalf("retained cache changed: %s %v", data, err)
	}
}

func TestOMPRemovalResumesAfterRegistryWasAlreadyRemoved(t *testing.T) {
	s, _, root, _ := ompRemovalFixture(t, false)
	writeFile(t, root, "installed_plugins.json", `{"version":2,"plugins":{}}`)
	applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
	if _, err := os.Lstat(filepath.Join(root, "node_modules", "demo-runtime")); !os.IsNotExist(err) {
		t.Fatalf("partial removal link remained: %v", err)
	}
	t.Run("forget after native uninstall with another plugin", func(t *testing.T) {
		s, _, root, cache := ompRemovalFixture(t, false)
		writeFile(t, root, "installed_plugins.json", `{"version":2,"plugins":{}}`)
		lock := `{"plugins":{"other":{"enabled":false}},"settings":{"other":{"keep":true}}}`
		writeFile(t, root, "omp-plugins.lock.json", lock)
		writeFile(t, root, "node_modules/other/keep.txt", "KEEP")
		for _, path := range []string{filepath.Join(root, "node_modules", "demo-runtime"), filepath.Join(cache, "package.json"), filepath.Join(cache, "sentinel.txt"), cache} {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
		r := Request{Action: "remove", Name: "demo"}
		p, err := s.Preview(context.Background(), r)
		if err != nil || p.Blocked || p.Changes[0].Action != "forget" {
			t.Fatalf("native-uninstalled binding cannot be forgotten: %+v %v", p, err)
		}
		if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(filepath.Join(root, "omp-plugins.lock.json")); err != nil || string(data) != lock {
			t.Fatalf("forget changed unrelated runtime state: %s %v", data, err)
		}
		if data, err := os.ReadFile(filepath.Join(root, "node_modules", "other", "keep.txt")); err != nil || string(data) != "KEEP" {
			t.Fatalf("forget damaged another plugin: %s %v", data, err)
		}
		inv, err := s.Packages()
		if err != nil || len(inv.Packages) != 0 {
			t.Fatalf("forgotten binding remained: %+v %v", inv, err)
		}
	})
}
