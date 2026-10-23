//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"skillshare/internal/testutil"
	"skillshare/internal/utils"
)

// linkSandbox holds a checkout with one skill outside the source.
func linkSandbox(t *testing.T) (*testutil.Sandbox, string) {
	t.Helper()
	sb := testutil.NewSandbox(t)
	checkout := filepath.Join(sb.Root, "code", "dev-skills")
	sb.WriteFile(filepath.Join(checkout, "foo", "SKILL.md"), "---\nname: foo\n---\n# foo")
	return sb, checkout
}

func TestLink_EnableThenListShowsLinkedSkills(t *testing.T) {
	sb, checkout := linkSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("link", checkout, "--enable")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Linked _dev-skills")
	if !utils.IsSymlinkOrJunction(filepath.Join(sb.SourcePath, "_dev-skills")) {
		t.Fatal("link not created")
	}
	if got := sb.ReadFile(sb.ConfigPath); !contains(got, "follow_source_links: true") {
		t.Fatalf("config not enabled:\n%s", got)
	}
	sb.RunCLI("list", "--json").AssertOutputContains(t, `"relPath": "_dev-skills/foo"`)
}

func TestLink_EnableFailureRollsBackLink(t *testing.T) {
	sb, checkout := linkSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	if err := os.Chmod(sb.ConfigPath, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(sb.ConfigPath, 0644) })
	if f, err := os.OpenFile(sb.ConfigPath, os.O_WRONLY, 0); err == nil {
		f.Close()
		t.Skip("requires file write permissions to be enforced")
	}

	result := sb.RunCLI("link", checkout, "--enable")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "failed to enable follow_source_links")
	if _, err := os.Lstat(filepath.Join(sb.SourcePath, "_dev-skills")); !os.IsNotExist(err) {
		t.Fatalf("link left behind after enable failure: %v", err)
	}
	if !sb.FileExists(filepath.Join(checkout, "foo", "SKILL.md")) {
		t.Fatal("rollback touched the target")
	}
}

func TestLink_WithoutEnablePrintsDoctorHint(t *testing.T) {
	sb, checkout := linkSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("link", checkout, "--name", "_mine")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "_mine: not followed by discovery; its contents are invisible to skillshare. Set follow_source_links: true to follow it")
	if contains(sb.ReadFile(sb.ConfigPath), "follow_source_links") {
		t.Fatal("config changed without --enable")
	}
}

func TestLink_RefusesTargetInsideSource(t *testing.T) {
	sb, _ := linkSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	sb.CreateSkill("local", map[string]string{"SKILL.md": "---\nname: local\n---\n# local"})

	result := sb.RunCLI("link", filepath.Join(sb.SourcePath, "local"))
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "target is inside the source")
	result = sb.RunCLI("link", filepath.Dir(sb.SourcePath), "--name", "_up")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "target is the source or a parent of it")
	if sb.FileExists(filepath.Join(sb.SourcePath, "_local")) || sb.FileExists(filepath.Join(sb.SourcePath, "_up")) {
		t.Fatal("refused link was created")
	}
}

func TestLink_RefusesTargetOverlappingSyncTarget(t *testing.T) {
	sb, _ := linkSandbox(t)
	defer sb.Cleanup()
	target := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + target + "\n")

	result := sb.RunCLI("link", filepath.Dir(target), "--name", "_claude")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "target overlaps sync target")
	if sb.FileExists(filepath.Join(sb.SourcePath, "_claude")) {
		t.Fatal("refused link was created")
	}
}

func TestLink_WarnsWhenTargetIsNotCheckout(t *testing.T) {
	sb, checkout := linkSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\nfollow_source_links: true\ntargets: {}\n")

	result := sb.RunCLI("link", checkout)
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "target is not a git checkout")

	if err := os.Mkdir(filepath.Join(checkout, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	result = sb.RunCLI("link", checkout, "--name", "_again")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "not a git checkout")
}

func TestUnlink_OptionTerminator(t *testing.T) {
	for _, name := range []string{"-local", "-g", "-p"} {
		t.Run(name, func(t *testing.T) {
			sb, checkout := linkSandbox(t)
			defer sb.Cleanup()
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			sb.RunCLIInDir(sb.Root, "link", checkout).AssertSuccess(t)
			if err := os.Rename(filepath.Join(sb.SourcePath, "_dev-skills"), filepath.Join(sb.SourcePath, name)); err != nil {
				t.Fatal(err)
			}
			sb.RunCLIInDir(sb.Root, "unlink", "--global", "--", name).AssertSuccess(t)
			if _, err := os.Lstat(filepath.Join(sb.SourcePath, name)); !os.IsNotExist(err) {
				t.Fatalf("link still present: %v", err)
			}
			if !sb.FileExists(filepath.Join(checkout, "foo", "SKILL.md")) {
				t.Fatal("unlink touched the target")
			}
		})
	}
}

func TestLink_HelpWithoutConfig(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	os.Remove(sb.ConfigPath)
	for _, args := range [][]string{{"link", "-h"}, {"link", "--help", "-p"}, {"unlink", "--help"}} {
		result := sb.RunCLIInDir(sb.Root, args...)
		result.AssertSuccess(t)
		result.AssertOutputContains(t, "Usage")
	}
	if _, err := os.Stat(sb.ConfigPath); !os.IsNotExist(err) {
		t.Fatalf("help created a config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sb.Root, ".skillshare")); !os.IsNotExist(err) {
		t.Fatalf("help created a project config: %v", err)
	}
}

func TestLink_NameValueLooksLikeModeFlag(t *testing.T) {
	sb, checkout := linkSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	sb.RunCLI("link", checkout, "--name", "-g").AssertSuccess(t)
	if !utils.IsSymlinkOrJunction(filepath.Join(sb.SourcePath, "-g")) {
		t.Fatal("link -g not created")
	}
}

func TestLink_OptionTerminatorPath(t *testing.T) {
	sb, _ := linkSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	checkout := filepath.Join(sb.Root, "-checkout")
	sb.WriteFile(filepath.Join(checkout, "foo", "SKILL.md"), "---\nname: foo\n---\n# foo")
	cwd := sb.Root
	if runtime.GOOS != "windows" {
		// Preserve coverage for a path reached through a symlinked working directory.
		cwd = filepath.Join(filepath.Dir(sb.Root), "root-alias")
		sb.CreateSymlink(sb.Root, cwd)
	}
	sb.RunCLIInDir(cwd, "link", "--global", "--", "-checkout").AssertSuccess(t)
	// macOS temp dirs live under /var -> /private/var, so compare canonical paths.
	want, err := filepath.EvalSymlinks(checkout)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := filepath.EvalSymlinks(filepath.Join(sb.SourcePath, "_-checkout")); err != nil || got != want {
		t.Fatalf("link target = %q, %v; want %s", got, err, want)
	}
}

func TestLink_ProjectUsesProjectSourceAndTargets(t *testing.T) {
	sb, checkout := linkSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	source := filepath.Join(projectRoot, ".skillshare", "skills")

	result := sb.RunCLIInDir(projectRoot, "link", filepath.Join(projectRoot, ".claude"), "--name", "_claude", "-p")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "target overlaps sync target")

	sb.RunCLIInDir(projectRoot, "link", checkout, "--enable", "-p").AssertSuccess(t)
	if !utils.IsSymlinkOrJunction(filepath.Join(source, "_dev-skills")) {
		t.Fatal("link not created in the project source")
	}
	if got := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml")); !contains(got, "follow_source_links: true") {
		t.Fatalf("project config not enabled:\n%s", got)
	}
	sb.RunCLIInDir(projectRoot, "unlink", "_dev-skills", "-p").AssertSuccess(t)
	if sb.FileExists(filepath.Join(source, "_dev-skills")) || !sb.FileExists(filepath.Join(checkout, "foo", "SKILL.md")) {
		t.Fatal("project unlink did not remove only the link")
	}
}
