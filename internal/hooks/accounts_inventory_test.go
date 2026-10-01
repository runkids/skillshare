package hooks

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInventoryListsAccounts(t *testing.T) {
	e := accountEnv(t)
	path := filepath.Join(e.home, ".codex-2", "hooks.json")
	write(t, path, `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"echo account"}]}]}}`)
	write(t, filepath.Join(e.home, ".codex-2", "config.toml"), "[hooks]\nStop = []\n")
	inv, err := e.service.List()
	must(t, err)
	got := inv.Targets[len(Targets)]
	if got.Name != "codex-2" || got.Kind != KindCommand || got.Agent != "codex" || inv.Paths["codex-2"] != path {
		t.Fatalf("account inventory: %+v", got)
	}
	found := map[string]bool{}
	for _, u := range inv.Unmanaged {
		if u.Target == "codex-2" {
			found[u.Path] = true
		}
	}
	if !found[path] || !found[filepath.Join(e.home, ".codex-2", "config.toml")] {
		t.Fatalf("unmanaged: %+v", inv.Unmanaged)
	}
}

func TestImportFromAccount(t *testing.T) {
	e := accountEnv(t)
	path := filepath.Join(e.home, ".codex-2", "hooks.json")
	write(t, path, `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"echo account"}]}]}}`)
	candidates, err := e.service.Import(ImportRequest{From: "codex-2"})
	must(t, err)
	if len(candidates) != 1 || candidates[0].Name != "codex-2-sessionstart" || len(candidates[0].Problems) != 0 {
		t.Fatalf("%+v", candidates)
	}
	if _, ok := candidates[0].Entry.Bindings["codex-2"]; !ok {
		t.Fatal("account key lost")
	}
	before := read(t, path)
	c := candidates[0]
	r := save(t, e.service, Mutation{Name: c.Name, Entry: &c.Entry, Adopt: true})
	if actions(r.Plan) != "codex-2:codex-2-sessionstart:adopt" || read(t, path) != before {
		t.Fatal("adoption changed file", actions(r.Plan))
	}
	// Pasted content uses the Agent format but preserves the account binding key.
	candidates, err = e.service.Import(ImportRequest{From: "codex-2", Content: before})
	must(t, err)
	if len(candidates) != 1 || len(candidates[0].Problems) != 0 {
		t.Fatalf("%+v", candidates)
	}
}

func TestImportFromAccountRefusesProjectRoot(t *testing.T) {
	e := accountEnv(t)
	_, err := e.service.Import(ImportRequest{From: "codex-2", Root: "/p"})
	if err == nil || err.Error() != "hooks.projects roots are read by every account; import --from codex" {
		t.Fatalf("%v", err)
	}
}

func TestReferenceWarning(t *testing.T) {
	e := newEnv(t)
	write(t, e.config, "hooks:\n  entries:\n    b: {bindings: {codex-2: {code: x}}}\n    a: {bindings: {codex-2: {code: x}}}\n")
	want := "hooks.entries.a, hooks.entries.b still name codex-2; remove it there too or the next hooks sync fails"
	if got := ReferenceWarning(e.config, "codex-2"); got != want {
		t.Fatalf("%s", got)
	}
	if got := ReferenceWarning(e.config, "codex-3"); got != "" {
		t.Fatal(got)
	}
	write(t, e.config, "hooks:\n  entries:\n    a: {bindings: {codex-2: {code: x}}}\n")
	if got := ReferenceWarning(e.config, "codex-2"); !strings.Contains(got, "still names") {
		t.Fatal(got)
	}
}
