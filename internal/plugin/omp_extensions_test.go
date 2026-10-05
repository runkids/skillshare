package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ompFile(t *testing.T, root, name, data string) string {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func ompFixture(t *testing.T) (*Service, string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, key := range []string{"PI_CONFIG_DIR", "PI_CODING_AGENT_DIR", "OMP_PROFILE", "PI_PROFILE", "PI_CONFIG_FILES", "XDG_DATA_HOME"} {
		t.Setenv(key, "")
	}
	return &Service{ConfigPath: filepath.Join(home, "skillshare/config.yaml"), StateDir: filepath.Join(home, "state"), Run: func(context.Context, string, []string, string, ...string) ([]byte, error) {
		t.Fatal("inventory executed a CLI")
		return nil, nil
	}}, home, filepath.Join(home, ".omp/agent")
}

func ompView(t *testing.T, s *Service, target string) *OMPExtensionsView {
	t.Helper()
	v, err := s.OMPExtensions(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if !v.ReadOnly || v.Rows == nil || v.Reasons == nil || v.Warnings == nil {
		t.Fatalf("invalid view: %+v", v)
	}
	return v
}

func ompRow(t *testing.T, v *OMPExtensionsView, p, source string) OMPExtensionRow {
	t.Helper()
	for _, row := range v.Rows {
		if row.Path == p && row.Source == source {
			return row
		}
	}
	t.Fatalf("missing %s (%s): %+v", p, source, v.Rows)
	return OMPExtensionRow{}
}

func TestOMPExtensionsNativeAndConfiguredPrecedence(t *testing.T) {
	s, home, agent := ompFixture(t)
	project := filepath.Join(home, "repo")
	s.ProjectRoot = project
	global := ompFile(t, agent, "extensions/same.ts", "throw new Error('never execute');")
	local := ompFile(t, project, ".omp/extensions/same.ts", "export default () => {}")
	other := ompFile(t, agent, "extensions/index.ts", "x")
	native := ompFile(t, agent, "extensions/other.js", "x")
	pack := filepath.Join(home, "pack")
	index := ompFile(t, pack, "index.ts", "x")
	ompFile(t, pack, "ignored.ts", "x")
	ompFile(t, project, ".omp/config.yml", "extensions:\n  - "+pack+"\n")
	v := ompView(t, s, "omp")
	if v.Scope != "project" || v.Root != filepath.Join(project, ".omp") {
		t.Fatalf("scope: %+v", v)
	}
	for _, p := range []string{local, other, native} {
		if r := ompRow(t, v, p, "native"); r.Selection != "selected" {
			t.Fatalf("row: %+v", r)
		}
	}
	if r := ompRow(t, v, global, "native"); r.Selection != "shadowed" {
		t.Fatalf("global collision: %+v", r)
	}
	if r := ompRow(t, v, index, "configured"); r.Selection != "selected" || r.DerivedID != "extension-module:pack" {
		t.Fatalf("configured: %+v", r)
	}
	for _, r := range v.Rows {
		if r.Path == filepath.Join(pack, "ignored.ts") {
			t.Fatal("configured index did not suppress scan")
		}
	}
	// Native project discovery is cwd-only, not an ancestor walk.
	ancestor := ompFile(t, home, ".omp/extensions/ancestor.ts", "x")
	for _, r := range v.Rows {
		if r.Path == ancestor {
			t.Fatal("native walked ancestors")
		}
	}
}

func TestOMPExtensionsManifestAndExplicitBypass(t *testing.T) {
	s, home, agent := ompFixture(t)
	native := ompFile(t, agent, "extensions/native.ts", "x")
	explicit := ompFile(t, home, "explicit.mjs", "x")
	dir := filepath.Join(home, "pack")
	chosen := ompFile(t, dir, "selected.cjs", "x")
	ompFile(t, dir, "index.ts", "x")
	ompFile(t, dir, "legacy.js", "x")
	ompFile(t, dir, "package.json", `{"omp":{"extensions":["selected.cjs","missing.ts"]},"pi":{"extensions":["legacy.js"]}}`)
	ompFile(t, agent, "config.yml", "extensions:\n  - "+explicit+"\n  - "+dir+"\ndisabledExtensions:\n  - extension-module:native\n  - extension-module:explicit\n  - extension-module:selected\n")
	v := ompView(t, s, "omp")
	if ompRow(t, v, native, "native").Selection != "disabled" || ompRow(t, v, explicit, "configured").Selection != "selected" || ompRow(t, v, chosen, "configured").Selection != "disabled" {
		t.Fatalf("selection: %+v", v.Rows)
	}
	for _, row := range v.Rows {
		if row.Path == filepath.Join(dir, "index.ts") || row.Path == filepath.Join(dir, "legacy.js") {
			t.Fatal("manifest precedence ignored")
		}
	}
}

func TestOMPExtensionsLegacyAndHookIDs(t *testing.T) {
	s, home, agent := ompFixture(t)
	ext := ompFile(t, home, "legacy.ts", "x")
	hook := ompFile(t, agent, "hooks/pre/bash.ts", "x")
	ompFile(t, agent, "hooks/bash.ts", "x")
	ompFile(t, agent, "settings.json", `{"extensions":["`+ext+`"]}`)
	ompFile(t, agent, "config.yaml", "disabledExtensions:\n  - extension-module:bash\n  - extension-module:legacy\n")
	v := ompView(t, s, "omp")
	if ompRow(t, v, ext, "native").Selection != "disabled" {
		t.Fatal("legacy native path not filtered")
	}
	r := ompRow(t, v, hook, "hook")
	if r.DerivedID == "extension-module:bash" || r.Selection != "selected" {
		t.Fatalf("hook ID: %+v", r)
	}
	for _, r := range v.Rows {
		if r.Path == filepath.Join(agent, "hooks/bash.ts") {
			t.Fatal("scanned hooks outside pre/post")
		}
	}
}

func TestOMPExtensionsAccountIsolationAndUnsupportedPaths(t *testing.T) {
	s, home, agent := ompFixture(t)
	ompFile(t, agent, "extensions/default.ts", "x")
	ompFile(t, home, ".omp/plugins/package.json", `{"dependencies":{"default-plugin":"1"}}`)
	ompFile(t, home, ".omp/plugins/node_modules/default-plugin/package.json", `{"omp":{"extensions":["index.ts"]}}`)
	ompFile(t, home, ".omp/plugins/node_modules/default-plugin/index.ts", "x")
	account := filepath.Join(home, "unrelated/account")
	p := ompFile(t, account, "extensions/account.ts", "x")
	ompFile(t, account, "config.yml", "extensions:\n  - https://user:do-not-send@example.com/ext.ts\n  - local://secret-alias\n")
	s.Accounts = map[string]Account{"work": {Agent: "omp", Dir: account}}
	v := ompView(t, s, "work")
	if v.Scope != "account" || v.Root != account || ompRow(t, v, p, "native").Selection != "selected" {
		t.Fatalf("account: %+v", v)
	}
	for _, r := range v.Rows {
		if strings.Contains(r.Path, "default") {
			t.Fatalf("default-user leak: %+v", r)
		}
	}
	data, _ := json.Marshal(v)
	if strings.Contains(string(data), "do-not-send") || strings.Contains(string(data), "secret-alias") {
		t.Fatal("source credentials leaked")
	}
	if len(v.Warnings) == 0 {
		t.Fatal("unknown plugin root or source must be reported")
	}
	if _, err := s.OMPExtensions(context.Background(), "pi"); err == nil {
		t.Fatal("Pi accepted")
	}
}

func TestOMPExtensionsMalformedAndNoExecution(t *testing.T) {
	s, home, agent := ompFixture(t)
	sentinel := filepath.Join(home, "executed")
	p := ompFile(t, agent, "extensions/danger.ts", "require('fs').writeFileSync('"+sentinel+"','bad');")
	ompFile(t, agent, "config.yml", "extensions: [\npassword: do-not-send")
	before, _ := os.ReadFile(filepath.Join(agent, "config.yml"))
	v := ompView(t, s, "omp")
	if ompRow(t, v, p, "native").Selection != "unknown" || len(v.Warnings) == 0 {
		t.Fatalf("malformed: %+v", v)
	}
	after, _ := os.ReadFile(filepath.Join(agent, "config.yml"))
	if string(before) != string(after) {
		t.Fatal("settings changed or migrated")
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("extension executed")
	}
	data, _ := json.Marshal(v)
	if strings.Contains(string(data), "do-not-send") || strings.Contains(string(data), "writeFileSync") {
		t.Fatal("raw content leaked")
	}
}

func TestOMPExtensionsLockOnlyPlugins(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(fmt.Sprintf("linked=%t", linked), func(t *testing.T) {
			s, home, _ := ompWritableFixture(t)
			root := filepath.Join(home, ".omp/plugins")
			ompFile(t, root, "omp-plugins.lock.json", `{"plugins":{"demo":{"enabled":true,"enabledFeatures":null}},"settings":{}}`)
			dir := filepath.Join(root, "node_modules/demo")
			if linked {
				dir = filepath.Join(root, "cache/demo")
				if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(dir, filepath.Join(root, "node_modules/demo")); err != nil {
					t.Fatal(err)
				}
			}
			ompFile(t, dir, "package.json", `{"name":"demo","version":"1.2.3","omp":{"extensions":["index.ts"]}}`)
			ompFile(t, dir, "index.ts", "throw new Error('must never execute');")
			for _, withPackage := range []bool{false, true} {
				if withPackage {
					ompFile(t, root, "package.json", `{"dependencies":{}}`)
				}
				v, err := s.OMPExtensions(context.Background(), "omp")
				if err != nil {
					t.Fatal(err)
				}
				row := ompRow(t, v, filepath.Join(root, "node_modules/demo/index.ts"), "plugin")
				if row.Selection != "selected" || v.ReadOnly {
					t.Fatalf("valid lock-only plugin became unknown: %+v", v)
				}
				if row.PluginName != "demo" || row.PluginRoot != filepath.Join(root, "node_modules/demo") || row.PluginVersion != "1.2.3" {
					t.Fatalf("plugin identity was replaced by its extension folder: %+v", row)
				}
			}
		})
	}
}

func TestOMPExtensionsPluginInventory(t *testing.T) {
	s, home, _ := ompFixture(t)
	root := filepath.Join(home, ".omp/plugins")
	ompFile(t, root, "package.json", `{"dependencies":{"tools":"1","off":"1"}}`)
	ompFile(t, root, "omp-plugins.lock.json", `{"plugins":{"tools":{"enabled":true,"enabledFeatures":["extra"]},"off":{"enabled":false}}}`)
	p := ompFile(t, root, "node_modules/tools/ext/index.mjs", "x")
	extra := ompFile(t, root, "node_modules/tools/extra.cjs", "x")
	off := ompFile(t, root, "node_modules/off/index.ts", "x")
	ompFile(t, root, "node_modules/tools/package.json", `{"omp":{"extensions":["ext"],"features":{"extra":{"extensions":["extra.cjs"]}}}}`)
	ompFile(t, root, "node_modules/off/package.json", `{"omp":{"extensions":"index.ts"}}`)
	v := ompView(t, s, "omp")
	if ompRow(t, v, p, "plugin").Selection != "selected" || ompRow(t, v, extra, "plugin").Owner != "plugin" || ompRow(t, v, off, "plugin").Selection != "disabled" {
		t.Fatalf("plugins: %+v", v.Rows)
	}
	if row := ompRow(t, v, p, "plugin"); row.PluginName != "tools" || row.PluginRoot != filepath.Join(root, "node_modules/tools") || row.PluginVersion != "" {
		t.Fatalf("native plugin identity or unknown version: %+v", row)
	}
	// Missing or malformed state is unknown rather than a fake disabled state.
	ompFile(t, root, "omp-plugins.lock.json", `{"plugins":`)
	v = ompView(t, s, "omp")
	if ompRow(t, v, p, "plugin").Selection != "unknown" {
		t.Fatalf("malformed lock: %+v", v.Rows)
	}
}

func TestOMPExtensionsNativeDirectFileWinsIndex(t *testing.T) {
	s, _, agent := ompFixture(t)
	direct := ompFile(t, agent, "extensions/same.ts", "x")
	index := ompFile(t, agent, "extensions/same/index.ts", "x")
	v := ompView(t, s, "omp")
	if ompRow(t, v, direct, "native").Selection != "selected" || ompRow(t, v, index, "native").Selection != "shadowed" {
		t.Fatalf("native ordering: %+v", v.Rows)
	}
}

func TestOMPExtensionsEmptyYAMLAndUnsupportedRootEnv(t *testing.T) {
	s, home, agent := ompFixture(t)
	p := ompFile(t, agent, "extensions/default.ts", "x")
	ompFile(t, agent, "config.yml", "# intentionally empty\n")
	v := ompView(t, s, "omp")
	if ompRow(t, v, p, "native").Selection != "selected" || len(v.Warnings) != 0 {
		t.Fatalf("empty yaml: %+v", v)
	}
	t.Setenv("OMP_PROFILE", "work")
	ompFile(t, home, ".omp/profiles/work/agent/extensions/profile.ts", "x")
	v = ompView(t, s, "omp")
	if v.Root != agent || ompRow(t, v, p, "native").Selection != "unknown" {
		t.Fatalf("profile auto-routing: %+v", v)
	}
	for _, row := range v.Rows {
		if strings.Contains(row.Path, "profiles/work") {
			t.Fatal("automatic profile discovery")
		}
	}
}

func TestOMPExtensionsUnsupportedRowsHaveSafeUniqueIDs(t *testing.T) {
	s, _, agent := ompFixture(t)
	ompFile(t, agent, "config.yml", "extensions:\n  - https://user:do-not-send@example.com/a.ts\n  - local://do-not-send\n  - ./unknown\n")
	v := ompView(t, s, "omp")
	ids := map[string]bool{}
	for _, row := range v.Rows {
		if row.Name == "" || row.DerivedID == "" || ids[row.DerivedID] || row.Selection != "unknown" {
			t.Fatalf("unsafe identity: %+v", row)
		}
		ids[row.DerivedID] = true
	}
	if len(v.Rows) != 2 {
		t.Fatalf("rows: %+v", v.Rows)
	}
}

func TestOMPExtensionsProjectHooksOwnership(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(fmt.Sprint(global), func(t *testing.T) {
			s, home, _ := ompFixture(t)
			s.ProjectRoot = filepath.Join(home, "repo")
			owner := s.ConfigPath
			root := ""
			if global {
				root = s.ProjectRoot
			} else {
				s.ConfigPath = filepath.Join(s.ProjectRoot, ".skillshare/config.yaml")
				owner = s.ConfigPath
			}
			p := ompFile(t, s.ProjectRoot, ".omp/extensions/skillshare-project.ts", "x")
			ledger := `{"version":1,"records":{"` + hash([]byte("file\x00"+p)) + `":{"owner":"` + owner + `","target":"omp","root":"` + root + `","path":"` + p + `","hash":"` + hash([]byte("x")) + `"}}}`
			ompFile(t, s.StateDir, "hooks/state.json", ledger)
			row := ompRow(t, ompView(t, s, "omp"), p, "native")
			if row.Owner != "hooks" || row.HooksTarget != "omp" {
				t.Fatalf("project ownership: %+v", row)
			}
		})
	}
}

func TestOMPExtensionsNativeProjectLegacyWinsGlobalName(t *testing.T) {
	s, home, agent := ompFixture(t)
	s.ProjectRoot = filepath.Join(home, "repo")
	global := ompFile(t, agent, "extensions/shared.ts", "x")
	project := ompFile(t, s.ProjectRoot, "local/shared.ts", "x")
	ompFile(t, s.ProjectRoot, ".omp/settings.json", `{"extensions":["`+project+`"]}`)
	v := ompView(t, s, "omp")
	if ompRow(t, v, project, "native").Selection != "selected" || ompRow(t, v, global, "native").Selection != "shadowed" {
		t.Fatalf("project legacy precedence: %+v", v.Rows)
	}
}

func TestOMPExtensionsOverlayUnknownAndAbsoluteDedup(t *testing.T) {
	s, _, agent := ompFixture(t)
	p := ompFile(t, agent, "extensions/shared.ts", "x")
	ompFile(t, agent, "config.yml", "extensions:\n  - "+p+"\n")
	v := ompView(t, s, "omp")
	if ompRow(t, v, p, "configured").Selection != "shadowed" {
		t.Fatalf("path dedup: %+v", v.Rows)
	}
	t.Setenv("PI_CONFIG_FILES", "/unknown/overlay.yml")
	v = ompView(t, s, "omp")
	if ompRow(t, v, p, "native").Selection != "unknown" {
		t.Fatalf("overlay: %+v", v.Rows)
	}
}

func TestOMPExtensionsPluginProjectPrecedenceAndMissingLock(t *testing.T) {
	s, home, _ := ompFixture(t)
	s.ProjectRoot = filepath.Join(home, "repo")
	paths := map[string]string{}
	for _, scope := range []string{"global", "project"} {
		root := filepath.Join(home, ".omp/plugins")
		if scope == "project" {
			root = filepath.Join(s.ProjectRoot, ".omp/plugins")
		}
		ompFile(t, root, "package.json", `{"dependencies":{"tools":"1"}}`)
		ompFile(t, root, "omp-plugins.lock.json", `{"plugins":{}}`)
		ompFile(t, root, "node_modules/tools/package.json", `{"omp":{"extensions":["ext"]}}`)
		paths[scope] = ompFile(t, root, "node_modules/tools/ext/index.mjs", "x")
		ompFile(t, root, "node_modules/tools/ext/ignored.d.mts", "x")
	}
	v := ompView(t, s, "omp")
	if ompRow(t, v, paths["global"], "plugin").Selection != "shadowed" || ompRow(t, v, paths["project"], "plugin").Selection != "selected" {
		t.Fatalf("project plugins: %+v", v.Rows)
	}
	// A lockless dependency is inventoried but does not claim an effective enabled state.
	if err := os.Remove(filepath.Join(home, ".omp/plugins/omp-plugins.lock.json")); err != nil {
		t.Fatal(err)
	}
	s.ProjectRoot = ""
	if ompRow(t, ompView(t, s, "omp"), paths["global"], "plugin").Selection != "unknown" {
		t.Fatal("missing lock reported selected")
	}
}

func TestOMPExtensionsUnsupportedManifestDoesNotExposeSources(t *testing.T) {
	s, home, agent := ompFixture(t)
	dir := filepath.Join(home, "pack")
	ompFile(t, dir, "package.json", `{"omp":{"extensions":["https://user:do-not-send@example.com/a.ts"]}}`)
	ompFile(t, agent, "config.yml", "extensions:\n  - "+dir+"\n")
	data, _ := json.Marshal(ompView(t, s, "omp"))
	if strings.Contains(string(data), "do-not-send") {
		t.Fatal("manifest source leaked")
	}
}

func TestOMPExtensionsAccountIgnoresRelativeAgentEnv(t *testing.T) {
	s, home, _ := ompFixture(t)
	account := filepath.Join(home, "work/agent")
	p := ompFile(t, account, "extensions/work.ts", "x")
	s.Accounts = map[string]Account{"work": {Agent: "omp", Dir: account}}
	t.Setenv("PI_CODING_AGENT_DIR", "relative/default")
	if ompRow(t, ompView(t, s, "work"), p, "native").Selection != "selected" {
		t.Fatal("explicit account consulted ambient agent env")
	}
}

func TestOMPExtensionsMissingInstalledManifest(t *testing.T) {
	s, home, _ := ompFixture(t)
	root := filepath.Join(home, ".omp/plugins")
	ompFile(t, root, "package.json", `{"dependencies":{"tools":"1"}}`)
	ompFile(t, root, "omp-plugins.lock.json", `{"plugins":{}}`)
	ompFile(t, root, "node_modules/tools/index.ts", "x")
	v := ompView(t, s, "omp")
	if len(v.Rows) != 1 || v.Rows[0].Selection != "unknown" || v.Rows[0].DerivedID == "" {
		t.Fatalf("missing manifest: %+v", v.Rows)
	}
}

func TestOMPExtensionsRepeatedUnsupportedPluginEntries(t *testing.T) {
	s, home, _ := ompFixture(t)
	root := filepath.Join(home, ".omp/plugins")
	ompFile(t, root, "package.json", `{"dependencies":{"tools":"1"}}`)
	ompFile(t, root, "omp-plugins.lock.json", `{"plugins":{}}`)
	ompFile(t, root, "node_modules/tools/package.json", `{"omp":{"extensions":["https://user:do-not-send@example.com/a.ts","local://do-not-send"]}}`)
	v := ompView(t, s, "omp")
	if len(v.Rows) != 1 || v.Rows[0].DerivedID == "" || v.Rows[0].Selection != "unknown" {
		t.Fatalf("repeated diagnostic identities: %+v", v.Rows)
	}
	data, _ := json.Marshal(v)
	if strings.Contains(string(data), "do-not-send") {
		t.Fatal("plugin source leaked")
	}
}

func TestOMPExtensionsHooksOwnershipRequiresLedger(t *testing.T) {
	s, _, agent := ompFixture(t)
	p := ompFile(t, agent, "extensions/skillshare-owned.ts", "x")
	spoof := ompFile(t, agent, "extensions/skillshare-spoof.ts", "x")
	ledger := `{"version":1,"records":{"` + hash([]byte("file\x00"+p)) + `":{"owner":"` + s.ConfigPath + `","target":"omp","path":"` + p + `","hash":"` + hash([]byte("x")) + `"}}}`
	ompFile(t, s.StateDir, "hooks/state.json", ledger)
	v := ompView(t, s, "omp")
	if r := ompRow(t, v, p, "native"); r.Owner != "hooks" || r.HooksTarget != "omp" {
		t.Fatalf("ownership: %+v", r)
	}
	if ompRow(t, v, spoof, "native").Owner != "native" {
		t.Fatal("filename-only ownership")
	}
	ompFile(t, agent, "extensions/skillshare-owned.ts", "changed")
	if ompRow(t, ompView(t, s, "omp"), p, "native").Owner == "hooks" {
		t.Fatal("changed file claimed")
	}
}
