package hooks

import (
	"path/filepath"
	"strings"
	"testing"
)

const agyNative = `{"lint":{"PostToolUse":[{"matcher":"run_command","hooks":[{"command":"./lint.sh","timeout":10}]}]},"off":{"enabled":false,"Stop":[{"command":"echo off"}]},"mine":{"Stop":[{"command":"echo mine"}]}}`

func TestAntigravity_ImportTakesOverNamedBlockInPlace(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".gemini", "config", "hooks.json")
	write(t, path, agyNative)
	candidates, err := e.service.Import(ImportRequest{From: "agy"})
	must(t, err)
	got := map[string]int{}
	for _, c := range candidates {
		got[c.Name] = len(c.Problems)
	}
	if len(got) != 3 || got["lint"] != 0 || got["off"] != 1 {
		t.Fatalf("one candidate per block, disabled blocks blocked: %+v", candidates)
	}
	c := importOne(t, e.service, "antigravity-cli", "lint")
	m := Mutation{Name: "lint", Entry: &c.Entry, Adopt: true}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	if p.Blocked || actions(p) != "antigravity:lint:adopt" {
		t.Fatalf("import preview: %s", actions(p))
	}
	save(t, e.service, m)
	if read(t, path) != agyNative {
		t.Fatalf("taking over a block must not rewrite the file:\n%s", read(t, path))
	}
	save(t, e.service, Mutation{Name: "lint", Remove: true})
	if got := read(t, path); strings.Contains(got, `"lint"`) || !strings.Contains(got, `"mine"`) || !strings.Contains(got, `"enabled":false`) {
		t.Fatalf("remove prunes only the owned block:\n%s", got)
	}
}

func TestAntigravity_ImportUnderAnotherNameIsAConflict(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".gemini", "config", "hooks.json")
	write(t, path, agyNative)
	c := importOne(t, e.service, "agy", "lint")
	p, err := e.service.PreviewMutation(Mutation{Name: "renamed", Entry: &c.Entry, Adopt: true})
	must(t, err)
	if !p.Blocked || actions(p) != "antigravity:renamed:conflict" {
		t.Fatalf("a renamed takeover would leave the original block running beside a copy: %s", actions(p))
	}
}

func TestAntigravity_SameNamedUnmanagedBlockIsAConflict(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".gemini", "config", "hooks.json")
	write(t, path, agyNative)
	m := Mutation{Name: "mine", Entry: entry(t, `{"bindings":{"antigravity":{"events":{"Stop":[{"command":"echo other"}]}}}}`)}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	if !p.Blocked {
		t.Fatalf("a different block with the same name is a conflict: %s", actions(p))
	}
	m.Replace = true
	p, err = e.service.PreviewMutation(m)
	must(t, err)
	if c := p.Changes[0]; c.Action != "update" || strings.Join(c.Events.Updated, ",") != "Stop" {
		t.Fatalf("replace takes the block over: %+v %+v", c, c.Events)
	}
	save(t, e.service, m)
	if got := read(t, path); strings.Count(got, `"mine"`) != 1 || !strings.Contains(got, "echo other") {
		t.Fatalf("the block is replaced in place:\n%s", got)
	}
}
