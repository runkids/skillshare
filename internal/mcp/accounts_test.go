package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// accountService has a second Claude account: the target claude-work is Claude with
// another config directory, so its servers go to that directory's .claude.json.
func accountService(t *testing.T, mcp string) (*Service, string) {
	t.Helper()
	s := testService(t)
	work := filepath.Join(s.Home, ".claude-work")
	config := "targets:\n  claude-work:\n    agent: claude\n    config_dir: " + work + "\n" + mcp
	if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	return s, filepath.Join(work, ".claude.json")
}

func TestAccountTargetSyncsIntoItsConfigDir(t *testing.T) {
	s, workFile := accountService(t, "mcp:\n  targets: [claude, claude-work]\n  servers:\n    docs:\n      url: https://example.com/mcp\n")
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, change := range plan.Changes {
		got[change.Target] = change.Path
	}
	if len(got) != 2 || got["claude"] != filepath.Join(s.Home, ".claude.json") || got["claude-work"] != workFile {
		t.Fatalf("changes: %+v", plan.Changes)
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(workFile); !strings.Contains(string(data), "https://example.com/mcp") {
		t.Fatalf("the account's file: %s", data)
	}
}

func TestAccountTargetOnlyReceivesItsOwnServers(t *testing.T) {
	s, workFile := accountService(t, "mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n    work:\n      url: https://work.example.com/mcp\n      targets: [claude-work]\n")
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(workFile); strings.Contains(string(data), "docs") || !strings.Contains(string(data), "work.example.com") {
		t.Fatalf("the account's file: %s", data)
	}
}

func TestAccountTargetRejected(t *testing.T) {
	for name, mcp := range map[string]string{
		"a name that is no target":   "mcp:\n  targets: [claude-home]\n",
		"a server in a project file": "mcp:\n  projects:\n    /repo:\n      targets: [claude-work]\n      servers:\n        docs:\n          url: https://example.com/mcp\n",
	} {
		s, _ := accountService(t, mcp)
		if _, err := s.Preview(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Claude Code keeps a project's off list in the account's own .claude.json, so a server
// turned off for a project is turned off in every account that has it.
func TestAccountTargetProjectSwitchReachesEveryAccount(t *testing.T) {
	root := t.TempDir()
	s, workFile := accountService(t, "mcp:\n  targets: [claude, claude-work]\n  servers:\n    docs:\n      url: https://example.com/mcp\n  projects:\n    "+root+":\n      targets: [claude]\n      servers:\n        docs:\n          disabled: true\n")
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(s.Home, ".claude.json"), workFile} {
		if data, _ := os.ReadFile(path); !strings.Contains(string(data), "disabledMcpServers") {
			t.Errorf("%s has no off list: %s", path, data)
		}
	}
}

func TestAccountDetectedByItsConfigDir(t *testing.T) {
	s, workFile := accountService(t, "")
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := DetectedAccounts(source.Accounts); len(got) != 0 {
		t.Fatalf("detected without a directory: %v", got)
	}
	if err := os.MkdirAll(filepath.Dir(workFile), 0700); err != nil {
		t.Fatal(err)
	}
	if got := DetectedAccounts(source.Accounts); len(got) != 1 || got[0] != "claude-work" {
		t.Fatalf("got %v", got)
	}
}

// An account has its own file, so it is an import source under its own name.
func TestImportFromAccountReadsItsFile(t *testing.T) {
	s, workFile := accountService(t, "mcp:\n  targets: [claude-work]\n  servers: {}\n")
	if err := os.MkdirAll(filepath.Dir(workFile), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workFile, []byte(`{"mcpServers":{"work":{"url":"https://work.example.com/mcp"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	items, err := s.ImportClient("claude-work")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "work" || items[0].From != "claude-work" {
		t.Fatalf("candidates: %+v", items)
	}
}

func TestImportFromUnknownNameRejected(t *testing.T) {
	s, _ := accountService(t, "mcp:\n  targets: [claude]\n  servers: {}\n")
	if _, err := s.ImportClient("claude-home"); err == nil {
		t.Fatal("a name that is no Agent and no account was accepted")
	}
}

// A conflict resolution names the account, so importing the entry it already has adopts it:
// the server is Skillshare's from then on, and removing it takes the entry out of that file.
func TestImportResolutionAdoptsTheAccountsEntry(t *testing.T) {
	s, workFile := accountService(t, "mcp:\n  targets: [claude-work]\n  servers: {}\n")
	if err := os.MkdirAll(filepath.Dir(workFile), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workFile, []byte(`{"mcpServers":{"work":{"url":"https://work.example.com/mcp"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	server := Server{URL: "https://work.example.com/mcp"}
	adopt := []Resolution{{Target: "claude-work", Name: "work", Action: "adopt"}}
	if _, err := s.Mutate(Mutation{Name: "work", Server: &server, Resolutions: adopt}, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate(Mutation{Name: "work", Remove: true}, "", true); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(workFile); strings.Contains(string(data), "work.example.com") {
		t.Fatalf("the adopted entry stayed behind: %s", data)
	}
}

// agentAccountService has one account of agent, named <agent>-work.
func agentAccountService(t *testing.T, agent, mcp string) (*Service, string) {
	t.Helper()
	s := testService(t)
	work := filepath.Join(s.Home, "."+agent+"-work")
	config := "targets:\n  " + agent + "-work:\n    agent: " + agent + "\n    config_dir: " + work + "\n" + mcp
	if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	return s, work
}

// A Codex account keeps its own config.toml, the file CODEX_HOME moves.
func TestCodexAccountSyncsIntoItsConfigToml(t *testing.T) {
	s, work := agentAccountService(t, "codex", "mcp:\n  targets: [codex-work]\n  servers:\n    docs:\n      url: https://example.com/mcp\n")
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(work, "config.toml")
	if len(plan.Changes) != 1 || plan.Changes[0].Target != "codex-work" || plan.Changes[0].Path != want {
		t.Fatalf("changes: %+v", plan.Changes)
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(want); !strings.Contains(string(data), "https://example.com/mcp") {
		t.Fatalf("the account's file: %s", data)
	}
}

// A Pi account keeps its own mcp.json, in the directory PI_CODING_AGENT_DIR moves.
func TestPiAccountSyncsIntoItsOwnFile(t *testing.T) {
	s, work := agentAccountService(t, "pi", "mcp:\n  targets: [pi-work]\n  servers:\n    docs:\n      url: https://example.com/mcp\n")
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(work, "mcp.json")
	if len(plan.Changes) != 1 || plan.Changes[0].Target != "pi-work" || plan.Changes[0].Path != want {
		t.Fatalf("changes: %+v", plan.Changes)
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(want); !strings.Contains(string(data), "https://example.com/mcp") {
		t.Fatalf("the account's file: %s", data)
	}
}

// What an account's server had in the account's mcp-adapter.json is removed under the
// account's name, as its move to mcp.json is added.
func TestPiAccountAdapterEntryRemovedAsTheAccount(t *testing.T) {
	s, work := agentAccountService(t, "pi", "mcp:\n  targets: [pi-work]\n  servers:\n    docs:\n      url: https://example.com/mcp\n      piExtension: pi-mcp-adapter\n")
	adapter, entry := filepath.Join(work, "mcp-adapter.json"), map[string]any{"url": "https://example.com/mcp"}
	if err := writeJSONFile(adapter, map[string]any{"mcpServers": map[string]any{"docs": entry}}); err != nil {
		t.Fatal(err)
	}
	record := ownership{Owner: s.ConfigPath, Target: "pi", Path: adapter, Name: "docs", Hash: entryHash(managedEntry("pi", entry))}
	if err := writeJSONFile(s.statePath(), ledger{Version: 1, Entries: map[string]ownership{ownershipKey("pi", adapter, "docs"): record}}); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Preview()
	if err != nil || plan.Blocked {
		t.Fatalf("%+v %v", plan, err)
	}
	if c := changeFor(plan, adapter, "docs"); c == nil || c.Action != "remove" || c.Target != "pi-work" {
		t.Fatalf("adapter entry: %+v", c)
	}
}

// Removing a target leaves its name in the MCP config, so a removal can say where it is.
func TestReferencesNameWhereATargetIsStillSelected(t *testing.T) {
	s, _ := accountService(t, "mcp:\n  targets: [claude, claude-work]\n  servers:\n    docs:\n      url: https://example.com/mcp\n      targets: [claude-work]\n    jira:\n      url: https://example.com/mcp\n      targets: [claude]\n")
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	got := source.References("claude-work")
	if len(got) != 2 || got[0] != "mcp.targets" || got[1] != "mcp.servers.docs" {
		t.Fatalf("got %v", got)
	}
	if got := source.References("claude-home"); len(got) != 0 {
		t.Fatalf("a name nothing selects: %v", got)
	}
	warning := ReferenceWarning(s.ConfigPath, "claude-work")
	if !strings.Contains(warning, "mcp.servers.docs") || !strings.Contains(warning, "claude-work") {
		t.Fatalf("warning: %q", warning)
	}
	if got := ReferenceWarning(filepath.Join(s.Home, "gone.yaml"), "claude-work"); got != "" {
		t.Fatalf("a config that cannot be read must not block a removal: %q", got)
	}
}
