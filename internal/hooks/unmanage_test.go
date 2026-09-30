package hooks

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestUnmanage_KeepsNativeEntriesAndSyncLeavesThemAlone(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	before := read(t, path)
	m := Mutation{Name: "guard", Remove: true, Unmanage: true}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	if len(p.Changes) != 0 {
		t.Fatalf("stop managing previews no change: %s", actions(p))
	}
	if _, err := e.service.Mutate(m, "", false); err != nil {
		t.Fatal(err)
	}
	if read(t, path) != before {
		t.Fatalf("native file changed:\n%s", read(t, path))
	}
	state, _, err := e.service.loadLedger()
	must(t, err)
	if len(state.Records) != 0 {
		t.Fatalf("ownership kept: %+v", state.Records)
	}
	if p, _ = e.service.Preview(); len(p.Changes) != 0 {
		t.Fatalf("sync still plans: %s", actions(p))
	}
	inv, err := e.service.List()
	must(t, err)
	if len(inv.Unmanaged) != 1 || inv.Unmanaged[0].Path != path {
		t.Fatalf("entry is not listed as unmanaged: %+v", inv.Unmanaged)
	}
	if c, _ := e.service.Import(ImportRequest{From: "claude"}); len(c) != 1 {
		t.Fatalf("entry is not importable again: %+v", c)
	}
}

func TestUnmanage_InAProjectLeavesTheGlobalHookManaged(t *testing.T) {
	e := newEnv(t)
	root := filepath.Join(e.home, "projA")
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	save(t, e.service, Mutation{Project: root, Name: "guard", Entry: entry(t, claudeEntry)})
	if _, err := e.service.Mutate(Mutation{Project: root, Name: "guard", Remove: true, Unmanage: true}, "", false); err != nil {
		t.Fatal(err)
	}
	p, err := e.service.Preview()
	must(t, err)
	if actions(p) != "claude:guard:unchanged" {
		t.Fatalf("only the global hook stays managed: %s", actions(p))
	}
}

func TestUnmanage_Rejected(t *testing.T) {
	e := newEnv(t)
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	if _, err := e.service.Mutate(Mutation{Name: "guard", Remove: true, Unmanage: true}, "", true); err == nil || !strings.Contains(err.Error(), "sync") {
		t.Fatalf("with sync: %v", err)
	}
	if _, err := e.service.Mutate(Mutation{Name: "guard", Unmanage: true}, "", false); err == nil || !strings.Contains(err.Error(), "removing a named hook") {
		t.Fatalf("without remove: %v", err)
	}
}
