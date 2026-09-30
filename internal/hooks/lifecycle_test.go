package hooks

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const userSettings = `{
  // user's own comment
  "model": "opus",
  "hooks": {
    "PreToolUse": [
      {"matcher": "Edit", "hooks": [{"type": "command", "command": "echo mine"}]}
    ],
    "Stop": [{"hooks": [{"type": "command", "command": "echo stop"}]}]
  },
  "permissions": {"allow": ["Bash(ls)"]}
}
`

func TestSync_PreservesUnrelatedSettingsCommentsAndOrder(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, userSettings)
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	got := read(t, path)
	for _, keep := range []string{"// user's own comment", `"model": "opus"`, `"permissions": {"allow": ["Bash(ls)"]}`, `{"matcher": "Edit", "hooks": [{"type": "command", "command": "echo mine"}]}`, `"Stop": [{"hooks": [{"type": "command", "command": "echo stop"}]}]`} {
		if !strings.Contains(got, keep) {
			t.Fatalf("lost %q:\n%s", keep, got)
		}
	}
	pre := events(t, path, true)["PreToolUse"]
	if len(pre) != 2 || !strings.Contains(got, "echo guard") || strings.Index(got, "echo mine") > strings.Index(got, "echo guard") {
		t.Fatalf("owned group must be appended after the user's:\n%s", got)
	}

	// Disable removes only the owned group; the definition stays.
	disabled := *entry(t, claudeEntry)
	off := false
	disabled.Enabled = &off
	save(t, e.service, Mutation{Name: "guard", Entry: &disabled})
	if got := read(t, path); got != userSettings {
		t.Fatalf("disable must restore the user's file byte for byte:\n%s", got)
	}
	source, err := LoadSource(e.config)
	must(t, err)
	if source.Entries["guard"].IsEnabled() {
		t.Fatal("definition must stay, disabled")
	}

	// Re-enable republishes; remove prunes.
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	if !strings.Contains(read(t, path), "echo guard") {
		t.Fatal("re-enable must republish")
	}
	save(t, e.service, Mutation{Name: "guard", Remove: true})
	if got := read(t, path); got != userSettings {
		t.Fatalf("remove must prune only the owned group:\n%s", got)
	}
}

func TestSync_IdenticalUnmanagedGroupIsConflictUntilReplace(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo guard","timeout":5}]}]}}`)
	m := Mutation{Name: "guard", Entry: entry(t, claudeEntry)}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	if !p.Blocked || !strings.Contains(actions(p), "conflict") {
		t.Fatalf("equality alone must not adopt: %s", actions(p))
	}
	if _, err := e.service.Mutate(m, p.Revision, true); !errors.Is(err, ErrConflict) {
		t.Fatalf("sync must refuse conflicts, got %v", err)
	}
	if _, err := LoadSource(e.config); err != nil {
		t.Fatal(err)
	}
	if source, _ := LoadSource(e.config); len(source.Entries) != 0 {
		t.Fatal("a refused sync must not save the source")
	}
	m.Replace = true
	save(t, e.service, m)
	if n := len(events(t, path, true)["PreToolUse"]); n != 1 {
		t.Fatalf("replace must take over in place, not duplicate: %d groups", n)
	}
	// Now owned: removing prunes it.
	save(t, e.service, Mutation{Name: "guard", Remove: true})
	if n := len(events(t, path, true)["PreToolUse"]); n != 0 {
		t.Fatalf("adopted group must be pruned on remove: %d", n)
	}
}

func TestSync_ExternallyEditedOutputIsNeverOverwritten(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	edited := strings.Replace(read(t, path), "echo guard", "echo edited", 1)
	write(t, path, edited)

	for name, m := range map[string]Mutation{
		"update":  {Name: "guard", Entry: entry(t, strings.Replace(claudeEntry, "echo guard", "echo v2", 1))},
		"remove":  {Name: "guard", Remove: true},
		"disable": {Name: "guard", Entry: func() *Entry { x := *entry(t, claudeEntry); f := false; x.Enabled = &f; return &x }()},
	} {
		p, err := e.service.PreviewMutation(m)
		must(t, err)
		if !p.Blocked {
			t.Fatalf("%s: edited output must conflict: %s", name, actions(p))
		}
	}
	// Replace on remove releases ownership and leaves the user's edit.
	save(t, e.service, Mutation{Name: "guard", Remove: true, Replace: true})
	if read(t, path) != edited {
		t.Fatal("released output must stay as the user edited it")
	}
	p, err := e.service.Preview()
	must(t, err)
	if len(p.Changes) != 0 {
		t.Fatalf("released output is the user's: %s", actions(p))
	}
}

func TestSync_WholeFilesRespectUnmanagedAndEditedFiles(t *testing.T) {
	e := newEnv(t)
	ts := filepath.Join(e.home, ".pi", "agent", "extensions", "skillshare-demo.ts")
	write(t, ts, "// the user's own file\n")
	m := Mutation{Name: "demo", Entry: entry(t, `{"bindings":{"pi":`+adapterEntry["pi"]+`}}`)}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	if !p.Blocked {
		t.Fatal("an unmanaged file at an output path must conflict")
	}
	m.Replace = true
	save(t, e.service, m)
	backups, err := e.service.Backups()
	must(t, err)
	if len(backups) != 1 {
		t.Fatalf("overwrite needs a backup: %v", backups)
	}
	write(t, ts, "// edited later\n")
	p, err = e.service.PreviewMutation(Mutation{Name: "demo", Remove: true})
	must(t, err)
	if !p.Blocked {
		t.Fatal("an edited owned file must not be removed")
	}
}

func TestSync_ScriptFilesArePrunedWithTheirFolder(t *testing.T) {
	e := newEnv(t)
	save(t, e.service, Mutation{Name: "demo", Entry: entry(t, `{"bindings":{"cursor":`+adapterEntry["cursor"]+`}}`)})
	script := filepath.Join(e.home, ".cursor", "hooks", "skillshare", "demo", "guard.sh")
	info, err := os.Stat(script)
	must(t, err)
	if info.Mode().Perm()&0100 == 0 {
		t.Fatal("scripts must be executable")
	}
	save(t, e.service, Mutation{Name: "demo", Remove: true})
	if exists(filepath.Join(e.home, ".cursor", "hooks", "skillshare")) {
		t.Fatal("empty script folders must be pruned")
	}
	if !exists(filepath.Join(e.home, ".cursor", "hooks.json")) {
		t.Fatal("hooks.json itself stays")
	}
}

func TestSync_EmptyBindingsKeepsDefinitionWithoutPublishing(t *testing.T) {
	e := newEnv(t)
	save(t, e.service, Mutation{Name: "idea", Entry: entry(t, `{"description":"later","bindings":{}}`)})
	source, err := LoadSource(e.config)
	must(t, err)
	if _, ok := source.Entries["idea"]; !ok {
		t.Fatal("definition must be saved")
	}
	if exists(filepath.Join(e.home, ".claude")) {
		t.Fatal("empty bindings must publish nothing")
	}
}

func TestMutate_StaleRevisionAndExternalConfigEdit(t *testing.T) {
	e := newEnv(t)
	m := Mutation{Name: "guard", Entry: entry(t, claudeEntry)}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo new"}]}]}}`)
	if _, err := e.service.Mutate(m, p.Revision, true); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("native hooks edited after preview: want ErrStaleRevision, got %v", err)
	}
	p, err = e.service.PreviewMutation(m)
	must(t, err)
	write(t, e.config, "targets: {}\n# edited\n")
	if _, err := e.service.Mutate(m, p.Revision, true); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("config edited after preview: want ErrStaleRevision, got %v", err)
	}
	// Unrelated settings the Agent rewrites between preview and apply are kept.
	p, err = e.service.PreviewMutation(m)
	must(t, err)
	write(t, path, `{"theme":"dark","hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo new"}]}]}}`)
	if _, err := e.service.Mutate(m, p.Revision, true); err != nil {
		t.Fatalf("unrelated setting change must not be stale: %v", err)
	}
	if !strings.Contains(read(t, path), `"theme":"dark"`) {
		t.Fatal("setting written after preview was lost")
	}
}

func TestSync_RejectsMalformedAndUnsafeDestinations(t *testing.T) {
	cases := map[string]func(e *env){
		"malformed":       func(e *env) { write(t, filepath.Join(e.home, ".claude", "settings.json"), `{"hooks":`) },
		"duplicate key":   func(e *env) { write(t, filepath.Join(e.home, ".claude", "settings.json"), `{"hooks":{},"hooks":{}}`) },
		"event not array": func(e *env) { write(t, filepath.Join(e.home, ".claude", "settings.json"), `{"hooks":{"Stop":{}}}`) },
		"symlinked file": func(e *env) {
			target := filepath.Join(e.home, "dotfiles", "settings.json")
			write(t, target, "{}")
			must(t, os.MkdirAll(filepath.Join(e.home, ".claude"), 0755))
			must(t, os.Symlink(target, filepath.Join(e.home, ".claude", "settings.json")))
		},
		"symlinked script dir": func(e *env) {
			must(t, os.MkdirAll(filepath.Join(e.home, "outside"), 0755))
			must(t, os.MkdirAll(filepath.Join(e.home, ".claude", "hooks"), 0755))
			must(t, os.Symlink(filepath.Join(e.home, "outside"), filepath.Join(e.home, ".claude", "hooks", "skillshare")))
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			setup(e)
			doc := `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}]},"files":{"a.sh":"echo"}}}}`
			if _, err := e.service.PreviewMutation(Mutation{Name: "demo", Entry: entry(t, doc)}); err == nil {
				t.Fatal("preview must fail closed")
			}
		})
	}
}

func TestSync_CrossConfigOwnershipIsRespected(t *testing.T) {
	e := newEnv(t)
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	other := *e.service
	other.ConfigPath = filepath.Join(filepath.Dir(e.config), "other.yaml")
	write(t, other.ConfigPath, "targets: {}\n")
	p, err := other.PreviewMutation(Mutation{Name: "guard", Entry: entry(t, claudeEntry), Replace: true})
	must(t, err)
	if !p.Blocked || !strings.Contains(p.Changes[0].Message, "another Skillshare config") {
		t.Fatalf("replace must not take another live config's hook: %+v", p.Changes)
	}
	// The other config's sync never prunes what this config owns.
	sync(t, &other)
	if n := len(events(t, filepath.Join(e.home, ".claude", "settings.json"), true)["PreToolUse"]); n != 1 {
		t.Fatalf("other config removed a hook it does not own: %d", n)
	}
	// Once the owner is gone, an explicit replace may take over.
	must(t, os.Remove(e.config))
	save(t, &other, Mutation{Name: "guard", Entry: entry(t, claudeEntry), Replace: true})
	if n := len(events(t, filepath.Join(e.home, ".claude", "settings.json"), true)["PreToolUse"]); n != 1 {
		t.Fatalf("takeover must not duplicate: %d", n)
	}
}

func TestSync_DroidNeverShadowsInlineSettingsHooks(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(e.home, ".factory", "settings.json"), `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo inline"}]}]}}`)
	p, err := e.service.PreviewMutation(Mutation{Name: "demo", Entry: entry(t, `{"bindings":{"droid":`+adapterEntry["droid"]+`}}`)})
	must(t, err)
	if !p.Blocked {
		t.Fatal("creating hooks.json would silently stop Droid reading settings.json hooks")
	}
	inv, err := e.service.List()
	must(t, err)
	found := false
	for _, u := range inv.Unmanaged {
		found = found || u.Target == "droid" && strings.HasSuffix(u.Path, "settings.json")
	}
	if !found {
		t.Fatalf("inline Droid hooks must be disclosed: %+v", inv.Unmanaged)
	}
}

func TestRecovery_InterruptedWriteRecordsOwnership(t *testing.T) {
	e := newEnv(t)
	m := Mutation{Name: "guard", Entry: entry(t, claudeEntry)}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	source, replace, adopt, err := e.service.draft(m)
	must(t, err)
	must(t, source.save())
	p, err = e.service.previewSource(source, replace, adopt)
	must(t, err)
	// Simulate a crash after the native write and before the ledger update.
	f := p.files[0]
	must(t, writeJSONFile(e.service.journalPath(), journal{Path: f.path, After: digest(f.after), State: p.state}))
	must(t, atomicWrite(f.path, f.after, 0644))
	plan, err := e.service.Preview()
	must(t, err)
	if plan.Blocked || !strings.Contains(actions(plan), "unchanged") {
		t.Fatalf("preview must read recovered ownership: %s", actions(plan))
	}
	sync(t, e.service)
	if exists(e.service.journalPath()) {
		t.Fatal("journal must be cleared by the next operation")
	}
	save(t, e.service, Mutation{Name: "guard", Remove: true})
	if n := len(events(t, f.path, true)["PreToolUse"]); n != 0 {
		t.Fatal("recovered ownership must allow pruning")
	}
}

func TestRestore_RevertsOnlyThatWriteAndKeepsLaterChanges(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, userSettings)
	r := save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	id := r.BackupIDs[0]
	// A later, unrelated user change.
	write(t, path, strings.Replace(read(t, path), `"model": "opus"`, `"model": "sonnet"`, 1))
	p, err := e.service.PreviewRestore(id)
	must(t, err)
	if p.Blocked {
		t.Fatalf("restore blocked: %+v", p.Changes)
	}
	_, err = e.service.Restore(id, p.Revision)
	must(t, err)
	got := read(t, path)
	if strings.Contains(got, "echo guard") || !strings.Contains(got, `"model": "sonnet"`) || !strings.Contains(got, "echo mine") {
		t.Fatalf("restore must undo only its write:\n%s", got)
	}
	// Ownership went back too: a sync republishes rather than conflicting.
	sync(t, e.service)
	if !strings.Contains(read(t, path), "echo guard") {
		t.Fatal("sync after restore must republish the source")
	}
	// A backup whose output was edited since is a conflict.
	backups, err := e.service.Backups()
	must(t, err)
	latest := backups[0].ID
	write(t, path, strings.Replace(read(t, path), "echo guard", "echo tampered", 1))
	p, err = e.service.PreviewRestore(latest)
	must(t, err)
	if !p.Blocked {
		t.Fatal("restore must not overwrite newer changes")
	}
	if _, err := e.service.Restore(latest, p.Revision); !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestRestore_WholeFileAndOwnershipScope(t *testing.T) {
	e := newEnv(t)
	ts := filepath.Join(e.home, ".config", "amp", "plugins", "skillshare-demo.ts")
	r := save(t, e.service, Mutation{Name: "demo", Entry: entry(t, `{"bindings":{"amp":`+adapterEntry["amp"]+`}}`)})
	other := *e.service
	other.ConfigPath = filepath.Join(filepath.Dir(e.config), "other.yaml")
	write(t, other.ConfigPath, "targets: {}\n")
	if _, err := other.PreviewRestore(r.BackupIDs[0]); err == nil {
		t.Fatal("another config must not restore this config's backup")
	}
	if _, err := e.service.PreviewRestore("../../x"); err == nil {
		t.Fatal("backup IDs must not traverse")
	}
	p, err := e.service.PreviewRestore(r.BackupIDs[0])
	must(t, err)
	_, err = e.service.Restore(r.BackupIDs[0], p.Revision)
	must(t, err)
	if exists(ts) {
		t.Fatal("restoring a creation removes the file")
	}
}

func TestMutate_SourceOnlySaveIgnoresBlockedNativeFiles(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(e.home, ".claude", "settings.json"), `{"hooks":`)
	if _, err := e.service.Mutate(Mutation{Name: "guard", Entry: entry(t, claudeEntry)}, "", false); err != nil {
		t.Fatalf("saving the definition must not need a readable native file: %v", err)
	}
	source, err := LoadSource(e.config)
	must(t, err)
	if _, ok := source.Entries["guard"]; !ok {
		t.Fatal("definition not saved")
	}
	if _, err := e.service.Mutate(Mutation{}, "", true); err == nil {
		t.Fatal("sync must still fail closed on the malformed file")
	}
}

func TestSource_CodeRoundTripsVerbatimThroughYAML(t *testing.T) {
	e := newEnv(t)
	code := "export default function (pi) {\n\tpi.on(\"x\", () => {}); \n}\n\n// tail  "
	save(t, e.service, Mutation{Name: "demo", Entry: &Entry{Bindings: map[string]Binding{"pi": {Code: code}}}})
	source, err := LoadSource(e.config)
	must(t, err)
	if source.Entries["demo"].Bindings["pi"].Code != code {
		t.Fatalf("code changed through config.yaml: %q", source.Entries["demo"].Bindings["pi"].Code)
	}
	if read(t, filepath.Join(e.home, ".pi", "agent", "extensions", "skillshare-demo.ts")) != code {
		t.Fatal("code changed on its way to Pi")
	}
}

func TestMutate_ReplayedRevisionIsStale(t *testing.T) {
	e := newEnv(t)
	m := Mutation{Name: "guard", Entry: entry(t, claudeEntry)}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	_, err = e.service.Mutate(m, p.Revision, true)
	must(t, err)
	if _, err := e.service.Mutate(m, p.Revision, true); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("a replayed preview must be refused, got %v", err)
	}
	if n := len(events(t, filepath.Join(e.home, ".claude", "settings.json"), true)["PreToolUse"]); n != 1 {
		t.Fatalf("replay must not duplicate: %d", n)
	}
}
