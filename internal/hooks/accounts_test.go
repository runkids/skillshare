package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"skillshare/internal/config"
	"slices"
	"strings"
	"testing"
)

func TestAccountDestinations(t *testing.T) {
	e := newEnv(t)
	e.service.Accounts = map[string]Account{}
	for _, tc := range []struct{ key, agent, suffix string }{
		{"codex-2", "codex", "hooks.json"}, {"claude-work", "claude", "settings.json"}, {"pi-2", "pi", "extensions"},
	} {
		dir := filepath.Join(e.home, "."+tc.key)
		e.service.Accounts[tc.key] = Account{Agent: tc.agent, Dir: dir}
		sc, agent := e.service.forTarget(tc.key)
		if agent != tc.agent {
			t.Fatalf("agent = %s", agent)
		}
		path, err := sc.nativePath(agent)
		must(t, err)
		if path != filepath.Join(dir, tc.suffix) {
			t.Fatalf("path = %s", path)
		}
		path, err = sc.scriptDir(agent, "x")
		must(t, err)
		if path != filepath.Join(dir, "hooks", "skillshare", "x") {
			t.Fatalf("script = %s", path)
		}
	}
}

func TestAccountsIgnoredInProjectScope(t *testing.T) {
	e := newEnv(t)
	e.service.Accounts = map[string]Account{"codex-2": {Agent: "codex", Dir: filepath.Join(e.home, ".codex-2")}}
	e.service.ProjectRoot = t.TempDir()
	if _, ok := e.service.lookupAccount("codex-2"); ok {
		t.Fatal("project account resolved")
	}
}

func TestEnvShadowedByAccount(t *testing.T) {
	e := newEnv(t)
	dir := filepath.Join(e.home, ".codex-2")
	e.service.Accounts = map[string]Account{"codex-2": {Agent: "codex", Dir: dir}}
	e.service.ConfigDirs["codex"] = dir
	got, err := e.service.configDir("codex")
	must(t, err)
	if got != filepath.Join(e.home, ".codex") || e.service.envShadow("codex") != "codex-2" {
		t.Fatalf("shadow: %s", got)
	}
	other := filepath.Join(e.home, ".codex-9")
	e.service.ConfigDirs["codex"] = other
	got, err = e.service.configDir("codex")
	must(t, err)
	if got != other {
		t.Fatalf("unshadowed: %s", got)
	}
}

func TestEnvShadowFollowsSymlink(t *testing.T) {
	e := newEnv(t)
	dir := filepath.Join(e.home, ".codex-2")
	must(t, os.MkdirAll(dir, 0755))
	link := filepath.Join(e.home, "alias")
	must(t, os.Symlink(dir, link))
	e.service.Accounts = map[string]Account{"codex-2": {Agent: "codex", Dir: dir}}
	e.service.ConfigDirs["codex"] = link
	if e.service.envShadow("codex") != "codex-2" {
		t.Fatal("symlink not shadowed")
	}
}

func TestAccountAgentsMatchConfig(t *testing.T) {
	got := slices.Clone(accountAgents)
	slices.Sort(got)
	want := slices.DeleteFunc(config.AgentConfigDirAgents(), func(agent string) bool {
		_, supported := targetDef(agent)
		return !supported
	})
	slices.Sort(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v != %v", got, want)
	}
}

func TestOMPAccountExposesHooks(t *testing.T) {
	e := newEnv(t)
	e.service.Accounts = map[string]Account{"omp-work": {Agent: "omp", Dir: e.home}}
	if a, ok := e.service.lookupAccount("omp-work"); !ok || a.Agent != "omp" {
		t.Fatal("OMP accounts carry hooks like Pi accounts")
	}
}

func TestAccountMissingHome(t *testing.T) {
	e := newEnv(t)
	dir := filepath.Join(e.home, ".codex-2")
	e.service.Accounts = map[string]Account{"codex-2": {Agent: "codex", Dir: dir}}
	if !e.service.accountMissing("codex-2") {
		t.Fatal("missing not detected")
	}
	must(t, os.MkdirAll(dir, 0755))
	if e.service.accountMissing("codex-2") {
		t.Fatal("existing home skipped")
	}
}

func TestParseEntryDefersAccountKeys(t *testing.T) {
	_, err := ParseEntry([]byte(`{"bindings":{"codex-2":{"events":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}]}}}}`))
	must(t, err)
	_, err = ParseEntry([]byte(`{"bindings":{"codex 2":{"code":"x"}}}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported Agent") {
		t.Fatalf("invalid key: %v", err)
	}
}

func accountEntry(t *testing.T, keys ...string) *Entry {
	t.Helper()
	b := Binding{Events: map[string]any{"SessionStart": []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "echo account"}}}}}}
	out := &Entry{Bindings: map[string]Binding{}}
	for _, key := range keys {
		out.Bindings[key] = b
	}
	return out
}

func TestSourceValidatesAccountBindings(t *testing.T) {
	for _, tc := range []struct{ agent, body, want string }{
		{"codex", `events: {Stop: [{hooks: [{type: command, command: x}]}]}`, ""},
		{"codex", `code: x`, "hook x: codex-2 takes events, not code"},
		{"pi", `events: {Stop: [{hooks: [{type: command, command: x}]}]}`, "hook x: codex-2 takes code only, not events or files"},
	} {
		t.Run(tc.agent+tc.body, func(t *testing.T) {
			e := newEnv(t)
			e.service.Accounts = map[string]Account{"codex-2": {Agent: tc.agent, Dir: filepath.Join(e.home, "account")}}
			write(t, e.config, "hooks:\n  entries:\n    x:\n      bindings:\n        codex-2: {"+tc.body+"}\n")
			_, err := e.service.loadSource()
			if tc.want == "" {
				must(t, err)
			} else if err == nil || err.Error() != tc.want {
				t.Fatalf("%v != %s", err, tc.want)
			}
		})
	}
}

func TestSourceRejectsUndeclaredAccount(t *testing.T) {
	e := newEnv(t)
	write(t, e.config, "hooks:\n  entries:\n    x:\n      bindings:\n        codex-2: {code: x}\n")
	_, err := e.service.loadSource()
	want := `hook x: unsupported Agent "codex-2": it is neither an Agent nor a target with agent and config_dir`
	if err == nil || err.Error() != want {
		t.Fatalf("%v", err)
	}
}

func TestSourceRefusesAccountInProjects(t *testing.T) {
	e := newEnv(t)
	e.service.Accounts = map[string]Account{"codex-2": {Agent: "codex", Dir: filepath.Join(e.home, "account")}}
	write(t, e.config, "hooks:\n  projects:\n    /p:\n      entries:\n        x:\n          bindings:\n            codex-2: {code: x}\n")
	_, err := e.service.loadSource()
	want := "hook x: codex-2 is an account of codex, and every account reads the same project files; bind codex"
	if err == nil || err.Error() != want {
		t.Fatalf("%v", err)
	}
}

func TestEventWarningsUseAccountAgent(t *testing.T) {
	e := newEnv(t)
	e.service.Accounts = map[string]Account{"codex-2": {Agent: "codex", Dir: filepath.Join(e.home, "account")}}
	x := accountEntry(t, "codex-2")
	b := x.Bindings["codex-2"]
	b.Events = map[string]any{"Stopp": b.Events["SessionStart"]}
	x.Bindings["codex-2"] = b
	got := eventWarnings("x", *x, e.service.agentOf)
	want := []string{`hook x: codex-2 does not document the event "Stopp"; check its spelling`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
}

// Account-looking keys can be parsed independently of config, but never escape
// config-aware validation or permit unsafe output file names.
func FuzzAccountBindingValidation(f *testing.F) {
	f.Add("codex-2", "guard.sh")
	f.Add("pi-2", "../escape")
	f.Add("codex 2", "x")
	f.Fuzz(func(t *testing.T, key, file string) {
		if _, builtin := targetDef(canonicalTarget(key)); builtin {
			return
		}
		x := accountEntry(t, key)
		b := x.Bindings[key]
		b.Files = map[string]string{file: "x"}
		x.Bindings[key] = b
		data, err := json.Marshal(x)
		must(t, err)
		parsed, err := ParseEntry(data)
		if err != nil {
			return
		}
		if err := parsed.Validate("x"); err == nil {
			t.Fatal("undeclared key accepted")
		}
		err = parsed.validate("x", map[string]string{key: "codex"}, validateGlobal)
		if !fileName.MatchString(file) || !filepath.IsLocal(file) {
			if err == nil {
				t.Fatal("unsafe file accepted")
			}
		} else {
			must(t, err)
		}
	})
}
