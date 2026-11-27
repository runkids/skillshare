package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRexManifest(t *testing.T) {
	for _, tc := range []struct{ name, extra, key string }{
		{"valid", `config = "rex/config.lua"`, ""},
		{"scripts", "config = \"rex/config.lua\"\n[[scripts]]\nname = \"x\"", "plugins.problem.rexScripts"},
		{"build", "config = \"rex/config.lua\"\n[[build]]\nrun = [\"make\"]", "plugins.problem.rexScripts"},
		{"missing config", "", "plugins.problem.rexNoConfig"},
		{"escaping config", `config = "../outside.lua"`, "plugins.problem.rexConfigEscapes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Dir(root), "outside.lua", "-- outside")
			writeFile(t, root, "rex-plugin.toml", "id = \"acme.test\"\nname = \"Test\"\nversion = \"1\"\n"+tc.extra+"\n")
			writeFile(t, root, "rex/config.lua", "-- config")
			c, err := inspect(root)
			if err != nil {
				t.Fatal(err)
			}
			if c.Name != "acme.test" || c.TargetInfo["rex"].ProblemKey != tc.key {
				t.Fatalf("candidate = %+v, rex = %+v", c, c.TargetInfo["rex"])
			}
		})
	}
}

func TestRexConfigPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("REX_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", root)
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", root)
	}
	if got := rexInitPath(); got != filepath.Join(root, "rex", "init.lua") {
		t.Fatal(got)
	}
	t.Setenv("REX_CONFIG", filepath.Join(root, "custom.lua"))
	if got := rexInitPath(); got != os.Getenv("REX_CONFIG") {
		t.Fatal(got)
	}
}

func rexFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writePluginFile(t, root, "rex-plugin.toml", "id = \"acme.test\"\nname = \"Test\"\nversion = \"1\"\nconfig = \"rex/config.lua\"\n")
	writePluginFile(t, root, "rex/config.lua", "-- test")
	return root
}

func TestRexLifecycle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REX_CONFIG", filepath.Join(home, "rex", "init.lua"))
	source := rexFixture(t)
	writePluginFile(t, home, "rex/init.lua", "-- user config\n")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if args[0] == "version" {
			return []byte(`{"rex_version":"1"}`), nil
		}
		return []byte(`{}`), nil
	}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"rex"}})
	init := os.Getenv("REX_CONFIG")
	data, _ := os.ReadFile(init)
	if string(data) != rexLine+"-- user config\n" {
		t.Fatalf("init = %q", data)
	}
	root, _, _ := s.localRoot("rex")
	installed := filepath.Join(root, "acme.test", "rex", "config.lua")
	writePluginFile(t, source, "rex/config.lua", "-- changed")
	applyPluginRequest(t, s, Request{Action: "update", Name: "acme.test"})
	if data, err := os.ReadFile(installed); err != nil || string(data) != "-- changed" {
		t.Fatalf("updated bundle = %q, %v", data, err)
	}
	applyPluginRequest(t, s, Request{Action: "remove", Name: "acme.test"})
	data, _ = os.ReadFile(init)
	if string(data) != "-- user config\n" {
		t.Fatalf("restored = %q", data)
	}
	if _, err := os.Stat(filepath.Join(home, "rex", "skillshare_rex.lua")); !os.IsNotExist(err) {
		t.Fatal("loader remains", err)
	}
}

func TestRexMissingSavesIntentThenSyncs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REX_CONFIG", filepath.Join(home, "rex", "init.lua"))
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	missing := true
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if missing {
			return nil, ErrCLIMissing
		}
		if args[0] == "version" {
			return []byte(`{"rex_version":"1"}`), nil
		}
		return nil, nil
	}
	r := Request{Action: "add", Source: rexFixture(t), Targets: []string{"rex"}}
	p, err := s.Preview(context.Background(), r)
	if err != nil || p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "selection" {
		t.Fatalf("preview = %+v %v", p, err)
	}
	result, err := s.Apply(context.Background(), r, p.Revision)
	if err != nil || result.Results[0].Status != "saved" || !strings.Contains(result.Results[0].Message, "not installed") {
		t.Fatalf("result = %+v %v", result, err)
	}
	if _, err := os.Stat(os.Getenv("REX_CONFIG")); !os.IsNotExist(err) {
		t.Fatal("wrote init while missing", err)
	}
	if err := os.Rename(r.Source, r.Source+"-unavailable"); err != nil {
		t.Fatal(err)
	}
	for _, req := range []Request{{Action: "sync"}, {Action: "update", Name: "acme.test"}, {Action: "check"}, {Action: "update", Name: "acme.test", SourceRef: "next"}} {
		t.Run(req.Action+req.SourceRef, func(t *testing.T) {
			p, err := s.Preview(context.Background(), req)
			if err != nil || p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "skip" || p.Changes[0].MessageKey != "plugins.note.rexMissing" {
				t.Fatalf("missing host preview = %+v %v", p, err)
			}
		})
	}
}

func TestRexLoaderQuotesNonASCIIBytes(t *testing.T) {
	entry := rexEntry{rexManifest: rexManifest{ID: "acme.test", Config: "config.lua"}, Dir: "/tmp/café"}
	loader := rexLoader([]rexEntry{entry}, false)
	if !strings.Contains(loader, `caf\195\169`) {
		t.Fatalf("loader lacks Lua decimal escapes: %s", loader)
	}
	if got := rexLuaQuote("a\n\x00\"\\9"); got != `"a\010\000\"\\9"` {
		t.Fatalf("quoted bytes = %s", got)
	}
}

func TestRexProbePropagatesErrors(t *testing.T) {
	entry := rexEntry{rexManifest: rexManifest{ID: "acme.test", Config: "config.lua"}, Dir: t.TempDir()}
	if probe := rexLoader([]rexEntry{entry}, true); !strings.Contains(probe, "if not ok then error(\"Rex plugin acme.test: \" .. tostring(err)) end") {
		t.Fatal(probe)
	}
	if loader := rexLoader([]rexEntry{entry}, false); !strings.Contains(loader, "if not ok then rex.log(") {
		t.Fatal(loader)
	}
}

func TestRexRefusesForeignLoaderAndPreservesSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	init := filepath.Join(home, "rex", "init.lua")
	t.Setenv("REX_CONFIG", init)
	writePluginFile(t, home, "actual.lua", "-- user\n")
	if err := os.MkdirAll(filepath.Dir(init), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "actual.lua"), init); err != nil {
		t.Fatal(err)
	}
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if args[0] == "version" {
			return []byte(`{"rex_version":"1"}`), nil
		}
		return nil, nil
	}
	source := rexFixture(t)
	loader := filepath.Join(filepath.Dir(init), "skillshare_rex.lua")
	if err := os.WriteFile(loader, []byte("-- user loader"), 0644); err != nil {
		t.Fatal(err)
	}
	r := Request{Action: "add", Source: source, Targets: []string{"rex"}}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(context.Background(), r, p.Revision); err == nil {
		t.Fatal("overwrote foreign loader")
	}
	if data, _ := os.ReadFile(loader); string(data) != "-- user loader" {
		t.Fatal("foreign loader changed")
	}
	if err := os.Remove(loader); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, r)
	if info, err := os.Lstat(init); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink lost: %v", err)
	}
	applyPluginRequest(t, s, Request{Action: "remove", Name: "acme.test"})
	if data, _ := os.ReadFile(filepath.Join(home, "actual.lua")); string(data) != "-- user\n" {
		t.Fatalf("user config = %q", data)
	}
}

func TestRexLineIndexIgnoresComment(t *testing.T) {
	commented := "-- " + rexLine + "print('kept')\n"
	if rexLineIndex(commented) >= 0 {
		t.Fatal("matched a commented-out line")
	}
	if i := rexLineIndex(commented + rexLine); i != len(commented) {
		t.Fatalf("line index = %d", i)
	}
}

func TestRexConfigLinkMustStayInsidePlugin(t *testing.T) {
	root := rexFixture(t)
	path := filepath.Join(root, "rex", "config.lua")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "rex", "actual.lua"), []byte("-- config"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("actual.lua", path); err != nil {
		t.Fatal(err)
	}
	c, err := inspect(root)
	if err != nil || c.TargetInfo["rex"].Problem != "" {
		t.Fatalf("internal link: %+v %v", c, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.lua")
	if err := os.WriteFile(outside, []byte("-- outside"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	c, err = inspect(root)
	if err != nil || c.TargetInfo["rex"].ProblemKey != "plugins.problem.rexConfigEscapes" {
		t.Fatalf("escaping link: %+v %v", c, err)
	}
}

func TestRexInvalidIDExplainsTheManifestProblem(t *testing.T) {
	root := t.TempDir()
	writePluginFile(t, root, "rex-plugin.toml", "id = \"invalid/id\"\nname = \"Test\"\nversion = \"1\"\nconfig = \"config.lua\"\n")
	writePluginFile(t, root, "config.lua", "-- config")
	_, err := inspect(root)
	if err == nil || !strings.Contains(err.Error(), "valid plugin name") {
		t.Fatalf("error = %v", err)
	}
}

func TestRexRemovalKeepsOtherPlugin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	init := filepath.Join(home, "rex", "init.lua")
	t.Setenv("REX_CONFIG", init)
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if args[0] == "version" {
			return []byte(`{"rex_version":"1"}`), nil
		}
		return nil, nil
	}
	first, second := rexFixture(t), rexFixture(t)
	writePluginFile(t, second, "rex-plugin.toml", "id = \"acme.other\"\nname = \"Other\"\nversion = \"2\"\nconfig = \"rex/config.lua\"\n")
	applyPluginRequest(t, s, Request{Action: "add", Source: first, Targets: []string{"rex"}})
	applyPluginRequest(t, s, Request{Action: "add", Source: second, Targets: []string{"rex"}})
	applyPluginRequest(t, s, Request{Action: "remove", Name: "acme.test"})
	data, err := os.ReadFile(filepath.Join(filepath.Dir(init), "skillshare_rex.lua"))
	if err != nil || strings.Contains(string(data), "acme.test") || !strings.Contains(string(data), "acme.other") {
		t.Fatalf("remaining loader = %s, %v", data, err)
	}
	if data, err = os.ReadFile(init); err != nil || rexLineIndex(string(data)) < 0 {
		t.Fatalf("missing loader line: %q %v", data, err)
	}
	applyPluginRequest(t, s, Request{Action: "remove", Name: "acme.other"})
	if _, err = os.Stat(init); !os.IsNotExist(err) {
		t.Fatalf("init remains: %v", err)
	}
}

func TestRexImportIsUnsupported(t *testing.T) {
	s, _, _ := rexRecordTestService(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: rexFixture(t), Targets: []string{"rex"}})
	_, err := s.Preview(context.Background(), Request{Action: "import", From: "rex", Plugin: "acme.test"})
	if err == nil || !strings.Contains(err.Error(), "importing existing or marketplace installations is not supported") {
		t.Fatalf("error = %v", err)
	}
}

func TestRexRestoresEmptyInit(t *testing.T) {
	for _, kind := range []string{"empty", "absent", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			init := filepath.Join(home, "init.lua")
			t.Setenv("REX_CONFIG", init)
			target := init
			if kind == "symlink" {
				target = filepath.Join(home, "target.lua")
				if err := os.Symlink(target, init); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "empty" {
				if err := os.WriteFile(target, nil, 0644); err != nil {
					t.Fatal(err)
				}
			}
			s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
			s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
				if args[0] == "version" {
					return []byte(`{"rex_version":"1"}`), nil
				}
				return nil, nil
			}
			applyPluginRequest(t, s, Request{Action: "add", Source: rexFixture(t), Targets: []string{"rex"}})
			applyPluginRequest(t, s, Request{Action: "remove", Name: "acme.test"})
			data, err := os.ReadFile(target)
			if kind == "absent" {
				if !os.IsNotExist(err) {
					t.Fatalf("init exists: %v", err)
				}
				return
			}
			if kind == "empty" && (err != nil || len(data) != 0) {
				t.Fatalf("init = %q, %v", data, err)
			}
			if kind == "symlink" {
				info, err := os.Lstat(init)
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("link: %v", err)
				}
			}
		})
	}
}

func TestRexRefusesUnknownLoaderVersion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REX_CONFIG", filepath.Join(home, "init.lua"))
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	loader := filepath.Join(home, "skillshare_rex.lua")
	data := "-- Generated by Skillshare; format 999; do not edit.\n-- skillshare-root: " + hash([]byte(s.managedRoot())) + "\n"
	if err := os.WriteFile(loader, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if err := s.applyRex(context.Background(), Change{Target: "rex", Action: "install"}, Binding{ID: "acme.test"}, rexFixture(t)); err == nil {
		t.Fatal("accepted an unknown loader version")
	}
	if got, err := os.ReadFile(loader); err != nil || string(got) != data {
		t.Fatalf("loader changed: %q, %v", got, err)
	}
}

func TestRexAmberDiscovery(t *testing.T) {
	root := t.TempDir()
	writePluginFile(t, root, "lua/bell.lua", "return { setup = function() end }")
	d, err := discoverRoot(root, "https://github.com/acme/bell.rex.git")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 1 || d.Candidates[0].Name != "acme.bell" || d.Candidates[0].TargetInfo["rex"].Entry != "bell" {
		t.Fatalf("candidates: %+v", d.Candidates)
	}
}

func TestRexAmberLocalSourceName(t *testing.T) {
	root := filepath.Join(t.TempDir(), "bell.rex")
	writePluginFile(t, root, "lua/other.lua", "return {}")
	writePluginFile(t, root, "lua/bell.lua", "return {}")
	// Preserve the final dot to exercise native path cleaning on every platform.
	location := root + string(filepath.Separator) + "."
	m, err := rexAdopt(root, location, "")
	if err != nil || m.ID != "local.bell" || m.Main != "bell" {
		t.Fatalf("adopt = %+v, %v", m, err)
	}
}

func TestRexMissingHostSavesSelection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REX_CONFIG", filepath.Join(home, "init.lua"))
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, _ string, _ ...string) ([]byte, error) {
		return nil, ErrCLIMissing
	}
	applyPluginRequest(t, s, Request{Action: "add", Source: rexFixture(t), Targets: []string{"rex"}})
	for _, action := range []string{"disable", "enable"} {
		req := Request{Action: action, Name: "acme.test", Targets: []string{"rex"}}
		p, err := s.Preview(context.Background(), req)
		if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "selection" {
			t.Fatalf("%s preview = %+v, %v", action, p, err)
		}
		if _, err := s.Apply(context.Background(), req, p.Revision); err != nil {
			t.Fatal(err)
		}
		d, err := s.load()
		if err != nil || d.intent["acme.test"].Bindings["rex"].runtime().Selected() != (action == "enable") {
			t.Fatalf("%s selection not saved: %+v, %v", action, d, err)
		}
	}
}

func TestRexProbeOnlyIncludesIncomingPlugin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	init := filepath.Join(home, "rex", "init.lua")
	t.Setenv("REX_CONFIG", init)
	writePluginFile(t, home, "rex/init.lua", "-- user config\n")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	checks := 0
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if args[0] == "version" {
			return []byte(`{"rex_version":"1"}`), nil
		}
		checks++
		data, err := os.ReadFile(args[2])
		if err != nil {
			return nil, err
		}
		if strings.Contains(string(data), "-- user config") || !strings.Contains(string(data), "acme.test") {
			return nil, errors.New("probe must contain only the incoming plugin")
		}
		return nil, nil
	}
	applyPluginRequest(t, s, Request{Action: "add", Source: rexFixture(t), Targets: []string{"rex"}})
	if checks != 1 {
		t.Fatalf("checks = %d", checks)
	}
}

func TestRexAmberLifecycle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REX_CONFIG", filepath.Join(home, "rex", "init.lua"))
	source := filepath.Join(t.TempDir(), "bell.rex")
	writePluginFile(t, source, "lua/other.lua", "return {}")
	writePluginFile(t, source, "lua/bell.lua", "return { setup = function(options) end }")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if args[0] == "version" {
			return []byte(`{"rex_version":"1"}`), nil
		}
		return nil, nil
	}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"rex"}})
	loader, err := os.ReadFile(filepath.Join(home, "rex", "skillshare_rex.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(loader), `require("bell").setup(rexd.plugin.options)`) {
		t.Fatal(string(loader))
	}
	writePluginFile(t, source, "lua/bell.lua", "return { setup = function(options) print('changed') end }")
	applyPluginRequest(t, s, Request{Action: "update", Name: "local.bell"})
	root, _, _ := s.localRoot("rex")
	data, err := os.ReadFile(filepath.Join(root, "local.bell", "lua", "bell.lua"))
	if err != nil || !strings.Contains(string(data), "changed") {
		t.Fatalf("update: %q %v", data, err)
	}
	applyPluginRequest(t, s, Request{Action: "remove", Name: "local.bell"})
	if _, err := os.Stat(filepath.Join(root, "local.bell")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestRexProbeRejectsAndRestores(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	init := filepath.Join(home, "rex", "init.lua")
	t.Setenv("REX_CONFIG", init)
	writePluginFile(t, home, "rex/init.lua", "-- user config\n")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	reject := false
	incoming := "acme.other"
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if args[0] == "version" {
			return []byte(`{"rex_version":"1"}`), nil
		}
		if reject {
			config, e := os.ReadFile(args[2])
			if e != nil {
				return nil, e
			}
			if strings.Contains(string(config), "-- user config") || !strings.Contains(string(config), incoming) || (incoming == "acme.other" && strings.Contains(string(config), "acme.test")) {
				t.Fatalf("wrong probe: %s", config)
			}
			return nil, errors.New("conflicting plugins")
		}
		return nil, nil
	}
	first := rexFixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: first, Targets: []string{"rex"}})
	beforeInit, _ := os.ReadFile(init)
	beforeLoader, _ := os.ReadFile(filepath.Join(filepath.Dir(init), "skillshare_rex.lua"))
	second := rexFixture(t)
	writePluginFile(t, second, "rex-plugin.toml", "id = \"acme.other\"\nname = \"Other\"\nversion = \"1\"\nconfig = \"rex/config.lua\"\n")
	reject = true
	req := Request{Action: "add", Source: second, Targets: []string{"rex"}}
	preview, err := s.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(context.Background(), req, preview.Revision); err == nil {
		t.Fatal("expected rejection")
	}
	afterInit, _ := os.ReadFile(init)
	afterLoader, _ := os.ReadFile(filepath.Join(filepath.Dir(init), "skillshare_rex.lua"))
	if string(afterInit) != string(beforeInit) || string(afterLoader) != string(beforeLoader) {
		t.Fatal("configuration changed after rejection")
	}
	root, _, _ := s.localRoot("rex")
	if _, err := os.Stat(filepath.Join(root, "acme.other")); !os.IsNotExist(err) {
		t.Fatal("rejected bundle remains", err)
	}
	writePluginFile(t, first, "rex/config.lua", "-- changed")
	incoming = "acme.test"
	req = Request{Action: "update", Name: incoming}
	preview, err = s.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(context.Background(), req, preview.Revision); err == nil {
		t.Fatal("rejected update succeeded")
	}
	if data, err := os.ReadFile(filepath.Join(root, incoming, "rex", "config.lua")); err != nil || string(data) != "-- test" {
		t.Fatalf("bundle changed on rejection: %q, %v", data, err)
	}
}

func TestRexDiscoveryDoesNotShadowPi(t *testing.T) {
	root := t.TempDir()
	writePluginFile(t, root, "package.json", `{"name":"pi-example","pi":{"extensions":["index.ts"]}}`)
	writePluginFile(t, root, "lua/helper.lua", "return {}")
	d, err := discoverRoot(root, "https://github.com/acme/pi-example")
	if err != nil || len(d.Candidates) != 1 || d.Candidates[0].Name != "pi-example" {
		t.Fatalf("discovery: %+v %v", d, err)
	}
}

func rexRecordTestService(t *testing.T) (*Service, string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	init := filepath.Join(home, "init.lua")
	t.Setenv("REX_CONFIG", init)
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if args[0] == "version" {
			return []byte(`{"rex_version":"1"}`), nil
		}
		return []byte(`{}`), nil
	}
	root, _, err := s.localRoot("rex")
	if err != nil {
		t.Fatal(err)
	}
	return s, init, filepath.Join(root, ".skillshare-rex-init.json")
}

func rexRecordTestAddTwo(t *testing.T, s *Service) {
	t.Helper()
	first, second := rexFixture(t), rexFixture(t)
	writePluginFile(t, second, "rex-plugin.toml", "id = \"acme.other\"\nname = \"Other\"\nversion = \"1\"\nconfig = \"rex/config.lua\"\n")
	applyPluginRequest(t, s, Request{Action: "add", Source: first, Targets: []string{"rex"}})
	applyPluginRequest(t, s, Request{Action: "add", Source: second, Targets: []string{"rex"}})
}

func TestRexKeepsSyncedLoaderLine(t *testing.T) {
	s, init, record := rexRecordTestService(t)
	original := "-- user\n" + rexLine
	if err := os.WriteFile(init, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	rexRecordTestAddTwo(t, s)
	applyPluginRequest(t, s, Request{Action: "remove", Name: "acme.test"})
	applyPluginRequest(t, s, Request{Action: "remove", Name: "acme.other"})
	if data, err := os.ReadFile(init); err != nil || string(data) != original {
		t.Errorf("init = %q, %v; want %q", data, err, original)
	}
	for _, path := range []string{filepath.Join(filepath.Dir(init), "skillshare_rex.lua"), record} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s remains: %v", path, err)
		}
	}
}

func TestRexPendingInstallRetriesReconcile(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix directory permissions and a non-root user")
	}
	s, init, record := rexRecordTestService(t)
	dir := filepath.Join(filepath.Dir(init), "readonly")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	actual := filepath.Join(dir, "init.lua")
	if err := os.WriteFile(actual, []byte("-- user\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(actual, init); err != nil {
		t.Fatal(err)
	}
	req := Request{Action: "add", Source: rexFixture(t), Targets: []string{"rex"}}
	p, err := s.Preview(context.Background(), req)
	if err != nil || p.Blocked {
		t.Fatalf("preview: %+v %v", p, err)
	}
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
	if _, err := s.Apply(context.Background(), req, p.Revision); err == nil {
		t.Fatal("expected init write failure")
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, Request{Action: "sync"})
	root, _, err := s.localRoot("rex")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, "acme.test", "rex", "config.lua"), filepath.Join(filepath.Dir(init), "skillshare_rex.lua"), record} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	data, err := os.ReadFile(actual)
	if err != nil || string(data) != rexLine+"-- user\n" {
		t.Fatalf("init = %q, %v", data, err)
	}
}

func TestRexPendingRemoveRetriesReconcile(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix directory permissions and a non-root user")
	}
	s, init, record := rexRecordTestService(t)
	dir := filepath.Join(filepath.Dir(init), "readonly")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	actual := filepath.Join(dir, "init.lua")
	if err := os.WriteFile(actual, []byte("-- user\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(actual, init); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, Request{Action: "add", Source: rexFixture(t), Targets: []string{"rex"}})
	req := Request{Action: "remove", Name: "acme.test"}
	p, err := s.Preview(context.Background(), req)
	if err != nil || p.Blocked {
		t.Fatalf("preview: %+v %v", p, err)
	}
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
	if _, err := s.Apply(context.Background(), req, p.Revision); err == nil {
		t.Fatal("expected init write failure")
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, Request{Action: "sync"})
	data, err := os.ReadFile(actual)
	if err != nil || string(data) != "-- user\n" {
		t.Fatalf("init = %q, %v", data, err)
	}
	for _, path := range []string{filepath.Join(filepath.Dir(init), "skillshare_rex.lua"), record} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s remains: %v", path, err)
		}
	}
}

func TestRexPendingUninstallRetriesReconcile(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix directory permissions and a non-root user")
	}
	s, init, record := rexRecordTestService(t)
	dir := filepath.Join(filepath.Dir(init), "readonly")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	actual := filepath.Join(dir, "init.lua")
	if err := os.WriteFile(actual, []byte("-- user\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(actual, init); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, Request{Action: "add", Source: rexFixture(t), Targets: []string{"rex"}})
	applyPluginRequest(t, s, Request{Action: "disable", Name: "acme.test", Targets: []string{"rex"}})
	req := Request{Action: "sync"}
	p, err := s.Preview(context.Background(), req)
	if err != nil || p.Blocked {
		t.Fatalf("preview: %+v %v", p, err)
	}
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
	if _, err := s.Apply(context.Background(), req, p.Revision); err == nil {
		t.Fatal("expected init write failure")
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, req)
	data, err := os.ReadFile(actual)
	if err != nil || string(data) != "-- user\n" {
		t.Fatalf("init = %q, %v", data, err)
	}
	for _, path := range []string{filepath.Join(filepath.Dir(init), "skillshare_rex.lua"), record} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s remains: %v", path, err)
		}
	}
}
