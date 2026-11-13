package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

func TestExtraConfigCallbacks_RepairAndBaseline(t *testing.T) {
	entries := []config.ExtraConfig{{Name: "rules", Remote: "journal", Branch: "old", Targets: []config.ExtraTargetConfig{{Path: "target"}}}, {Name: "other"}}
	load := func() ([]config.ExtraConfig, error) { return append([]config.ExtraConfig(nil), entries...), nil }
	save := saveExtraEntries(&entries)
	callbacks, err := newExtraConfigCallbacks("rules", "journal", "requested", load, save)
	if err != nil {
		t.Fatal(err)
	}
	if err := callbacks.RepairConfig("journal", "journal-branch"); err != nil {
		t.Fatal(err)
	}
	if err := callbacks.RepairConfig("journal", "journal-branch"); err != nil {
		t.Fatal(err)
	}
	if entries[0].Branch != "journal-branch" || entries[0].Targets[0].Path != "target" || entries[1].Name != "other" {
		t.Fatalf("repair: %+v", entries)
	}
	if err := callbacks.Recheck(); err == nil {
		t.Fatal("stale baseline accepted")
	}
	if err := callbacks.RefreshAfterRepair(); err != nil {
		t.Fatal(err)
	}
	if err := callbacks.Recheck(); err != nil {
		t.Fatal(err)
	}
	if err := callbacks.WriteConfig(""); err != nil {
		t.Fatal(err)
	}
	if entries[0].Branch != "requested" {
		t.Fatalf("requested branch: %+v", entries[0])
	}
	if err := callbacks.RevertConfig(); err != nil {
		t.Fatal(err)
	}
	if entries[0].Branch != "journal-branch" || entries[1].Name != "other" {
		t.Fatalf("revert: %+v", entries)
	}
}

func TestExtraConfigCallbacks_RefuseConflictingRepair(t *testing.T) {
	for _, entry := range []config.ExtraConfig{{Name: "rules"}, {Name: "rules", Remote: "different"}, {Name: "Rules", Remote: "journal"}} {
		entries := []config.ExtraConfig{entry}
		load := func() ([]config.ExtraConfig, error) { return append([]config.ExtraConfig(nil), entries...), nil }
		saved := false
		save := func(update extraConfigUpdate) error {
			_, err := update(entries)
			if err != nil {
				return err
			}
			saved = true
			return errors.New("unexpected save")
		}
		callbacks, err := newExtraConfigCallbacks("rules", "requested", "", load, save)
		if err != nil {
			t.Fatal(err)
		}
		if err := callbacks.RepairConfig("journal", "main"); !errors.Is(err, install.ErrExtraConfigNotWritten) || saved {
			t.Fatalf("conflicting entry accepted: %+v %v", entry, err)
		}
	}
}

func TestExtraConfigCallbacks_RecheckDetectsConcurrentEdit(t *testing.T) {
	entries := []config.ExtraConfig{{Name: "rules", Remote: "journal"}}
	load := func() ([]config.ExtraConfig, error) { return append([]config.ExtraConfig(nil), entries...), nil }
	callbacks, err := newExtraConfigCallbacks("rules", "journal", "", load, func(extraConfigUpdate) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	entries[0].Targets = []config.ExtraTargetConfig{{Path: "new"}}
	if err := callbacks.Recheck(); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("recheck: %v", err)
	}
}

func TestExtraConfigCallbacks_RevertRefusesChangedEntry(t *testing.T) {
	entries := []config.ExtraConfig{{Name: "rules", Remote: "journal"}}
	load := func() ([]config.ExtraConfig, error) { return append([]config.ExtraConfig(nil), entries...), nil }
	save := saveExtraEntries(&entries)
	callbacks, err := newExtraConfigCallbacks("rules", "journal", "branch", load, save)
	if err != nil {
		t.Fatal(err)
	}
	if err := callbacks.RevertConfig(); err != nil {
		t.Fatal(err)
	}
	if err := callbacks.WriteConfig(""); err != nil {
		t.Fatal(err)
	}
	entries[0].Targets = []config.ExtraTargetConfig{{Path: "concurrent"}}
	if err := callbacks.RevertConfig(); err == nil {
		t.Fatal("changed own entry was overwritten")
	}
	if entries[0].Targets[0].Path != "concurrent" {
		t.Fatal("concurrent target lost")
	}
}

func TestExtraConfigCallbacks_RevertRemovesNewEntry(t *testing.T) {
	entries := []config.ExtraConfig{{Name: "other"}}
	load := func() ([]config.ExtraConfig, error) { return append([]config.ExtraConfig(nil), entries...), nil }
	save := saveExtraEntries(&entries)
	callbacks, err := newExtraConfigCallbacks("rules", "journal", "", load, save)
	if err != nil {
		t.Fatal(err)
	}
	if err := callbacks.WriteConfig(""); err != nil {
		t.Fatal(err)
	}
	if err := callbacks.RevertConfig(); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "other" {
		t.Fatalf("revert: %+v", entries)
	}
}

func TestExtraConfigCallbacks_RevertKeepsUnrelatedChanges(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "existing"}[existing], func(t *testing.T) {
			entries := []config.ExtraConfig{{Name: "other"}}
			if existing {
				entries = append(entries, config.ExtraConfig{Name: "rules", Remote: "journal", Branch: "before", Targets: []config.ExtraTargetConfig{{Path: "target", Include: []string{"*.md"}, Exclude: []string{"private.md"}}}})
			}
			load := func() ([]config.ExtraConfig, error) {
				out := make([]config.ExtraConfig, len(entries))
				for i := range entries {
					out[i] = *cloneExtraConfig(&entries[i])
				}
				return out, nil
			}
			save := saveExtraEntries(&entries)
			c, err := newExtraConfigCallbacks("rules", "journal", "after", load, save)
			if err != nil {
				t.Fatal(err)
			}
			if err := c.WriteConfig(""); err != nil {
				t.Fatal(err)
			}
			entries[0].Targets = []config.ExtraTargetConfig{{Path: "concurrent"}}
			entries = append(entries, config.ExtraConfig{Name: "added"})
			if err := c.RevertConfig(); err != nil {
				t.Fatal(err)
			}
			if entries[0].Targets[0].Path != "concurrent" || entries[len(entries)-1].Name != "added" {
				t.Fatalf("unrelated save lost: %+v", entries)
			}
			i, err := extraConfigAt(entries, "rules")
			if err != nil {
				t.Fatal(err)
			}
			if !existing && i >= 0 {
				t.Fatal("new entry retained")
			}
			if existing && (i < 0 || entries[i].Branch != "before" || entries[i].Targets[0].Include[0] != "*.md" || entries[i].Targets[0].Exclude[0] != "private.md") {
				t.Fatalf("previous entry lost: %+v", entries)
			}
		})
	}
}

func TestExtraConfigCallbacks_RevertNoops(t *testing.T) {
	for _, state := range []string{"not saved", "save failed", "removed"} {
		t.Run(state, func(t *testing.T) {
			entries := []config.ExtraConfig{{Name: "other"}}
			saves := 0
			c, err := newExtraConfigCallbacks("rules", "journal", "", func() ([]config.ExtraConfig, error) { return append([]config.ExtraConfig(nil), entries...), nil }, func(update extraConfigUpdate) error {
				next, err := update(entries)
				if err != nil || next == nil {
					return err
				}
				saves++
				if state == "save failed" {
					return errors.New("synthetic save failure")
				}
				entries = next
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if state != "not saved" {
				err = c.WriteConfig("")
				if state == "save failed" && err == nil {
					t.Fatal("save unexpectedly succeeded")
				}
			}
			if state == "removed" {
				entries = entries[:1]
			}
			before := saves
			if err := c.RevertConfig(); err != nil || saves != before {
				t.Fatalf("revert wrote or failed: saves=%d err=%v", saves, err)
			}
		})
	}
}

func TestExtraConfigCallbacks_ChangesOnlyOwnOriginAtFreshSave(t *testing.T) {
	entries := []config.ExtraConfig{{Name: "rules", Remote: "journal", Targets: []config.ExtraTargetConfig{{Path: "before"}}}, {Name: "other"}}
	load := func() ([]config.ExtraConfig, error) { return append([]config.ExtraConfig(nil), entries...), nil }
	save := func(update extraConfigUpdate) error {
		// A supported fresh save observes an intervening target and another entry.
		entries[0].Targets = append(entries[0].Targets, config.ExtraTargetConfig{Path: "concurrent"})
		entries[1].Targets = []config.ExtraTargetConfig{{Path: "other-target"}}
		next, err := update(entries)
		if err == nil {
			entries = next
		}
		return err
	}
	c, err := newExtraConfigCallbacks("rules", "journal", "main", load, save)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.WriteConfig(""); err != nil {
		t.Fatal(err)
	}
	if entries[0].Branch != "main" || len(entries[0].Targets) != 2 || entries[1].Targets[0].Path != "other-target" {
		t.Fatalf("fresh save overwritten: %+v", entries)
	}
}

func saveExtraEntries(entries *[]config.ExtraConfig) func(extraConfigUpdate) error {
	return func(update extraConfigUpdate) error {
		next, err := update(*entries)
		if err == nil && next != nil {
			*entries = next
		}
		return err
	}
}

func TestExtraConfigCallbacks_OmittedBranchInheritsRefreshedBaseline(t *testing.T) {
	entries := []config.ExtraConfig{{Name: "rules", Remote: "journal", Branch: "main"}}
	load := func() ([]config.ExtraConfig, error) { return entries, nil }
	callbacks, err := newExtraConfigCallbacks("rules", "journal", "", load, saveExtraEntries(&entries))
	if err != nil {
		t.Fatal(err)
	}
	if callbacks.PinBranch("") != "main" {
		t.Fatal("initial branch not read")
	}
	if err := callbacks.RepairConfig("journal", "release"); err != nil {
		t.Fatal(err)
	}
	if err := callbacks.RefreshAfterRepair(); err != nil {
		t.Fatal(err)
	}
	if callbacks.PinBranch("") != "release" {
		t.Fatal("repair branch baseline not refreshed")
	}
	if err := callbacks.WriteConfig(""); err != nil {
		t.Fatal(err)
	}
	if entries[0].Branch != "release" {
		t.Fatal("omitted branch erased repaired branch")
	}
}

func TestExtraConfigCallbacks_URLBranchOnlyDefaultsNewDeclaration(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing=%t", existing), func(t *testing.T) {
			var entries []config.ExtraConfig
			if existing {
				entries = []config.ExtraConfig{{Name: "rules", Remote: "journal"}}
			}
			load := func() ([]config.ExtraConfig, error) { return entries, nil }
			callbacks, err := newExtraConfigCallbacks("rules", "journal", "", load, saveExtraEntries(&entries))
			if err != nil {
				t.Fatal(err)
			}
			want := "url-release"
			if existing {
				want = ""
			}
			if got := callbacks.PinBranch("url-release"); got != want {
				t.Fatalf("followed branch %q want %q", got, want)
			}
			if err := callbacks.WriteConfig("url-release"); err != nil || entries[0].Branch != want {
				t.Fatalf("URL default changed declaration: %+v %v", entries, err)
			}
			if err := callbacks.RepairConfig("journal", "repaired-main"); err != nil {
				t.Fatal(err)
			}
			if err := callbacks.RefreshAfterRepair(); err != nil {
				t.Fatal(err)
			}
			if got := callbacks.PinBranch("url-release"); got != "repaired-main" {
				t.Fatalf("URL default bypassed repair refresh: %q", got)
			}
			if err := callbacks.WriteConfig("url-release"); err != nil || entries[0].Branch != "repaired-main" {
				t.Fatalf("repaired declaration replaced: %+v %v", entries, err)
			}
		})
	}
}
