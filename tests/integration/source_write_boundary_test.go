//go:build !online

package integration

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// treeSnapshot records every path, link target, and file content below dir.
func treeSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, _ := os.Readlink(p)
			out[rel] = "-> " + target
		case d.IsDir():
			out[rel] = "<dir>"
		default:
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			out[rel] = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func assertTreeUnchanged(t *testing.T, before map[string]string, dir string) {
	t.Helper()
	after := treeSnapshot(t, dir)
	if len(after) != len(before) {
		t.Fatalf("%s changed: %d entries before, %d after", dir, len(before), len(after))
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("%s changed at %s", dir, k)
		}
	}
}

// install --track --force onto an existing tracked repo that is a link must
// not remove the link and clone a real directory in its place: that would
// silently disconnect the external repo.
func TestInstall_TrackForce_RefusesLinkedRepo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	repoURL := setupBareRepoWithCleanContent(t, sb, "linked")
	external := filepath.Join(sb.Root, "external-repo")
	run(t, sb.Root, "git", "clone", repoURL, external)
	link := filepath.Join(sb.SourcePath, "_linked")
	sb.CreateSymlink(external, link)
	before := treeSnapshot(t, external)

	result := sb.RunCLI("install", repoURL, "--track", "--name", "linked", "--force")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "is a link; edit its target directly")

	if !sb.IsSymlink(link) || sb.SymlinkTarget(link) != external {
		t.Fatalf("_linked is no longer a link to %s", external)
	}
	assertTreeUnchanged(t, before, external)
}

// An explicit --into path through an undeclared link in the source is
// refused instead of creating directories in the link's target.
func TestInstall_Into_RefusesPathThroughLink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	localSkill := filepath.Join(sb.Root, "pdf-skill")
	sb.WriteFile(filepath.Join(localSkill, "SKILL.md"), "---\nname: pdf-skill\n---\n# PDF Skill")
	external := filepath.Join(sb.Root, "external-group")
	sb.WriteFile(filepath.Join(external, "README.md"), "external")
	sb.CreateSymlink(external, filepath.Join(sb.SourcePath, "alias"))
	before := treeSnapshot(t, external)

	result := sb.RunCLI("install", localSkill, "--into", "alias/new")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "is a link; edit its target directly")

	assertTreeUnchanged(t, before, external)
}

// Uninstall moves a skill to the trash, a move across the source's edge. A
// skill reached through a link in the source stays where it is.
func TestUninstall_RefusesSkillThroughLink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	external := filepath.Join(sb.Root, "external-group")
	sb.WriteFile(filepath.Join(external, "child", "SKILL.md"), "---\nname: child\n---\n# Child")
	sb.CreateSymlink(external, filepath.Join(sb.SourcePath, "alias"))
	before := treeSnapshot(t, external)

	// --json because the plain single-target output does not print the
	// trash error today.
	result := sb.RunCLI("uninstall", "alias/child", "--force", "--json")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "is a link; edit its target directly")

	assertTreeUnchanged(t, before, external)
}
