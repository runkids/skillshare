package mcp

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPiRender(t *testing.T) {
	out, err := Render("pi", Server{URL: "https://example.com/mcp"})
	if err != nil || out["url"] != "https://example.com/mcp" || out["transport"] != nil {
		t.Fatalf("%v %v", out, err)
	}
	if _, err := Render("pi", Server{Command: "echo", Env: map[string]Value{"LITERAL": {Literal: "!date"}}}); err == nil {
		t.Fatal("a literal beginning with ! would run a command")
	}
	out, err = Render("pi", Server{Command: "echo", Env: map[string]Value{"TOKEN": {FromEnv: "TOKEN"}}})
	if err != nil || out["env"].(map[string]string)["TOKEN"] != "${TOKEN}" {
		t.Fatalf("%v %v", out, err)
	}
}

func TestPiSyncScopesAndPreservation(t *testing.T) {
	for _, project := range []bool{false, true} {
		s := testService(t)
		if project {
			s.ProjectRoot = filepath.Join(s.Home, "project")
		}
		path := filepath.Join(s.Home, ".pi", "agent", "mcp.json")
		if project {
			path = filepath.Join(s.ProjectRoot, ".pi", "mcp.json")
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		original := `{"settings":{"maxRetries":3},"mcpServers":{"mine":{"command":"mine"}}}`
		if err := os.WriteFile(path, []byte(original), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [pi]\n  servers:\n    docs:\n      url: https://example.com/mcp\n"), 0600); err != nil {
			t.Fatal(err)
		}
		p, err := s.Preview()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Apply(p.Revision); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), `"mine"`) || !strings.Contains(string(data), `"maxRetries"`) || !strings.Contains(string(data), `"https://example.com/mcp"`) {
			t.Fatal(string(data))
		}
		p, err = s.Preview()
		if err != nil || p.Changes[0].Action != "unchanged" {
			t.Fatalf("%+v %v", p, err)
		}
	}
}

func TestPiImportTransport(t *testing.T) {
	candidates, err := Import("pi", []byte(`{"mcpServers":{"docs":{"transport":"streamable-http","url":"https://example.com/mcp","timeout":60}}}`), "")
	if err != nil || len(candidates) != 1 || len(candidates[0].Problems) != 0 || candidates[0].Server.Transport != "streamable-http" || candidates[0].Server.PiOptions["timeout"] == nil {
		t.Fatalf("%+v %v", candidates, err)
	}
	candidates, err = Import("pi", []byte(`{"mcpServers":{"docs":{"transport":"sse","url":"https://example.com/sse"}}}`), "")
	if err != nil || len(candidates[0].Problems) == 0 {
		t.Fatal("legacy SSE silently converted")
	}
}

func TestImportTimeoutDoesNotIdentifyPi(t *testing.T) {
	for _, input := range []string{
		`{"mcpServers":{"local":{"command":"docs","timeout":60},"remote":{"httpUrl":"https://example.com/mcp","timeout":60}}}`,
		`{"mcpServers":{"remote":{"type":"streamableHttp","url":"https://example.com/mcp","timeout":60}}}`,
		`{"command":"docs","timeout":60}`,
	} {
		for i := 0; i < 20; i++ {
			candidates, err := Import("", []byte(input), "docs")
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range candidates {
				if len(c.Problems) != 0 || c.Server.PiOptions != nil {
					t.Fatalf("%+v", c)
				}
			}
		}
	}
}

// pi-mcp-adapter escaped a literal beginning with ! as !!. Only its own file says so.
func TestPiAdapterFileImportUnescapesLiterals(t *testing.T) {
	input := []byte(`{"mcpServers":{"local":{"command":"c","excludeTools":[],"env":{"MODE":"!!x"}},"remote":{"url":"https://example.com/mcp","headers":{"X-Mode":"!!x"}}}}`)
	candidates, err := importNative("pi", input, "", true, false)
	if err != nil || len(candidates) != 2 {
		t.Fatalf("%+v %v", candidates, err)
	}
	for _, c := range candidates {
		if len(c.Problems) != 0 || c.Name == "local" && c.Server.Env["MODE"].Literal != "!x" || c.Name == "remote" && c.Server.Headers["X-Mode"].Literal != "!x" {
			t.Fatalf("escaped literal: %+v", c)
		}
	}
	candidates, err = Import("pi", input, "")
	if err != nil || len(candidates[0].Problems) == 0 {
		t.Fatalf("mcp.json runs !: %+v %v", candidates, err)
	}
}

func TestPiOptionsRejectNonSecretCommands(t *testing.T) {
	for _, adapter := range []bool{false, true} {
		input := []byte(`{"mcpServers":{"docs":{"url":"https://example.com/mcp","oauth":{"clientId":"!echo client"}}}}`)
		candidates, err := importNative("pi", input, "", adapter, false)
		if err != nil || len(candidates) != 1 || len(candidates[0].Problems) == 0 {
			t.Fatalf("unsafe import: %+v %v", candidates, err)
		}
	}
	if _, err := Render("pi", Server{URL: "https://example.com/mcp", PiOptions: PiOptions{"oauth": map[string]any{"clientId": "!echo client"}}}); err == nil {
		t.Fatal("unsafe render")
	}
}

func TestPiOAuthMetadataURLNeedsHTTPS(t *testing.T) {
	for metadata, ok := range map[string]bool{
		"https://example.okta.com/.well-known/openid-configuration":    true,
		"http://localhost:8080/.well-known/oauth-authorization-server": true,
		"http://[::1]/metadata":                               true,
		"http://example.com/.well-known/openid-configuration": false,
		"https://user:pass@example.com/metadata":              false,
		"/.well-known/openid-configuration":                   false,
	} {
		_, err := Render("pi", Server{URL: "https://example.com/mcp", PiOptions: PiOptions{"oauth": map[string]any{"authServerMetadataUrl": metadata}}})
		if (err == nil) != ok {
			t.Errorf("%s: want accepted=%v, got %v", metadata, ok, err)
		}
	}
}

func TestPiImportSourcesAndPaths(t *testing.T) {
	s := testService(t)
	dir := filepath.Join(s.Home, "custom-pi")
	s.ConfigDirs = map[string]string{"pi": dir}
	source := &Source{Servers: map[string]Server{}}
	found := map[string]string{}
	for _, item := range s.ImportSources(source)[""] {
		if item.Target == "pi" {
			found[item.PiExtension] = item.Path
		}
	}
	if len(found) != 2 || found["builtin"] != filepath.Join(dir, "mcp.json") || found["pi-mcp-adapter"] != filepath.Join(dir, "mcp-adapter.json") {
		t.Fatalf("import sources: %v", found)
	}
	if got := s.ConfiguredClientPaths(source)["pi"]; got != filepath.Join(dir, "mcp.json") {
		t.Fatal(got)
	}
}

func TestPiCredentials(t *testing.T) {
	out, err := Render("pi", Server{URL: "https://example.com/mcp", BearerToken: &Value{FromEnv: "TOKEN"}})
	if err != nil || out["headers"].(map[string]string)["Authorization"] != "Bearer ${TOKEN}" {
		t.Fatalf("%+v %v", out, err)
	}
}

func TestPiDirectoryOverride(t *testing.T) {
	s := testService(t)
	s.ConfigDirs = map[string]string{"pi": filepath.Join(s.Home, "custom-pi")}
	path, err := s.nativePath("pi")
	if err != nil || path != filepath.Join(s.Home, "custom-pi", "mcp.json") {
		t.Fatalf("%s %v", path, err)
	}
	candidates, err := Import("", []byte(`{"transport":"sse","url":"https://example.com/sse"}`), "docs")
	if err != nil || len(candidates[0].Problems) == 0 {
		t.Fatal("pasted SSE must not become streamable HTTP")
	}
}

func TestPiWritesOnlyBuiltinFile(t *testing.T) {
	for _, project := range []bool{false, true} {
		s := testService(t)
		dir := filepath.Join(s.Home, ".pi", "agent")
		if project {
			s.ProjectRoot = filepath.Join(s.Home, "project")
			dir = filepath.Join(s.ProjectRoot, ".pi")
		}
		if path, _, err := s.destination("pi", Server{Command: "docs"}); err != nil || path != filepath.Join(dir, "mcp.json") {
			t.Errorf("project=%v: %s %v", project, path, err)
		}
	}
}

// piMigration is a Pi home an earlier Skillshare synced docs into for the given extension,
// with the ledger owning the entry where that extension read it, next to a server of the
// user's own. An empty extension leaves piExtension out of the config and the ledger owning
// the entry in mcp-adapter.json, so only the ledger tells that the adapter had it.
func piMigration(t *testing.T, extension string, entry map[string]any) (s *Service, builtin, adapter string) {
	t.Helper()
	s = testService(t)
	config := "mcp:\n  targets: [pi]\n  servers:\n    docs:\n      command: docs\n"
	if extension != "" {
		config += "      piExtension: " + extension + "\n"
	}
	if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.Home, ".pi", "agent")
	builtin, adapter = filepath.Join(dir, "mcp.json"), filepath.Join(dir, "mcp-adapter.json")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	owned := builtin
	if extension == "pi-mcp-adapter" || extension == "" {
		owned = adapter
	}
	if err := writeJSONFile(owned, map[string]any{"mcpServers": map[string]any{"docs": entry, "mine": map[string]any{"command": "mine"}}}); err != nil {
		t.Fatal(err)
	}
	record := ownership{Owner: s.ConfigPath, Target: "pi", Path: owned, Name: "docs", Hash: entryHash(managedEntry("pi", entry))}
	if err := writeJSONFile(s.statePath(), ledger{Version: 1, Entries: map[string]ownership{ownershipKey("pi", owned, "docs"): record}}); err != nil {
		t.Fatal(err)
	}
	return s, builtin, adapter
}

// A server synced for pi-mcp-adapter moves to Pi's built-in mcp.json, and the entry
// Skillshare wrote to mcp-adapter.json goes; the user's own entry there stays.
func TestPiAdapterServerMovesToBuiltin(t *testing.T) {
	s, builtin, adapter := piMigration(t, "pi-mcp-adapter", map[string]any{"command": "docs"})
	plan, err := s.Preview()
	if err != nil || plan.Blocked {
		t.Fatalf("%+v %v", plan, err)
	}
	if c := changeFor(plan, adapter, "docs"); c == nil || c.Action != "remove" {
		t.Fatalf("adapter entry: %+v", c)
	}
	if c := changeFor(plan, builtin, "docs"); c == nil || c.Action != "add" {
		t.Fatalf("built-in entry: %+v", c)
	}
	if len(plan.Notices) != 2 || plan.Notices[0] != "Pi now uses its built-in MCP; the next sync updates the config: docs" || plan.Notices[1] != PiBuiltinNotice {
		t.Fatalf("notices: %v", plan.Notices)
	}
	result, err := s.Apply(plan.Revision)
	if err != nil || len(result.BackupIDs) == 0 {
		t.Fatalf("%+v %v", result, err)
	}
	if data, _ := os.ReadFile(adapter); strings.Contains(string(data), `"docs"`) || !strings.Contains(string(data), `"mine"`) {
		t.Fatalf("mcp-adapter.json: %s", data)
	}
	if data, _ := os.ReadFile(builtin); !strings.Contains(string(data), `"docs"`) {
		t.Fatalf("mcp.json: %s", data)
	}
}

// pi-mcp-extension read mcp.json too, so its entry is rewritten where it is.
func TestPiExtensionServerRewrittenInPlace(t *testing.T) {
	s, builtin, _ := piMigration(t, "pi-mcp-extension", map[string]any{"command": "docs", "transport": "stdio"})
	plan, err := s.Preview()
	if err != nil || plan.Blocked || len(plan.Changes) != 1 {
		t.Fatalf("%+v %v", plan, err)
	}
	if c := changeFor(plan, builtin, "docs"); c == nil || c.Action != "update" {
		t.Fatalf("entry: %+v", c)
	}
	if !slices.Contains(plan.Notices, PiBuiltinNotice) {
		t.Fatalf("notices: %v", plan.Notices)
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(builtin); strings.Contains(string(data), `"transport"`) || !strings.Contains(string(data), `"mine"`) {
		t.Fatalf("mcp.json: %s", data)
	}
}

// A config without piExtension can still own an entry in mcp-adapter.json, e.g. one edited
// by hand after a sync; the ledger alone shows the server leaving pi-mcp-adapter, and the
// sync must still say what Pi needs now.
func TestPiAdapterLedgerOnlyMoveWarns(t *testing.T) {
	s, builtin, adapter := piMigration(t, "", map[string]any{"command": "docs"})
	plan, err := s.Preview()
	if err != nil || plan.Blocked {
		t.Fatalf("%+v %v", plan, err)
	}
	if c := changeFor(plan, adapter, "docs"); c == nil || c.Action != "remove" {
		t.Fatalf("adapter entry: %+v", c)
	}
	if c := changeFor(plan, builtin, "docs"); c == nil || c.Action != "add" {
		t.Fatalf("built-in entry: %+v", c)
	}
	if len(plan.Notices) != 1 || plan.Notices[0] != PiBuiltinNotice {
		t.Fatalf("notices: %v", plan.Notices)
	}
	result, err := s.Apply(plan.Revision)
	if err != nil || !slices.Contains(result.Plan.Notices, PiBuiltinNotice) {
		t.Fatalf("%+v %v", result, err)
	}
	again, err := s.Preview()
	if err != nil || len(again.Notices) != 0 {
		t.Fatalf("after sync: %v %v", again.Notices, err)
	}
}

// A server already on Pi's built-in MCP syncs without the warning, including one whose
// config still says so with piExtension: builtin.
func TestPiBuiltinSyncHasNoMigrationNotice(t *testing.T) {
	for _, extra := range []string{"", "      piExtension: builtin\n"} {
		s := testService(t)
		if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [pi]\n  servers:\n    docs:\n      command: docs\n"+extra), 0600); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			plan, err := s.Preview()
			if err != nil || slices.Contains(plan.Notices, PiBuiltinNotice) {
				t.Fatalf("%q: %v %v", extra, plan.Notices, err)
			}
			if _, err := s.Apply(plan.Revision); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// A config set up for an old extension warns before its first sync, when nothing is
// written yet for the ledger to show.
func TestPiLegacyExtensionSettingsWarn(t *testing.T) {
	for _, server := range []string{"piExtension: pi-mcp-extension", "piExtension: pi-mcp-adapter", "directTools: true", "piOptions:\n        lifecycle: lazy"} {
		s := testService(t)
		if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [pi]\n  servers:\n    docs:\n      command: docs\n      "+server+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Preview()
		if err != nil || !slices.Contains(plan.Notices, PiBuiltinNotice) {
			t.Fatalf("%q: %v %v", server, plan.Notices, err)
		}
	}
}

// Before 0.23.0 Pi took one mode per scope and refused a mix.
func TestPiMixedModesShareBuiltinFile(t *testing.T) {
	s := testService(t)
	if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [pi]\n  servers:\n    a:\n      command: a\n      piExtension: pi-mcp-adapter\n    b:\n      command: b\n      piExtension: pi-mcp-extension\n    c:\n      command: c\n      piExtension: builtin\n"), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Preview()
	if err != nil || plan.Blocked || len(plan.Changes) != 3 {
		t.Fatalf("%+v %v", plan, err)
	}
	for _, c := range plan.Changes {
		if c.Path != filepath.Join(s.Home, ".pi", "agent", "mcp.json") || c.Action != "add" {
			t.Fatalf("%+v", c)
		}
	}
}

// Pi's built-in mcp.json comes first; the adapter's file is still read so hand-written
// servers there can be imported.
func TestPiImportReadsBothFiles(t *testing.T) {
	s := testService(t)
	dir := filepath.Join(s.Home, ".pi", "agent")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for file, entries := range map[string]map[string]any{
		"mcp-adapter.json": {"adapted": map[string]any{"command": "adapted"}, "both": map[string]any{"command": "adapter"}},
		"mcp.json":         {"builtin": map[string]any{"command": "builtin"}, "both": map[string]any{"command": "builtin"}},
	} {
		if err := writeJSONFile(filepath.Join(dir, file), map[string]any{"mcpServers": entries}); err != nil {
			t.Fatal(err)
		}
	}
	candidates, err := s.ImportClient("pi")
	if err != nil || len(candidates) != 3 {
		t.Fatalf("%+v %v", candidates, err)
	}
	for _, c := range candidates {
		if c.Name == "both" && c.Server.Command != "builtin" {
			t.Errorf("mcp.json must win: %+v", c)
		}
		if c.Name == "adapted" && !strings.Contains(strings.Join(c.Warnings, ";"), "mcp-adapter.json") {
			t.Errorf("adapter origin not reported: %+v", c)
		}
	}
	var names []string
	for _, found := range s.FindUnmanaged(&Source{ConfigPath: s.ConfigPath}) {
		names = append(names, found.Names...)
	}
	if strings.Join(names, ",") != "both,builtin,adapted,both" {
		t.Fatalf("unmanaged: %v", names)
	}
}

// Pi's /mcp writes a project entry with only enabled, exposure or toolExposure to override the
// global server of that name. It has no server to import, and import says what it is.
func TestPiProjectOverrideIsNotImported(t *testing.T) {
	s, tmp := projectsService(t, "mcp:\n  servers: {}\n  projects:\n    $TMP/p1:\n      targets: [pi]\n")
	writePiProjectFile(t, filepath.Join(tmp, "p1"), `{"mcpServers":{"full":{"command":"tool"},"off":{"enabled":false},"direct":{"exposure":"direct"}}}`)
	candidates, err := s.ImportProjectClient(filepath.Join(tmp, "p1"), "pi")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range candidates {
		override := c.Name != "full"
		if got := slices.ContainsFunc(c.Problems, func(p string) bool { return strings.HasPrefix(p, "a Pi project override") }); got != override {
			t.Fatalf("%s: problems %q", c.Name, c.Problems)
		}
	}
}

// Pi gives a connection-less entry override meaning only in a project's .pi/mcp.json. In the
// global file or a pasted snippet it is an invalid server and keeps the usual problem.
func TestPiOverrideOnlyInProjectImport(t *testing.T) {
	const file = `{"mcpServers":{"off":{"enabled":false}}}`
	s, tmp := projectsService(t, "mcp:\n  servers: {}\n")
	if err := os.MkdirAll(filepath.Join(tmp, ".pi", "agent"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".pi", "agent", "mcp.json"), []byte(file), 0600); err != nil {
		t.Fatal(err)
	}
	global, err := s.ImportClient("pi")
	if err != nil {
		t.Fatal(err)
	}
	pasted, err := Import("pi", []byte(file), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range append(global, pasted...) {
		if !slices.Equal(c.Problems, []string{"MCP off requires exactly one of command or url"}) {
			t.Fatalf("%s: problems %q", c.Name, c.Problems)
		}
	}
}

// A server the project defines meets Pi's override of the same name: the conflict says to
// replace it or remove the override in Pi, since import cannot take it.
func TestPiProjectOverrideConflictSaysReplace(t *testing.T) {
	s, tmp := projectsService(t, "mcp:\n  servers: {}\n  projects:\n    $TMP/p1:\n      targets: [pi]\n      servers:\n        direct:\n          command: tool\n")
	writePiProjectFile(t, filepath.Join(tmp, "p1"), `{"mcpServers":{"direct":{"exposure":"direct"}}}`)
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	c := changeFor(plan, filepath.Join(tmp, "p1", ".pi", "mcp.json"), "direct")
	if c == nil || c.Action != "conflict" || !strings.HasPrefix(c.Message, "existing entry is a Pi project override") {
		t.Fatalf("%+v", c)
	}
}

// An entry this config synced and that was later changed into Pi's override is no server to
// import either, so it gets the same conflict instead of the generic drift one.
func TestPiProjectOverrideOfAManagedEntrySaysReplace(t *testing.T) {
	s, tmp := projectsService(t, "mcp:\n  servers: {}\n  projects:\n    $TMP/p1:\n      targets: [pi]\n      servers:\n        direct:\n          command: tool\n")
	applyProjects(t, s)
	writePiProjectFile(t, filepath.Join(tmp, "p1"), `{"mcpServers":{"direct":{"enabled":false}}}`)
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	c := changeFor(plan, filepath.Join(tmp, "p1", ".pi", "mcp.json"), "direct")
	if c == nil || c.Action != "conflict" || !strings.HasPrefix(c.Message, "existing entry is a Pi project override") {
		t.Fatalf("%+v", c)
	}
}

// A cleared Pi field that Pi changed since sync is a prune conflict, but when Pi replaced the
// entry with its override, that override still has nothing to import.
func TestPiProjectOverrideBeatsPruneConflict(t *testing.T) {
	s, tmp := projectsService(t, "mcp:\n  servers: {}\n  projects:\n    $TMP/p1:\n      targets: [pi]\n      servers:\n        direct:\n          command: tool\n          piOptions: {exposure: direct}\n")
	applyProjects(t, s)
	config, _ := os.ReadFile(s.ConfigPath)
	if err := os.WriteFile(s.ConfigPath, []byte(strings.Replace(string(config), "          piOptions: {exposure: direct}\n", "", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	writePiProjectFile(t, filepath.Join(tmp, "p1"), `{"mcpServers":{"direct":{"exposure":"deferred"}}}`)
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	c := changeFor(plan, filepath.Join(tmp, "p1", ".pi", "mcp.json"), "direct")
	if c == nil || c.Action != "conflict" || !strings.HasPrefix(c.Message, "existing entry is a Pi project override") {
		t.Fatalf("%+v", c)
	}
}

// A live owner keeps its message, since only it can release the entry. A removed owner never
// can, so its entry turned into Pi's override gets the replace-only conflict too.
func TestPiProjectOverrideOfARemovedOwnersEntry(t *testing.T) {
	s := ownedProject(t)
	writePiProjectFile(t, s.ProjectRoot, `{"mcpServers":{"mcp-test1":{"enabled":false}}}`)
	p, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.Changes[0].Message, "managed by another Skillshare config: ") {
		t.Fatalf("live owner: %s", p.Changes[0].Message)
	}
	if err := os.RemoveAll(filepath.Join(s.ProjectRoot, ".skillshare")); err != nil {
		t.Fatal(err)
	}
	if p, err = s.Preview(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.Changes[0].Message, "existing entry is a Pi project override") {
		t.Fatalf("removed owner: %s", p.Changes[0].Message)
	}
}

// The conflict offers only Replace, so Replace has to settle it.
func TestPiProjectOverrideIsSettledByReplace(t *testing.T) {
	s := ownedProject(t)
	if err := os.RemoveAll(filepath.Join(s.ProjectRoot, ".skillshare")); err != nil {
		t.Fatal(err)
	}
	writePiProjectFile(t, s.ProjectRoot, `{"mcpServers":{"mcp-test1":{"enabled":false}}}`)
	p, err := s.PreviewMutation(Mutation{Resolutions: []Resolution{{Target: "pi", Name: "mcp-test1", Action: "replace"}}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Blocked {
		t.Fatalf("replace cannot settle it: %+v", p.Changes)
	}
}

func writePiProjectFile(t *testing.T, root, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".pi"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pi", "mcp.json"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
