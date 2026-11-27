package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTernManifestValidation(t *testing.T) {
	cases := []struct {
		name, manifest string
		valid          bool
	}{
		{"valid", "schema=1\nid='tern-kata'\nname='Kata'\nversion='1'\nhost='main.luau'\n", true},
		{"empty fields", "schema=1\nid='tern-kata'\nname=''\nversion=''\n", true},
		{"bad toml", "id = [", false},
		{"uppercase", "schema=1\nid='Upper'\nname='Kata'\nversion='1'\nhost='main.luau'\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "plugin.toml", tc.manifest)
			writeFile(t, root, "main.luau", "return {}\n")
			if !tc.valid {
				writeFile(t, root, ".claude-plugin/plugin.json", `{ "name": "tern-kata" }`)
			}
			c, err := inspect(root)
			info, found := c.TargetInfo["tern"]
			if err != nil || !found || info.Manifest == "" || (info.Problem == "") != tc.valid {
				t.Fatalf("candidate %+v err %v", c, err)
			}
		})
	}
}

func TestTernManifestMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "plugin.toml", "id='tern-kata'\nversion='2'\ndescription='Example'\n")
	id, description, info, found := inspectTern(root)
	if !found || id != "tern-kata" || description != "Example" || info.Version != "2" || info.Problem != "" {
		t.Fatalf("%q %q %+v %v", id, description, info, found)
	}
}

func TestTernManifestAbsent(t *testing.T) {
	_, _, _, found := inspectTern(t.TempDir())
	if found {
		t.Fatal("absent manifest discovered")
	}
}

func TestTernInventoryInvalid(t *testing.T) {
	for _, tc := range []struct {
		data      string
		wantError bool
	}{
		{"{", true}, {`{}`, true}, {`{"plugins":[{"id":"INVALID"}]}`, false},
	} {
		rows, err := parseCommandInventory("tern", []byte(tc.data))
		if (err != nil) != tc.wantError || len(rows) != 0 {
			t.Fatalf("%s: rows=%+v error=%v", tc.data, rows, err)
		}
	}
}

// Tern 0.9.18 list --json without a daemon returned:
// {"plugins":[{"id":"tern-kata","name":"Kata","version":"0.17.0","description":"Example","icon":"puzzle","css":"","blocks":[],"lenses":[],"host":true,"window":false,"status":"ready"}],"problems":[],"jit":false,"canvas":true}
// Installing id "tern" returned: plugin id "tern" is reserved for native blocks.
func TestTernInventoryIdentity(t *testing.T) {
	rows, err := parseCommandInventory("tern", []byte(`{"plugins":[{"id":"tern-kata","name":"Kata","status":"ready"}]}`))
	if err != nil || len(rows) != 1 || rows[0].Name != "tern-kata" {
		t.Fatalf("%+v %v", rows, err)
	}
}

type ternFixture struct {
	service   *Service
	source    string
	installed map[string]bool
	missing   bool
	commands  [][]string
}

func newTernFixture(t *testing.T) *ternFixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	f := &ternFixture{source: fixture(t), installed: map[string]bool{}, missing: true}
	writeFile(t, f.source, "plugin.toml", "schema = 1\nid = 'demo'\nname = 'Demo'\nversion = '1'\nwindow = 'main.luau'\n")
	writeFile(t, f.source, "main.luau", "return {}")
	f.service = &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	f.service.Run = func(_ context.Context, _ string, _ []string, bin string, args ...string) ([]byte, error) {
		if bin == "tern" && f.missing {
			return nil, agentError{cause: ErrCLIMissing, key: "plugins.error.cliMissing", message: "tern CLI is not installed", args: map[string]string{"cli": "tern"}}
		}
		switch strings.Join(args, " ") {
		case "--version":
			return []byte("1"), nil
		case "plugin --help", "plugin install --help", "plugin uninstall --help":
			return []byte("help"), nil
		case "plugin list --json":
			if bin == "tern" {
				// Real Tern 0.9.18 list --json printed the row and envelope shown
				// above TestTernInventoryIdentity (with its installed values).
				if f.installed[bin] {
					return []byte(`{"plugins":[{"id":"demo","name":"Demo","version":"1","description":"","icon":"puzzle","css":"","blocks":[],"lenses":[],"host":false,"window":true,"status":"ready"}],"problems":[],"jit":false,"canvas":true}`), nil
				}
				return []byte(`{"plugins":[],"problems":[],"jit":false,"canvas":true}`), nil
			}
			if f.installed[bin] {
				return []byte(`[{"name":"demo","enabled":true}]`), nil
			}
			return []byte(`[]`), nil
		}
		if len(args) > 2 && args[0] == "plugin" && args[1] == "install" {
			if bin == "tern" {
				if _, err := os.Stat(filepath.Join(args[2], "plugin.toml")); err != nil {
					return nil, fmt.Errorf("snapshot missing manifest: %w", err)
				}
				f.commands = append(f.commands, append([]string(nil), args...))
			}
			f.installed[bin] = true
			return nil, nil
		}
		if len(args) == 3 && args[0] == "plugin" && ((bin == "tern" && args[1] == "remove") || (bin != "tern" && args[1] == "uninstall")) {
			if bin == "tern" {
				f.commands = append(f.commands, append([]string(nil), args...))
			}
			f.installed[bin] = false
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected %s %v", bin, args)
	}
	return f
}

func assertTernMissingSkips(t *testing.T, s *Service) {
	t.Helper()
	for _, action := range []string{"sync", "check", "update"} {
		p, err := s.Preview(context.Background(), Request{Action: action, Name: "demo"})
		if err != nil || p.Blocked {
			t.Fatalf("%s: %+v %v", action, p, err)
		}
		found := false
		for _, c := range p.Changes {
			if c.Target == "tern" && c.Action == "skip" && c.MessageKey == "plugins.error.cliMissing" {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: %+v", action, p.Changes)
		}
	}
}

func TestTernMissingDefersIntentAndCleanup(t *testing.T) {
	t.Run("missing saves intent and skips", ternMissingSavesIntent)
	t.Run("other host reconciles while missing", ternMissingOtherHostReconciles)
	t.Run("receipt cleanup", ternMissingReceiptCleanup)
}

func ternMissingSavesIntent(t *testing.T) {
	f := newTernFixture(t)
	applyPluginRequest(t, f.service, Request{Action: "add", Source: f.source, Targets: []string{"tern", "copilot"}})
	state, err := f.service.load()
	if err != nil || state.intent["demo"].Bindings["tern"].Source == "" || !f.installed["copilot"] {
		t.Fatalf("intent: %+v %v", state, err)
	}
	assertTernMissingSkips(t, f.service)
}

func ternMissingOtherHostReconciles(t *testing.T) {
	f := newTernFixture(t)
	applyPluginRequest(t, f.service, Request{Action: "add", Source: f.source, Targets: []string{"tern", "copilot"}})
	if err := os.Rename(f.source, f.source+"-offline"); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, f.service, Request{Action: "disable", Name: "demo", Targets: []string{"copilot"}})
	assertTernMissingSkips(t, f.service)
	for _, action := range []string{"update", "sync"} {
		applyPluginRequest(t, f.service, Request{Action: action, Name: "demo"})
	}
	if f.installed["copilot"] {
		t.Fatal("Copilot was not reconciled")
	}
}

func ternMissingReceiptCleanup(t *testing.T) {
	f := newTernFixture(t)
	applyPluginRequest(t, f.service, Request{Action: "add", Source: f.source, Targets: []string{"tern"}})
	other := &Service{ConfigPath: f.service.ConfigPath, StateDir: filepath.Join(filepath.Dir(f.service.StateDir), "other-state"), Run: f.service.Run}
	f.missing = false
	applyPluginRequest(t, other, Request{Action: "sync", Name: "demo"})
	if !f.installed["tern"] {
		t.Fatal("Tern not installed")
	}
	writeFile(t, f.source, "extra.txt", "changed")
	applyPluginRequest(t, other, Request{Action: "update", Name: "demo"})
	if len(f.commands) != 2 || len(f.commands[0]) != 3 || f.commands[0][1] != "install" || f.commands[0][2] == f.source || len(f.commands[1]) != 4 || f.commands[1][1] != "install" || f.commands[1][3] != "--force" {
		t.Fatalf("install/update commands: %v", f.commands)
	}
	f.missing = true
	assertTernMissingSkips(t, other)
	applyPluginRequest(t, other, Request{Action: "remove", Name: "demo", Targets: []string{"tern"}})
	state, err := f.service.load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.intent["demo"].Bindings["tern"]; ok {
		t.Fatal("declaration retained")
	}
	state, err = other.load()
	if err != nil {
		t.Fatal(err)
	}
	if state.state.Bindings["demo"]["tern"].Applied == nil {
		t.Fatal("receipt lost while Tern missing")
	}
	f.missing = false
	applyPluginRequest(t, other, Request{Action: "sync"})
	if f.installed["tern"] || len(f.commands) != 3 || strings.Join(f.commands[2], " ") != "plugin remove demo" {
		t.Fatalf("orphan cleanup: %v", f.commands)
	}
}

// Real Tern 0.9.18 list --json printed the row and envelope shown above
// TestTernInventoryIdentity (with its installed plugin values).
func TestTernExistingInstallationRequiresImport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	source := t.TempDir()
	writeFile(t, source, "plugin.toml", "schema=1\nid='tern-kata'\nname='Kata'\nversion='1'\nhost='main.luau'\n")
	writeFile(t, source, "main.luau", "return {}")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "plugin list --json" {
			return []byte(`{"plugins":[{"id":"tern-kata","name":"Kata","version":"0.17.0","description":"","icon":"puzzle","css":"","blocks":[],"lenses":[],"host":true,"window":false,"status":"ready"}],"problems":[],"jit":false,"canvas":true}`), nil
		}
		return []byte("help"), nil
	}
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: source, Targets: []string{"tern"}})
	if err != nil || !p.Blocked {
		t.Fatalf("add: %+v %v", p, err)
	}
	applyPluginRequest(t, s, Request{Action: "import", From: "tern", Plugin: "tern-kata"})
	d, err := s.load()
	if err != nil || d.packages["tern-kata"].Bindings["tern"].ID != "tern-kata" || d.packages["tern-kata"].Bindings["tern"].Version != "0.17.0" {
		t.Fatalf("import: %+v %v", d, err)
	}
}
