package hooks

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjects_GlobalConfigPublishesIntoRootsOnly(t *testing.T) {
	e := newEnv(t)
	a := filepath.Join(filepath.Dir(e.home), "a")
	b := filepath.Join(filepath.Dir(e.home), "b")
	save(t, e.service, Mutation{Project: a})
	save(t, e.service, Mutation{Project: a, Name: "guard", Entry: entry(t, claudeEntry)})
	save(t, e.service, Mutation{Project: b, Name: "guard", Entry: entry(t, claudeEntry)})
	for _, root := range []string{a, b} {
		if n := len(events(t, filepath.Join(root, ".claude", "settings.json"), true)["PreToolUse"]); n != 1 {
			t.Fatalf("%s: %d groups", root, n)
		}
	}
	if exists(filepath.Join(e.home, ".claude")) {
		t.Fatal("project entries must not reach the global scope")
	}
	source, err := LoadSource(e.config)
	must(t, err)
	if len(source.Projects) != 2 || len(source.Entries) != 0 {
		t.Fatalf("source: %+v", source.Projects)
	}

	// Removing a block saves without sync; ApplyProject then prunes only that root.
	p, err := e.service.PreviewMutation(Mutation{Project: a, Remove: true})
	must(t, err)
	_, err = e.service.Mutate(Mutation{Project: a, Remove: true}, p.Revision, false)
	must(t, err)
	if _, err := e.service.ApplyProject("x", a); !errors.Is(err, ErrUnknownProject) {
		t.Fatalf("a removed block is no longer a known root: %v", err)
	}
	// Change b's entry without syncing, then sync only b.
	_, err = e.service.Mutate(Mutation{Project: b, Name: "guard", Entry: entry(t, strings.Replace(claudeEntry, "echo guard", "echo v2", 1))}, "", false)
	must(t, err)
	p, err = e.service.Preview()
	must(t, err)
	roots := map[string]bool{}
	for _, c := range p.Changes {
		roots[c.Root] = true
	}
	if !roots[a] || !roots[b] {
		t.Fatalf("changes must carry their root: %+v", p.Changes)
	}
	_, err = e.service.ApplyProject(p.Revision, b)
	must(t, err)
	if !strings.Contains(read(t, filepath.Join(b, ".claude", "settings.json")), "echo v2") {
		t.Fatal("b not synced")
	}
	if !strings.Contains(read(t, filepath.Join(a, ".claude", "settings.json")), "echo guard") {
		t.Fatal("syncing b must not prune a")
	}
	// A full sync then prunes a.
	sync(t, e.service)
	if exists(filepath.Join(a, ".claude", "settings.json")) {
		t.Fatal("removed block's output must be pruned on sync")
	}
}

func TestProjects_ApplyProjectNeedsFreshRevision(t *testing.T) {
	e := newEnv(t)
	root := filepath.Join(filepath.Dir(e.home), "a")
	_, err := e.service.Mutate(Mutation{Project: root, Name: "guard", Entry: entry(t, claudeEntry)}, "", false)
	must(t, err)
	p, err := e.service.Preview()
	must(t, err)
	write(t, filepath.Join(root, ".claude", "settings.json"), `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}]}}`)
	if _, err := e.service.ApplyProject(p.Revision, root); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("want ErrStaleRevision, got %v", err)
	}
}

func TestProjects_MutationSyncsOnlyItsRoot(t *testing.T) {
	e := newEnv(t)
	a := filepath.Join(filepath.Dir(e.home), "a")
	b := filepath.Join(filepath.Dir(e.home), "b")
	// Pending global output, and a pending project B output that conflicts with an
	// identical hook B's user wrote.
	_, err := e.service.Mutate(Mutation{Name: "global", Entry: entry(t, claudeEntry)}, "", false)
	must(t, err)
	bPath := filepath.Join(b, ".claude", "settings.json")
	write(t, bPath, `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo guard","timeout":5}]}]}}`)
	_, err = e.service.Mutate(Mutation{Project: b, Name: "guard", Entry: entry(t, claudeEntry)}, "", false)
	must(t, err)
	bBefore := read(t, bPath)

	r := save(t, e.service, Mutation{Project: a, Name: "guard", Entry: entry(t, claudeEntry)})
	aPath := filepath.Join(a, ".claude", "settings.json")
	if len(r.Applied) != 1 || r.Applied[0] != aPath {
		t.Fatalf("only A's output may be written: %v", r.Applied)
	}
	if exists(filepath.Join(e.home, ".claude")) {
		t.Fatal("pending global output must wait for its own sync")
	}
	if read(t, bPath) != bBefore {
		t.Fatal("project B must not be touched")
	}

	// The revision still covers the whole plan: a change in B makes A's preview stale.
	m := Mutation{Project: a, Name: "guard", Entry: entry(t, strings.Replace(claudeEntry, "echo guard", "echo v2", 1))}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	write(t, bPath, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}]}}`)
	if _, err := e.service.Mutate(m, p.Revision, true); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("want ErrStaleRevision, got %v", err)
	}
	if strings.Contains(read(t, aPath), "echo v2") {
		t.Fatal("a stale mutation must not write")
	}
}

func TestProjects_MutationConflictInItsRootBlocksSourceAndNative(t *testing.T) {
	e := newEnv(t)
	a := filepath.Join(filepath.Dir(e.home), "a")
	aPath := filepath.Join(a, ".claude", "settings.json")
	write(t, aPath, `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo guard","timeout":5}]}]}}`)
	before, config := read(t, aPath), read(t, e.config)
	m := Mutation{Project: a, Name: "guard", Entry: entry(t, claudeEntry)}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	if _, err := e.service.Mutate(m, p.Revision, true); !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	if read(t, aPath) != before || read(t, e.config) != config {
		t.Fatal("a conflict in the selected root must block the source and native writes")
	}
}

func TestProjects_BlockRemovalSyncPrunesOnlyItsRoot(t *testing.T) {
	e := newEnv(t)
	a := filepath.Join(filepath.Dir(e.home), "a")
	b := filepath.Join(filepath.Dir(e.home), "b")
	save(t, e.service, Mutation{Project: a, Name: "guard", Entry: entry(t, claudeEntry)})
	save(t, e.service, Mutation{Project: b, Name: "guard", Entry: entry(t, claudeEntry)})
	_, err := e.service.Mutate(Mutation{Name: "global", Entry: entry(t, claudeEntry)}, "", false)
	must(t, err)
	_, err = e.service.Mutate(Mutation{Project: b, Name: "guard", Entry: entry(t, strings.Replace(claudeEntry, "echo guard", "echo v2", 1))}, "", false)
	must(t, err)

	save(t, e.service, Mutation{Project: a, Remove: true})
	if exists(filepath.Join(a, ".claude", "settings.json")) {
		t.Fatal("removing a block with sync must prune its root")
	}
	if !strings.Contains(read(t, filepath.Join(b, ".claude", "settings.json")), "echo guard") {
		t.Fatal("B's pending change must wait for its own sync")
	}
	if exists(filepath.Join(e.home, ".claude")) {
		t.Fatal("pending global output must wait for its own sync")
	}
}

func TestProjects_RootsWithOwnConfigAreBlockedAndDisclosed(t *testing.T) {
	e := newEnv(t)
	root := filepath.Join(filepath.Dir(e.home), "own")
	e.project(root)
	_, err := e.service.Mutate(Mutation{Project: root, Name: "guard", Entry: entry(t, claudeEntry)}, "", false)
	must(t, err)
	inv, err := e.service.List()
	must(t, err)
	if len(inv.ProjectConfigs) != 1 || inv.ProjectConfigs[0] != root || !inv.Plan.Blocked {
		t.Fatalf("own-config root must be blocked and disclosed: %+v %+v", inv.ProjectConfigs, inv.Plan)
	}
	if _, ok := inv.ProjectPaths[root]; ok {
		t.Fatal("no native paths are shown for a root the global config cannot manage")
	}
}

func TestProjects_ProjectScopeCannotDeclareProjects(t *testing.T) {
	e := newEnv(t)
	project := e.project(filepath.Join(filepath.Dir(e.home), "repo"))
	if _, err := project.PreviewMutation(Mutation{Project: "/tmp/other"}); err == nil {
		t.Fatal("project mode must reject hooks.projects mutations")
	}
	write(t, project.ConfigPath, "hooks:\n  projects:\n    /tmp/x:\n      entries: {}\n")
	if _, err := project.Preview(); err == nil {
		t.Fatal("project config must not declare hooks.projects")
	}
	if _, err := project.Import(ImportRequest{From: "claude", Root: "/tmp/x"}); err == nil {
		t.Fatal("project mode must reject import roots")
	}
}

func TestList_InventoryShowsTargetsPathsUnmanagedAndBackups(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(e.home, ".claude", "settings.json"), userSettings)
	write(t, filepath.Join(e.home, ".codex", "config.toml"), "[[hooks.Stop]]\n[[hooks.Stop.hooks]]\ntype = \"command\"\ncommand = \"echo\"\n")
	write(t, filepath.Join(e.home, ".config", "opencode", "plugins", "mine.ts"), "export const Mine = async () => ({})\n")
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	inv, err := e.service.List()
	must(t, err)
	if len(inv.Targets) != 12 || inv.Paths["droid"] != filepath.Join(e.home, ".factory", "hooks.json") || len(inv.Backups) != 1 || inv.Plan == nil {
		t.Fatalf("inventory: %+v", inv)
	}
	got := map[string]string{}
	for _, u := range inv.Unmanaged {
		got[u.Target+":"+filepath.Base(u.Path)] = strings.Join(u.Names, ",")
	}
	// PreToolUse holds both the user's group and the owned one; only the user's counts.
	if got["claude:settings.json"] != "PreToolUse,Stop" || got["codex:config.toml"] != "Stop" || got["opencode:plugins"] != "mine.ts" {
		t.Fatalf("unmanaged: %v", got)
	}
	for _, target := range inv.Targets {
		if target.Note == "" || (target.Kind != KindCommand && target.Kind != KindCode && target.Kind != KindGit) {
			t.Fatalf("target definition: %+v", target)
		}
	}
}

func TestImport_OffersOnlyUnmanagedHooksWithoutExecuting(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, userSettings)
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	candidates, err := e.service.Import(ImportRequest{From: "claude"})
	must(t, err)
	names := []string{}
	for _, c := range candidates {
		names = append(names, c.Name)
		if len(c.Problems) != 0 {
			t.Fatalf("%s: %v", c.Name, c.Problems)
		}
		if strings.Contains(c.Name, "pretooluse") && len(c.Entry.Bindings["claude"].Events["PreToolUse"].([]any)) != 1 {
			t.Fatal("the owned group must not be offered")
		}
	}
	if strings.Join(names, ",") != "claude-pretooluse,claude-stop" {
		t.Fatalf("candidates: %v", names)
	}
	one, err := e.service.Import(ImportRequest{From: "claude", Name: "mine"})
	must(t, err)
	if len(one) != 1 || one[0].Name != "mine" || len(one[0].Entry.Bindings["claude"].Events) != 2 {
		t.Fatalf("named import merges everything: %+v", one)
	}
	// Saving the import and syncing with replace takes over in place.
	save(t, e.service, Mutation{Name: "mine", Entry: &one[0].Entry, Replace: true})
	if n := len(events(t, path, true)["PreToolUse"]); n != 2 {
		t.Fatalf("import+replace must not duplicate: %d", n)
	}
	if rest, _ := e.service.Import(ImportRequest{From: "claude"}); len(rest) != 0 {
		t.Fatalf("everything is managed now: %+v", rest)
	}
}

func TestImport_ContentCodeAndCopilot(t *testing.T) {
	e := newEnv(t)
	code := "export default Plugin.define({ id: \"x\", async setup() {} })\n"
	c, err := e.service.Import(ImportRequest{From: "opencode", Content: code, Name: "v2"})
	must(t, err)
	if len(c) != 1 || c[0].Entry.Bindings["opencode"].Code != code {
		t.Fatalf("code must be imported verbatim: %+v", c)
	}
	write(t, filepath.Join(e.home, ".pi", "agent", "extensions", "guard.ts"), "export default function (pi) {}\n")
	c, err = e.service.Import(ImportRequest{From: "pi"})
	must(t, err)
	if len(c) != 1 || c[0].Name != "guard" || len(c[0].Warnings) == 0 {
		t.Fatalf("pi file import: %+v", c)
	}
	write(t, filepath.Join(e.home, ".copilot", "hooks", "team.json"), `{"version":1,"hooks":{"sessionStart":[{"type":"command","bash":"echo hi"}]}}`)
	c, err = e.service.Import(ImportRequest{From: "copilot"})
	must(t, err)
	if len(c) != 1 || c[0].Name != "team" || len(c[0].Problems) != 0 {
		t.Fatalf("copilot import: %+v", c)
	}
	c, err = e.service.Import(ImportRequest{From: "factory", Content: `{"Stop":[{"hooks":[{"type":"command","command":"echo"}]}]}`})
	must(t, err)
	if len(c) != 1 || c[0].Entry.Bindings["droid"].Events["Stop"] == nil {
		t.Fatalf("droid content uses the unwrapped event map: %+v", c)
	}
	if _, err := e.service.Import(ImportRequest{From: "claude", Content: `{"hooks":`}); err == nil {
		t.Fatal("malformed content must be rejected")
	}
}

func TestImport_NamePicksOneCodeFile(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(e.home, ".config", "amp", "plugins", "one.ts"), "export default function (amp) {}\n")
	write(t, filepath.Join(e.home, ".config", "amp", "plugins", "two.js"), "export default function (amp) {}\n")
	c, err := e.service.Import(ImportRequest{From: "amp", Name: "two"})
	must(t, err)
	if len(c) != 1 || c[0].Name != "two" || c[0].Entry.Bindings["amp"].Code == "" {
		t.Fatalf("name must pick one file: %+v", c)
	}
}
