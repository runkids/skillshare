package hooks

import (
	"path/filepath"
	"strings"
	"testing"
)

const trialSettings = `{"model":"opus","hooks":{"Stop":[{"hooks":[{"type":"command","command":"afplay /System/Library/Sounds/Glass.aiff","timeout":5}]}],"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"~/.claude/guard.sh"}]}]}}`

func importOne(t *testing.T, s *Service, from, name string) Candidate {
	t.Helper()
	c, err := s.Import(ImportRequest{From: from, Name: name})
	must(t, err)
	if len(c) != 1 {
		t.Fatalf("import %s: %+v", name, c)
	}
	return c[0]
}

func TestImport_CandidateNameTakesOnlyItsEvent(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(e.home, ".claude", "settings.json"), trialSettings)
	c := importOne(t, e.service, "claude", "claude-stop")
	if events := c.Entry.Bindings["claude"].Events; len(events) != 1 || events["Stop"] == nil {
		t.Fatalf("claude-stop must hold only Stop: %v", events)
	}
}

func TestImport_SavingTakesOverWithoutReplace(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, trialSettings)
	c := importOne(t, e.service, "claude", "claude-stop")
	for _, w := range c.Warnings {
		if strings.Contains(w, "replace") {
			t.Fatalf("import must not ask for replace: %q", w)
		}
	}
	m := Mutation{Name: c.Name, Entry: &c.Entry, Adopt: true}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	if p.Blocked || actions(p) != "claude:claude-stop:adopt" {
		t.Fatalf("import preview: %s", actions(p))
	}
	// Saved without sync, the takeover is still recorded.
	if _, err := e.service.Mutate(m, p.Revision, false); err != nil {
		t.Fatal(err)
	}
	p, err = e.service.Preview()
	must(t, err)
	if p.Blocked || actions(p) != "claude:claude-stop:adopt" {
		t.Fatalf("sync after import: %s", actions(p))
	}
	r := sync(t, e.service)
	if len(r.Applied) != 0 || read(t, path) != trialSettings {
		t.Fatalf("taking over must not rewrite the file: %v\n%s", r.Applied, read(t, path))
	}
	if p, _ = e.service.Preview(); actions(p) != "claude:claude-stop:unchanged" {
		t.Fatalf("after the adopting sync: %s", actions(p))
	}
	if rest, _ := e.service.Import(ImportRequest{From: "claude"}); len(rest) != 1 || rest[0].Name != "claude-pretooluse" {
		t.Fatalf("PreToolUse stays unmanaged: %+v", rest)
	}
	write(t, path, strings.Replace(trialSettings, `"timeout":5`, `"timeout":9`, 1))
	if p, _ = e.service.Preview(); !p.Blocked {
		t.Fatalf("an outside edit after import is a conflict: %s", actions(p))
	}
}

func TestImport_AdoptNeverClaimsOtherConfigsOrPlainSaves(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, trialSettings)
	c := importOne(t, e.service, "claude", "claude-stop")
	p, err := e.service.PreviewMutation(Mutation{Name: "copy", Entry: &c.Entry})
	must(t, err)
	if !p.Blocked {
		t.Fatalf("a plain add of an identical hook is still a conflict: %s", actions(p))
	}
}

func TestPlan_NarrowedEntryIsAnUpdateWithEventDetail(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, `{"model":"opus"}`)
	both := entry(t, `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"echo stop"}]}],"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo guard"}]}]}}}}`)
	p, err := e.service.PreviewMutation(Mutation{Name: "trial", Entry: both})
	must(t, err)
	if c := p.Changes[0]; c.Action != "add" || c.Events == nil || strings.Join(c.Events.Added, ",") != "PreToolUse,Stop" {
		t.Fatalf("add detail: %+v %+v", c, c.Events)
	}
	save(t, e.service, Mutation{Name: "trial", Entry: both})

	stop := entry(t, `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"echo stop2"}]}]}}}}`)
	p, err = e.service.PreviewMutation(Mutation{Name: "trial", Entry: stop})
	must(t, err)
	c := p.Changes[0]
	if c.Action != "update" || c.Events == nil || strings.Join(c.Events.Removed, ",") != "PreToolUse" || strings.Join(c.Events.Updated, ",") != "Stop" || len(c.Events.Added) != 0 {
		t.Fatalf("narrowed entry: %+v %+v", c, c.Events)
	}

	narrowed := entry(t, `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"echo stop"}]}]}}}}`)
	p, err = e.service.PreviewMutation(Mutation{Name: "trial", Entry: narrowed})
	must(t, err)
	if c := p.Changes[0]; c.Action != "update" || strings.Join(c.Events.Removed, ",") != "PreToolUse" || len(c.Events.Updated) != 0 {
		t.Fatalf("an entry that keeps Stop in the file is an update: %+v %+v", c, c.Events)
	}

	p, err = e.service.PreviewMutation(Mutation{Name: "trial", Remove: true})
	must(t, err)
	if c := p.Changes[0]; c.Action != "remove" || strings.Join(c.Events.Removed, ",") != "PreToolUse,Stop" {
		t.Fatalf("leaving the file entirely is a remove: %+v %+v", c, c.Events)
	}
}

func TestReplace_OwnedRegistrationEditedOutsideIsReplacedInPlace(t *testing.T) {
	for _, viaSync := range []bool{false, true} {
		e := newEnv(t)
		path := filepath.Join(e.home, ".claude", "settings.json")
		write(t, path, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"say done"}]}]}}`)
		c := importOne(t, e.service, "claude", "claude-stop")
		save(t, e.service, Mutation{Name: c.Name, Entry: &c.Entry, Adopt: true})
		write(t, path, strings.Replace(read(t, path), "say done", "say finished", 1))
		if p, _ := e.service.Preview(); !p.Blocked {
			t.Fatalf("an outside edit is a conflict: %s", actions(p))
		}
		m := Mutation{Name: c.Name, Entry: &c.Entry, Replace: true}
		if viaSync {
			m = Mutation{Name: c.Name, Replace: true}
		}
		p, err := e.service.PreviewMutation(m)
		must(t, err)
		files := p.Files()
		save(t, e.service, m)
		got := events(t, path, true)["Stop"]
		if len(got) != 1 || !strings.Contains(read(t, path), "say done") {
			t.Fatalf("replace must take the edited group over in place (sync=%t):\n%s", viaSync, read(t, path))
		}
		if len(files) != 1 || files[0].After != read(t, path) {
			t.Fatalf("preview after must match the written file: %+v", files)
		}
		if p, _ := e.service.Preview(); actions(p) != "claude:claude-stop:unchanged" {
			t.Fatalf("after replace: %s", actions(p))
		}
	}
}

func TestImport_CodeAgentAdoptsOriginalFileAndRetiresItOnSync(t *testing.T) {
	for _, ext := range []string{".ts", ".js"} {
		for _, viaSync := range []bool{false, true} {
			e := newEnv(t)
			extDir := filepath.Join(e.home, ".pi", "agent", "extensions")
			orig := filepath.Join(extDir, "activity"+ext)
			code := "export default function (pi) {\n  pi.on(\"tool_call\", async () => {});\n}\n"
			write(t, orig, code)

			c := importOne(t, e.service, "pi", "activity")
			if c.Source != "activity"+ext {
				t.Fatalf("Source = %q, want activity%s", c.Source, ext)
			}
			m := Mutation{Name: "custom-activity", Entry: &c.Entry, Adopt: true, AdoptSource: c.Source}
			p, err := e.service.PreviewMutation(m)
			must(t, err)
			if p.Blocked {
				t.Fatalf("preview blocked: %s", actions(p))
			}
			var sawRetireMsg bool
			for _, ch := range p.Changes {
				if ch.Action == "remove" && ch.Path == orig && strings.Contains(ch.Message, "retires the imported original") {
					sawRetireMsg = true
				}
			}
			if !sawRetireMsg {
				t.Fatalf("expected retirement message on remove change: %+v", p.Changes)
			}
			res, err := e.service.Mutate(m, p.Revision, viaSync)
			must(t, err)

			// Immediately after saving (even before sync), the original file is owned so List/Import do not re-offer it as -2.
			inv, err := e.service.List()
			must(t, err)
			for _, u := range inv.Unmanaged {
				if u.Target == "pi" {
					t.Fatalf("adopted file still listed as unmanaged (ext=%s viaSync=%t): %+v", ext, viaSync, u)
				}
			}
			if again, err := e.service.Import(ImportRequest{From: "pi"}); err != nil || len(again) != 0 {
				t.Fatalf("adopted file offered again (ext=%s viaSync=%t): %+v, err=%v", ext, viaSync, again, err)
			}

			if !viaSync {
				if !exists(orig) {
					t.Fatal("source-only save must keep original file on disk until sync")
				}
				res = sync(t, e.service)
			}

			managed := filepath.Join(extDir, "skillshare-custom-activity.ts")
			// Managed output must be written before the retired original is deleted.
			if len(res.Applied) != 2 || res.Applied[0] != managed || res.Applied[1] != orig {
				t.Fatalf("expected write before retirement removal, got Applied=%v", res.Applied)
			}
			if exists(orig) || read(t, managed) != code {
				t.Fatalf("expected %s replaced by %s (ext=%s viaSync=%t)", orig, managed, ext, viaSync)
			}
			if p, _ := e.service.Preview(); p.Blocked || actions(p) != "pi:custom-activity:unchanged" {
				t.Fatalf("after sync: %s", actions(p))
			}
			if len(res.BackupIDs) != 2 {
				t.Fatalf("expected 2 backups (managed write + retired removal), got %v", res.BackupIDs)
			}

			// Previewing restore of the retired original names the hook, warns about double execution, and restores it unowned.
			rp, err := e.service.PreviewRestore(res.BackupIDs[1])
			must(t, err)
			if len(rp.Changes) != 1 || rp.Changes[0].Name != "custom-activity" {
				t.Fatalf("restore preview change: %+v", rp.Changes)
			}
			if !strings.Contains(strings.Join(rp.Warnings, "\n"), "custom-activity") {
				t.Fatalf("expected double-execution warning on restore preview, got %v", rp.Warnings)
			}
			// Source-only disable leaves the synced file live on disk, so restore preview must still warn until synced.
			disabled := false
			disabledEntry := c.Entry
			disabledEntry.Enabled = &disabled
			_, err = e.service.Mutate(Mutation{Name: "custom-activity", Entry: &disabledEntry}, "", false)
			must(t, err)
			rpUnsynced, err := e.service.PreviewRestore(res.BackupIDs[1])
			must(t, err)
			if !strings.Contains(strings.Join(rpUnsynced.Warnings, "\n"), "hooks sync") {
				t.Fatalf("expected unsynced live-output warning on restore preview, got %v", rpUnsynced.Warnings)
			}
			// Re-enable in source so the rest of the restore test leaves source enabled.
			_, err = e.service.Mutate(Mutation{Name: "custom-activity", Entry: &c.Entry}, "", false)
			must(t, err)
			rp, err = e.service.PreviewRestore(res.BackupIDs[1])
			must(t, err)
			_, err = e.service.Restore(res.BackupIDs[1], rp.Revision)
			must(t, err)
			if !exists(orig) {
				t.Fatal("expected retired original file restored")
			}
			// Restored retired original comes back unowned (offered by Import again).
			if restoredCandidates, err := e.service.Import(ImportRequest{From: "pi"}); err != nil || len(restoredCandidates) != 1 {
				t.Fatalf("expected restored original to be unowned: %+v, err=%v", restoredCandidates, err)
			}
		}
	}
}

func TestImport_CodeAgentEditedBeforeSyncBlocksRemoval(t *testing.T) {
	e := newEnv(t)
	orig := filepath.Join(e.home, ".pi", "agent", "extensions", "activity.ts")
	write(t, orig, "export default function () {}\n")
	c := importOne(t, e.service, "pi", "activity")
	m := Mutation{Name: c.Name, Entry: &c.Entry, Adopt: true, AdoptSource: c.Source}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	_, err = e.service.Mutate(m, p.Revision, false)
	must(t, err)

	// User edits the adopted file before running sync: sync must block instead of deleting their edit.
	write(t, orig, "export default function () { /* edited */ }\n")
	p, err = e.service.Preview()
	must(t, err)
	if !p.Blocked {
		t.Fatalf("expected conflict when adopted file was edited before sync: %s", actions(p))
	}
}

func TestImport_CopilotProjectAccountAndOpenCodeV2AdoptFiles(t *testing.T) {
	// 1. Copilot: both plain foo.json (retired) and skillshare-bar.json (adopted in place).
	e := newEnv(t)
	copilotDir := filepath.Join(e.home, ".copilot", "hooks")
	hookJSON := "{\n  \"hooks\": {\n    \"sessionStart\": [\n      {\n        \"bash\": \"echo hi\",\n        \"type\": \"command\"\n      }\n    ]\n  },\n  \"version\": 1\n}\n"
	write(t, filepath.Join(copilotDir, "foo.json"), hookJSON)
	write(t, filepath.Join(copilotDir, "skillshare-bar.json"), hookJSON)

	foo := importOne(t, e.service, "copilot", "foo")
	save(t, e.service, Mutation{Name: foo.Name, Entry: &foo.Entry, Adopt: true, AdoptSource: foo.Source})
	if exists(filepath.Join(copilotDir, "foo.json")) || !exists(filepath.Join(copilotDir, "skillshare-foo.json")) {
		t.Fatal("expected foo.json retired into skillshare-foo.json")
	}

	bar := importOne(t, e.service, "copilot", "bar")
	if !strings.Contains(strings.Join(bar.Warnings, "\n"), "leaves them as they are") {
		t.Fatalf("expected in-place adopt warning for skillshare-bar.json: %v", bar.Warnings)
	}
	p, err := e.service.PreviewMutation(Mutation{Name: bar.Name, Entry: &bar.Entry, Adopt: true, AdoptSource: bar.Source})
	must(t, err)
	if actions(p) != "copilot:bar:adopt,copilot:foo:unchanged" {
		t.Fatalf("expected in-place adopt for skillshare-bar.json, got %s", actions(p))
	}
	res, err := e.service.Mutate(Mutation{Name: bar.Name, Entry: &bar.Entry, Adopt: true, AdoptSource: bar.Source}, p.Revision, true)
	must(t, err)
	if len(res.Applied) != 0 {
		t.Fatalf("in-place adopt must not rewrite unchanged file: %v", res.Applied)
	}

	// 2. Account target (omp-work) + hooks.projects root + OpenCode v2 flat .ts retirement.
	ompWorkDir := filepath.Join(e.home, ".omp-work", "agent")
	e.service.Accounts = map[string]Account{"omp-work": {Agent: "omp", Dir: ompWorkDir}}
	e.service.OpenCodeVersion = func(string) (string, error) { return "2.1.0", nil }

	write(t, filepath.Join(ompWorkDir, "extensions", "guard.ts"), "export default function () {}\n")
	ompCand := importOne(t, e.service, "omp-work", "guard")
	save(t, e.service, Mutation{Name: ompCand.Name, Entry: &ompCand.Entry, Adopt: true, AdoptSource: ompCand.Source})
	if exists(filepath.Join(ompWorkDir, "extensions", "guard.ts")) || !exists(filepath.Join(ompWorkDir, "extensions", "skillshare-guard.ts")) {
		t.Fatal("expected omp-work guard.ts retired into skillshare-guard.ts")
	}

	ocPlugin := filepath.Join(e.home, ".config", "opencode", "plugins", "legacy.ts")
	write(t, ocPlugin, "export default async () => ({})\n")
	ocCand := importOne(t, e.service, "opencode", "legacy")
	save(t, e.service, Mutation{Name: ocCand.Name, Entry: &ocCand.Entry, Adopt: true, AdoptSource: ocCand.Source})
	if exists(ocPlugin) || !exists(filepath.Join(e.home, ".config", "opencode", "plugins", "skillshare-legacy", "index.ts")) {
		t.Fatal("expected OpenCode v2 flat legacy.ts retired into skillshare-legacy/index.ts")
	}

	projRoot := filepath.Join(e.home, "proj")
	projOrig := filepath.Join(projRoot, ".pi", "extensions", "local.ts")
	write(t, projOrig, "export default function () {}\n")
	_, err = e.service.Mutate(Mutation{Project: projRoot}, "", false)
	must(t, err)
	projCands, err := e.service.Import(ImportRequest{From: "pi", Root: projRoot})
	must(t, err)
	if len(projCands) != 1 {
		t.Fatalf("expected 1 project candidate, got %+v", projCands)
	}
	pm := Mutation{Project: projRoot, Name: projCands[0].Name, Entry: &projCands[0].Entry, Adopt: true, AdoptSource: projCands[0].Source}
	pp, err := e.service.PreviewMutation(pm)
	must(t, err)
	_, err = e.service.Mutate(pm, pp.Revision, true)
	must(t, err)
	if exists(projOrig) || !exists(filepath.Join(projRoot, ".pi", "extensions", "skillshare-local.ts")) {
		t.Fatal("expected hooks.projects pi extension retired into skillshare-local.ts")
	}

	// 3. Disabled entry with AdoptSource is rejected instead of silently dropping adoption.
	disabled := false
	write(t, filepath.Join(ompWorkDir, "extensions", "off.ts"), "export default function () {}\n")
	offCand := importOne(t, e.service, "omp-work", "off")
	offCand.Entry.Enabled = &disabled
	if _, err := e.service.PreviewMutation(Mutation{Name: "off", Entry: &offCand.Entry, Adopt: true, AdoptSource: offCand.Source}); err == nil {
		t.Fatal("expected error when adopting file into disabled entry with no active output")
	}

	// 4. Mixed-case skillshare-Foo.ts on case-insensitive filesystems is adopted in place (not deleted),
	// and on case-sensitive filesystems is retired after skillshare-foo.ts is written.
	piDir := filepath.Join(e.home, ".pi", "agent", "extensions")
	mixed := filepath.Join(piDir, "skillshare-MixedCase.ts")
	lower := filepath.Join(piDir, "skillshare-mixedcase.ts")
	write(t, mixed, "export default function () {}\n")
	caseInsensitive := sameExistingFile(mixed, lower)
	mc := importOne(t, e.service, "pi", "mixedcase")
	if caseInsensitive && !strings.Contains(strings.Join(mc.Warnings, "\n"), "leaves them as they are") {
		t.Fatalf("expected in-place warning on case-insensitive filesystem: %v", mc.Warnings)
	}
	save(t, e.service, Mutation{Name: mc.Name, Entry: &mc.Entry, Adopt: true, AdoptSource: mc.Source})
	if !exists(lower) {
		t.Fatalf("expected managed hook file to exist after sync (caseInsensitive=%t)", caseInsensitive)
	}
	if !caseInsensitive && exists(mixed) {
		t.Fatal("expected mixed-case source retired on case-sensitive filesystem")
	}
	if again, err := e.service.Import(ImportRequest{From: "pi"}); err != nil || len(again) != 0 {
		t.Fatalf("mixed-case file must not remain unmanaged: %+v, err=%v", again, err)
	}
}

func TestReplace_MultipleConflictingHooksAdoptedInOneSync(t *testing.T) {
	e := newEnv(t)
	claudePath := filepath.Join(e.home, ".claude", "settings.json")
	codexPath := filepath.Join(e.home, ".codex", "hooks.json")
	write(t, claudePath, trialSettings)
	write(t, codexPath, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo codex-stop"}]}]}}`)

	c1 := importOne(t, e.service, "claude", "claude-stop")
	c2 := importOne(t, e.service, "claude", "claude-pretooluse")
	c3 := importOne(t, e.service, "codex", "codex-stop")
	for _, c := range []Candidate{c1, c2, c3} {
		_, err := e.service.Mutate(Mutation{Name: c.Name, Entry: &c.Entry}, "", false)
		must(t, err)
	}

	if _, err := e.service.PreviewMutation(Mutation{Replace: true}); err == nil {
		t.Fatal("nameless Replace must still be rejected")
	}
	if p, err := e.service.Preview(); err != nil || !p.Blocked {
		t.Fatalf("expected conflict before takeover: %+v, err=%v", p, err)
	}

	m := Mutation{ReplaceNames: []string{"claude-stop", "claude-pretooluse", "codex-stop"}}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	if p.Blocked {
		t.Fatalf("multi-hook takeover preview still blocked: %s", actions(p))
	}
	_, err = e.service.Mutate(m, p.Revision, true)
	must(t, err)

	if p, _ := e.service.Preview(); p.Blocked || actions(p) != "claude:claude-pretooluse:unchanged,claude:claude-stop:unchanged,codex:codex-stop:unchanged" {
		t.Fatalf("after multi-hook takeover: %s", actions(p))
	}

	// Verify ReplaceNames also works inside a hooks.projects root and rejects unknown roots.
	projRoot := t.TempDir()
	if _, err := e.service.PreviewMutation(Mutation{Project: projRoot, ReplaceNames: []string{"claude-stop"}}); err == nil {
		t.Fatal("expected ErrUnknownProject for unregistered project root")
	}
	write(t, filepath.Join(projRoot, ".claude", "settings.json"), trialSettings)
	_, err = e.service.Mutate(Mutation{Project: projRoot}, "", false)
	must(t, err)
	for _, c := range []Candidate{c1, c2} {
		_, err = e.service.Mutate(Mutation{Project: projRoot, Name: c.Name, Entry: &c.Entry}, "", false)
		must(t, err)
	}
	pm := Mutation{Project: projRoot, ReplaceNames: []string{"claude-stop", "claude-pretooluse"}}
	pp, err := e.service.PreviewMutation(pm)
	must(t, err)
	if pp.Blocked {
		t.Fatalf("project multi-hook takeover still blocked: %s", actions(pp))
	}
	_, err = e.service.Mutate(pm, pp.Revision, true)
	must(t, err)
}
