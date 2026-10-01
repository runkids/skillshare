package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccountEnvOverrideDoesNotPruneDefaultHome(t *testing.T) {
	for agent, env := range map[string]string{"codex": "CODEX_HOME", "claude": "CLAUDE_CONFIG_DIR", "pi": "PI_CODING_AGENT_DIR"} {
		t.Run(agent, func(t *testing.T) {
			s, work := agentAccountService(t, agent, fmt.Sprintf("mcp:\n  targets: [%s, %s-work]\n  servers:\n    demo:\n      url: https://example.com/mcp\n", agent, agent))
			plan, err := s.Apply("")
			if err != nil {
				t.Fatal(err)
			}
			before := map[string][]byte{}
			for _, c := range plan.Plan.Changes {
				before[c.Path], err = os.ReadFile(c.Path)
				if err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv(env, work)
			s.ConfigDirs = ConfigDirsFromEnv()
			p, err := s.Preview()
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Changes) != 2 {
				t.Fatalf("changes: %+v", p.Changes)
			}
			for _, c := range p.Changes {
				if c.Action != "unchanged" || before[c.Path] == nil {
					t.Fatalf("account override changed the plan: %+v", c)
				}
			}
			if !strings.Contains(strings.Join(p.Notices, "\n"), env) {
				t.Fatalf("missing shadowing warning: %v", p.Notices)
			}
			if _, err := s.Apply(p.Revision); err != nil {
				t.Fatal(err)
			}
			for path, want := range before {
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("sync changed %s: %s (%v)", path, got, err)
				}
			}
		})
	}
}

func TestChangedAgentHomeParksOwnedEntries(t *testing.T) {
	for _, agent := range []string{"codex", "claude", "pi", "copilot"} {
		for _, project := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/project=%t", agent, project), func(t *testing.T) {
				s := testService(t)
				config := fmt.Sprintf("mcp:\n  targets: [%s]\n  servers:\n    demo:\n      url: https://example.com/mcp\n", agent)
				if project {
					config += fmt.Sprintf("  projects:\n    %s:\n      servers:\n        local:\n          command: local-tool\n          targets: [%s]\n", s.Home, agent)
				}
				if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
					t.Fatal(err)
				}
				first, err := s.Apply("")
				if err != nil {
					t.Fatal(err)
				}
				old := first.Plan.Changes[0].Path
				before, _ := os.ReadFile(old)
				s.ConfigDirs = map[string]string{agent: filepath.Join(s.Home, "override")}
				p, err := s.Preview()
				if err != nil {
					t.Fatal(err)
				}
				count, entries := 1, 2
				if project {
					count, entries = 2, 3
				}
				if len(p.Changes) != count || changeFor(p, old, "demo") != nil {
					t.Fatalf("old home must be parked: %+v", p.Changes)
				}
				for _, c := range p.Changes {
					if c.Name == "demo" && c.Action != "add" {
						t.Fatalf("override must receive the server: %+v", c)
					}
				}
				if !strings.Contains(strings.Join(p.Notices, "\n"), old) {
					t.Fatalf("missing parked-path warning: %v", p.Notices)
				}
				if _, err := s.Apply(p.Revision); err != nil {
					t.Fatal(err)
				}
				got, _ := os.ReadFile(old)
				state, _, err := s.loadLedger()
				if !bytes.Equal(got, before) || err != nil || len(state.Entries) != entries {
					t.Fatalf("parked file or ownership lost: %s, %+v, %v", got, state, err)
				}
				s.ConfigDirs = nil
				p, err = s.Preview()
				if err != nil || changeFor(p, old, "demo").Action != "unchanged" {
					t.Fatalf("restored home did not resume ownership: %+v %v", p, err)
				}
			})
		}
	}
}

func TestAccountEnvResolution(t *testing.T) {
	for _, agent := range []string{"codex", "claude", "pi"} {
		t.Run(agent, func(t *testing.T) {
			s, work := agentAccountService(t, agent, "mcp:\n  servers: {}\n")
			source, err := LoadSource(s.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			plain := s.ClientPaths()[agent]
			s.ConfigDirs = map[string]string{agent: work}
			// With no declared accounts, the plain Agent honors the override.
			override := s.ClientPaths()[agent]
			if filepath.Dir(override) != work {
				t.Fatalf("override ignored: %s", override)
			}
			alias := filepath.Join(s.Home, "account-alias")
			if err := os.MkdirAll(work, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(work, alias); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			s.ConfigDirs[agent] = alias
			paths := s.ConfiguredClientPaths(source)
			if paths[agent] != plain || paths[agent+"-work"] != override {
				t.Fatalf("account alias redirected plain Agent: %v", paths)
			}
			client, target, err := s.importClient(agent)
			if err != nil {
				t.Fatal(err)
			}
			if path, err := client.nativePath(target); err != nil || path != plain {
				t.Fatalf("import disagrees with sync: %s, %v", path, err)
			}
			s.ConfigDirs[agent] = filepath.Join(s.Home, "unrelated")
			paths = s.ConfiguredClientPaths(source)
			if filepath.Dir(paths[agent]) != s.ConfigDirs[agent] {
				t.Fatalf("unrelated override ignored: %v", paths)
			}
		})
	}
}

func TestRemovedAccountParksOwnedEntries(t *testing.T) {
	s, work := agentAccountService(t, "codex", "mcp:\n  targets: [codex-work]\n  servers:\n    demo:\n      url: https://example.com/mcp\n")
	if _, err := s.Apply(""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(work, "config.toml")
	before, _ := os.ReadFile(path)
	if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  servers: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := s.Apply("")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	state, _, stateErr := s.loadLedger()
	if len(result.Plan.Changes) != 0 || len(result.Plan.Notices) == 0 || !bytes.Equal(got, before) || stateErr != nil || len(state.Entries) != 1 {
		t.Fatalf("removed account was not parked: %+v, %s, %+v, %v", result.Plan, got, state, stateErr)
	}
}

func TestAccountEnvDashboardReadsDefaultHome(t *testing.T) {
	for _, agent := range []string{"codex", "claude", "pi"} {
		t.Run(agent, func(t *testing.T) {
			s, work := agentAccountService(t, agent, "mcp:\n  servers: {}\n")
			source, err := LoadSource(s.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			plain := s.ClientPaths()[agent]
			s.ConfigDirs = map[string]string{agent: work}
			account := s.AccountPaths(source.Accounts)[agent+"-work"]
			for path, name := range map[string]string{plain: "default-only", account: "account-only"} {
				native, err := ParseNative(agent, nil)
				if err != nil {
					t.Fatal(err)
				}
				data, err := native.Edit(map[string]map[string]any{name: {"command": "test-tool"}})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			paths := map[string]string{}
			for _, item := range s.ImportSources(source)[""] {
				if item.PiExtension != "pi-mcp-adapter" {
					paths[item.Target] = item.Path
				}
			}
			if paths[agent] != plain || paths[agent+"-work"] != account {
				t.Errorf("dashboard import sources disagree with sync: %v", paths)
			}
			found := map[string]Unmanaged{}
			for _, item := range s.FindUnmanaged(source) {
				found[item.Target] = item
			}
			for target, path := range map[string]string{agent: plain, agent + "-work": account} {
				name := "default-only"
				if target != agent {
					name = "account-only"
				}
				if item := found[target]; item.Path != path || len(item.Names) != 1 || item.Names[0] != name {
					t.Errorf("dashboard unmanaged scan for %s: %+v", target, item)
				}
			}
			if s.accounts != nil {
				t.Fatal("dashboard reads mutated the service scope")
			}
		})
	}
}

func TestAccountEnvCaseResolution(t *testing.T) {
	for _, agent := range []string{"codex", "claude", "pi"} {
		t.Run(agent, func(t *testing.T) {
			s, work := agentAccountService(t, agent, "mcp:\n  servers: {}\n")
			source, err := LoadSource(s.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			plain := s.ClientPaths()[agent]
			otherCase := filepath.Join(s.Home, strings.ToUpper(filepath.Base(work)))
			for _, dir := range []string{work, otherCase} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			workInfo, err := os.Stat(work)
			if err != nil {
				t.Fatal(err)
			}
			caseInfo, err := os.Stat(otherCase)
			if err != nil {
				t.Fatal(err)
			}
			alias := os.SameFile(workInfo, caseInfo)
			t.Logf("case variants name the same directory: %t", alias)
			s.ConfigDirs = map[string]string{agent: otherCase}
			want := filepath.Join(otherCase, filepath.Base(s.AccountPaths(source.Accounts)[agent+"-work"]))
			if alias {
				want = plain
			}
			if got := s.ConfiguredClientPaths(source)[agent]; got != want {
				t.Fatalf("case variant resolved to %s, want %s (alias=%t)", got, want, alias)
			}
		})
	}
}

func TestLegacyChangedHomeDoesNotSuggestReaddingProject(t *testing.T) {
	for _, agent := range []string{"codex", "claude", "pi"} {
		t.Run(agent, func(t *testing.T) {
			s := testService(t)
			config := fmt.Sprintf("mcp:\n  targets: [%s]\n  servers:\n    demo:\n      command: demo-tool\n", agent)
			if agent == "claude" {
				config += "  projects:\n    " + filepath.Join(s.Home, "project") + ":\n      servers:\n        demo:\n          disabled: true\n"
			}
			if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			applyProjects(t, s)
			state, _, err := s.loadLedger()
			if err != nil {
				t.Fatal(err)
			}
			for key, owned := range state.Entries {
				owned.Root = nil
				state.Entries[key] = owned
			}
			if err := writeJSONFile(s.statePath(), state); err != nil {
				t.Fatal(err)
			}
			s.ConfigDirs = map[string]string{agent: filepath.Join(s.Home, "override")}
			p, err := s.Preview()
			if err != nil {
				t.Fatal(err)
			}
			notices := strings.Join(p.Notices, "\n")
			if !strings.Contains(notices, "sync where its home resolves") || strings.Contains(notices, "add the project back") {
				t.Fatalf("moved home must get home recovery guidance: %v", p.Notices)
			}
		})
	}
}

func FuzzAccountHomeResolution(f *testing.F) {
	f.Add(uint8(0))
	f.Add(uint8(1))
	f.Fuzz(func(t *testing.T, depth uint8) {
		home := t.TempDir()
		account := filepath.Join(home, ".codex-2")
		// Different lexical paths still name the declared account home.
		dir := account + strings.Repeat(string(filepath.Separator)+"child"+string(filepath.Separator)+"..", int(depth)%16)
		s := &Service{Home: home, ConfigDirs: map[string]string{"codex": dir}}
		paths := s.ConfiguredClientPaths(&Source{Accounts: map[string]Account{"codex-2": {Agent: "codex", Dir: account}}})
		if paths["codex"] != filepath.Join(home, ".codex", "config.toml") || paths["codex-2"] != filepath.Join(account, "config.toml") {
			t.Fatalf("lexical alias changed homes: %v", paths)
		}
	})
}

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
	config, err := os.ReadFile(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.ConfigPath, config[:strings.Index(string(config), "  projects:\n")], 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(""); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(s.Home, ".claude.json"), workFile} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var document struct {
			Projects map[string]struct{ DisabledMcpServers []string }
		}
		if err := json.Unmarshal(data, &document); err != nil || len(document.Projects[root].DisabledMcpServers) != 0 {
			t.Errorf("%s kept the removed project's switch: %s (%v)", path, data, err)
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
