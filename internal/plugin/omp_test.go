package plugin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestOMPPreviewBindsNativeCacheRoot(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("OMP XDG routing is Unix-only")
	}
	agents := &fakeAgents{}
	s := agents.service(t)
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)
	r := Request{Action: "add", Source: fixture(t), Targets: []string{"omp"}}
	before, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(xdg, "omp"), 0700); err != nil {
		t.Fatal(err)
	}
	after, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if before.Revision == after.Revision {
		t.Fatal("native cache relocation did not invalidate preview")
	}
}

func TestOMPInstallGuardFailsClosed(t *testing.T) {
	root := t.TempDir()
	fileRoot := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(fileRoot, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, version, cache, source string }{
		{"unknown-root", "omp/18.6.1", "", "/reviewed"},
		{"unknown-version", "omp/19.0.0", root, "/reviewed"},
		{"unreviewed-source", "omp/18.6.1", root, ""},
		{"unreadable-root", "omp/18.6.1", fileRoot, "/reviewed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Change{Action: "install", ID: "demo@market", Binding: Binding{Version: "1.0.0", Source: tc.source}}
			ompGuard(Host{Version: tc.version, ompCacheRoot: tc.cache}, &c)
			if c.Action != "blocked" {
				t.Fatalf("unsafe install allowed: %+v", c)
			}
		})
	}
}

// OMP v18.6.1 reads a snapshot's .claude-plugin catalog as a marketplace, so the plugin domain
// manages it like Claude: a reviewed snapshot, a Skillshare marketplace, native install.
func TestOMPTargetTakesTheClaudeManifestAndItsExtensions(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, "package.json", `{"name":"demo","version":"1.0.0","omp":{"extensions":["./index.ts"]}}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	c := d.Candidates[0]
	if !slices.Contains(c.Targets, "omp") || !slices.Equal(c.TargetInfo["omp"].Components, []string{"extensions", "skills"}) {
		t.Fatalf("omp target: %+v", c.TargetInfo["omp"])
	}
	if slices.Contains(c.TargetInfo["claude"].Components, "extensions") {
		t.Fatalf("claude components changed: %v", c.TargetInfo["claude"].Components)
	}
	def := slices.IndexFunc(TargetDefinitions(), func(d TargetDefinition) bool { return d.Target == "omp" })
	if def < 0 || !TargetDefinitions()[def].Project || len(TargetDefinitions()[def].Operations) != 7 {
		t.Fatalf("omp definition: %+v", TargetDefinitions())
	}
}

// OMP rejects names with underscores; a plugin OMP cannot install is blocked there only.
func TestOMPBlocksANameItsNativeRulesRefuse(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".claude-plugin/plugin.json", `{"name":"my_demo","version":"1.0.0"}`)
	writeFile(t, root, "skills/demo/SKILL.md", "---\nname: demo\ndescription: Demo\n---\nHello")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	c := d.Candidates[0]
	if slices.Contains(c.Targets, "omp") || c.TargetInfo["omp"].ProblemKey != "plugins.problem.ompName" || !slices.Contains(c.Targets, "claude") {
		t.Fatalf("omp name rule: %+v", c)
	}
}

func TestOMPInventoryKeepsMarketplaceScopeAndListsPackagePlugins(t *testing.T) {
	data := []byte(`{"npm":[{"name":"@scope/tool","version":"2.0.0","enabled":false,"manifest":{}}],"marketplace":[
		{"id":"demo@team","scope":"project","entries":[{"scope":"project","installPath":"/p","version":"1.0.0"}]},
		{"id":"demo@team","scope":"user","entries":[{"scope":"user","installPath":"/u","version":"0.9.0","enabled":false}],"shadowedBy":"project"}]}`)
	global, err := parseInventory("omp", data, "")
	if err != nil || len(global) != 2 || global[0].ID != "demo@team" || global[0].Version != "0.9.0" || global[0].Enabled || !global[0].EnabledKnown || global[1].ID != "@scope/tool" || global[1].Enabled {
		t.Fatalf("global inventory: %+v %v", global, err)
	}
	project, err := parseInventory("omp", data, "/project")
	if err != nil || len(project) != 1 || project[0].Scope != "project" || !project[0].Enabled || project[0].Name != "demo" || project[0].Marketplace != "team" {
		t.Fatalf("project inventory: %+v %v", project, err)
	}
	if _, err := parseInventory("omp", []byte(`[]`), ""); err == nil {
		t.Fatal("a Claude-shaped list was accepted as OMP's")
	}
}

// omp plugin marketplace list has no --json in v18.6.1; its two-column text is parsed with colors removed.
func TestOMPMarketplaceListParsesTheNativeText(t *testing.T) {
	text := "Configured Marketplaces:\n\n  \x1b[36mteam\x1b[39m  \x1b[2mhttps://example.com/team\x1b[22m\n  skillshare-0123456789abcdef  /home/u/state/plugins/x/skillshare-0123456789abcdef\n"
	markets, err := parseOMPMarketplaces([]byte(text))
	if err != nil || markets["team"] != "https://example.com/team" || markets["skillshare-0123456789abcdef"] != "/home/u/state/plugins/x/skillshare-0123456789abcdef" {
		t.Fatalf("markets: %v %v", markets, err)
	}
	if markets, err := parseOMPMarketplaces([]byte("No marketplaces configured\n\nAdd one with: omp plugin marketplace add <source>\n")); err != nil || len(markets) != 0 {
		t.Fatalf("empty list: %v %v", markets, err)
	}
}

func TestOMPAddInstallsFromTheSnapshotMarketplace(t *testing.T) {
	agents := &fakeAgents{version: "1.0.0"}
	s := agents.service(t)
	source := fixture(t)
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: source, Targets: []string{"omp"}})
	if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "install" || p.Changes[0].MessageKey != "plugins.note.ompRuntime" {
		t.Fatalf("preview did not plan an install with the runtime note: %+v %v", p, err)
	}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"omp"}})
	id := p.Changes[0].ID
	if !strings.HasPrefix(id, "demo@skillshare-") || len(id) > len("demo@skillshare-")+16 {
		t.Fatalf("binding id: %s", id)
	}
	_, market, _ := strings.Cut(id, "@")
	want := []string{"omp plugin marketplace add " + agents.markets["omp"][market], "omp plugin install " + id + " --scope user"}
	if !slices.Equal(agents.commands, want) {
		t.Fatalf("commands %q, want %q", agents.commands, want)
	}
}

// omp deletes a plugin's cache folder on uninstall and replaces it on upgrade, checking only the
// registries visible from its working directory. A project elsewhere that installed the same
// identity is invisible, so Skillshare never runs those commands. Scoped removal uses
// its non-destructive adapter; reinstall chooses a new cache identity.
func TestOMPNeverRunsCacheDestructiveNativeCommands(t *testing.T) {
	agents := &fakeAgents{version: "1.0.0"}
	s := agents.service(t)
	source := fixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"omp"}})
	id := agents.installed["omp"][0].ID
	cache := ompCachePath(filepath.Join(filepath.Dir(s.ConfigPath), ".omp/plugins/cache/plugins"), id, "1.0.0")
	if err := os.MkdirAll(cache, 0755); err != nil {
		t.Fatal(err)
	}
	bumpDemo(t, source, agents)
	for _, r := range []Request{{Action: "update", Name: "demo", Targets: []string{"omp"}}, {Action: "remove", Name: "demo"}} {
		p, err := s.Preview(context.Background(), r)
		if err != nil || !p.Blocked || p.Changes[0].MessageKey != map[string]string{"update": "plugins.error.ompNativeDestructive", "remove": "plugins.error.ompRemovalSafety"}[r.Action] {
			t.Fatalf("%s was not blocked: %+v %v", r.Action, p, err)
		}
		if _, err := s.Apply(context.Background(), r, p.Revision); err == nil {
			t.Fatalf("%s applied", r.Action)
		}
	}
	applyPluginRequest(t, s, Request{Action: "disable", Name: "demo", Targets: []string{"omp"}})
	if p, err := s.Preview(context.Background(), Request{Action: "sync"}); err != nil || !p.Blocked || p.Changes[0].Action != "blocked" {
		t.Fatalf("deselected sync was not blocked: %+v %v", p, err)
	}
	if len(agents.commands) != 0 {
		t.Fatalf("native commands ran: %q", agents.commands)
	}
	// The installation is gone but project B still needs its retained cache. A
	// reinstall must choose another identity, never replace that directory.
	agents.installed["omp"] = nil
	// Reinstall the reviewed version; the earlier update probe changed this local source.
	writeFile(t, source, ".claude-plugin/plugin.json", `{"name":"demo","version":"1.0.0"}`)
	writeFile(t, source, ".codex-plugin/plugin.json", `{"name":"demo","version":"1.0.0","skills":"./skills"}`)
	applyPluginRequest(t, s, Request{Action: "enable", Name: "demo", Targets: []string{"omp"}})
	if p, err := s.Preview(context.Background(), Request{Action: "sync"}); err != nil || p.Blocked || p.Changes[0].MessageKey != "plugins.note.ompFreshCache" || p.Changes[0].ID == id {
		t.Fatalf("reinstall did not choose a fresh cache identity: %+v %v", p, err)
	}
	// Removing now only forgets the binding; nothing native runs.
	p, err := s.Preview(context.Background(), Request{Action: "remove", Name: "demo"})
	if err != nil || p.Blocked || p.Changes[0].Action != "forget" {
		t.Fatalf("remove of an uninstalled plugin: %+v %v", p, err)
	}
	applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
	if len(agents.commands) != 0 || agents.markets["omp"] == nil {
		t.Fatalf("forget ran native commands %q or dropped the marketplace", agents.commands)
	}
	// Once the folder is gone, the identity is fresh again and the install is allowed.
	if err := os.RemoveAll(cache); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"omp"}})
	if len(agents.commands) != 1 || !strings.HasPrefix(agents.commands[0], "omp plugin install ") {
		t.Fatalf("fresh install: %q", agents.commands)
	}
}

func TestOMPTargetDoesNotAdvertiseUpdate(t *testing.T) {
	i := slices.IndexFunc(TargetDefinitions(), func(d TargetDefinition) bool { return d.Target == "omp" })
	if d := TargetDefinitions()[i]; slices.Contains(d.Operations, "update") || d.ReasonKey != "plugins.reason.omp" {
		t.Fatalf("omp definition: %+v", d)
	}
}

func TestOMPImportOfAPackagePluginIsRefusedWithItsReason(t *testing.T) {
	agents := &fakeAgents{installed: map[string][]Installed{"omp": {{ID: "@scope/tool", Name: "@scope/tool", Installed: true, Enabled: true, Scope: "user"}}}}
	s := agents.service(t)
	_, err := s.Preview(context.Background(), Request{Action: "import", From: "omp", Plugin: "@scope/tool"})
	if key, _ := ErrorKey(err); key != "plugins.error.ompPackageImport" {
		t.Fatalf("import of an npm plugin: %v", err)
	}
	agents.installed["omp"] = []Installed{{ID: "demo@team", Installed: true, Enabled: true, Scope: "user", Version: "1.0.0"}}
	agents.market("omp")["team"] = "https://example.com/team"
	applyPluginRequest(t, s, Request{Action: "import", From: "omp", Plugin: "demo@team"})
	// An upgrade would take whatever the native marketplace holds, which was never reviewed.
	agents.commands = nil
	p, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo", Targets: []string{"omp"}})
	if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "skip" || p.Changes[0].MessageKey != "plugins.skip.imported" {
		t.Fatalf("imported update: %+v %v", p, err)
	}
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo", Targets: []string{"omp"}})
	if len(agents.commands) != 0 {
		t.Fatalf("imported update ran native commands: %q", agents.commands)
	}
	if p, err := s.Preview(context.Background(), Request{Action: "check", Name: "demo"}); err != nil || p.Changes[0].Action != "native-check" {
		t.Fatalf("imported check: %+v %v", p, err)
	}
	if p, err := s.Preview(context.Background(), Request{Action: "remove", Name: "demo"}); err != nil || !p.Blocked || p.Changes[0].MessageKey != "plugins.error.ompRemovalSafety" {
		t.Fatalf("unverifiable imported remove ran natively: %+v %v", p, err)
	}
}

// omp prefers .omp-plugin/marketplace.json (discovery/claude-plugins.ts) and reads
// .omp-plugin/plugin.json ahead of .claude-plugin/plugin.json, but records the version it finds
// in .claude-plugin/plugin.json, plugin.json or package.json (marketplace/manager.ts).
func TestOMPNativeCatalogAndManifestAreDiscovered(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".omp-plugin/marketplace.json", `{"name":"omp-market","owner":{"name":"me"},"plugins":[{"name":"demo","source":"./demo"}]}`)
	writeFile(t, root, "demo/.omp-plugin/plugin.json", `{"name":"demo","version":"9.9.9","mcpServers":{"x":{"command":"x"}}}`)
	writeFile(t, root, "demo/package.json", `{"name":"demo","version":"1.2.3"}`)
	writeFile(t, root, "demo/skills/demo/SKILL.md", "---\nname: demo\ndescription: Demo\n---\nHello")
	d, err := Discover(context.Background(), root)
	if err != nil || len(d.Candidates) != 1 {
		t.Fatalf("discovery: %+v %v", d, err)
	}
	c := d.Candidates[0]
	info := c.TargetInfo["omp"]
	if c.Marketplace != "omp-market" || !slices.Equal(c.Targets, []string{"omp"}) || info.Manifest != ".omp-plugin/plugin.json" || info.Version != "1.2.3" || !slices.Equal(info.Components, []string{"mcpServers", "skills"}) {
		t.Fatalf("omp candidate: %+v", c)
	}
	agents := &fakeAgents{version: "1.2.3"}
	s := agents.service(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: root, Targets: []string{"omp"}})
	if len(agents.commands) != 2 || !strings.HasPrefix(agents.commands[1], "omp plugin install demo@skillshare-") {
		t.Fatalf("install: %q", agents.commands)
	}
}

// Project plugins live where OMP anchors them: the nearest .omp folder, else the nearest Git
// root. A project elsewhere would be written into another root, so it is blocked.
func TestOMPProjectHostRequiresTheNativeAnchor(t *testing.T) {
	agents := &fakeAgents{}
	s := agents.service(t)
	home := filepath.Dir(s.ConfigPath)
	parent := filepath.Join(home, "work")
	writeFile(t, parent, ".git/HEAD", "ref: refs/heads/main\n")
	s.ProjectRoot = filepath.Join(parent, "nested")
	if err := os.MkdirAll(s.ProjectRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if h := s.host(context.Background(), "omp"); h.Status != HostBlocked || h.ErrorKey != "plugins.error.ompProjectAnchor" || !strings.Contains(h.Error, parent) {
		t.Fatalf("nested project: %+v", h)
	}
	if err := os.MkdirAll(filepath.Join(s.ProjectRoot, ".omp"), 0755); err != nil {
		t.Fatal(err)
	}
	if h := s.host(context.Background(), "omp"); h.Error != "" {
		t.Fatalf("anchored project: %+v", h)
	}
	applyPluginRequest(t, s, Request{Action: "add", Source: fixture(t), Targets: []string{"omp"}})
	if len(agents.commands) != 2 || !strings.HasSuffix(agents.commands[1], "--scope project") || agents.installed["omp"][0].Scope != "project" {
		t.Fatalf("project install: %q %+v", agents.commands, agents.installed["omp"])
	}
}

// A profile or config-dir override moves OMP's roots; Skillshare cannot prove where its commands
// would write, so the host is blocked instead of guessing.
func TestOMPHostBlocksWhenNativeRootOverridesAreSet(t *testing.T) {
	agents := &fakeAgents{}
	s := agents.service(t)
	t.Setenv("OMP_PROFILE", "work")
	if h := s.host(context.Background(), "omp"); h.Status != HostBlocked || h.ErrorKey != "plugins.error.ompRootEnv" {
		t.Fatalf("profile override: %+v", h)
	}
}

// An OMP account is another config directory; plugins there are not routed (#259), so the
// account is not offered as a plugin target.
func TestOMPAccountsAreNotPluginTargets(t *testing.T) {
	s := &Service{ConfigPath: filepath.Join(t.TempDir(), "config.yaml"), Accounts: map[string]Account{"omp-work": {Agent: "omp", Dir: t.TempDir()}}}
	if slices.Contains(s.targets(), "omp-work") || slices.ContainsFunc(s.TargetDefinitions(), func(d TargetDefinition) bool { return d.Target == "omp-work" }) {
		t.Fatalf("omp account advertised: %v", s.targets())
	}
}
