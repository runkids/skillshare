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
	t        *testing.T
	disabled map[string]bool
	writes   []map[string]string
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
	return hashes[filepath.Join(home, "hooks.json")+":"+key]
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
	must(t, server.SetTrust(map[string]string{path + ":session_start:0:0": "sha256:user"}))
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
	key := filepath.Join(codex, "hooks.json") + ":session_start:0:0"
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
