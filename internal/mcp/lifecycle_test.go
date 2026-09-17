package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalMutationLeavesConfigUntouched(t *testing.T) {
	s := testService(t)
	config := "sources:\n  mcp: ./shared.yaml\nmcp:\n  targets: [claude]\n"
	if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Home, "shared.yaml")
	if err := os.WriteFile(path, []byte("servers: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m := Mutation{Name: "docs", Server: &Server{URL: "https://example.com/mcp"}}
	p, err := s.PreviewMutation(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate(m, p.Revision, true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(s.ConfigPath)
	if string(data) != config {
		t.Fatal("rewrote config instead of external source")
	}
	source, err := LoadSource(s.ConfigPath)
	if err != nil || source.Servers["docs"].URL == "" {
		t.Fatalf("external definition not saved: %v", err)
	}
}

func TestImportSaveOnlyAdoptsBaselineAndDetectsDrift(t *testing.T) {
	s := testService(t)
	path := filepath.Join(s.Home, ".claude.json")
	before := `{"mcpServers":{"local":{"command":"tool"}},"unrelated":true}`
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	m := Mutation{Name: "local", Server: &Server{Command: "tool", Targets: []string{"claude"}}, Resolutions: []Resolution{{Target: "claude", Name: "local", Action: "replace"}}}
	p, err := s.PreviewMutation(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate(m, p.Revision, false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != before {
		t.Fatal("save only changed native config")
	}
	p, err = s.Preview()
	if err != nil || p.Blocked {
		t.Fatalf("saved import cannot sync: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(before, "tool", "changed", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	p, err = s.Preview()
	if err != nil || !p.Blocked {
		t.Fatalf("adopted native drift was not detected: %v", err)
	}
}

func TestRestorePreservesUnrelatedChangesAndRejectsNewerEntry(t *testing.T) {
	s := testService(t)
	r, err := s.Apply("")
	if err != nil {
		t.Fatal(err)
	}
	id := r.BackupIDs[0]
	p, err := s.PreviewRestore(id)
	if err != nil {
		t.Fatal(err)
	}
	path := p.Changes[0].Path
	data, _ := os.ReadFile(path)
	native, err := ParseNative(p.Changes[0].Target, data)
	if err != nil {
		t.Fatal(err)
	}
	data, err = native.Edit(map[string]map[string]any{"personal": {"command": "private-tool"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	p, err = s.PreviewRestore(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Restore(id, p.Revision); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	native, err = ParseNative(p.Changes[0].Target, data)
	if err != nil || native.Entries["personal"] == nil || native.Entries["docs"] != nil {
		t.Fatalf("restore modified unrelated entry: %v", err)
	}
	p, err = s.PreviewRestore(id)
	if err != nil || !p.Blocked {
		t.Fatalf("restore accepted changed entry: %v", err)
	}
}

func TestInterruptedWriteRecovery(t *testing.T) {
	for _, written := range []bool{false, true} {
		t.Run(map[bool]string{false: "before", true: "after"}[written], func(t *testing.T) {
			s := testService(t)
			p, err := s.Preview()
			if err != nil {
				t.Fatal(err)
			}
			f := p.files[0]
			state := ledger{Version: 1, Entries: map[string]ownership{}}
			for k, v := range p.state.Entries {
				if v.Path == f.path {
					state.Entries[k] = v
				}
			}
			j := journal{Path: f.path, Before: digest(f.before), After: digest(f.after), BeforeExists: f.exists, State: state}
			if err := writeJSONFile(filepath.Join(s.StateDir, "mcp", "pending.json"), j); err != nil {
				t.Fatal(err)
			}
			if written {
				if err := atomicWrite(f.path, f.after, 0600); err != nil {
					t.Fatal(err)
				}
			}
			p, err = s.Preview()
			if err != nil || p.Blocked {
				t.Fatalf("interrupted write blocked preview: %v", err)
			}
			if _, err := os.Stat(filepath.Join(s.StateDir, "mcp", "pending.json")); err != nil {
				t.Fatal("preview wrote recovery state")
			}
			if _, err := s.Apply(p.Revision); err != nil {
				t.Fatal(err)
			}
			p, err = s.Preview()
			if err != nil || p.Blocked {
				t.Fatalf("recovery failed: %v", err)
			}
			for _, c := range p.Changes {
				if c.Action != "unchanged" {
					t.Fatalf("recovery not converged: %+v", c)
				}
			}
		})
	}
}

func TestInterruptedWriteThenExternalChangeRecovers(t *testing.T) {
	s := testService(t)
	p, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	f := p.files[0]
	j := journal{Path: f.path, Before: digest(f.before), After: digest(f.after), BeforeExists: f.exists, State: ledger{Version: 1, Entries: map[string]ownership{}}}
	if err := writeJSONFile(filepath.Join(s.StateDir, "mcp", "pending.json"), j); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.path, []byte(`{"personal":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(""); err != nil {
		t.Fatalf("stale journal blocked MCP: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.StateDir, "mcp", "pending.json")); !os.IsNotExist(err) {
		t.Fatal("stale journal was kept")
	}
}

func TestBackupsAreBoundedAndSkipDamagedFiles(t *testing.T) {
	s := testService(t)
	dir := filepath.Join(s.StateDir, "mcp", "backups")
	for i := 0; i < keepBackups+2; i++ {
		id := fmt.Sprintf("17000000000000000%02d-%s", i, backupSuffix("/agent.json"))
		if err := writeJSONFile(filepath.Join(dir, id+".json"), backupRecord{ID: id, Owner: s.ConfigPath, Target: "claude", Path: "/agent.json"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "1800000000000000000-abcdef12.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	s.pruneBackups("/agent.json")
	backups, err := s.Backups()
	if err != nil || len(backups) != keepBackups {
		t.Fatalf("got %d backups: %v", len(backups), err)
	}
}

func TestImportAdoptsIdenticalEntry(t *testing.T) {
	s := testService(t)
	path := filepath.Join(s.Home, ".claude.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"docs":{"type":"http","url":"https://example.com/mcp"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	m := Mutation{Name: "docs", Server: &Server{URL: "https://example.com/mcp", Targets: []string{"claude"}}, Replace: true, Resolutions: []Resolution{{Target: "claude", Name: "docs", Action: "adopt"}}}
	if _, err := s.Mutate(m, "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate(Mutation{Name: "docs", Remove: true}, "", true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "example.com") {
		t.Fatalf("adopted entry was not managed: %s", data)
	}
}

func TestSaveOnlyRejectsServerWithoutTargets(t *testing.T) {
	s := testService(t)
	config := "mcp:\n  servers: {}\n"
	if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &Server{URL: "https://example.com/mcp"}}, "", false); err == nil {
		t.Fatal("saved a server that can never sync")
	}
	if data, _ := os.ReadFile(s.ConfigPath); string(data) != config {
		t.Fatal("rejected server was saved")
	}
}

func TestScopeIsolationAndForeignOwnership(t *testing.T) {
	s := testService(t)
	if _, err := s.Apply(""); err != nil {
		t.Fatal(err)
	}
	other := *s
	other.ConfigPath = filepath.Join(s.Home, "other.yaml")
	data, _ := os.ReadFile(s.ConfigPath)
	if err := os.WriteFile(other.ConfigPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := other.PreviewMutation(Mutation{Resolutions: []Resolution{{Target: "claude", Name: "docs", Action: "replace"}}})
	if err != nil || !p.Blocked {
		t.Fatalf("foreign ownership bypassed: %v", err)
	}
	other.ProjectRoot = filepath.Join(s.Home, "project")
	if _, err := other.Apply(""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(other.ProjectRoot, ".mcp.json")); err != nil {
		t.Fatal(err)
	}
}

func TestUnselectedResolutionCannotAcquireOwnership(t *testing.T) {
	s := testService(t)
	path := filepath.Join(s.Home, ".claude.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"private":{"command":"tool"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	m := Mutation{Resolutions: []Resolution{{Target: "claude", Name: "private", Action: "replace"}}}
	if _, err := s.Mutate(m, "", false); err == nil {
		t.Fatal("unselected native entry was adopted")
	}
	state, _, err := s.loadLedger()
	if err != nil || len(state.Entries) != 0 {
		t.Fatalf("ownership changed: %v", err)
	}
}

func TestImportSecretsAndUnsupportedFields(t *testing.T) {
	candidates, err := Import("claude", []byte(`{"mcpServers":{"docs":{"url":"https://example.com/mcp","headers":{"Authorization":"Bearer do-not-copy"},"type":"sse"}}}`), "")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(candidates)
	if strings.Contains(string(data), "do-not-copy") || len(candidates[0].Problems) == 0 || len(candidates[0].Warnings) == 0 {
		t.Fatalf("unsafe import: %s", data)
	}
	for _, s := range []Server{
		{URL: "https://example.com/mcp?api_key=secret"},
		{Command: "tool", Env: map[string]Value{"API_TOKEN": {Literal: "secret"}}},
		{Command: "tool", Args: []string{"${workspaceFolder}"}},
		{URL: "https://example.com/mcp", Transport: "sse"},
	} {
		if err := s.Validate("docs"); err == nil {
			t.Fatalf("unsafe portable definition accepted: %+v", s)
		}
	}
}

func TestImportWarnsAboutAgentSpecificFields(t *testing.T) {
	for target, data := range map[string]string{
		"codex":  "[mcp_servers.docs]\ncommand = 'tool'\nstartup_timeout_sec = 20\n",
		"claude": `{"mcpServers":{"docs":{"type":"streamable-http","url":"https://example.com/mcp","timeout":5}}}`,
	} {
		items, err := Import(target, []byte(data), "")
		if err != nil || len(items) != 1 || len(items[0].Problems) != 0 || len(items[0].Warnings) != 1 {
			t.Fatalf("%s: %+v %v", target, items, err)
		}
	}
}

func TestImportFlagsCredentialsOutsideSecretKeys(t *testing.T) {
	items, err := Import("claude", []byte(`{"mcpServers":{"db":{"command":"tool","args":["--api-key","plain-arg"],"env":{"DATABASE_URL":"postgres://app:do-not-copy@db/app"}}}}`), "")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(items)
	if strings.Contains(string(data), "do-not-copy") || items[0].Server.Env["DATABASE_URL"].FromEnv != "DATABASE_URL" || len(items[0].Warnings) != 2 {
		t.Fatalf("credential not handled: %s", data)
	}
}
