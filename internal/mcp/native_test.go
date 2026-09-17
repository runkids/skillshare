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

func TestTOMLEditsKeepLayout(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		input := strings.ReplaceAll("model = 'example'\n\n[mcp_servers.docs]\nurl = 'https://old.example/mcp'\n\n[features]\nmulti_agent = true\n", "\n", newline)
		n, err := ParseNative("codex", []byte(input))
		if err != nil {
			t.Fatal(err)
		}
		out, err := n.Edit(map[string]map[string]any{"docs": {"url": "https://example.com/mcp"}})
		if err != nil {
			t.Fatal(err)
		}
		want := strings.ReplaceAll("model = 'example'\n\n[mcp_servers.docs]\nurl = 'https://example.com/mcp'\n\n[features]\nmulti_agent = true\n", "\n", newline)
		if string(out) != want {
			t.Fatalf("update moved or reformatted the entry:\n%q", out)
		}
	}
}

func TestTOMLRemoveThenAddDoesNotGrowBlankLines(t *testing.T) {
	input := "model = 'example'\n"
	data := []byte(input)
	for i := 0; i < 2; i++ {
		for _, entry := range []map[string]any{{"url": "https://example.com/mcp"}, nil} {
			n, err := ParseNative("codex", data)
			if err != nil {
				t.Fatal(err)
			}
			if data, err = n.Edit(map[string]map[string]any{"docs": entry}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if string(data) != input {
		t.Fatalf("got %q", data)
	}
}

func TestJSONEditDoesNotEscapeHTML(t *testing.T) {
	n, err := ParseNative("claude", nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := n.Edit(map[string]map[string]any{"docs": {"url": "https://example.com/mcp?a=1&b=2"}})
	if err != nil || !strings.Contains(string(out), "a=1&b=2") {
		t.Fatalf("got %s: %v", out, err)
	}
}
