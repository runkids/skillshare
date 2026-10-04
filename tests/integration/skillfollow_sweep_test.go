//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// Switching a target with local files to symlink mode merges them into the
// source; a folder named like an offline declared entry must not be merged,
// which would create the entry as a real directory.
func TestSkillfollowSymlinkSyncRefusesDeclaredEntry(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	for _, skill := range []string{"team", "keep"} {
		sb.WriteFile(filepath.Join(targetPath, skill, "SKILL.md"), "---\nname: "+skill+"\n---\n# "+skill+"\n")
	}
	sb.WriteConfig("source: " + sb.SourcePath + "\nmode: symlink\ntargets:\n  claude:\n    path: " + targetPath + "\n")
	sb.WriteFile(filepath.Join(sb.SourcePath, ".skillfollow"), "team\n")

	result := sb.RunCLI("sync", "-g")
	result.AssertAnyOutputContains(t, filepath.Join(sb.SourcePath, "team")+" is a link; edit its target directly")
	if _, err := os.Lstat(filepath.Join(sb.SourcePath, "team")); !os.IsNotExist(err) {
		t.Errorf("sync created the declared entry: %v\n%s", err, result.Output())
	}
	if _, err := os.Stat(filepath.Join(targetPath, "team", "SKILL.md")); err != nil {
		t.Errorf("the target's files were removed: %v", err)
	}
}

// init never creates an offline declared entry: neither by importing a tool's
// skill with that name nor by the built-in skill's offline fallback.
func TestSkillfollowInitRefusesDeclaredEntry(t *testing.T) {
	t.Run("import", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()
		os.Remove(sb.ConfigPath)
		claudeSkills := filepath.Join(sb.Home, ".claude", "skills")
		for _, skill := range []string{"team", "keep"} {
			sb.WriteFile(filepath.Join(claudeSkills, skill, "SKILL.md"), "# "+skill)
		}
		sb.WriteFile(filepath.Join(sb.SourcePath, ".skillfollow"), "team\n")

		result := sb.RunCLI("init", "--copy-from", "claude", "--no-targets", "--no-git", "--no-skill")
		result.AssertSuccess(t)
		result.AssertOutputContains(t, filepath.Join(sb.SourcePath, "team")+" is a link; edit its target directly")
		if _, err := os.Lstat(filepath.Join(sb.SourcePath, "team")); !os.IsNotExist(err) {
			t.Errorf("init created the declared entry: %v\n%s", err, result.Output())
		}
		if !sb.FileExists(filepath.Join(sb.SourcePath, "keep", "SKILL.md")) {
			t.Errorf("skill outside the declaration was not imported\n%s", result.Output())
		}
	})
	t.Run("builtin-skill", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()
		os.Remove(sb.ConfigPath)
		sb.WriteFile(filepath.Join(sb.SourcePath, ".skillfollow"), "skillshare\n")

		result := sb.RunCLI("init", "--no-copy", "--no-targets", "--no-git", "--skill")
		result.AssertSuccess(t)
		result.AssertOutputContains(t, filepath.Join(sb.SourcePath, "skillshare")+" is a link; edit its target directly")
		if _, err := os.Lstat(filepath.Join(sb.SourcePath, "skillshare")); !os.IsNotExist(err) {
			t.Errorf("init created the declared entry: %v\n%s", err, result.Output())
		}
	})
	// An unreadable declaration may name skillshare: neither the download nor
	// the offline fallback creates it. Without a declaration the fallback stays.
	t.Run("builtin-skill-unreadable-declaration", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()
		os.Remove(sb.ConfigPath)
		if err := os.MkdirAll(filepath.Join(sb.SourcePath, ".skillfollow"), 0755); err != nil {
			t.Fatal(err)
		}

		result := sb.RunCLIEnv(offlineEnv(), "init", "--no-copy", "--no-targets", "--no-git", "--skill")
		result.AssertSuccess(t)
		result.AssertOutputContains(t, "read skillfollow declaration "+filepath.Join(sb.SourcePath, ".skillfollow"))
		if _, err := os.Lstat(filepath.Join(sb.SourcePath, "skillshare")); !os.IsNotExist(err) {
			t.Errorf("init created skillshare under an unreadable declaration: %v\n%s", err, result.Output())
		}
	})
	t.Run("builtin-skill-offline-fallback", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()
		os.Remove(sb.ConfigPath)

		result := sb.RunCLIEnv(offlineEnv(), "init", "--no-copy", "--no-targets", "--no-git", "--skill")
		result.AssertSuccess(t)
		data, err := os.ReadFile(filepath.Join(sb.SourcePath, "skillshare", "SKILL.md"))
		if err != nil || !strings.Contains(string(data), "description: Manage and sync skills across AI CLI tools") {
			t.Errorf("offline init did not write the fallback skill: %v\n%s", err, result.Output())
		}
	})
}

// offlineEnv points every download at a closed port, so the built-in skill
// download fails and init takes its offline path.
func offlineEnv() map[string]string {
	proxy := "http://127.0.0.1:1"
	return map[string]string{
		"HTTPS_PROXY": proxy, "https_proxy": proxy, "HTTP_PROXY": proxy, "http_proxy": proxy,
		"NO_PROXY": "", "no_proxy": "", "GIT_TERMINAL_PROMPT": "0",
	}
}
