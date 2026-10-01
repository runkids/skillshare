package hooks

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGitInventory(t *testing.T) {
	e := gitEnv(t)
	e.service.GitGlobalConfig = filepath.Join(e.home, "explicit.gitconfig")
	write(t, e.service.GitGlobalConfig, "[core]\n hooksPath = /custom/hooks\n")
	r := save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	if r.Applied[len(r.Applied)-1] != e.service.GitGlobalConfig {
		t.Fatal("explicit target ignored")
	}
	inv, err := e.service.List()
	must(t, err)
	if inv.Git == nil || !inv.Git.ConfigHooks || !inv.Git.Parallel || inv.Git.Version == "" || !inv.Git.Include.Present || !inv.Git.Include.Owned || !inv.Git.Include.Writable || inv.Git.HooksPath.Value != "/custom/hooks" {
		t.Fatalf("inventory: %+v", inv.Git)
	}
	_, err = e.service.Import(ImportRequest{From: "git"})
	if err == nil || !strings.Contains(err.Error(), "not supported yet") {
		t.Fatalf("Phase 1 import boundary: %v", err)
	}
}

func TestGitInventoryConditionalAncestryAndMissingOutput(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	write(t, d.includeTarget, includeLines(d.includeValue))
	info := e.service.gitInventory("", ledger{})
	if !info.Include.Present || info.Include.Active == nil || *info.Include.Active {
		t.Fatalf("missing output reported active: %+v", info.Include)
	}
	group := filepath.Join(e.home, "group.gitconfig")
	write(t, group, includeLines(d.includeValue))
	write(t, d.includeTarget, "[includeIf \"gitdir:/never/\"]\n path = "+gitQuote(group)+"\n")
	info = e.service.gitInventory("", ledger{})
	if !info.Include.Present || !info.Include.Conditional || info.Include.Active != nil {
		t.Fatalf("conditional ancestry lost: %+v", info.Include)
	}
}
