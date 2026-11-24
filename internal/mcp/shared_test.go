package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sharedConfig = `mcp:
  targets: [cursor]
  servers:
    everywhere:
      command: everywhere-server
    docs:
      global: false
      url: https://docs.example.com/mcp
  projects:
    $TMP/api:
      use: [docs]
    $TMP/web:
      use: [docs]
      servers:
        local:
          command: local-server
    $TMP/blog: {}
`

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

func TestSharedServerReachesOnlyTheRootsThatUseIt(t *testing.T) {
	s, tmp := projectsService(t, sharedConfig)
	applyProjects(t, s)
	global := readFile(t, filepath.Join(tmp, ".cursor", "mcp.json"))
	if !strings.Contains(global, "everywhere-server") || strings.Contains(global, "docs.example.com") {
		t.Fatalf("global file: %s", global)
	}
	for _, root := range []string{"api", "web"} {
		if got := readFile(t, filepath.Join(tmp, root, ".cursor", "mcp.json")); !strings.Contains(got, "docs.example.com") {
			t.Fatalf("%s: shared server missing: %s", root, got)
		}
	}
	if got := readFile(t, filepath.Join(tmp, "blog", ".cursor", "mcp.json")); strings.Contains(got, "docs.example.com") {
		t.Fatalf("blog got a server it does not use: %s", got)
	}
}

func TestEditingASharedServerUpdatesEveryRootThatUsesIt(t *testing.T) {
	s, tmp := projectsService(t, sharedConfig)
	applyProjects(t, s)
	server := Server{URL: "https://docs.example.com/v2"}
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, "", false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, s.ConfigPath); !strings.Contains(got, "global: false") {
		t.Fatalf("an edit that did not mention global dropped it:\n%s", got)
	}
	applyProjects(t, s)
	for _, root := range []string{"api", "web"} {
		if got := readFile(t, filepath.Join(tmp, root, ".cursor", "mcp.json")); !strings.Contains(got, "/v2") {
			t.Fatalf("%s not updated: %s", root, got)
		}
	}
	if got := readFile(t, filepath.Join(tmp, ".cursor", "mcp.json")); strings.Contains(got, "docs.example.com") {
		t.Fatalf("edit made the shared server global: %s", got)
	}
}

func TestTurningAGlobalServerIntoASharedOneMovesItsEntries(t *testing.T) {
	s, tmp := projectsService(t, sharedConfig)
	applyProjects(t, s)
	off := false
	server := Server{Command: "everywhere-server", Global: &off}
	if _, err := s.MutateBatch([]Mutation{
		{Name: "everywhere", Server: &server, Replace: true},
		{Project: tmp + "/blog", Name: "everywhere", Using: new(true)},
	}, "", false); err != nil {
		t.Fatal(err)
	}
	applyProjects(t, s)
	if got := readFile(t, filepath.Join(tmp, ".cursor", "mcp.json")); strings.Contains(got, "everywhere-server") {
		t.Fatalf("global entry kept: %s", got)
	}
	if got := readFile(t, filepath.Join(tmp, "blog", ".cursor", "mcp.json")); !strings.Contains(got, "everywhere-server") {
		t.Fatalf("blog entry missing: %s", got)
	}
}

func TestUsingTurnsOneSharedServerOnOrOff(t *testing.T) {
	s, tmp := projectsService(t, sharedConfig)
	if _, err := s.Mutate(Mutation{Project: tmp + "/blog", Name: "docs", Using: new(true)}, "", false); err != nil {
		t.Fatal(err)
	}
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := source.Projects[filepath.Join(tmp, "blog")].Use; len(got) != 1 || got[0] != "docs" {
		t.Fatalf("use = %v", got)
	}
	if _, err := s.Mutate(Mutation{Project: tmp + "/api", Name: "docs", Using: new(false)}, "", false); err != nil {
		t.Fatal(err)
	}
	if source, err = LoadSource(s.ConfigPath); err != nil {
		t.Fatal(err)
	}
	if got := source.Projects[filepath.Join(tmp, "api")]; got.Use != nil {
		t.Fatalf("empty use kept: %+v", got)
	}
}

func TestUsingNeedsOnlyAProjectAndAName(t *testing.T) {
	s, tmp := projectsService(t, sharedConfig)
	for _, m := range []Mutation{
		{Name: "docs", Using: new(true)},
		{Project: tmp + "/blog", Using: new(true)},
		{Project: tmp + "/blog", Name: "docs", Using: new(true), Remove: true},
	} {
		if _, err := s.Mutate(m, "", false); err == nil || !strings.Contains(err.Error(), "using") {
			t.Fatalf("%+v: want a refusal, got %v", m, err)
		}
	}
}

func TestRemovingOrGlobalizingASharedServerClearsEveryUse(t *testing.T) {
	for name, m := range map[string]Mutation{
		"remove": {Name: "docs", Remove: true},
		"global": {Name: "docs", Server: &Server{URL: "https://docs.example.com/mcp", Global: new(bool(true))}, Replace: true},
	} {
		t.Run(name, func(t *testing.T) {
			s, tmp := projectsService(t, sharedConfig)
			if _, err := s.Mutate(m, "", false); err != nil {
				t.Fatal(err)
			}
			source, err := LoadSource(s.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, root := range []string{"api", "web"} {
				if use := source.Projects[filepath.Join(tmp, root)].Use; len(use) != 0 {
					t.Fatalf("%s still uses docs: %v", root, use)
				}
			}
			if strings.Contains(readFile(t, s.ConfigPath), "global: true") {
				t.Fatalf("global: true was written:\n%s", readFile(t, s.ConfigPath))
			}
		})
	}
}

func TestSharedServerRules(t *testing.T) {
	for name, tc := range map[string]struct{ config, want string }{
		"unknown name": {`mcp:
  targets: [cursor]
  projects:
    $TMP/api: {use: [missing]}
`, "missing"},
		"global server": {`mcp:
  targets: [cursor]
  servers:
    everywhere: {command: x}
  projects:
    $TMP/api: {use: [everywhere]}
`, "already loads in every project"},
		"listed twice": {`mcp:
  targets: [cursor]
  servers:
    docs: {command: x, global: false}
  projects:
    $TMP/api: {use: [docs, docs]}
`, "twice"},
		"used and declared": {`mcp:
  targets: [cursor]
  servers:
    docs: {command: x, global: false}
  projects:
    $TMP/api:
      use: [docs]
      servers:
        docs: {command: y}
`, "both"},
		"used and switched off": {`mcp:
  targets: [cursor]
  servers:
    docs: {command: x, global: false}
  projects:
    $TMP/api:
      use: [docs]
      servers:
        docs: {disabled: true, targets: [opencode]}
`, "both"},
		"global inside a project": {`mcp:
  targets: [cursor]
  projects:
    $TMP/api:
      servers:
        docs: {command: x, global: false}
`, "global"},
		"global with disabled": {`mcp:
  targets: [opencode]
  servers:
    docs: {disabled: true, global: false}
`, "global"},
		"account target": {`targets:
  claude-work: {agent: claude, config_dir: ~/.claude-work}
mcp:
  targets: [cursor]
  servers:
    docs: {command: x, global: false, targets: [claude-work]}
`, "claude-work"},
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := projectsService(t, tc.config)
			_, err := s.Preview()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestSharedServerIsRefusedInProjectMode(t *testing.T) {
	s, tmp := projectsService(t, `mcp:
  targets: [cursor]
  servers:
    docs: {command: x, global: false}
`)
	s.ProjectRoot = tmp
	if _, err := s.Preview(); err == nil || !strings.Contains(err.Error(), "global config") {
		t.Fatalf("want a refusal, got %v", err)
	}
	off := false
	s2, tmp2 := projectsService(t, "mcp:\n  targets: [cursor]\n")
	s2.ProjectRoot = tmp2
	if _, err := s2.Mutate(Mutation{Name: "docs", Server: &Server{Command: "x", Global: &off}}, "", false); err == nil || !strings.Contains(err.Error(), "global config") {
		t.Fatalf("want a refusal, got %v", err)
	}
}

func TestSwitchNamingASharedServerIsNotAGlobalSwitch(t *testing.T) {
	// A shared server is never in the Agent's global file, so turning it off elsewhere
	// follows the same rules as turning off a name Skillshare does not define.
	s, _ := projectsService(t, `mcp:
  targets: [opencode]
  servers:
    docs: {command: x, global: false}
  projects:
    $TMP/api:
      servers:
        docs: {disabled: true}
`)
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, server := range followingSwitches(source.Projects[filepath.Join(filepath.Dir(s.ConfigPath), "api")].Servers, source.Targets, source) {
		if server.global != nil {
			t.Fatal("a shared server was treated as the global server of that name")
		}
	}
}

func TestCheckCoversASharedServer(t *testing.T) {
	s, _ := projectsService(t, sharedConfig)
	report, err := s.Check(offlineCheck())
	if err != nil {
		t.Fatal(err)
	}
	synced := 0
	for _, f := range findings(t, report, "docs") {
		if f.Level == "error" {
			t.Fatalf("shared server reported as broken: %+v", f)
		}
		if f.Check == "sync" {
			synced++
			if !strings.Contains(f.Message, string(filepath.Separator)) {
				t.Fatalf("sync finding does not name its project: %+v", f)
			}
		}
	}
	// One entry in each of the two projects that use it.
	if synced != 2 {
		t.Fatalf("want a sync finding per project that uses docs, got %d", synced)
	}
}

func TestCheckPassesTheOwnServersOfAProjectThatUsesASharedOne(t *testing.T) {
	s, _ := projectsService(t, sharedConfig)
	report, err := s.Check(offlineCheck())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings(t, report, "local") {
		if f.Level == "error" {
			t.Fatalf("own server beside a shared one reported as broken: %+v", f)
		}
	}
}

func TestCheckRunsASharedServerForTheTargetsOfItsProjects(t *testing.T) {
	// Claude Desktop takes no remote server, but only the global list names it.
	s, _ := projectsService(t, `mcp:
  targets: [claude-desktop]
  servers:
    docs:
      global: false
      url: https://docs.example.com/mcp
  projects:
    $TMP/api:
      targets: [cursor]
      use: [docs]
`)
	report, err := s.Check(offlineCheck())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings(t, report, "docs") {
		if f.Level == "error" {
			t.Fatalf("checked for an Agent no project that uses it has: %+v", f)
		}
	}
}

func TestCheckSaysASharedServerNoProjectUsesIsUnused(t *testing.T) {
	s, _ := projectsService(t, `mcp:
  targets: [cursor]
  servers:
    docs:
      global: false
      url: https://docs.example.com/mcp
`)
	report, err := s.Check(offlineCheck())
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(findings(t, report, "docs"), "info", "use", "") {
		t.Fatalf("want an info finding that no project uses docs, got %+v", findings(t, report, "docs"))
	}
}

func TestRenderNativeShowsASharedServerInTheProjectsThatUseIt(t *testing.T) {
	s, tmp := projectsService(t, sharedConfig)
	off := false
	rendered := s.RenderNative("docs", Server{URL: "https://docs.example.com/mcp", Targets: TargetList{"cursor"}, Global: &off})
	var paths []string
	for _, r := range rendered {
		paths = append(paths, r.Path)
	}
	want := []string{filepath.Join(tmp, "api", ".cursor", "mcp.json"), filepath.Join(tmp, "web", ".cursor", "mcp.json")}
	if strings.Join(paths, "\n") != strings.Join(want, "\n") {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestUnmanagingASharedServerKeepsItsProjectEntries(t *testing.T) {
	s, tmp := projectsService(t, sharedConfig)
	applyProjects(t, s)
	if _, err := s.Mutate(Mutation{Name: "docs", Remove: true, Unmanage: true}, "", false); err != nil {
		t.Fatal(err)
	}
	applyProjects(t, s)
	for _, root := range []string{"api", "web"} {
		if got := readFile(t, filepath.Join(tmp, root, ".cursor", "mcp.json")); !strings.Contains(got, "docs.example.com") {
			t.Fatalf("%s: entry removed although Skillshare stopped managing it: %s", root, got)
		}
	}
}

func TestDroppingARootRemovesOnlyItsCopyOfASharedServer(t *testing.T) {
	s, tmp := projectsService(t, sharedConfig)
	applyProjects(t, s)
	if _, err := s.Mutate(Mutation{Project: tmp + "/api", Remove: true}, "", false); err != nil {
		t.Fatal(err)
	}
	applyProjects(t, s)
	if got := readFile(t, filepath.Join(tmp, "api", ".cursor", "mcp.json")); strings.Contains(got, "docs.example.com") {
		t.Fatalf("api kept the shared server after it left mcp.projects: %s", got)
	}
	if got := readFile(t, filepath.Join(tmp, "web", ".cursor", "mcp.json")); !strings.Contains(got, "docs.example.com") {
		t.Fatalf("web lost the shared server: %s", got)
	}
}

func TestSharedServerInAnExternalSourceSavesBothFiles(t *testing.T) {
	for name, m := range map[string]Mutation{
		"remove": {Name: "docs", Remove: true},
		"global": {Name: "docs", Server: &Server{URL: "https://docs.example.com/mcp", Global: new(bool(true))}, Replace: true},
	} {
		t.Run(name, func(t *testing.T) {
			s, tmp := projectsService(t, "sources:\n  mcp: mcp.yaml\nmcp:\n  targets: [cursor]\n  projects:\n    $TMP/api:\n      use: [docs]\n")
			if err := os.WriteFile(filepath.Join(tmp, "mcp.yaml"), []byte("servers:\n  docs:\n    global: false\n    url: https://docs.example.com/mcp\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Mutate(m, "", false); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadSource(s.ConfigPath); err != nil {
				t.Fatalf("config no longer loads: %v\nconfig:\n%s\nexternal:\n%s", err, readFile(t, s.ConfigPath), readFile(t, filepath.Join(tmp, "mcp.yaml")))
			}
		})
	}
}

func TestCheckKeepsTheProjectTargetsOfASharedServerOneAgentRefuses(t *testing.T) {
	// Claude reserves the name workspace; Cursor, which only the project names, takes it.
	s, _ := projectsService(t, `mcp:
  targets: [claude]
  servers:
    workspace:
      global: false
      url: https://example.com/mcp
  projects:
    $TMP/api:
      targets: [claude, cursor]
      use: [workspace]
`)
	report, err := s.Check(offlineCheck())
	if err != nil {
		t.Fatal(err)
	}
	list := findings(t, report, "workspace")
	if !hasFinding(list, "error", "client-rule", "claude") {
		t.Fatalf("Claude must refuse its reserved name: %+v", list)
	}
	if !hasFinding(list, "warning", "sync", "cursor") {
		t.Fatalf("the project's Cursor entry must still report its sync state: %+v", list)
	}
}
