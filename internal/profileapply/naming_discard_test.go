package profileapply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	ssync "skillshare/internal/sync"
)

func TestDiscardFinishesTargetNamingMigration(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, syncOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("project=%t/sync=%t", project, syncOnly), func(t *testing.T) {
				f := newFixture(t, project, "copy")
				const original = "---\nname: alpha\n---\n# Nested source\n"
				write(t, filepath.Join(f.source, "group", "alpha", "SKILL.md"), original)
				text := strings.Replace(read(t, f.path), "include: [alpha]", "include: [group__alpha]", 1)
				write(t, f.path, "target_naming: flat\n"+text)
				if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err != nil {
					t.Fatal(err)
				}
				legacy := filepath.Join(f.target, "group__alpha")
				const edited = "# Authored deployment edit\n"
				write(t, filepath.Join(legacy, "SKILL.md"), edited)
				write(t, f.path, strings.Replace(read(t, f.path), "target_naming: flat", "target_naming: standard", 1))
				before := tree(t, f.target)
				preview, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a", DryRun: true, DiscardEdits: true})
				if err != nil || len(preview.Blocked) != 1 || preview.Blocked[0].Entry != "group__alpha" || len(preview.Snapshot) != 0 {
					t.Fatalf("preview must report the original edited identity: %+v, %v", preview, err)
				}
				if !reflect.DeepEqual(before, tree(t, f.target)) {
					t.Fatal("preview changed the target")
				}
				var out Result
				if syncOnly {
					out, err = WithSkillSync(context.Background(), SyncRequest{Scope: f.scope, DiscardEdits: true})
				} else {
					out, err = Apply(context.Background(), Request{Scope: f.scope, Name: "a", DiscardEdits: true})
				}
				if err != nil || !out.Complete || out.Selection.Pending || len(out.Snapshot) != 1 {
					t.Fatalf("discard must finish naming migration: %+v, %v", out, err)
				}
				if _, err := os.Lstat(legacy); !os.IsNotExist(err) {
					t.Fatalf("legacy name remains after completed migration: %v", err)
				}
				if got := read(t, filepath.Join(f.target, "alpha", "SKILL.md")); got != original {
					t.Fatalf("desired name was not refreshed: %q", got)
				}
				if got := read(t, filepath.Join(out.Snapshot[0].Path, "group__alpha", "SKILL.md")); got != edited {
					t.Fatal("snapshot lost the edited legacy copy")
				}
				manifest, err := ssync.ReadManifest(f.target)
				if err != nil || manifest.Naming["alpha"] != "standard" || manifest.Managed["group__alpha"] != "" {
					t.Fatalf("manifest did not migrate: %+v, %v", manifest, err)
				}
			})
		}
	}
}

func TestDiscardNamingMigrationRejectsStaleSnapshot(t *testing.T) {
	for _, project := range []bool{false, true} {
		f := newFixture(t, project, "copy")
		write(t, filepath.Join(f.source, "group", "alpha", "SKILL.md"), "---\nname: alpha\n---\n# Source\n")
		text := strings.Replace(read(t, f.path), "include: [alpha]", "include: [group__alpha]", 1)
		write(t, f.path, "target_naming: flat\n"+text)
		if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err != nil {
			t.Fatal(err)
		}
		legacy := filepath.Join(f.target, "group__alpha", "SKILL.md")
		write(t, legacy, "# Snapshotted edit\n")
		write(t, f.path, strings.Replace(read(t, f.path), "target_naming: flat", "target_naming: standard", 1))
		out, err := Apply(context.Background(), Request{
			Scope: f.scope, Name: "a", DiscardEdits: true,
			afterSnapshot: func() { write(t, legacy, "# New edit after snapshot\n") },
		})
		if err == nil || out.Complete || len(out.Snapshot) != 1 {
			t.Fatalf("stale discard grant must leave migration incomplete: %+v, %v", out, err)
		}
		if read(t, legacy) != "# New edit after snapshot\n" {
			t.Fatal("stale grant discarded a later edit")
		}
		if _, err := os.Lstat(filepath.Join(f.target, "alpha")); !os.IsNotExist(err) {
			t.Fatalf("stale grant migrated the modified legacy entry: %v", err)
		}
	}
}
