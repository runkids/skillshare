package hooks

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
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
	starts      []string // each home started, with "(default)" when CODEX_HOME was left unset
	beforeList  func(home string)
	setTrustErr error
	failHome    string // only this home's write fails, when set
	listErrHome string // this home's hooks/list fails
}

var fakeCodexEvents = map[string]string{"SessionStart": "session_start", "Stop": "stop"}

type fakeCodexServer struct {
	f    *fakeCodex
	home string
}

func (f *fakeCodex) start(home string, defaultHome bool) (CodexAppServer, error) {
	f.starts = append(f.starts, home+map[bool]string{true: "(default)"}[defaultHome])
	return &fakeCodexServer{f, home}, nil
}

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
	if s.f.listErrHome == s.home {
		return nil, fmt.Errorf("hooks/list failed")
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
	if s.f.setTrustErr != nil && (s.f.failHome == "" || s.f.failHome == s.home) {
		return s.f.setTrustErr
	}
	s.f.writes = append(s.f.writes, hashes)
	state := s.state()
	for key, hash := range hashes {
		if state[key] == nil {
			state[key] = map[string]any{}
		}
		state[key]["trusted_hash"] = hash
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

// trustNow previews and records trust in one step, as --yes does.
func trustNow(t *testing.T, s *Service) []CodexTrust {
	t.Helper()
	plan, err := s.PreviewCodexTrust()
	must(t, err)
	plan, err = s.TrustCodex(plan.Revision)
	must(t, err)
	return plan.Hooks
}

func trustActions(t *testing.T, trust []CodexTrust) string {
	t.Helper()
	var out []string
	for _, r := range trust {
		out = append(out, fmt.Sprintf("%s:%s:%s:%s", r.Target, r.Entry, r.Key[strings.LastIndex(r.Key, "hooks.json:")+len("hooks.json:"):], r.Action))
	}
	slices.Sort(out)
	return strings.Join(out, ",")
}

// trustedHash is the trusted_hash a home's config.toml holds for "<event>:<group>:<handler>".
func trustedHash(t *testing.T, home, key string) string {
	t.Helper()
	for full, state := range (&fakeCodexServer{&fakeCodex{t: t}, home}).state() {
		if strings.HasSuffix(full, "hooks.json:"+key) {
			hash, _ := state["trusted_hash"].(string)
			return hash
		}
	}
	return ""
}

func TestCodexTrustRecordsOnlyOwnedHooksInEachHome(t *testing.T) {
	e, f := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	write(t, filepath.Join(codex, "hooks.json"), `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"mine"}]}]}}`)
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex", "codex-2")})

	plan, err := e.service.PreviewCodexTrust()
	must(t, err)
	if got, want := trustActions(t, plan.Hooks), "codex-2:k:session_start:0:0:needed,codex:k:session_start:1:0:needed"; got != want {
		t.Fatalf("preview = %s, want %s", got, want)
	}
	if len(plan.Needed()) != 2 || plan.Needed()[0].Command == "" || plan.Needed()[0].Command == "mine" {
		t.Fatalf("needed hooks must carry Skillshare's own command: %+v", plan.Needed())
	}
	if len(f.writes) != 0 || exists(filepath.Join(codex, "config.toml")) {
		t.Fatal("a preview recorded trust")
	}

	if got, want := trustActions(t, trustNow(t, e.service)), "codex-2:k:session_start:0:0:trusted,codex:k:session_start:1:0:trusted"; got != want {
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
	writes := len(f.writes)
	if got := trustActions(t, trustNow(t, e.service)); got != "codex-2:k:session_start:0:0:unchanged,codex:k:session_start:1:0:unchanged" || len(f.writes) != writes {
		t.Fatalf("second run = %s, writes %d -> %d", got, writes, len(f.writes))
	}

	// A missing account home is skipped, not an error.
	must(t, os.RemoveAll(filepath.Join(e.home, ".codex-2")))
	if got := trustActions(t, trustNow(t, e.service)); got != "codex:k:session_start:1:0:unchanged" {
		t.Fatalf("missing home = %s", got)
	}
}

// Trust is recorded only for what a preview showed: a hook that changed after the
// listing is not trusted unseen.
func TestCodexTrustRefusesARevisionThatNoLongerMatches(t *testing.T) {
	e, f := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	save(t, e.service, Mutation{Name: "k", Entry: codexTrustEntry(t, "echo reviewed")})
	plan, err := e.service.PreviewCodexTrust()
	must(t, err)

	save(t, e.service, Mutation{Name: "k", Entry: codexTrustEntry(t, "echo changed"), Replace: true})
	if _, err := e.service.TrustCodex(plan.Revision); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("err = %v, want ErrStaleRevision", err)
	}
	if _, err := e.service.TrustCodex(""); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("empty revision err = %v, want ErrStaleRevision", err)
	}
	if len(f.writes) != 0 || exists(filepath.Join(codex, "config.toml")) {
		t.Fatalf("trusted a hook nobody reviewed: %v", f.writes)
	}
}

// Trust follows a hook that moved; the entry at its old position stays, where it
// matches only the same hook again.
func TestCodexTrustFollowsMovesAndLeavesOldTrust(t *testing.T) {
	e, _ := trustEnv(t)
	codex := filepath.Join(e.home, ".codex")
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex")})
	trustNow(t, e.service)
	old := trustedHash(t, codex, "session_start:0:0")

	// Another hook takes position 0, so Skillshare's moves to 1.
	path := filepath.Join(codex, "hooks.json")
	groups := append([]any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "mine"}}}}, events(t, path, true)["SessionStart"]...)
	data, err := json.Marshal(map[string]any{"hooks": map[string]any{"SessionStart": groups}})
	must(t, err)
	write(t, path, string(data))
	if got := trustActions(t, trustNow(t, e.service)); got != "codex:k:session_start:1:0:trusted" {
		t.Fatalf("after move = %s", got)
	}
	if trustedHash(t, codex, "session_start:0:0") != old || trustedHash(t, codex, "session_start:1:0") != old {
		t.Fatal("trust did not follow the hook, or the old entry was touched")
	}

	save(t, e.service, Mutation{Name: "k", Remove: true})
	if got := trustActions(t, trustNow(t, e.service)); got != "" {
		t.Fatalf("after remove = %s", got)
	}
	if trustedHash(t, codex, "session_start:1:0") != old {
		t.Fatal("removal changed Codex trust")
	}
}

func TestCodexTrustReportsDisabled(t *testing.T) {
	e, f := trustEnv(t)
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex")})
	f.disabled[filepath.Join(e.home, ".codex", "hooks.json")+":session_start:0:0"] = true
	trust := trustNow(t, e.service)
	if len(trust) != 1 || trust[0].Action != "trusted" || !trust[0].Disabled {
		t.Fatalf("trust = %+v", trust)
	}
}

func TestCodexTrustRefusesProjectScope(t *testing.T) {
	e, _ := trustEnv(t)
	_, err := e.project(filepath.Join(t.TempDir(), "repo")).PreviewCodexTrust()
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
	if trust := trustNow(t, &second); len(trust) != 0 || len(f.writes) != 0 || exists(filepath.Join(codex, "config.toml")) {
		t.Fatalf("second config trusted first config's hook: trust=%+v writes=%v", trust, f.writes)
	}
	if trust := trustNow(t, first); len(trust) != 1 || trust[0].Action != "trusted" {
		t.Fatalf("first trust = %+v", trust)
	}

	save(t, &second, Mutation{Name: "second", Entry: codexTrustEntry(t, "echo second")})
	if trust := trustNow(t, &second); len(trust) != 1 || trust[0].Entry != "second" || trust[0].Action != "trusted" {
		t.Fatalf("second trust = %+v", trust)
	}
	if trustedHash(t, codex, "session_start:0:0") == "" || trustedHash(t, codex, "session_start:1:0") == "" {
		t.Fatal("missing per-config trust")
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

			trust := trustNow(t, &second)
			if len(trust) != 1 || trust[0].Entry != "second" || trust[0].Action != "trusted" {
				t.Fatalf("second config trust = %+v", trust)
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
			if !strings.HasSuffix(trust[0].Key, fmt.Sprintf("hooks.json:session_start:%d:0", secondIndex)) {
				t.Fatalf("trusted key = %q, want second config's group %d", trust[0].Key, secondIndex)
			}
			if trustedHash(t, codex, fmt.Sprintf("session_start:%d:0", firstIndex)) != "" || trustedHash(t, codex, fmt.Sprintf("session_start:%d:0", secondIndex)) == "" {
				t.Fatal("want only the second config's key trusted")
			}
		})
	}
}

// Codex resolves CODEX_HOME's symlinks but not ~/.codex's, and the trust key holds
// that path: the app-server finds each home the way the user's Codex does.
func TestCodexTrustStartsCodexAsTheUserDoes(t *testing.T) {
	e, f := trustEnv(t)
	t.Setenv("HOME", e.home)
	codex := filepath.Join(e.home, ".codex")
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex", "codex-2")})

	trustNow(t, e.service)
	want := []string{filepath.Join(e.home, ".codex-2"), codex + "(default)"}
	if got := f.starts[:2]; !slices.Equal(got, want) {
		t.Fatalf("starts = %v, want %v", got, want)
	}

	// The same home reached through CODEX_HOME is started with it, and only once.
	alias := filepath.Join(e.home, "codex-alias")
	if err := os.Symlink(codex, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	e.service.ConfigDirs["codex"] = alias
	f.starts = nil
	if _, err := e.service.PreviewCodexTrust(); err != nil {
		t.Fatal(err)
	}
	if want := []string{filepath.Join(e.home, ".codex-2"), alias}; !slices.Equal(f.starts, want) {
		t.Fatalf("starts through CODEX_HOME = %v, want %v", f.starts, want)
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

	plan, err := e.service.PreviewCodexTrust()
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("plan = %+v, err = %v; expected changed hooks.json to fail closed", plan, err)
	}
	if len(f.writes) != 0 || exists(filepath.Join(codex, "config.toml")) {
		t.Fatalf("stale snapshot was trusted: writes=%v", f.writes)
	}
}

func TestCodexTrustDoesNotReportUnconfirmedEdits(t *testing.T) {
	e, f := trustEnv(t)
	save(t, e.service, Mutation{Name: "owned", Entry: codexTrustEntry(t, "echo owned")})
	plan, err := e.service.PreviewCodexTrust()
	must(t, err)
	f.setTrustErr = fmt.Errorf("batch write failed")
	plan, err = e.service.TrustCodex(plan.Revision)
	if err == nil {
		t.Fatal("expected failed config/batchWrite")
	}
	if got := trustActions(t, plan.Hooks); got != "codex:owned:session_start:0:0:needed" {
		t.Fatalf("a failed write was reported as %s", got)
	}
}

// One home's failed write leaves the other home trusted, and the error names both.
func TestCodexTrustKeepsWritingOtherHomesWhenOneFails(t *testing.T) {
	e, f := trustEnv(t)
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex", "codex-2")})
	plan, err := e.service.PreviewCodexTrust()
	must(t, err)
	f.setTrustErr, f.failHome = fmt.Errorf("batch write failed"), filepath.Join(e.home, ".codex-2")

	plan, err = e.service.TrustCodex(plan.Revision)
	if err == nil || !strings.Contains(err.Error(), "recorded for codex, but not for every home: codex-2: batch write failed") {
		t.Fatalf("err = %v, want it to name the written and the failed home", err)
	}
	if got, want := trustActions(t, plan.Hooks), "codex-2:k:session_start:0:0:needed,codex:k:session_start:0:0:trusted"; got != want {
		t.Fatalf("trust = %s, want %s", got, want)
	}
	if trustedHash(t, filepath.Join(e.home, ".codex"), "session_start:0:0") == "" {
		t.Fatal("the home after the failed one was not written")
	}
}

// A home Codex cannot list is reported and left out; the other home is still trusted.
func TestCodexTrustLeavesOutAHomeCodexCannotList(t *testing.T) {
	e, f := trustEnv(t)
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex", "codex-2")})
	f.listErrHome = filepath.Join(e.home, ".codex-2")

	plan, err := e.service.PreviewCodexTrust()
	must(t, err)
	if got := trustActions(t, plan.Hooks); got != "codex:k:session_start:0:0:needed" {
		t.Fatalf("preview = %s", got)
	}
	if len(plan.Failed) != 1 || plan.Failed[0].Target != "codex-2" || !strings.Contains(plan.Failed[0].Error, "hooks/list failed") {
		t.Fatalf("failed = %+v, want codex-2 with its error", plan.Failed)
	}
	plan, err = e.service.TrustCodex(plan.Revision)
	if err == nil || !strings.Contains(err.Error(), "recorded for codex, but not for every home: codex-2: ") {
		t.Fatalf("err = %v, want it to name the written and the unlisted home", err)
	}
	if got := trustActions(t, plan.Hooks); got != "codex:k:session_start:0:0:trusted" {
		t.Fatalf("trust = %s", got)
	}

	// With no home left to list, the preview itself fails.
	must(t, os.RemoveAll(filepath.Join(e.home, ".codex")))
	if _, err := e.service.PreviewCodexTrust(); err == nil || !strings.Contains(err.Error(), "codex-2: ") {
		t.Fatalf("err = %v, want the only home's failure", err)
	}
}

// Codex reports a home it cannot load inside a successful reply.
func TestParseCodexHooksListFailsWhenCodexLoadedNothing(t *testing.T) {
	_, err := parseCodexHooksList(json.RawMessage(`{"data":[{"cwd":"/h","hooks":[],"warnings":[],"errors":[{"path":"/h","message":"/h/config.toml:3:6: key with no value"}]}]}`))
	if err == nil || !strings.Contains(err.Error(), "config.toml:3:6") {
		t.Fatalf("err = %v, want Codex's own reason", err)
	}
	hooks, err := parseCodexHooksList(json.RawMessage(`{"data":[{"hooks":[{"key":"k"}],"errors":[]}]}`))
	if err != nil || len(hooks) != 1 {
		t.Fatalf("hooks = %+v, err = %v", hooks, err)
	}
}

// Codex can take a while to answer a preview; other hooks operations must not wait for it.
func TestCodexTrustPreviewDoesNotHoldTheOperationLock(t *testing.T) {
	e, f := trustEnv(t)
	save(t, e.service, Mutation{Name: "owned", Entry: codexTrustEntry(t, "echo owned")})
	f.beforeList = func(string) {
		lock, err := e.service.lock()
		if err != nil {
			t.Fatalf("a preview blocked other hooks operations: %v", err)
		}
		must(t, lock.Unlock())
	}
	if _, err := e.service.PreviewCodexTrust(); err != nil {
		t.Fatal(err)
	}
}

func codexTrustEntry(t *testing.T, command string) *Entry {
	t.Helper()
	return entry(t, fmt.Sprintf(`{"bindings":{"codex":{"events":{"SessionStart":[{"hooks":[{"type":"command","command":%q}]}]}}}}`, command))
}
