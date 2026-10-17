package mcp

import (
	"os"
	"path/filepath"
	"slices"
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
	// An explicit replace lets the source win and removes the field.
	server := Server{URL: "https://example.com/mcp"}
	plan, err := s.PreviewMutation(Mutation{Name: "docs", Server: &server, Replace: true, Resolutions: []Resolution{{Target: "claude", Name: "docs", Action: "replace"}}})
	if err != nil || plan.Blocked || plan.Changes[0].Fields == nil || !slices.Contains(plan.Changes[0].Fields.Removed, "timeout") {
		t.Fatalf("replace: %+v %v", plan, err)
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
	reference := Server{URL: "https://example.com", Options: map[string]map[string]any{"opencode": {"oauth": map[string]any{"clientSecret": "{env:DOCS_SECRET}"}}, "claude": {"api_key": "${DOCS_KEY}"}}}
	if err := reference.Validate("docs"); err != nil {
		t.Fatalf("environment reference: %v", err)
	}
	// Only a whole reference in the Agent's own syntax keeps the secret out of the file.
	for name, options := range map[string]map[string]map[string]any{
		"text around a reference":     {"opencode": {"api_key": "secret-{env:DOCS_KEY}"}},
		"another Agent's syntax":      {"claude": {"api_key": "{env:DOCS_KEY}"}},
		"an Agent without references": {"codex": {"api_key": "${DOCS_KEY}"}},
		// YAML reads an unquoted number as one, and a credentials mapping names its fields plainly.
		"a number":               {"opencode": {"api_key": 123456}},
		"under a sensitive name": {"opencode": {"api_key": map[string]any{"value": "abc"}}},
	} {
		if err := (Server{URL: "https://example.com", Options: options}).Validate("docs"); err == nil || !strings.Contains(err.Error(), "api_key") {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

// Kilo discards a project file holding any {env:} reference, options included. Refs: #578.
func TestKiloCodeProjectRefusesReferencesInOptions(t *testing.T) {
	s := testService(t)
	if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [kilocode]\n  servers:\n    docs:\n      url: https://example.com/mcp\n      options:\n        kilocode:\n          oauth: {clientSecret: \"{env:DOCS_SECRET}\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s.ProjectRoot = t.TempDir()
	if _, err := s.Preview(); err == nil || !strings.Contains(err.Error(), "Kilo") {
		t.Fatalf("project config must refuse the reference: %v", err)
	}
}

// Taking over an entry that matches writes the source's options in the same sync. Refs: #578.
func TestAdoptionWritesOptions(t *testing.T) {
	s := testService(t)
	path := filepath.Join(s.Home, ".claude.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"docs":{"type":"http","url":"https://example.com/mcp"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n      options:\n        claude:\n          timeout: 600000\n"), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Preview()
	if err != nil || plan.Blocked {
		t.Fatalf("%+v %v", plan, err)
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), `"timeout": 600000`) {
		t.Fatalf("option not written on takeover: %s", data)
	}
}

// Copilot's tools list is not a managed field, but Render writes it, so an option cannot replace it.
func TestOptionsCannotReplaceRenderedField(t *testing.T) {
	server := Server{Command: "echo", Options: map[string]map[string]any{"copilot": {"tools": []any{"a"}}}}
	if _, err := Render("copilot", server); err == nil || !strings.Contains(err.Error(), "options") {
		t.Fatalf("got %v", err)
	}
}
