package mcp

import (
	"strings"
	"testing"
)

func TestNativePreservesUnrelatedContent(t *testing.T) {
	for _, tc := range []struct{ target, input, preserved string }{
		{"claude", "{\n  \"preferences\": {\"theme\": \"dark\"},\n  \"mcpServers\": {\"personal\": {\"command\": \"custom\"}}\n}\n", `"preferences": {"theme": "dark"}`},
		{"vscode", "{\n // keep this comment\n \"servers\": {},\n \"inputs\": [],\n}\n", "// keep this comment"},
		{"codex", "# personal preferences\nmodel = \"example\"\n\n[mcp_servers.personal]\ncommand = \"custom\"\n\n[features]\n# keep this comment\nmulti_agent = true\n", "[features]\n# keep this comment\nmulti_agent = true\n"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			n, err := ParseNative(tc.target, []byte(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			out, err := n.Edit(map[string]map[string]any{"docs": {"url": "https://example.com/mcp"}})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), tc.preserved) {
				t.Fatalf("unrelated content changed: %s", out)
			}
			n, err = ParseNative(tc.target, out)
			if err != nil {
				t.Fatal(err)
			}
			if n.Entries["docs"]["url"] != "https://example.com/mcp" {
				t.Fatal("missing entry")
			}
			out, err = n.Edit(map[string]map[string]any{"docs": nil})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(out), "https://example.com/mcp") {
				t.Fatal("entry not deleted")
			}
		})
	}
}

func TestNativeRejectsMalformedOrDuplicateEntries(t *testing.T) {
	for _, data := range []string{`{"mcpServers":`, `{"mcpServers":{},"mcpServers":{}}`, `{"mcpServers":null}`} {
		if _, err := ParseNative("claude", []byte(data)); err == nil {
			t.Fatalf("accepted unsafe config: %s", data)
		}
	}
}
