package main

import (
	"testing"

	"skillshare/internal/install"
)

func TestStoreToSkillEntryDTOs_FullPathKeyWithGroup(t *testing.T) {
	store := install.NewMetadataStore()
	store.Set("team/_demo", &install.MetadataEntry{
		Source:  "file:///tmp/demo",
		Tracked: true,
		Group:   "team",
	})

	dtos := storeToSkillEntryDTOs(store)
	if len(dtos) != 1 {
		t.Fatalf("expected 1 dto, got %d", len(dtos))
	}

	got := dtos[0]
	if got.Name != "_demo" {
		t.Fatalf("Name = %q, want %q", got.Name, "_demo")
	}
	if got.Group != "team" {
		t.Fatalf("Group = %q, want %q", got.Group, "team")
	}
	if got.FullName() != "team/_demo" {
		t.Fatalf("FullName() = %q, want %q", got.FullName(), "team/_demo")
	}
}

func TestStoreToSkillEntryDTOs_LegacyBasenameKeyWithGroup(t *testing.T) {
	store := install.NewMetadataStore()
	store.Set("_demo", &install.MetadataEntry{
		Source:  "file:///tmp/demo",
		Tracked: true,
		Group:   "team",
	})

	dtos := storeToSkillEntryDTOs(store)
	if len(dtos) != 1 {
		t.Fatalf("expected 1 dto, got %d", len(dtos))
	}

	got := dtos[0]
	if got.Name != "_demo" {
		t.Fatalf("Name = %q, want %q", got.Name, "_demo")
	}
	if got.Group != "team" {
		t.Fatalf("Group = %q, want %q", got.Group, "team")
	}
	if got.FullName() != "team/_demo" {
		t.Fatalf("FullName() = %q, want %q", got.FullName(), "team/_demo")
	}
}
