package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// OMP is a separate native client, not Pi's built-in MCP or its old adapters.
func TestOMPRender(t *testing.T) {
	for _, server := range []Server{
		{Command: "tool", Args: []string{"serve"}, Env: map[string]Value{"TOKEN": {FromEnv: "API_TOKEN"}}},
		{URL: "https://example.com/mcp", BearerToken: &Value{FromEnv: "API_TOKEN"}},
	} {
		entry, err := Render("omp", server)
		if err != nil {
			t.Fatal(err)
		}
		wantType := "stdio"
		if server.URL != "" {
			wantType = "http"
		}
		if entry["type"] != wantType {
			t.Fatalf("transport: %v", entry)
		}
		native, err := ParseNative("omp", nil)
		if err != nil {
			t.Fatal(err)
		}
		data, err := native.Edit(map[string]map[string]any{"docs": entry})
		if err != nil {
			t.Fatal(err)
		}
		candidates, err := Import("omp", data, "")
		if err != nil || len(candidates) != 1 || len(candidates[0].Problems) != 0 {
			t.Fatalf("roundtrip: %+v %v", candidates, err)
		}
		got := candidates[0].Server
		if server.Command != "" && got.Env["TOKEN"].FromEnv != "API_TOKEN" || server.URL != "" && (got.BearerToken == nil || got.BearerToken.FromEnv != "API_TOKEN") {
			t.Fatalf("environment reference lost: %+v", got)
		}
	}
	for _, key := range []string{"env", "headers"} {
		server := Server{Command: "tool", Env: map[string]Value{"VALUE": {Literal: "!touch /must-not-run"}}}
		if key == "headers" {
			server = Server{URL: "https://example.com/mcp", Headers: map[string]Value{"X-Value": {Literal: "!touch /must-not-run"}}}
		}
		if _, err := Render("omp", server); err == nil || !strings.Contains(err.Error(), "!") {
			t.Fatalf("executable %s literal accepted: %v", key, err)
		}
	}
}

func TestOMPPathsAndDetection(t *testing.T) {
	s := testService(t)
	want := filepath.Join(s.Home, ".omp", "agent", "mcp.json")
	if got, err := s.nativePath("omp"); err != nil || got != want {
		t.Fatalf("global path: %q %v", got, err)
	}
	if err := os.MkdirAll(filepath.Dir(want), 0700); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(s.DetectedClients(s.ClientPaths()), "omp") {
		t.Fatal("OMP agent directory was not detected")
	}
	s.ConfigDirs = map[string]string{"omp": filepath.Join(s.Home, "custom", "agent")}
	if got, err := s.nativePath("omp"); err != nil || got != filepath.Join(s.ConfigDirs["omp"], "mcp.json") {
		t.Fatalf("override path: %q %v", got, err)
	}
	s.ProjectRoot = filepath.Join(s.Home, "project")
	if got, err := s.nativePath("omp"); err != nil || got != filepath.Join(s.ProjectRoot, ".omp", "mcp.json") {
		t.Fatalf("project path: %q %v", got, err)
	}
	if err := s.checkScope(strings.Repeat("a", 101), "omp", Server{Command: "tool"}); err == nil {
		t.Fatal("OMP's 100-character name limit was not enforced")
	}
}

func TestOMPSyncPreservesNativeSettings(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			s := testService(t)
			if project {
				s.ProjectRoot = filepath.Join(s.Home, "project")
			}
			if err := os.WriteFile(s.ConfigPath, []byte("mcp: {targets: [omp], servers: {}}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			path, err := s.nativePath("omp")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			before := []byte(`{"$schema":"https://example.com/schema","disabledServers":["docs"],"enabledServers":["external"],"mcpServers":{"docs":{"type":"http","url":"https://example.com/mcp","enabled":false,"timeout":0,"instructions":false,"requestIdFormat":"string","oauth":{"scope":"read"},"auth":{"type":"oauth","credentialId":"local"}},"manual":{"command":"manual"}}}`)
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			server := Server{URL: "https://example.com/mcp", Targets: []string{"omp"}}
			if _, err := s.Mutate(Mutation{Name: "docs", Server: &server}, "", true); err != nil {
				t.Fatal(err)
			}
			server.URL = "https://example.com/updated"
			result, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, "", true)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.BackupIDs) != 1 {
				t.Fatalf("missing backup: %+v", result)
			}
			data, _ := os.ReadFile(path)
			var got map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			entry := got["mcpServers"].(map[string]any)["docs"].(map[string]any)
			for _, key := range []string{"enabled", "timeout", "instructions", "requestIdFormat", "oauth", "auth"} {
				if entry[key] == nil {
					t.Fatalf("lost OMP field %s: %s", key, data)
				}
			}
			if entry["enabled"] != false || entry["url"] != server.URL || got["disabledServers"] == nil || got["enabledServers"] == nil || got["$schema"] == nil {
				t.Fatalf("lost OMP settings: %s", data)
			}
			plan, err := s.Preview()
			if err != nil || plan.Blocked || len(plan.Changes) != 1 || plan.Changes[0].Action != "unchanged" {
				t.Fatalf("not idempotent: %+v %v", plan, err)
			}
			restore, err := s.PreviewRestore(result.BackupIDs[0])
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Restore(result.BackupIDs[0], restore.Revision); err != nil {
				t.Fatal(err)
			}
			data, _ = os.ReadFile(path)
			if !bytes.Contains(data, []byte("https://example.com/mcp")) || !bytes.Contains(data, []byte(`"disabledServers"`)) {
				t.Fatalf("restore lost native settings: %s", data)
			}
			if _, err := s.Apply(""); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Mutate(Mutation{Name: "docs", Remove: true}, "", true); err != nil {
				t.Fatal(err)
			}
			data, _ = os.ReadFile(path)
			if bytes.Contains(data, []byte(`"url"`)) || !bytes.Contains(data, []byte(`"manual"`)) || !bytes.Contains(data, []byte(`"disabledServers"`)) {
				t.Fatalf("remove lost unrelated settings: %s", data)
			}
		})
	}
}

func TestOMPImportSafety(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		problem       bool
	}{
		{"denylist", `{"disabledServers":["docs"],"enabledServers":["docs"],"mcpServers":{"docs":{"url":"https://example.com/mcp"}}}`, true},
		{"disabled entry", `{"mcpServers":{"docs":{"command":"tool","enabled":false}}}`, true},
		{"allowlist", `{"enabledServers":["docs"],"mcpServers":{"docs":{"command":"tool","enabled":false}}}`, false},
		{"secret command", `{"mcpServers":{"docs":{"command":"tool","env":{"VALUE":"!touch /must-not-run"}}}}`, true},
		{"bare reference", `{"mcpServers":{"docs":{"command":"tool","env":{"API_TOKEN":"API_TOKEN"}}}}`, false},
		{"native fields", `{"mcpServers":{"docs":{"command":"tool","timeout":0,"instructions":false}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidates, err := Import("omp", []byte(tc.content), "")
			if err != nil || len(candidates) != 1 {
				t.Fatalf("import: %+v %v", candidates, err)
			}
			c := candidates[0]
			if tc.name == "denylist" && DetectImportFormat([]byte(tc.content)) != "omp" {
				t.Fatal("OMP denylist pasted as another client's format")
			}
			if (len(c.Problems) > 0) != tc.problem {
				t.Fatalf("problem=%t: %+v", tc.problem, c)
			}
			if tc.name == "bare reference" && c.Server.Env["API_TOKEN"].FromEnv != "API_TOKEN" {
				t.Fatalf("bare reference became a literal: %+v", c)
			}
			if tc.name == "native fields" && len(c.Warnings) != 2 {
				t.Fatalf("native fields silently dropped: %+v", c)
			}
		})
	}
}

func TestOMPAccount(t *testing.T) {
	s, dir := agentAccountService(t, "omp", "mcp:\n  targets: [omp, omp-work]\n  servers:\n    docs:\n      url: https://example.com/mcp\n")
	if _, err := s.Apply(""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "mcp.json")
	if data, err := os.ReadFile(path); err != nil || !bytes.Contains(data, []byte(`"docs"`)) {
		t.Fatalf("account destination: %s %v", data, err)
	}
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	s.ConfigDirs = ConfigDirsFromEnv()
	plan, err := s.Preview()
	if err != nil || len(plan.Changes) != 2 {
		t.Fatalf("account shell: %+v %v", plan, err)
	}
	for _, c := range plan.Changes {
		if c.Action != "unchanged" {
			t.Fatalf("account shell redirected default OMP: %+v", c)
		}
	}
	candidates, err := s.ImportClient("omp-work")
	if err != nil || len(candidates) != 1 || candidates[0].From != "omp-work" {
		t.Fatalf("account import: %+v %v", candidates, err)
	}
}

func TestOMPProjectSwitch(t *testing.T) {
	s := testService(t)
	s.ProjectRoot = filepath.Join(s.Home, "project")
	if err := os.WriteFile(s.ConfigPath, []byte("mcp: {targets: [omp], servers: {}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	server := Server{Disabled: true, Targets: []string{"omp"}}
	result, err := s.Mutate(Mutation{Name: "docs", Server: &server}, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Plan.Changes) != 1 || !result.Plan.Changes[0].Switch {
		t.Fatalf("not a project switch: %+v", result.Plan)
	}
	path := filepath.Join(s.ProjectRoot, ".omp", "mcp.json")
	data, _ := os.ReadFile(path)
	native, err := ParseNative("omp", data)
	if err != nil || native.Entries["docs"]["enabled"] != false || len(native.Entries["docs"]) != 1 {
		t.Fatalf("project switch: %s %v", data, err)
	}
	if _, err := s.Mutate(Mutation{Name: "docs", Remove: true}, "", true); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if bytes.Contains(data, []byte(`"docs"`)) {
		t.Fatalf("switch not removed: %s", data)
	}
}
