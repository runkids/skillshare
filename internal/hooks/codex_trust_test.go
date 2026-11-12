package hooks

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// fakeCodex stands in for `codex app-server`: it lists every handler in a home's
// hooks.json under Codex's key format and keeps hooks.state in that home's config.toml.
type fakeCodex struct {
	t           *testing.T
	disabled    map[string]bool
	writes      []map[string]string
	beforeList  func(home string)
	setTrustErr error
}

var fakeCodexEvents = map[string]string{"SessionStart": "session_start", "Stop": "stop"}

type fakeCodexServer struct {
	f    *fakeCodex
	home string
}

func (f *fakeCodex) start(home string) (CodexAppServer, error) { return &fakeCodexServer{f, home}, nil }

func (s *fakeCodexServer) state() map[string]map[string]any {
	var doc struct {
		Hooks struct {
			State map[string]map[string]any `toml:"state"`
		} `toml:"hooks"`
	}
	if data, err := os.ReadFile(filepath.Join(s.home, "config.toml")); err == nil {
		must(s.f.t, toml.Unmarshal(data, &doc))
	}
	if doc.Hooks.State == nil {
		return map[string]map[string]any{}
	}
	return doc.Hooks.State
}

func (s *fakeCodexServer) ListHooks() ([]CodexHook, error) {
	if s.f.beforeList != nil {
		s.f.beforeList(s.home)
	}
	path := filepath.Join(s.home, "hooks.json")
	if !exists(path) {
		return nil, nil
	}
	state := s.state()
	var out []CodexHook
	for event, groups := range events(s.f.t, path, true) {
		for g, group := range groups {
			handlers, _ := group.(map[string]any)["hooks"].([]any)
			for h, handler := range handlers {
				data, _ := json.Marshal([]any{event, handler})
				hash := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
				key := fmt.Sprintf("%s:%s:%d:%d", path, fakeCodexEvents[event], g, h)
				status := "untrusted"
				if trusted, ok := state[key]["trusted_hash"].(string); ok {
					status = map[bool]string{true: "trusted", false: "modified"}[trusted == hash]
				}
				out = append(out, CodexHook{Key: key, SourcePath: path, Enabled: !s.f.disabled[key], CurrentHash: hash, TrustStatus: status})
			}
		}
	}
	return out, nil
}

func (s *fakeCodexServer) SetTrust(hashes map[string]string) error {
	if s.f.setTrustErr != nil {
		return s.f.setTrustErr
	}
	s.f.writes = append(s.f.writes, hashes)
	state := s.state()
	for key, hash := range hashes {
		if state[key] == nil {
			state[key] = map[string]any{}
		}
		if hash == "" {
			delete(state[key], "trusted_hash")
		} else {
			state[key]["trusted_hash"] = hash
		}
	}
	data, err := toml.Marshal(map[string]any{"hooks": map[string]any{"state": state}})
	must(s.f.t, err)
	write(s.f.t, filepath.Join(s.home, "config.toml"), string(data))
	return nil
}

func (s *fakeCodexServer) Close() error { return nil }

func trustEnv(t *testing.T) (*env, *fakeCodex) {
	t.Helper()
	e := accountEnv(t)
	f := &fakeCodex{t: t, disabled: map[string]bool{}}
	e.service.CodexAppServer = f.start
	return e, f
}

func trustActions(t *testing.T, s *Service) string {
	t.Helper()
	trust, err := s.TrustCodex()
	must(t, err)
	var out []string
	for _, r := range trust {
		out = append(out, fmt.Sprintf("%s:%s:%s:%s", r.Target, r.Entry, r.Key[strings.LastIndex(r.Key, "hooks.json:")+len("hooks.json:"):], r.Action))
	}
	slices.Sort(out)
	return strings.Join(out, ",")
}

func trustedHash(t *testing.T, home, key string) string {
	t.Helper()
	hashes, err := codexTrustedHashes(filepath.Join(home, "config.toml"))
	must(t, err)
	return hashes[canonicalCodexHooksPath(filepath.Join(home, "hooks.json"))+":"+key]
}

func TestCodexTrustRecordsOnlyOwnedHooksInEachHome(t *testing.T) {
	e, _ := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	write(t, filepath.Join(codex, "hooks.json"), `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"mine"}]}]}}`)
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex", "codex-2")})

	if got, want := trustActions(t, e.service), "codex-2:k:session_start:0:0:trusted,codex:k:session_start:1:0:trusted"; got != want {
		t.Fatalf("trust = %s, want %s", got, want)
	}
	if trustedHash(t, codex, "session_start:0:0") != "" {
		t.Fatal("trusted a hook Skillshare does not own")
	}
	if trustedHash(t, codex, "session_start:1:0") == "" || trustedHash(t, filepath.Join(e.home, ".codex-2"), "session_start:0:0") == "" {
		t.Fatal("owned hook not trusted")
	}
	if exists(filepath.Join(e.home, ".codex-3", "config.toml")) {
		t.Fatal("wrote a home with no Skillshare hooks")
	}
	if got := trustActions(t, e.service); got != "codex-2:k:session_start:0:0:unchanged,codex:k:session_start:1:0:unchanged" {
		t.Fatalf("second run = %s", got)
	}

	// A missing account home is skipped, not an error.
	must(t, os.RemoveAll(filepath.Join(e.home, ".codex-2")))
	if got := trustActions(t, e.service); got != "codex:k:session_start:1:0:unchanged" {
		t.Fatalf("missing home = %s", got)
	}
}

func TestCodexTrustFollowsMovesAndClearsWhatItWrote(t *testing.T) {
	e, f := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex")})
	trustActions(t, e.service)
	old := trustedHash(t, codex, "session_start:0:0")

	// Another hook takes position 0, so Skillshare's moves to 1.
	path := filepath.Join(codex, "hooks.json")
	groups := append([]any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "mine"}}}}, events(t, path, true)["SessionStart"]...)
	data, err := json.Marshal(map[string]any{"hooks": map[string]any{"SessionStart": groups}})
	must(t, err)
	write(t, path, string(data))
	if got := trustActions(t, e.service); got != "codex::session_start:0:0:cleared,codex:k:session_start:1:0:trusted" {
		t.Fatalf("after move = %s", got)
	}
	if trustedHash(t, codex, "session_start:0:0") != "" || trustedHash(t, codex, "session_start:1:0") != old {
		t.Fatal("trust did not follow the hook")
	}

	// Trust the user recorded themselves stays when Skillshare's hook leaves.
	server, _ := f.start(codex)
	must(t, server.SetTrust(map[string]string{canonicalCodexHooksPath(path) + ":session_start:0:0": "sha256:user"}))
	save(t, e.service, Mutation{Name: "k", Remove: true})
	if got := trustActions(t, e.service); got != "codex::session_start:1:0:cleared" {
		t.Fatalf("after remove = %s", got)
	}
	if trustedHash(t, codex, "session_start:0:0") != "sha256:user" || trustedHash(t, codex, "session_start:1:0") != "" {
		t.Fatal("removal cleared the wrong trust")
	}
	if got := trustActions(t, e.service); got != "" {
		t.Fatalf("nothing left to do, got %s", got)
	}
}

func TestCodexTrustKeepsTrustChangedSinceAndReportsDisabled(t *testing.T) {
	e, f := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex")})
	key := canonicalCodexHooksPath(filepath.Join(codex, "hooks.json")) + ":session_start:0:0"
	f.disabled[key] = true
	trust, err := e.service.TrustCodex()
	must(t, err)
	if len(trust) != 1 || trust[0].Action != "trusted" || !trust[0].Disabled {
		t.Fatalf("trust = %+v", trust)
	}

	// Someone re-trusts the position with another hash; removing the hook leaves it.
	server, _ := f.start(codex)
	must(t, server.SetTrust(map[string]string{key: "sha256:other"}))
	save(t, e.service, Mutation{Name: "k", Remove: true})
	writes := len(f.writes)
	if got := trustActions(t, e.service); got != "" {
		t.Fatalf("trust = %s", got)
	}
	if len(f.writes) != writes || trustedHash(t, codex, "session_start:0:0") != "sha256:other" {
		t.Fatal("cleared trust Skillshare did not write")
	}
}

func TestCodexTrustRefusesProjectScope(t *testing.T) {
	e, _ := trustEnv(t)
	_, err := e.project(filepath.Join(t.TempDir(), "repo")).TrustCodex()
	if err == nil || !strings.Contains(err.Error(), "-g") {
		t.Fatalf("err = %v", err)
	}
}

func TestCodexEventKey(t *testing.T) {
	for in, want := range map[string]string{"PreToolUse": "pre_tool_use", "Stop": "stop", "UserPromptSubmit": "user_prompt_submit"} {
		if got := codexEventKey(in); got != want {
			t.Errorf("%s = %s, want %s", in, got, want)
		}
	}
}

type discardCloser struct{}

func (discardCloser) Write(p []byte) (int, error) { return len(p), nil }
func (discardCloser) Close() error                { return nil }

func TestCodexCallSkipsServerRequestsWithTheSameID(t *testing.T) {
	p := &codexProcess{stdin: discardCloser{}, lines: make(chan []byte, 3)}
	p.lines <- []byte(`{"method":"item/started","params":{}}`)
	p.lines <- []byte(`{"id":1,"method":"account/refresh","params":{}}`)
	p.lines <- []byte(`{"id":1,"result":{"ok":true}}`)
	got, err := p.call("config/batchWrite", nil)
	must(t, err)
	if string(got) != `{"ok":true}` {
		t.Fatalf("result = %s", got)
	}
}

func TestCodexTrustIsScopedByConfigOwner(t *testing.T) {
	e, f := trustEnv(t)
	first := e.service
	second := *first
	second.ConfigPath = filepath.Join(filepath.Dir(e.config), "other.yaml")
	write(t, second.ConfigPath, "targets: {}\n")
	codex := filepath.Join(e.home, ".codex")

	save(t, first, Mutation{Name: "first", Entry: codexTrustEntry(t, "echo first")})
	trust, err := second.TrustCodex()
	if err != nil {
		t.Fatal(err)
	}
	if len(trust) != 0 || len(f.writes) != 0 || exists(filepath.Join(codex, "config.toml")) {
		t.Fatalf("second config trusted first config's hook: trust=%+v writes=%v", trust, f.writes)
	}

	trust, err = first.TrustCodex()
	if err != nil || len(trust) != 1 || trust[0].Action != "trusted" {
		t.Fatalf("first trust = %+v, %v", trust, err)
	}
	firstHash := trustedHash(t, codex, "session_start:0:0")

	save(t, &second, Mutation{Name: "second", Entry: codexTrustEntry(t, "echo second")})
	trust, err = second.TrustCodex()
	if err != nil || len(trust) != 1 || trust[0].Entry != "second" || trust[0].Action != "trusted" {
		t.Fatalf("second trust = %+v, %v", trust, err)
	}
	secondHash := trustedHash(t, codex, "session_start:1:0")
	if firstHash == "" || secondHash == "" {
		t.Fatalf("missing per-config trust: first=%q second=%q", firstHash, secondHash)
	}

	save(t, first, Mutation{Name: "first", Remove: true})
	trust, err = first.TrustCodex()
	if err != nil || len(trust) != 1 || trust[0].Action != "cleared" {
		t.Fatalf("first removal trust = %+v, %v", trust, err)
	}
	if got := trustedHash(t, codex, "session_start:0:0"); got != "" {
		t.Fatalf("first config's trust remains: %q", got)
	}
	if got := trustedHash(t, codex, "session_start:1:0"); got != secondHash {
		t.Fatalf("second config's trust changed: %q, want %q", got, secondHash)
	}
}

func TestCodexTrustLocatesIdenticalHooksAcrossConfigOwners(t *testing.T) {
	for _, tc := range []struct {
		name  string
		alias bool
	}{{name: "shared path"}, {name: "symlink alias", alias: true}} {
		t.Run(tc.name, func(t *testing.T) {
			e, _ := trustEnv(t)
			first := e.service
			second := *first
			second.ConfigPath = filepath.Join(filepath.Dir(e.config), "other.yaml")
			write(t, second.ConfigPath, "targets: {}\n")
			codex := filepath.Join(e.home, ".codex")
			hooksPath := filepath.Join(codex, "hooks.json")
			if tc.alias {
				must(t, os.MkdirAll(codex, 0755))
				alias := filepath.Join(e.home, "codex-alias")
				if err := os.Symlink(codex, alias); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				second.ConfigDirs = map[string]string{"codex": alias}
			}

			save(t, first, Mutation{Name: "first", Entry: codexTrustEntry(t, "echo shared")})
			save(t, &second, Mutation{Name: "second", Entry: codexTrustEntry(t, "echo different")})
			save(t, &second, Mutation{Name: "second", Entry: codexTrustEntry(t, "echo shared")})

			trust, err := second.TrustCodex()
			if err != nil || len(trust) != 1 || trust[0].Entry != "second" || trust[0].Action != "trusted" {
				t.Fatalf("second config trust = %+v, %v", trust, err)
			}
			firstOwner, err := filepath.Abs(first.ConfigPath)
			must(t, err)
			secondOwner, err := filepath.Abs(second.ConfigPath)
			must(t, err)
			state, _, err := second.loadLedger()
			must(t, err)
			firstIndex, secondIndex := -1, -1
			for _, r := range state.Records {
				if r.Owner == firstOwner && r.Entry == "first" {
					firstIndex = r.Index
				}
				if r.Owner == secondOwner && r.Entry == "second" {
					secondIndex = r.Index
				}
			}
			if firstIndex < 0 || secondIndex < 0 || firstIndex == secondIndex {
				t.Fatalf("hook indexes = first:%d second:%d", firstIndex, secondIndex)
			}
			hooksPath = canonicalCodexHooksPath(hooksPath)
			firstKey := fmt.Sprintf("%s:session_start:%d:0", hooksPath, firstIndex)
			secondKey := fmt.Sprintf("%s:session_start:%d:0", hooksPath, secondIndex)
			if trust[0].Key != secondKey {
				t.Fatalf("trusted key = %q, want second config's %q", trust[0].Key, secondKey)
			}
			hashes, err := codexTrustedHashes(filepath.Join(codex, "config.toml"))
			must(t, err)
			if hashes[firstKey] != "" || hashes[secondKey] == "" {
				t.Fatalf("trust hashes = %+v; want only second config's key", hashes)
			}
		})
	}
}

func TestCodexTrustKeepsReceiptsForParkedHomes(t *testing.T) {
	e, _ := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	save(t, e.service, Mutation{Name: "owned", Entry: codexTrustEntry(t, "echo owned")})
	trustActions(t, e.service)
	key := canonicalCodexHooksPath(filepath.Join(codex, "hooks.json")) + ":session_start:0:0"
	ownedHash := trustedHash(t, codex, "session_start:0:0")

	// Switching CODEX_HOME parks the existing registration until the old home is active again.
	other := filepath.Join(e.home, ".codex-other")
	must(t, os.MkdirAll(other, 0755))
	e.service.ConfigDirs["codex"] = other
	if _, err := e.service.TrustCodex(); err != nil {
		t.Fatal(err)
	}
	var saved codexTrustState
	must(t, json.Unmarshal([]byte(read(t, e.service.codexTrustPath())), &saved))
	if saved.Homes[codexTrustHomeKey(e.service.ConfigPath, canonicalCodexHooksPath(filepath.Join(codex, "hooks.json")))].Keys[key] != ownedHash {
		t.Fatalf("parked receipt was discarded: %+v", saved.Homes)
	}

	e.service.ConfigDirs["codex"] = ""
	save(t, e.service, Mutation{Name: "owned", Remove: true})
	trust, err := e.service.TrustCodex()
	if err != nil || len(trust) != 1 || trust[0].Action != "cleared" {
		t.Fatalf("restored home cleanup = %+v, %v", trust, err)
	}
	if trustedHash(t, codex, "session_start:0:0") != "" {
		t.Fatal("restored home kept trust after its hook was removed")
	}
}

func TestCodexTrustCoalescesSymlinkedHomes(t *testing.T) {
	e, f := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	path := filepath.Join(codex, "hooks.json")
	alias := filepath.Join(e.home, "codex-alias")
	must(t, os.Symlink(codex, alias))
	save(t, e.service, Mutation{Name: "owned", Entry: codexTrustEntry(t, "echo owned")})
	trustActions(t, e.service)

	owner, err := filepath.Abs(e.service.ConfigPath)
	must(t, err)
	var saved codexTrustState
	must(t, json.Unmarshal([]byte(read(t, e.service.codexTrustPath())), &saved))
	canonical := canonicalCodexHooksPath(path)
	pathKey := codexTrustHomeKey(owner, canonical)
	home := saved.Homes[pathKey]
	if home.Path != canonical || len(saved.Homes) != 1 {
		t.Fatalf("initial trust homes = %+v", saved.Homes)
	}
	aliasPath := filepath.Join(alias, "hooks.json")
	home.Path = aliasPath
	saved.Homes[codexTrustHomeKey(owner, aliasPath)] = home
	data, err := json.Marshal(saved)
	must(t, err)
	write(t, e.service.codexTrustPath(), string(data))

	// Trust reconciliation sees the same home through CODEX_HOME's symlink.
	e.service.ConfigDirs["codex"] = alias
	starts := 0
	e.service.CodexAppServer = func(home string) (CodexAppServer, error) {
		starts++
		return f.start(home)
	}
	trust, err := e.service.TrustCodex()
	if err != nil || len(trust) != 1 || trust[0].Action != "unchanged" {
		t.Fatalf("symlinked home trust = %+v, %v", trust, err)
	}
	if starts != 1 {
		t.Fatalf("Codex app-server starts = %d, want 1", starts)
	}
	must(t, json.Unmarshal([]byte(read(t, e.service.codexTrustPath())), &saved))
	canonicalPathKey := codexTrustHomeKey(owner, canonical)
	if len(saved.Homes) != 1 || saved.Homes[canonicalPathKey].Path != canonical {
		t.Fatalf("symlinked trust homes were not coalesced: %+v", saved.Homes)
	}
}

func TestCodexTrustKeepsParkedReceiptsWhenDuplicateCountsAreAmbiguous(t *testing.T) {
	e, _ := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	path := filepath.Join(codex, "hooks.json")
	save(t, e.service, Mutation{Name: "owned", Entry: codexTrustEntry(t, "echo owned")})
	trustActions(t, e.service)
	key := canonicalCodexHooksPath(path) + ":session_start:0:0"
	ownedHash := trustedHash(t, codex, "session_start:0:0")

	groups := events(t, path, true)["SessionStart"]
	original := slices.Clone(groups)
	groups = append(slices.Clone(groups), groups[0])
	data, err := json.Marshal(map[string]any{"hooks": map[string]any{"SessionStart": groups}})
	must(t, err)
	write(t, path, string(data))

	other := filepath.Join(e.home, ".codex-other")
	must(t, os.MkdirAll(other, 0755))
	e.service.ConfigDirs["codex"] = other
	if trust, err := e.service.TrustCodex(); err != nil || len(trust) != 0 {
		t.Fatalf("parked duplicate trust = %+v, %v", trust, err)
	}
	var saved codexTrustState
	must(t, json.Unmarshal([]byte(read(t, e.service.codexTrustPath())), &saved))
	if saved.Homes[codexTrustHomeKey(e.service.ConfigPath, canonicalCodexHooksPath(path))].Keys[key] != ownedHash {
		t.Fatalf("ambiguous parked receipt was discarded: %+v", saved.Homes)
	}

	data, err = json.Marshal(map[string]any{"hooks": map[string]any{"SessionStart": original}})
	must(t, err)
	write(t, path, string(data))
	e.service.ConfigDirs["codex"] = ""
	save(t, e.service, Mutation{Name: "owned", Remove: true})
	trust, err := e.service.TrustCodex()
	if err != nil || len(trust) != 1 || trust[0].Action != "cleared" {
		t.Fatalf("restored ambiguous receipt cleanup = %+v, %v", trust, err)
	}
}

func TestCodexTrustClearsRemovedReceiptsWhileAnotherHookIsParked(t *testing.T) {
	e, _ := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	save(t, e.service, Mutation{Name: "kept", Entry: codexTrustEntry(t, "echo kept")})
	save(t, e.service, Mutation{Name: "removed", Entry: codexTrustEntry(t, "echo removed")})
	trust, err := e.service.TrustCodex()
	if err != nil || len(trust) != 2 {
		t.Fatalf("initial trust = %+v, %v", trust, err)
	}
	keptHash := trustedHash(t, codex, "session_start:0:0")
	removedHash := trustedHash(t, codex, "session_start:1:0")
	if keptHash == "" || removedHash == "" {
		t.Fatal("initial trust hashes are missing")
	}

	// An ordinary sync removes one registration without reconciling its saved trust.
	save(t, e.service, Mutation{Name: "removed", Remove: true})
	other := filepath.Join(e.home, ".codex-other")
	must(t, os.MkdirAll(other, 0755))
	e.service.ConfigDirs["codex"] = other
	trust, err = e.service.TrustCodex()
	if err != nil || len(trust) != 1 || trust[0].Action != "cleared" {
		t.Fatalf("parked cleanup = %+v, %v", trust, err)
	}
	if trustedHash(t, codex, "session_start:0:0") != keptHash || trustedHash(t, codex, "session_start:1:0") != "" {
		t.Fatal("parked receipt cleanup changed the kept trust or retained removed trust")
	}
}

func TestCodexTrustRevalidatesHooksSnapshotAfterList(t *testing.T) {
	e, f := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	path := filepath.Join(codex, "hooks.json")
	save(t, e.service, Mutation{Name: "owned", Entry: codexTrustEntry(t, "echo owned")})
	f.beforeList = func(home string) {
		groups := events(t, path, true)["SessionStart"]
		groups = append([]any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "echo foreign"}}}}, groups...)
		data, err := json.Marshal(map[string]any{"hooks": map[string]any{"SessionStart": groups}})
		must(t, err)
		write(t, path, string(data))
	}

	trust, err := e.service.TrustCodex()
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("trust = %+v, err = %v; expected changed hooks.json to fail closed", trust, err)
	}
	if len(f.writes) != 0 || exists(filepath.Join(codex, "config.toml")) {
		t.Fatalf("stale snapshot was trusted: trust=%+v writes=%v", trust, f.writes)
	}
}

func TestCodexTrustDoesNotReportUnconfirmedEdits(t *testing.T) {
	e, f := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	save(t, e.service, Mutation{Name: "owned", Entry: codexTrustEntry(t, "echo owned")})
	f.setTrustErr = fmt.Errorf("batch write failed")
	trust, err := e.service.TrustCodex()
	if err == nil {
		t.Fatal("expected failed config/batchWrite")
	}
	for _, result := range trust {
		if result.Action == "trusted" || result.Action == "cleared" {
			t.Fatalf("reported an unconfirmed edit: %+v", result)
		}
	}
	if len(trust) != 0 || exists(filepath.Join(codex, "config.toml")) {
		t.Fatalf("failed write reported or applied trust: trust=%+v", trust)
	}

	f.setTrustErr = nil
	trust, err = e.service.TrustCodex()
	if err != nil || len(trust) != 1 || trust[0].Action != "trusted" {
		t.Fatalf("successful trust = %+v, %v", trust, err)
	}
	ownedHash := trustedHash(t, codex, "session_start:0:0")
	save(t, e.service, Mutation{Name: "owned", Remove: true})
	f.setTrustErr = fmt.Errorf("batch write failed")
	trust, err = e.service.TrustCodex()
	if err == nil {
		t.Fatal("expected failed clear")
	}
	for _, result := range trust {
		if result.Action == "cleared" {
			t.Fatalf("reported an unconfirmed clear: %+v", result)
		}
	}
	if len(trust) != 0 || trustedHash(t, codex, "session_start:0:0") != ownedHash {
		t.Fatalf("failed clear reported or changed trust: trust=%+v", trust)
	}
}

func TestCodexTrustMigratesLegacyStateForRemovedHooks(t *testing.T) {
	e, f := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	save(t, e.service, Mutation{Name: "owned", Entry: codexTrustEntry(t, "echo owned")})
	server, err := f.start(canonicalPath(codex))
	must(t, err)
	listed, err := server.ListHooks()
	must(t, err)
	keys := map[string]string{}
	for _, hook := range listed {
		keys[hook.Key] = hook.CurrentHash
	}
	must(t, server.SetTrust(keys))
	legacy, err := json.Marshal(map[string]any{
		"version": 1,
		"homes": map[string]any{
			filepath.Join(codex, "hooks.json"): map[string]any{"target": "codex", "keys": keys},
		},
	})
	must(t, err)
	write(t, e.service.codexTrustPath(), string(legacy))

	save(t, e.service, Mutation{Name: "owned", Remove: true})
	trust, err := e.service.TrustCodex()
	if err != nil || len(trust) != 1 || trust[0].Action != "cleared" {
		t.Fatalf("legacy removal trust = %+v, %v", trust, err)
	}
	if got := trustedHash(t, codex, "session_start:0:0"); got != "" {
		t.Fatalf("legacy trust remains after removal: %q", got)
	}
	data := read(t, e.service.codexTrustPath())
	var state codexTrustState
	must(t, json.Unmarshal([]byte(data), &state))
	if len(state.Homes) != 0 {
		t.Fatalf("legacy trust metadata remains after cleanup: %+v", state.Homes)
	}
}

func TestCodexTrustCoalescesLegacySymlinkedHomesForRemovedHooks(t *testing.T) {
	e, f := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	path := filepath.Join(codex, "hooks.json")
	alias := filepath.Join(e.home, "codex-alias")
	if err := os.Symlink(codex, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	save(t, e.service, Mutation{Name: "owned", Entry: codexTrustEntry(t, "echo owned")})
	server, err := f.start(canonicalPath(codex))
	must(t, err)
	listed, err := server.ListHooks()
	must(t, err)
	keys := map[string]string{}
	for _, hook := range listed {
		keys[hook.Key] = hook.CurrentHash
	}
	must(t, server.SetTrust(keys))
	legacy, err := json.Marshal(map[string]any{
		"version": 1,
		"homes": map[string]any{
			path:                               map[string]any{"target": "codex", "keys": keys},
			filepath.Join(alias, "hooks.json"): map[string]any{"target": "codex", "keys": keys},
		},
	})
	must(t, err)
	write(t, e.service.codexTrustPath(), string(legacy))
	save(t, e.service, Mutation{Name: "owned", Remove: true})

	starts := 0
	e.service.CodexAppServer = func(home string) (CodexAppServer, error) {
		starts++
		return f.start(home)
	}
	trust, err := e.service.TrustCodex()
	if err != nil || len(trust) != 1 || trust[0].Action != "cleared" {
		t.Fatalf("legacy symlink cleanup = %+v, %v", trust, err)
	}
	if starts != 1 {
		t.Fatalf("Codex app-server starts = %d, want 1", starts)
	}
	if trustedHash(t, codex, "session_start:0:0") != "" {
		t.Fatal("legacy symlink cleanup left the removed hook trusted")
	}
	var saved codexTrustState
	must(t, json.Unmarshal([]byte(read(t, e.service.codexTrustPath())), &saved))
	if len(saved.Homes) != 0 {
		t.Fatalf("legacy trust receipts remain: %+v", saved.Homes)
	}
}

func codexTrustEntry(t *testing.T, command string) *Entry {
	t.Helper()
	return entry(t, fmt.Sprintf(`{"bindings":{"codex":{"events":{"SessionStart":[{"hooks":[{"type":"command","command":%q}]}]}}}}`, command))
}
