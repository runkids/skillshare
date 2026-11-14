package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

func TestFollowedProjectLinkOwnership(t *testing.T) {
	source := linkedRepoSource(t)
	project := filepath.Dir(source)
	targetPath := filepath.Join(project, "target")
	target := config.TargetConfig{Path: targetPath}
	skills, err := DiscoverSourceSkills(source, sourcewalk.Options{Follow: sourcewalk.NewFollow(source, nil)})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		result, err := SyncTargetMergeWithSkills("test", target, skills, source, false, false, project)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Updated) != 0 || len(result.Linked) != 1 {
			t.Fatalf("sync %d: %+v", i, result)
		}
	}
	link := filepath.Join(targetPath, skills[0].FlatName)
	dest, err := utils.ResolveLinkTarget(link)
	if err != nil {
		t.Fatal(err)
	}
	if dest != skills[0].SourcePath {
		t.Fatalf("link points to %s, want logical %s", dest, skills[0].SourcePath)
	}
	raw, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if canCreateRelativeLink() && filepath.IsAbs(raw) {
		t.Fatalf("expected relative project link: %s", raw)
	}
	_, linked, local := CheckStatusMerge(targetPath, source)
	if linked != 1 || local != 0 {
		t.Fatalf("linked=%d local=%d", linked, local)
	}
	result, err := PruneOrphanLinksWithSkills(PruneOptions{TargetPath: targetPath, SourcePath: source, Skills: skills, TargetName: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 0 {
		t.Fatalf("pruned valid link: %+v", result)
	}
	detached, err := DetachSkills(map[string]config.TargetConfig{"test": target}, "test", source, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(detached.Removed) != 1 {
		t.Fatalf("unlink did not recognize ownership: %+v", detached)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("target link still exists: %v", err)
	}
	if _, err := os.Stat(skills[0].SourcePath); err != nil {
		t.Fatalf("unlink touched source: %v", err)
	}
}

func TestFollowedSkillRootCopy(t *testing.T) {
	source := linkedRepoSource(t)
	root := filepath.Join(source, "_dev-skills")
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# root"), 0644); err != nil {
		t.Fatal(err)
	}
	skills, err := DiscoverSourceSkills(source, sourcewalk.Options{Follow: sourcewalk.NewFollow(source, nil)})
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Fatalf("skills=%+v", skills)
	}
	target := config.TargetConfig{Path: filepath.Join(t.TempDir(), "target")}
	if _, err := SyncTargetMergeWithSkills("test", target, skills, source, false, false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncTargetCopyWithSkills("test", target, skills, source, false, false, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target.Path, "_dev-skills", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestPullForcePreservesFollowedLink(t *testing.T) {
	source := linkedRepoSource(t)
	link := filepath.Join(source, "_dev-skills")
	original, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	local := t.TempDir()
	if err := os.WriteFile(filepath.Join(local, "SKILL.md"), []byte("replacement"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := PullSkills([]LocalSkillInfo{{Name: "_dev-skills", Path: local}}, source, PullOptions{Force: true, Walk: sourcewalk.Options{Follow: sourcewalk.NewFollow(source, nil)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skipped) != 1 || len(result.Pulled) != 0 || result.Warnings["_dev-skills"] == "" {
		t.Fatalf("result=%+v", result)
	}
	got, err := os.Readlink(link)
	if err != nil || got != original {
		t.Fatalf("link changed: %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(link, "foo", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestFollowedLinkReformatKeepsIdentity(t *testing.T) {
	source := linkedRepoSource(t)
	targetPath := filepath.Join(filepath.Dir(source), "target")
	target := config.TargetConfig{Path: targetPath}
	skills, err := DiscoverSourceSkills(source, sourcewalk.Options{Follow: sourcewalk.NewFollow(source, nil)})
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range []string{"", filepath.Dir(source), ""} {
		if _, err := SyncTargetMergeWithSkills("test", target, skills, source, false, false, project); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(targetPath, skills[0].FlatName)
		raw, err := os.Readlink(link)
		if err != nil {
			t.Fatal(err)
		}
		if project == "" && raw != skills[0].SourcePath {
			t.Errorf("global link changed: %q", raw)
		}
		if project != "" && canCreateRelativeLink() && filepath.IsAbs(raw) {
			t.Errorf("project link not relative: %q", raw)
		}
		resolved, err := utils.ResolveLinkTarget(link)
		if err != nil || resolved != skills[0].SourcePath {
			t.Fatalf("identity changed: %q %v", resolved, err)
		}
	}
}

func TestFollowedLinkCanonicalRootAndParent(t *testing.T) {
	source := linkedRepoSource(t)
	aliases := t.TempDir()
	sourceAlias := filepath.Join(aliases, "source")
	target := t.TempDir()
	targetAlias := filepath.Join(aliases, "target")
	if err := os.Symlink(source, sourceAlias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, targetAlias); err != nil {
		t.Fatal(err)
	}
	logical := filepath.Join(sourceAlias, "_dev-skills", "foo")
	link := filepath.Join(targetAlias, "foo")
	if err := createLink(link, logical, true, sourceAlias); err != nil {
		t.Fatal(err)
	}
	raw, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(source, "_dev-skills", "foo")
	if got := resolveReadlink(raw, filepath.Join(target, "foo")); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(link, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestProfileAwareFollowedLinkOwnership(t *testing.T) {
	for _, mode := range []string{"merge", "copy"} {
		for _, project := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/project=%v", mode, project), func(t *testing.T) {
				source := linkedRepoSource(t)
				alias := filepath.Join(t.TempDir(), "source")
				if err := os.Symlink(source, alias); err != nil {
					t.Fatal(err)
				}
				skills, err := DiscoverSourceSkills(alias, sourcewalk.Options{Follow: sourcewalk.NewFollow(alias, nil)})
				if err != nil || len(skills) != 1 {
					t.Fatalf("discover: %v, %v", skills, err)
				}
				target := SkillTarget{Name: "test", Target: config.TargetConfig{Path: filepath.Join(filepath.Dir(source), "target")}, Mode: "merge"}
				opts := SkillRunOptions{Source: alias, ProfileAware: true}
				if project {
					opts.ProjectRoot = filepath.Dir(source)
				}
				before := snapshotDetachTree(t, source)
				if r := SyncSkillTarget(target, skills, opts); r.Err != nil || len(r.Linked) != 1 {
					t.Fatalf("seed: %+v", r)
				}
				entry := filepath.Join(target.Target.Path, skills[0].FlatName)
				manifest, err := ReadManifest(target.Target.Path)
				if err != nil {
					t.Fatal(err)
				}
				if got, allowed := PreviewEntry(skills[0].FlatName, target.Target.Path, alias, skills, manifest, nil); !allowed || got.Class != EntryManagedLink {
					t.Fatalf("followed preview ownership: %+v, %v", got, allowed)
				}
				target.Mode = mode
				for _, dry := range []bool{true, false} {
					opts.DryRun = dry
					if r := SyncSkillTarget(target, skills, opts); r.Err != nil || len(r.Blocked)+len(r.KeptModified) != 0 || len(r.Linked)+len(r.Updated) != 1 {
						t.Fatalf("retain dry=%v: %+v", dry, r)
					}
				}
				// Return to a managed link, then exclude it using either prune owner.
				target.Mode = "merge"
				if r := SyncSkillTarget(target, skills, opts); r.Err != nil || len(r.Blocked) != 0 {
					t.Fatalf("restore link: %+v", r)
				}
				target.Mode = mode
				target.Target.Exclude = []string{"*"}
				if r := SyncSkillTarget(target, skills, opts); r.Err != nil || r.PruneErr != nil || len(r.Pruned) != 1 {
					t.Fatalf("exclude followed link: %+v", r)
				}
				if _, err := os.Lstat(entry); !os.IsNotExist(err) {
					t.Fatalf("excluded link remains: %v", err)
				}
				if !reflect.DeepEqual(before, snapshotDetachTree(t, source)) {
					t.Fatal("sync changed followed source link")
				}
				if _, err := os.Stat(filepath.Join(skills[0].SourcePath, "SKILL.md")); err != nil {
					t.Fatalf("sync changed external checkout: %v", err)
				}
			})
		}
	}
}
