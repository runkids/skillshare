package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdditionalClients(t *testing.T) {
	for _, target := range []string{"opencode", "grok"} {
		t.Run(target, func(t *testing.T) {
			server := Server{Command: "tool", Args: []string{"serve"}, Env: map[string]Value{"TOKEN": {FromEnv: "API_TOKEN"}}}
			entry, err := Render(target, server)
			if err != nil {
				t.Fatal(err)
			}
			raw := []byte("# keep\nmodel = 'example'\n")
			if target == "opencode" {
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
