//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

// Bare install restores skills from metadata, but never below a declared entry:
// with its link offline that would turn the boundary into a real directory and
// mask the external tree when it returns.
func TestSkillfollowInstallFromConfigSkipsFollowedEntry(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			demo, _ := lockRemote(t, sb)
			repo := strings.TrimSuffix(demo, "//skills/demo")
			other := repo + "//skills/other"
			source, projectRoot := sb.SourcePath, ""
			if project {
				projectRoot = sb.SetupProjectDir("claude")
				source = filepath.Join(projectRoot, ".skillshare", "skills")
				sb.WriteProjectConfig(projectRoot, "targets:\n  - claude\nskills:\n"+
					"  - name: demo\n    source: "+demo+"\n    group: group\n"+
					"  - name: _repo\n    source: "+repo+"\n    group: group\n    tracked: true\n"+
					"  - name: other\n    source: "+other+"\n")
			} else {
				sb.WriteConfig("source: " + source + "\ntargets: {}\n")
				store := install.NewMetadataStore()
				store.Set("group/demo", &install.MetadataEntry{Source: demo})
				store.Set("group/_repo", &install.MetadataEntry{Source: repo, Tracked: true})
				store.Set("other", &install.MetadataEntry{Source: other})
				if err := store.Save(source); err != nil {
					t.Fatal(err)
				}
			}
			sb.WriteFile(filepath.Join(source, ".skillfollow"), "group\n")

			var result *testutil.Result
			if project {
				result = sb.RunCLIInDir(projectRoot, "install", "-p", "--skip-audit")
			} else {
				result = sb.RunCLI("install", "-g", "--skip-audit")
			}
			result.AssertSuccess(t)
			if _, err := os.Lstat(filepath.Join(source, "group")); !os.IsNotExist(err) {
				t.Errorf("install created the declared entry: %v\n%s", err, result.Output())
			}
			if _, err := os.Stat(filepath.Join(source, "other", "SKILL.md")); err != nil {
				t.Errorf("skill outside the declaration was not installed: %v\n%s", err, result.Output())
			}
		})
	}
}

// install --into below a declared entry is refused before any directory is
// created, including while the entry's link is offline.
func TestSkillfollowInstallIntoDeclaredEntryRefused(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	skill := filepath.Join(sb.Root, "local", "demo")
	sb.WriteFile(filepath.Join(skill, "SKILL.md"), "---\nname: demo\ndescription: Demo\n---\n# Demo\n")
	sb.WriteFile(filepath.Join(sb.SourcePath, ".skillfollow"), "group\n")

	result := sb.RunCLI("install", skill, "--into", "group/sub", "--skip-audit")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "is a link; edit its target directly")
	if _, err := os.Lstat(filepath.Join(sb.SourcePath, "group")); !os.IsNotExist(err) {
		t.Errorf("install created the declared entry: %v", err)
	}
}

// new refuses a skill named like a declared entry whose link is offline, which
// would otherwise create the entry as a real directory.
func TestSkillfollowNewRefusesOfflineEntry(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	sb.WriteFile(filepath.Join(sb.SourcePath, ".skillfollow"), "group\n")

	result := sb.RunCLI("new", "group", "-P", "none", "-g")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "is a link; edit its target directly")
	if _, err := os.Lstat(filepath.Join(sb.SourcePath, "group")); !os.IsNotExist(err) {
		t.Errorf("new created the declared entry: %v", err)
	}
}
