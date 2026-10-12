package mcp

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAdditionalClients(t *testing.T) {
	for _, target := range []string{"opencode", "kilocode", "grok"} {
		t.Run(target, func(t *testing.T) {
			server := Server{Command: "tool", Args: []string{"serve"}, Env: map[string]Value{"TOKEN": {FromEnv: "API_TOKEN"}}}
			entry, err := Render(target, server)
			if err != nil {
				t.Fatal(err)
			}
			raw := []byte("# keep\nmodel = 'example'\n")
			if openCodeFormat(target) {
				raw = []byte("{\n// keep\n\"model\":\"example\"\n}\n")
			}
			native, err := ParseNative(target, raw)
			if err != nil {
				t.Fatal(err)
			}
			data, err := native.Edit(map[string]map[string]any{"docs": entry})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "keep") || !strings.Contains(string(data), "example") {
				t.Fatal("lost unrelated config")
			}
			candidates, err := Import(target, data, "")
			if err != nil || len(candidates) != 1 {
				t.Fatalf("import: %v %v", candidates, err)
			}
			c := candidates[0]
			if len(c.Problems) > 0 || c.Server.Command != "tool" || c.Server.Env["TOKEN"].FromEnv != "API_TOKEN" {
				t.Fatalf("roundtrip: %+v", c)
			}
			s := testService(t)
			if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: ["+target+"]\n  servers:\n    docs:\n      url: https://example.com/mcp\n"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, project := range []string{"", filepath.Join(s.Home, "project")} {
				s.ProjectRoot = project
				plan, err := s.Preview()
				if err != nil {
					t.Fatal(err)
				}
				if _, err = s.Apply(plan.Revision); err != nil {
					t.Fatal(err)
				}
				plan, err = s.Preview()
				if err != nil {
					t.Fatal(err)
				}
				if len(plan.Changes) != 1 || plan.Changes[0].Action != "unchanged" {
					t.Fatalf("not idempotent: %+v", plan.Changes)
				}
			}
		})
	}
}

func TestOpenCodeJSONCPath(t *testing.T) {
	s := testService(t)
	s.ProjectRoot = t.TempDir()
	path := filepath.Join(s.ProjectRoot, "opencode.jsonc")
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := s.nativePath("opencode")
	if err != nil || got != path {
		t.Fatalf("path %q: %v", got, err)
	}
}

// OpenCode also loads opencode.json from a project's .opencode directory, so a file kept
// there is the one to write. Writing a second one at the root would mask it. Refs: #289.
func TestOpenCodeProjectPathReusesDotOpenCode(t *testing.T) {
	s := testService(t)
	s.ProjectRoot = t.TempDir()
	if got, err := s.nativePath("opencode"); err != nil || got != filepath.Join(s.ProjectRoot, "opencode.json") {
		t.Fatalf("new project file %q: %v", got, err)
	}
	nested := filepath.Join(s.ProjectRoot, ".opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(nested), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := s.nativePath("opencode"); err != nil || got != nested {
		t.Fatalf("existing .opencode file %q: %v", got, err)
	}
}

// Kilo merges every config file it finds, so an existing file is reused wherever it
// is, a new one goes where Kilo's docs put it, and two candidates are ambiguous.
func TestKiloCodePath(t *testing.T) {
	s := testService(t)
	if got, err := s.nativePath("kilocode"); err != nil || got != filepath.Join(s.Home, ".config", "kilo", "kilo.jsonc") {
		t.Fatalf("global %q: %v", got, err)
	}
	s.ProjectRoot = t.TempDir()
	if got, err := s.nativePath("kilocode"); err != nil || got != filepath.Join(s.ProjectRoot, "kilo.jsonc") {
		t.Fatalf("new project file %q: %v", got, err)
	}
	nested := filepath.Join(s.ProjectRoot, ".kilo", "kilo.jsonc")
	if err := os.MkdirAll(filepath.Dir(nested), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := s.nativePath("kilocode"); err != nil || got != nested {
		t.Fatalf("existing .kilo file %q: %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(s.ProjectRoot, "kilo.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.nativePath("kilocode"); err == nil || !strings.Contains(err.Error(), "consolidate") {
		t.Fatalf("two files must be refused: %v", err)
	}
}

// Kilo treats project config as untrusted: one {env:} reference makes it discard the whole
// project file, so every server in it would vanish. The user's own config may use them.
func TestKiloCodeProjectRefusesEnvReferences(t *testing.T) {
	s := testService(t)
	if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [kilocode]\n  servers:\n    docs:\n      url: https://example.com/mcp\n      bearerToken: {fromEnv: MCP_TOKEN}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Preview(); err != nil {
		t.Fatalf("global config may reference the environment: %v", err)
	}
	s.ProjectRoot = t.TempDir()
	if _, err := s.Preview(); err == nil || !strings.Contains(err.Error(), "fromEnv") {
		t.Fatalf("project config must refuse fromEnv: %v", err)
	}
}

// Claude Code skips a server named after one of its built-ins, and reads its own credentials
// as empty toward a remote server. Both fail silently in Claude, so they are refused here.
func TestClaudeRefusesWhatItWouldIgnore(t *testing.T) {
	s := testService(t)
	for name, server := range map[string]Server{
		"workspace": {Command: "tool"},
		"docs":      {URL: "https://example.com/mcp", BearerToken: &Value{FromEnv: "ANTHROPIC_API_KEY"}},
	} {
		if err := s.checkScope(name, "claude", server); err == nil {
			t.Errorf("%s accepted for claude", name)
		}
		if err := s.checkScope(name, "cursor", server); err != nil {
			t.Errorf("%s refused for cursor: %v", name, err)
		}
	}
	if err := s.checkScope("docs", "claude", Server{Command: "tool", Env: map[string]Value{"ANTHROPIC_API_KEY": {FromEnv: "ANTHROPIC_API_KEY"}}}); err != nil {
		t.Errorf("a local server may receive the key: %v", err)
	}
}

func TestImportDetectsPastedJSONFormat(t *testing.T) {
	for _, content := range []string{
		`{"mcpServers":{"docs":{"url":"https://example.com/mcp"}}}`,
		`{"servers":{"docs":{"type":"http","url":"https://example.com/mcp"}}}`,
		`{"$schema":"https://opencode.ai/config.json","mcp":{"docs":{"type":"remote","url":"https://example.com/mcp"}}}`,
	} {
		candidates, err := Import("", []byte(content), "")
		if err != nil || len(candidates) != 1 || candidates[0].Server.URL != "https://example.com/mcp" || len(candidates[0].Problems) != 0 {
			t.Fatalf("%s: %+v %v", content, candidates, err)
		}
	}
}

func TestDetectedClients(t *testing.T) {
	s := testService(t)
	paths := s.ClientPaths()
	// claude's file lives in home, so home alone must not count; cursor has only
	// its directory; codex has its file.
	for _, dir := range []string{filepath.Dir(paths["cursor"]), filepath.Dir(paths["codex"])} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(paths["codex"], nil, 0600); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(s.DetectedClients(paths), ","); got != "codex,cursor" {
		t.Fatalf("detected %q", got)
	}
}

// An Agent that is installed but has never had an MCP server has no MCP folder yet, only
// its own home folder. Antigravity lives inside ~/.gemini, so that folder alone does not
// mean Gemini CLI is installed.
func TestDetectedClientsByHomeFolder(t *testing.T) {
	s := testService(t)
	for _, dir := range []string{".claude", ".junie", ".kiro", ".cline", filepath.Join(".gemini", "config")} {
		if err := os.MkdirAll(filepath.Join(s.Home, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Join(s.DetectedClients(s.ClientPaths()), ","); got != "claude,antigravity,cline,junie,kiro" {
		t.Fatalf("detected %q", got)
	}
	if err := os.WriteFile(filepath.Join(s.Home, ".gemini", "settings.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := s.DetectedClients(s.ClientPaths()); !slices.Contains(got, "gemini") {
		t.Fatalf("gemini with its own settings file: %v", got)
	}
}

// A project has .github, .vscode and .agents for reasons unrelated to MCP, and Claude's
// project file sits in the root, so project folders say nothing about which Agents are
// in use. The project file itself does, and so does the Agent being installed.
func TestDetectedClientsInProject(t *testing.T) {
	s := testService(t)
	global := s.ClientPaths()
	if err := os.MkdirAll(filepath.Dir(global["cursor"]), 0700); err != nil {
		t.Fatal(err)
	}
	s.ProjectRoot = t.TempDir()
	for _, dir := range []string{".github", ".vscode", ".agents"} {
		if err := os.MkdirAll(filepath.Join(s.ProjectRoot, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(s.ProjectRoot, ".mcp.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(s.DetectedClients(s.ClientPaths()), ","); got != "claude,cursor" {
		t.Fatalf("detected %q", got)
	}
}

func TestAdditionalClientRemoteReferences(t *testing.T) {
	for _, target := range []string{"opencode", "grok"} {
		s := Server{URL: "https://example.com/mcp", BearerToken: &Value{FromEnv: "MCP_TOKEN"}}
		entry, err := Render(target, s)
		if err != nil {
			t.Fatal(err)
		}
		want := "Bearer ${MCP_TOKEN}"
		if target == "opencode" {
			want = "Bearer {env:MCP_TOKEN}"
		}
		if entry["headers"].(map[string]string)["Authorization"] != want {
			t.Fatalf("bad reference: %v", entry)
		}
		n, err := ParseNative(target, nil)
		if err != nil {
			t.Fatal(err)
		}
		data, err := n.Edit(map[string]map[string]any{"docs": entry})
		if err != nil {
			t.Fatal(err)
		}
		items, err := Import(target, data, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(items[0].Problems) > 0 || items[0].Server.BearerToken == nil || items[0].Server.BearerToken.FromEnv != "MCP_TOKEN" {
			t.Fatalf("bad import: %+v", items)
		}
	}
}

func TestAdditionalClientImportRejectsDisabled(t *testing.T) {
	for target, data := range map[string]string{
		"opencode": `{"mcp":{"docs":{"type":"remote","url":"https://example.com/mcp","enabled":false}}}`,
		"grok":     "[mcp_servers.docs]\nurl='https://example.com/mcp'\nenabled=false\n",
		"codex":    "[mcp_servers.docs]\nurl='https://example.com/mcp'\nenabled=false\n",
		"cursor":   `{"mcpServers":{"docs":{"url":"https://example.com/mcp","disabled":true}}}`,
	} {
		items, err := Import(target, []byte(data), "")
		if err != nil || len(items) != 1 || len(items[0].Problems) == 0 {
			t.Fatalf("disabled accepted: %+v %v", items, err)
		}
	}
}

// OpenCode 2 nests its servers under mcp.servers, as its own mcp add writes them.
func TestImportReadsOpenCodeServersSection(t *testing.T) {
	items, err := Import("opencode", []byte(`{"mcp":{"servers":{"docs":{"type":"local","command":["tool","--x"],"environment":{"KEY":"{env:KEY}"}}},"flat":{"type":"remote","url":"https://example.com/mcp"},"timeout":{"startup":45000}}}`), "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Server{}
	for _, item := range items {
		if len(item.Problems) > 0 {
			t.Fatalf("%s: %v", item.Name, item.Problems)
		}
		got[item.Name] = item.Server
	}
	docs := got["docs"]
	if len(got) != 2 || docs.Command != "tool" || !slices.Equal(docs.Args, []string{"--x"}) || docs.Env["KEY"].FromEnv != "KEY" || got["flat"].URL != "https://example.com/mcp" {
		t.Fatalf("candidates: %+v", got)
	}
}

func TestImportReadsAnEmptyOpenCodeServersSection(t *testing.T) {
	if items, err := Import("opencode", []byte(`{"mcp":{"servers":{},"timeout":{"startup":45000}}}`), ""); err == nil {
		t.Fatalf("candidates: %+v", items)
	}
}

// A server named servers is still a server: only a section of objects is OpenCode 2's.
func TestImportKeepsAnOpenCodeServerNamedServers(t *testing.T) {
	items, err := Import("opencode", []byte(`{"mcp":{"servers":{"enabled":false},"other":{"type":"local","command":["x"]}}}`), "")
	if err != nil || len(items) != 2 || items[1].Name != "servers" || len(items[1].Problems) == 0 {
		t.Fatalf("candidates: %+v %v", items, err)
	}
	if _, err := Import("opencode", []byte(`{"mcp":{"servers":{"docs":{"type":"local","command":["a"]}},"docs":{"type":"local","command":["b"]}}}`), ""); err == nil || !strings.Contains(err.Error(), "docs") {
		t.Fatalf("same name in both places: %v", err)
	}
}
