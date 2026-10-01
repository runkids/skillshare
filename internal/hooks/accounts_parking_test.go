package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvShadowedByAccountPlansTheSame(t *testing.T) {
	e := accountEnv(t)
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex", "codex-2", "codex-3")})
	before := map[string]string{}
	for _, key := range []string{"codex", "codex-2", "codex-3"} {
		before[key] = read(t, filepath.Join(e.home, "."+key, "hooks.json"))
	}
	e.service.ConfigDirs["codex"] = e.service.Accounts["codex-2"].Dir
	r := sync(t, e.service)
	for _, c := range r.Plan.Changes {
		if c.Action != "unchanged" {
			t.Fatal(actions(r.Plan))
		}
	}
	want := "CODEX_HOME is " + filepath.Join(e.home, ".codex-2") + ", the config_dir of target codex-2; the codex binding syncs " + filepath.Join(e.home, ".codex") + " instead"
	if !strings.Contains(strings.Join(r.Plan.Warnings, "\n"), want) {
		t.Fatalf("%v", r.Plan.Warnings)
	}
	for key, text := range before {
		if read(t, filepath.Join(e.home, "."+key, "hooks.json")) != text {
			t.Fatal("changed", key)
		}
	}
}

func TestEnvMovedPlainTargetParksRecords(t *testing.T) {
	e := newEnv(t)
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex")})
	path := filepath.Join(e.home, ".codex", "hooks.json")
	before := read(t, path)
	dir := filepath.Join(e.home, ".codex-9")
	e.service.ConfigDirs["codex"] = dir
	r := sync(t, e.service)
	if !strings.Contains(actions(r.Plan), "codex:k:add") || read(t, path) != before {
		t.Fatal("old home changed")
	}
	if !strings.Contains(strings.Join(r.Plan.Warnings, "\n"), "codex now resolves to "+dir+" (CODEX_HOME)") {
		t.Fatalf("%v", r.Plan.Warnings)
	}
	state, _, err := e.service.loadLedger()
	must(t, err)
	if len(state.Records) != 2 {
		t.Fatalf("parked record pruned: %+v", state.Records)
	}
}

func parkedAccount(t *testing.T) (*env, string, string) {
	t.Helper()
	e := accountEnv(t)
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex-2")})
	path := filepath.Join(e.home, ".codex-2", "hooks.json")
	before := read(t, path)
	// Change source and account declarations together, without an intermediate prune.
	write(t, e.config, "targets: {}\nhooks:\n  entries:\n    k: {bindings: {}}\n")
	e.service.Accounts = nil
	return e, path, before
}

func TestUndeclaredAccountRecordsPark(t *testing.T) {
	e, path, before := parkedAccount(t)
	r := sync(t, e.service)
	if strings.Contains(actions(r.Plan), "remove") || read(t, path) != before {
		t.Fatal("undeclared home changed")
	}
	if !strings.Contains(strings.Join(r.Plan.Warnings, "\n"), "target codex-2 is not declared in targets") {
		t.Fatalf("%v", r.Plan.Warnings)
	}
	state, _, err := e.service.loadLedger()
	must(t, err)
	if len(state.Records) != 1 {
		t.Fatal("ownership lost")
	}
}

func TestParkedRecordsReleaseWithReplace(t *testing.T) {
	e, path, before := parkedAccount(t)
	r := save(t, e.service, Mutation{Name: "k", Replace: true})
	if actions(r.Plan) != "codex-2:k:release" || read(t, path) != before {
		t.Fatal("release wrote home", actions(r.Plan))
	}
	state, _, err := e.service.loadLedger()
	must(t, err)
	if len(state.Records) != 0 {
		t.Fatal("release retained ownership")
	}
}

func TestMissingHomeRecordsPark(t *testing.T) {
	e := accountEnv(t)
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex-3")})
	dir := e.service.Accounts["codex-3"].Dir
	must(t, os.RemoveAll(dir))
	r := sync(t, e.service)
	if r.Plan.Blocked || exists(dir) {
		t.Fatal("missing home created or blocked")
	}
	if !strings.Contains(strings.Join(r.Plan.Warnings, "\n"), "config_dir "+dir+" does not exist on this machine") {
		t.Fatalf("%v", r.Plan.Warnings)
	}
	state, _, err := e.service.loadLedger()
	must(t, err)
	if len(state.Records) != 1 {
		t.Fatal("missing home pruned")
	}
}

func TestParkedAccountSharedPathKeepsOldRegistration(t *testing.T) {
	e, path, _ := parkedAccount(t)
	e.service.Accounts = map[string]Account{"codex-new": {Agent: "codex", Dir: filepath.Dir(path)}}
	x := accountEntry(t, "codex-new")
	b := x.Bindings["codex-new"]
	b.Events["SessionStart"] = []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "echo new-account"}}}}
	x.Bindings["codex-new"] = b
	r := save(t, e.service, Mutation{Name: "new-hook", Entry: x})
	if strings.Contains(actions(r.Plan), ":k:remove") || !strings.Contains(read(t, path), "echo account") {
		t.Fatal("new target pruned the parked account", actions(r.Plan))
	}
	state, _, err := e.service.loadLedger()
	must(t, err)
	old := 0
	for _, record := range state.Records {
		if record.Target == "codex-2" {
			old++
		}
	}
	if old != 1 || len(state.Records) != 2 {
		t.Fatalf("parked ownership lost: %+v", state.Records)
	}
}

func TestParkedAccountSamePathRequiresRelease(t *testing.T) {
	for _, kind := range []string{"command", "script", "code"} {
		t.Run(kind, func(t *testing.T) {
			e := accountEnv(t)
			dir := e.service.Accounts["codex-2"].Dir
			agent := "codex"
			x := accountEntry(t, "codex-2")
			b := x.Bindings["codex-2"]
			if kind == "script" {
				b.Files = map[string]string{"guard.sh": "echo script"}
			}
			if kind == "code" {
				agent = "pi"
				b = Binding{Code: "export default () => {}"}
				e.service.Accounts["codex-2"] = Account{Agent: agent, Dir: dir}
			}
			x.Bindings["codex-2"] = b
			save(t, e.service, Mutation{Name: "k", Entry: x})
			write(t, e.config, "targets: {}\nhooks:\n  entries:\n    k: {bindings: {}}\n")
			e.service.Accounts = map[string]Account{"new-account": {Agent: agent, Dir: dir}}
			x = &Entry{Bindings: map[string]Binding{"new-account": b}}
			p, err := e.service.PreviewMutation(Mutation{Name: "k", Entry: x})
			must(t, err)
			if !p.Blocked || len(p.Files()) != 0 {
				t.Fatal("parked output taken over without release", actions(p))
			}
			for _, record := range p.state.Records {
				if record.Target != "codex-2" {
					t.Fatalf("parked ownership retargeted: %+v", record)
				}
			}
			r := save(t, e.service, Mutation{Name: "k", Entry: x, Replace: true})
			if !strings.Contains(actions(r.Plan), "codex-2:k:release") || !strings.Contains(actions(r.Plan), "new-account:k:adopt") {
				t.Fatal("explicit release did not adopt in place", actions(r.Plan))
			}
		})
	}
}

func TestAccountHomeMovedToAncestorParksOldOutputs(t *testing.T) {
	e := accountEnv(t)
	old := filepath.Join(e.home, "accounts", "personal")
	must(t, os.MkdirAll(old, 0755))
	e.service.Accounts["codex-2"] = Account{Agent: "codex", Dir: old}
	x := accountEntry(t, "codex-2")
	b := x.Bindings["codex-2"]
	b.Files = map[string]string{"guard.sh": "echo script"}
	x.Bindings["codex-2"] = b
	save(t, e.service, Mutation{Name: "k", Entry: x})
	path := filepath.Join(old, "hooks.json")
	before := read(t, path)
	e.service.Accounts["codex-2"] = Account{Agent: "codex", Dir: filepath.Dir(old)}
	r := sync(t, e.service)
	if strings.Contains(actions(r.Plan), "remove") || !exists(path) || read(t, path) != before {
		t.Fatal("moving home to an ancestor pruned the old home", actions(r.Plan))
	}
	if read(t, filepath.Join(old, "hooks", "skillshare", "k", "guard.sh")) != "echo script" {
		t.Fatal("old script changed")
	}
	state, _, err := e.service.loadLedger()
	must(t, err)
	if len(state.Records) != 4 || !strings.Contains(strings.Join(r.Plan.Warnings, "\n"), "left as they are") {
		t.Fatal("former home not parked", r.Plan.Warnings, state.Records)
	}
}

func TestAccountCleanupSymlinkCollisionRefused(t *testing.T) {
	for _, agent := range []string{"codex", "pi"} {
		t.Run(agent, func(t *testing.T) {
			e := accountEnv(t)
			dir := e.service.Accounts["codex-2"].Dir
			e.service.Accounts["codex-2"] = Account{Agent: agent, Dir: dir}
			x := accountEntry(t, agent, "codex-2")
			if agent == "pi" {
				for key := range x.Bindings {
					x.Bindings[key] = Binding{Code: "export default () => {}"}
				}
			}
			save(t, e.service, Mutation{Name: "k", Entry: x})
			defaultDir, err := e.service.configDir(agent)
			must(t, err)
			path, err := e.service.nativePath(agent)
			if agent == "pi" {
				path, err = e.service.codePath(agent, "k")
			}
			must(t, err)
			before := read(t, path)
			must(t, os.Rename(dir, dir+"-parked"))
			must(t, os.Symlink(defaultDir, dir))
			delete(x.Bindings, "codex-2")
			_, err = e.service.PreviewMutation(Mutation{Name: "k", Entry: x})
			if err == nil || !strings.Contains(err.Error(), "both write") {
				t.Fatalf("cleanup through account alias accepted: %v", err)
			}
			if read(t, path) != before {
				t.Fatal("collision changed the default home")
			}
		})
	}
}
