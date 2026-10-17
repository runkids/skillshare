package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// options carries native per-server fields Skillshare has no setting for, per Agent. Refs: #578.
func TestOptionsSyncFollowsSource(t *testing.T) {
	s := testService(t)
	sync := func(servers string) (codex, claude string) {
		t.Helper()
		config := "mcp:\n  targets: [codex, claude]\n  servers:\n" + servers
		if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Preview()
		if err != nil || plan.Blocked {
			t.Fatalf("%+v %v", plan, err)
		}
		if _, err := s.Apply(plan.Revision); err != nil {
			t.Fatal(err)
		}
		a, _ := os.ReadFile(filepath.Join(s.Home, ".codex", "config.toml"))
		b, _ := os.ReadFile(filepath.Join(s.Home, ".claude.json"))
		return string(a), string(b)
	}
	options := "      options:\n        codex:\n          default_tools_approval_mode: approve\n        claude:\n          timeout: 600000\n"
	codex, claude := sync("    docs:\n      url: https://example.com/mcp\n" + options)
	if !strings.Contains(codex, "default_tools_approval_mode = 'approve'") || strings.Contains(codex, "timeout") {
		t.Fatalf("codex: %s", codex)
	}
	if !strings.Contains(claude, `"timeout": 600000`) || strings.Contains(claude, "approval") {
		t.Fatalf("claude: %s", claude)
	}
	// A recreated entry gets the fields too, which hand-added ones did not.
	codex, claude = sync("    renamed:\n      url: https://example.com/mcp\n" + options)
	if !strings.Contains(codex, "[mcp_servers.renamed]") || !strings.Contains(codex, "default_tools_approval_mode = 'approve'") || strings.Contains(codex, "[mcp_servers.docs]") {
		t.Fatalf("codex after rename: %s", codex)
	}
	if !strings.Contains(claude, `"timeout": 600000`) || strings.Contains(claude, `"docs"`) {
		t.Fatalf("claude after rename: %s", claude)
	}
	// Dropping an option removes the field sync wrote.
	codex, claude = sync("    renamed:\n      url: https://example.com/mcp\n")
	if strings.Contains(codex, "default_tools_approval_mode") || strings.Contains(claude, "timeout") {
		t.Fatalf("options not removed:\n%s\n%s", codex, claude)
	}
}

// An option the Agent changed since sync is not removed silently when the source drops it.
func TestDroppedOptionChangedInAgentConflicts(t *testing.T) {
	s := testService(t)
	write := func(config string) *Plan {
		t.Helper()
		if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Preview()
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	plan := write("mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n      options:\n        claude:\n          timeout: 600000\n")
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Home, ".claude.json")
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), "600000", "900000", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	plan = write("mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n")
	if !plan.Blocked || !strings.HasPrefix(plan.Changes[0].Message, "Agent setting changed since sync") {
		t.Fatalf("%+v", plan.Changes)
	}
}

func TestOptionsRejected(t *testing.T) {
	for name, server := range map[string]Server{
		"switch only":                    {Disabled: true, Options: map[string]map[string]any{"claude": {"timeout": 1}}},
		"unknown Agent":                  {URL: "https://example.com", Options: map[string]map[string]any{"nope": {"timeout": 1}}},
		"pi has piOptions":               {URL: "https://example.com", Options: map[string]map[string]any{"pi": {"timeout": 1}}},
		"a field Skillshare writes":      {URL: "https://example.com", Options: map[string]map[string]any{"codex": {"url": "https://other.example"}}},
		"a portable name Render renames": {Command: "echo", Options: map[string]map[string]any{"opencode": {"args": []any{"x"}}}},
		"null":                           {URL: "https://example.com", Options: map[string]map[string]any{"claude": {"timeout": nil}}},
		"array of tables":                {URL: "https://example.com", Options: map[string]map[string]any{"codex": {"x": []any{map[string]any{"a": 1}}}}},
	} {
		if err := server.Validate("docs"); err == nil || !strings.Contains(err.Error(), "options") {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

// A credential in options reaches the Agent's file, which a project may commit. Refs: #578.
func TestOptionsRefusePlainTextSecrets(t *testing.T) {
	secret := Server{URL: "https://example.com", Options: map[string]map[string]any{"opencode": {"oauth": map[string]any{"clientSecret": "abc"}}}}
	if err := secret.Validate("docs"); err == nil || !strings.Contains(err.Error(), "options.opencode.oauth.clientSecret") {
		t.Fatalf("plain text: got %v", err)
	}
	reference := Server{URL: "https://example.com", Options: map[string]map[string]any{"opencode": {"oauth": map[string]any{"clientSecret": "{env:DOCS_SECRET}"}}}}
	if err := reference.Validate("docs"); err != nil {
		t.Fatalf("environment reference: %v", err)
	}
}

// Copilot's tools list is not a managed field, but Render writes it, so an option cannot replace it.
func TestOptionsCannotReplaceRenderedField(t *testing.T) {
	server := Server{Command: "echo", Options: map[string]map[string]any{"copilot": {"tools": []any{"a"}}}}
	if _, err := Render("copilot", server); err == nil || !strings.Contains(err.Error(), "options") {
		t.Fatalf("got %v", err)
	}
}
