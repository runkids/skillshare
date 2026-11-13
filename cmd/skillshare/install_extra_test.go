package main

import (
	"errors"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestExtraConfigCallbacks_RepairAndBaseline(t *testing.T) {
	entries := []config.ExtraConfig{{Name: "rules", Remote: "journal", Branch: "old", Targets: []config.ExtraTargetConfig{{Path: "target"}}}, {Name: "other"}}
	load := func() ([]config.ExtraConfig, error) { return append([]config.ExtraConfig(nil), entries...), nil }
	save := func(next []config.ExtraConfig) error { entries = next; return nil }
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
	if err := callbacks.WriteConfig(); err != nil {
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
		save := func([]config.ExtraConfig) error { saved = true; return errors.New("unexpected save") }
		callbacks, err := newExtraConfigCallbacks("rules", "requested", "", load, save)
		if err != nil {
			t.Fatal(err)
		}
		if err := callbacks.RepairConfig("journal", "main"); err == nil || saved {
			t.Fatalf("conflicting entry accepted: %+v %v", entry, err)
		}
	}
}

func TestExtraConfigCallbacks_RecheckDetectsConcurrentEdit(t *testing.T) {
	entries := []config.ExtraConfig{{Name: "rules", Remote: "journal"}}
	load := func() ([]config.ExtraConfig, error) { return append([]config.ExtraConfig(nil), entries...), nil }
	callbacks, err := newExtraConfigCallbacks("rules", "journal", "", load, func([]config.ExtraConfig) error { return nil })
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
	save := func(next []config.ExtraConfig) error { entries = next; return nil }
	callbacks, err := newExtraConfigCallbacks("rules", "journal", "branch", load, save)
	if err != nil {
		t.Fatal(err)
	}
	if err := callbacks.RevertConfig(); err != nil {
		t.Fatal(err)
	}
	if err := callbacks.WriteConfig(); err != nil {
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
	save := func(next []config.ExtraConfig) error { entries = next; return nil }
	callbacks, err := newExtraConfigCallbacks("rules", "journal", "", load, save)
	if err != nil {
		t.Fatal(err)
	}
	if err := callbacks.WriteConfig(); err != nil {
		t.Fatal(err)
	}
	if err := callbacks.RevertConfig(); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "other" {
		t.Fatalf("revert: %+v", entries)
	}
}
