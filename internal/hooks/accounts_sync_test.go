package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func accountEnv(t *testing.T) *env {
	t.Helper()
	e := newEnv(t)
	e.service.Accounts = map[string]Account{}
	for _, key := range []string{"codex-2", "codex-3"} {
		dir := filepath.Join(e.home, "."+key)
		must(t, os.MkdirAll(dir, 0755))
		e.service.Accounts[key] = Account{Agent: "codex", Dir: dir}
	}
	return e
}

func TestAccountSyncWritesEachHome(t *testing.T) {
	e := accountEnv(t)
	r := save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex", "codex-2", "codex-3")})
	if !strings.Contains(actions(r.Plan), "codex-2:k:add") {
		t.Fatal(actions(r.Plan))
	}
	state, _, err := e.service.loadLedger()
	must(t, err)
	if len(state.Records) != 3 {
		t.Fatalf("records: %v", state.Records)
	}
	for _, key := range []string{"codex", "codex-2", "codex-3"} {
		path := filepath.Join(e.home, "."+key, "hooks.json")
		if len(events(t, path, true)["SessionStart"]) != 1 {
			t.Fatalf("missing group in %s", path)
		}
		found := false
		for _, r := range state.Records {
			if r.Target == key && r.Path == path {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing ownership for %s", key)
		}
	}
}

func TestAccountBindingRemovalPrunesOnlyThatHome(t *testing.T) {
	e := accountEnv(t)
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex", "codex-2", "codex-3")})
	before := read(t, filepath.Join(e.home, ".codex", "hooks.json"))
	r := save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex", "codex-2")})
	if !strings.Contains(actions(r.Plan), "codex-3:k:remove") {
		t.Fatal(actions(r.Plan))
	}
	if read(t, filepath.Join(e.home, ".codex", "hooks.json")) != before {
		t.Fatal("default home changed")
	}
	if exists(filepath.Join(e.home, ".codex-3", "hooks.json")) {
		t.Fatal("removed binding retained")
	}
}

func TestAccountConflictsAreIndependent(t *testing.T) {
	e := accountEnv(t)
	path := filepath.Join(e.home, ".codex-2", "hooks.json")
	write(t, path, `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"neighbor"}]},{"hooks":[{"type":"command","command":"echo account"}]}]}}`)
	m := Mutation{Name: "k", Entry: accountEntry(t, "codex", "codex-2")}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	if !p.Blocked || !strings.Contains(actions(p), "codex-2:k:conflict") || !strings.Contains(actions(p), "codex:k:add") {
		t.Fatal(actions(p))
	}
	before := read(t, path)
	m.Replace = true
	r := save(t, e.service, m)
	if !strings.Contains(actions(r.Plan), "codex-2:k:adopt") || read(t, path) != before {
		t.Fatal("adoption moved native elements")
	}
	for _, record := range r.Plan.state.Records {
		if record.Target == "codex-2" && record.Index != 1 {
			t.Fatal("adoption index", record.Index)
		}
	}
}

func TestAccountClaudeAndPiDestinations(t *testing.T) {
	e := newEnv(t)
	c, p := filepath.Join(e.home, "claude-work"), filepath.Join(e.home, "pi-2")
	must(t, os.MkdirAll(p, 0755))
	write(t, filepath.Join(c, "settings.json"), `{"theme":"dark"}`)
	e.service.Accounts = map[string]Account{"claude-work": {Agent: "claude", Dir: c}, "pi-2": {Agent: "pi", Dir: p}}
	x := accountEntry(t, "claude-work")
	x.Bindings["pi-2"] = Binding{Code: "export default function() {}\n"}
	b := x.Bindings["claude-work"]
	b.Files = map[string]string{"guard.sh": "echo guard"}
	x.Bindings["claude-work"] = b
	save(t, e.service, Mutation{Name: "k", Entry: x})
	if !strings.Contains(read(t, filepath.Join(c, "settings.json")), `"theme":"dark"`) {
		t.Fatal("lost theme")
	}
	if read(t, filepath.Join(p, "extensions", "skillshare-k.ts")) != "export default function() {}\n" {
		t.Fatal("pi code")
	}
	if read(t, filepath.Join(c, "hooks", "skillshare", "k", "guard.sh")) != "echo guard" {
		t.Fatal("script")
	}
}

func TestAccountSymlinkToDefaultHomeRefused(t *testing.T) {
	e := newEnv(t)
	dir := filepath.Join(e.home, ".codex")
	must(t, os.MkdirAll(dir, 0755))
	link := filepath.Join(e.home, ".codex-2")
	must(t, os.Symlink(dir, link))
	e.service.Accounts = map[string]Account{"codex-2": {Agent: "codex", Dir: link}}
	_, err := e.service.PreviewMutation(Mutation{Name: "k", Entry: accountEntry(t, "codex", "codex-2")})
	if err == nil || !strings.Contains(err.Error(), "hooks: codex and codex-2 both write") {
		t.Fatalf("%v", err)
	}
}

func TestMissingAccountHomeIsSkipped(t *testing.T) {
	e := newEnv(t)
	dir := filepath.Join(e.home, ".codex-3")
	e.service.Accounts = map[string]Account{"codex-3": {Agent: "codex", Dir: dir}}
	r := save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex", "codex-3")})
	if r.Plan.Blocked || strings.Contains(actions(r.Plan), "codex-3:") || exists(dir) {
		t.Fatal("missing account written")
	}
	want := "target codex-3: config_dir " + dir + " does not exist on this machine; its hook bindings are skipped"
	if !strings.Contains(strings.Join(r.Plan.Warnings, "\n"), want) {
		t.Fatalf("warnings: %v", r.Plan.Warnings)
	}
}

func TestAccountBackupRestores(t *testing.T) {
	e := accountEnv(t)
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex-2")})
	x := accountEntry(t, "codex-2")
	b := x.Bindings["codex-2"]
	b.Events["SessionStart"] = []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "echo new"}}}}
	x.Bindings["codex-2"] = b
	r := save(t, e.service, Mutation{Name: "k", Entry: x})
	p, err := e.service.PreviewRestore(r.BackupIDs[0])
	must(t, err)
	if p.Blocked || p.Changes[0].Target != "codex-2" || p.Changes[0].Action != "restore" {
		t.Fatalf("%+v", p.Changes)
	}
	_, err = e.service.Restore(r.BackupIDs[0], p.Revision)
	must(t, err)
	if !strings.Contains(read(t, filepath.Join(e.home, ".codex-2", "hooks.json")), "echo account") {
		t.Fatal("restore failed")
	}
	// A backup with an undeclared target must fail explicitly, without touching a home.
	save(t, e.service, Mutation{Name: "k", Entry: &Entry{Bindings: map[string]Binding{}}})
	e.service.Accounts = nil
	_, err = e.service.PreviewRestore(r.BackupIDs[0])
	want := "backup " + r.BackupIDs[0] + " belongs to codex-2, which is not declared in targets; add it back to restore"
	if err == nil || err.Error() != want {
		t.Fatalf("%v", err)
	}
}

func TestLegacyLedgerUnchanged(t *testing.T) {
	e := newEnv(t)
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex")})
	before := read(t, e.service.statePath())
	e.service.Accounts = map[string]Account{"codex-2": {Agent: "codex", Dir: filepath.Join(e.home, ".codex-2")}}
	r := sync(t, e.service)
	if actions(r.Plan) != "codex:k:unchanged" || read(t, e.service.statePath()) != before {
		t.Fatal("version 1 ledger changed")
	}
}

func TestAccountBackupRestoreMissingHomeRefused(t *testing.T) {
	e := accountEnv(t)
	r := save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex-2")})
	dir := e.service.Accounts["codex-2"].Dir
	must(t, os.RemoveAll(dir))
	_, err := e.service.PreviewRestore(r.BackupIDs[0])
	if err == nil {
		t.Fatal("restore could recreate an absent account home")
	}
	if exists(dir) {
		t.Fatal("restore created home")
	}
}

func TestAccountBackupRestoreAliasCollisionRefused(t *testing.T) {
	e := accountEnv(t)
	save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex", "codex-2")})
	x := accountEntry(t, "codex", "codex-2")
	for _, b := range x.Bindings {
		b.Events["SessionStart"] = []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "echo new"}}}}
	}
	save(t, e.service, Mutation{Name: "k", Entry: x})
	backups, err := e.service.Backups()
	must(t, err)
	id := ""
	for _, backup := range backups {
		if backup.Target == "codex-2" {
			id = backup.ID
			break
		}
	}
	dir := e.service.Accounts["codex-2"].Dir
	must(t, os.Rename(dir, dir+"-parked"))
	path := filepath.Join(e.home, ".codex", "hooks.json")
	must(t, os.Symlink(filepath.Dir(path), dir))
	before := read(t, path)
	// The account's binding is gone, but its backup and ownership still exist.
	write(t, e.config, "targets: {}\nhooks:\n  entries: {}\n")
	_, err = e.service.PreviewRestore(id)
	if err == nil || !strings.Contains(err.Error(), "both write") {
		t.Fatalf("restore through account alias accepted: %v", err)
	}
	if read(t, path) != before {
		t.Fatal("restore changed default home")
	}
}

func TestAccountBackupRestoreFormerHomeRefused(t *testing.T) {
	e := accountEnv(t)
	old := filepath.Join(e.home, "accounts", "personal")
	must(t, os.MkdirAll(old, 0755))
	e.service.Accounts["codex-2"] = Account{Agent: "codex", Dir: old}
	r := save(t, e.service, Mutation{Name: "k", Entry: accountEntry(t, "codex-2")})
	e.service.Accounts["codex-2"] = Account{Agent: "codex", Dir: filepath.Dir(old)}
	path := filepath.Join(old, "hooks.json")
	before := read(t, path)
	_, err := e.service.PreviewRestore(r.BackupIDs[0])
	if err == nil || !strings.Contains(err.Error(), "now resolves to") {
		t.Fatalf("restore accepted the former home: %v", err)
	}
	if read(t, path) != before {
		t.Fatal("former home changed")
	}
}
