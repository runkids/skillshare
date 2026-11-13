//go:build !online

package integration

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/testutil"
)

// Snapshot both presence and bytes; dry runs must not create even empty folders.
func extraFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			files[rel] = "link:" + target
			return err
		}
		if d.IsDir() {
			files[rel] = "dir"
			return nil
		}
		data, err := os.ReadFile(path)
		files[rel] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestInstallExtra_PinRestoreUpdateAndIgnore(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, bump := lockRemote(t, sb)
	project := sb.SetupProjectDir("claude")
	initGitRepo(t, project)
	args := []string{"install", source, "--kind", "extra", "--name", "rules", "--skip-audit", "-p"}
	sb.RunCLIInDir(project, args...).AssertSuccess(t)
	first := readLock(t, project).Extras["rules"]
	other := teammate(t, sb, project)
	head := bump("v2")
	sb.RunCLIInDir(other, args...).AssertSuccess(t)
	if got := readLock(t, other).Extras["rules"]; got != first {
		t.Fatalf("teammate pin = %+v, want %+v", got, first)
	}
	body, err := os.ReadFile(filepath.Join(other, ".skillshare", "extras", "rules", "SKILL.md"))
	if err != nil || !strings.Contains(string(body), "# v1") {
		t.Fatalf("teammate content: %q %v", body, err)
	}
	sb.RunCLIInDir(project, append(args, "--update")...).AssertSuccess(t)
	if got := readLock(t, project).Extras["rules"].Commit; got != head {
		t.Fatalf("updated pin %s, want %s", got, head)
	}
	for _, path := range []string{".skillshare/extras/rules/SKILL.md", ".skillshare/extras/.metadata.json", ".skillshare/extras/.skillshare-staging/probe"} {
		if got := strings.TrimSpace(testutil.RunGit(t, project, "check-ignore", path)); got != path {
			t.Errorf("not ignored: %q", got)
		}
	}
	// An unreachable pin must fail without branch fallback or publication writes.
	lock := readLock(t, other)
	pin := lock.Extras["rules"]
	pin.Commit = strings.Repeat("f", 40)
	lock.Extras["rules"] = pin
	if err := lock.Save(filepath.Join(other, ".skillshare")); err != nil {
		t.Fatal(err)
	}
	before := extraFiles(t, filepath.Join(other, ".skillshare", "extras"))
	result := sb.RunCLIInDir(other, args...)
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "fffffff")
	if after := extraFiles(t, filepath.Join(other, ".skillshare", "extras")); !reflect.DeepEqual(before, after) {
		t.Fatal("unreachable pin changed extras")
	}
}

func TestInstallExtra_BranchAndGlobalPin(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	remote := testutil.SetupBareRemoteRepo(t, sb.Root)
	testutil.SeedRemoteBranch(t, sb.Root, remote, "main", map[string]string{"rules.md": "main"})
	testutil.SeedRemoteBranch(t, sb.Root, remote, "release", map[string]string{"rules.md": "release"})
	source := "file://" + remote
	args := []string{"install", source, "--kind", "extra", "--name", "rules", "--branch", "release", "--skip-audit", "--json", "-g"}
	result := sb.RunCLI(args...)
	result.AssertSuccess(t)
	var out struct{ Branch, Commit string }
	if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(testutil.RunGit(t, filepath.Join(sb.Root, "seed-release"), "rev-parse", "HEAD"))
	if out.Branch != "release" || out.Commit != head {
		t.Fatalf("branch result: %+v", out)
	}
	configBytes, err := os.ReadFile(sb.ConfigPath)
	if err != nil || !strings.Contains(string(configBytes), "branch: release") {
		t.Fatalf("branch config: %s %v", configBytes, err)
	}
	seed := filepath.Join(sb.Root, "seed-release")
	if err := os.WriteFile(filepath.Join(seed, "rules.md"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, seed, "commit", "-am", "change release")
	testutil.RunGit(t, seed, "push", "origin", "HEAD:release")
	result = sb.RunCLI(args...)
	result.AssertSuccess(t)
	var second struct{ Action, Commit string }
	if err := json.Unmarshal([]byte(result.Stdout), &second); err != nil {
		t.Fatal(err)
	}
	if second.Commit != head || second.Action != "reinstalled" {
		t.Fatalf("global pin: %+v", second)
	}
}

func TestInstallExtra_AuditBlocksThenForceRecordsOverride(t *testing.T) {
	for _, projectMode := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[projectMode], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			remote := extraFixture(t, sb)
			repo := strings.TrimPrefix(remote, "file://")
			if err := os.WriteFile(filepath.Join(repo, "rules.md"), []byte("SYNTHETIC_BAD_CONTENT"), 0644); err != nil {
				t.Fatal(err)
			}
			testutil.RunGit(t, repo, "commit", "-am", "synthetic audit content")
			cwd, mode, configPath := sb.Root, "-g", sb.ConfigPath
			root := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
			rulesPath := filepath.Join(filepath.Dir(configPath), "audit-rules.yaml")
			if projectMode {
				cwd = sb.SetupProjectDir("claude")
				mode = "-p"
				configPath = filepath.Join(cwd, ".skillshare", "config.yaml")
				root = filepath.Join(cwd, ".skillshare", "extras")
				rulesPath = filepath.Join(cwd, ".skillshare", "audit-rules.yaml")
			}
			rule := "rules:\n  - id: synthetic-finding\n    severity: CRITICAL\n    pattern: synthetic\n    message: synthetic finding\n    regex: 'SYNTHETIC_BAD_CONTENT'\n"
			if err := os.WriteFile(rulesPath, []byte(rule), 0644); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"install", remote, "--kind", "extra", "--name", "rules", "--json", mode}
			result := sb.RunCLIInDir(cwd, args...)
			result.AssertFailure(t)
			if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatalf("audit created extras: %v", err)
			}
			after, err := os.ReadFile(configPath)
			if err != nil || string(after) != string(before) {
				t.Fatal("audit changed config")
			}
			result = sb.RunCLIInDir(cwd, append(args, "--force")...)
			result.AssertSuccess(t)
			var out struct {
				AuditOverridden bool `json:"audit_overridden"`
			}
			if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil || !out.AuditOverridden {
				t.Fatalf("override result %s %v", result.Stdout, err)
			}
			store, _, err := install.ReadMetadata(root)
			if err != nil || len(store.AuditAccepted["rules"]) == 0 {
				t.Fatalf("acceptance not recorded: %+v %v", store, err)
			}
		})
	}
}

func TestInstallExtra_DryRunNoWrites(t *testing.T) {
	for _, projectMode := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[projectMode], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			// Legacy embedded skills must not be migrated during read-only preparation.
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nskills:\n  - name: existing\n    source: example-org/ai-hub/skills/existing\n")
			remote := extraFixture(t, sb)
			cwd, mode := sb.Root, "-g"
			if projectMode {
				cwd = sb.SetupProjectDir("claude")
				mode = "-p"
			}
			before := extraFiles(t, sb.Root)
			result := sb.RunCLIInDir(cwd, "install", remote, "--kind", "extra", "--name", "rules", "--dry-run", "--json", mode)
			result.AssertSuccess(t)
			var out struct{ Action string }
			if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil || out.Action != "would install" {
				t.Fatalf("dry result %s %v", result.Stdout, err)
			}
			if after := extraFiles(t, sb.Root); !reflect.DeepEqual(before, after) {
				t.Fatal("dry run changed sandbox files or directories")
			}
		})
	}
}

func TestInstallExtra_LocalAndDifferentOriginRefused(t *testing.T) {
	for _, origin := range []string{"", "example-org/other-hub"} {
		t.Run("origin="+origin, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			extra := "  - name: rules\n    targets: []\n"
			if origin != "" {
				extra += "    remote: " + origin + "\n"
			}
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nextras:\n" + extra)
			remote := extraFixture(t, sb)
			before, err := os.ReadFile(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			result := sb.RunCLI("install", remote, "--kind", "extra", "--name", "rules", "--force", "--json", "-g")
			result.AssertFailure(t)
			result.AssertAnyOutputContains(t, "remove")
			after, err := os.ReadFile(sb.ConfigPath)
			if err != nil || string(after) != string(before) {
				t.Fatal("conflict changed config")
			}
		})
	}
}

func TestInstallExtra_GlobalSaveKeepsSymlinkAndLegacySkills(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges vary")
	}
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	target := filepath.Join(sb.Home, "target")
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + target + "\nskills:\n  - name: existing\n    source: example-org/ai-hub/skills/existing\n")
	actual := filepath.Join(sb.Root, "shared-config.yaml")
	if err := os.Rename(sb.ConfigPath, actual); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(actual, sb.ConfigPath); err != nil {
		t.Fatal(err)
	}
	remote := extraFixture(t, sb)
	result := sb.RunCLI("install", remote, "--kind", "extra", "--name", "rules", "--skip-audit", "-g")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "--add-target")
	if info, err := os.Lstat(sb.ConfigPath); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config link lost: %v", err)
	}
	data, err := os.ReadFile(actual)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"name: existing", "source: example-org/ai-hub/skills/existing", target, "remote: " + remote} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("lost %q in config: %s", want, data)
		}
	}
	if _, err := os.Stat(config.RegistryPath(sb.SourcePath)); !os.IsNotExist(err) {
		t.Fatalf("extra logging migrated the skill registry: %v", err)
	}
	entries, err := oplog.Read(sb.ConfigPath, oplog.OpsFile, 0)
	if err != nil || len(entries) == 0 {
		t.Fatalf("oplog: %+v %v", entries, err)
	}
	entry := entries[0]
	if entry.Command != "install" || entry.Args["kind"] != "extra" || entry.Args["mode"] != "global" || entry.Args["source"] != remote {
		t.Fatalf("oplog: %+v", entry)
	}
}

func TestInstallExtra_AddTargetSyncAndSkillLockWriters(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	remote, bump := lockRemote(t, sb)
	project := sb.SetupProjectDir("claude")
	args := []string{"install", remote, "--kind", "extra", "--name", "rules", "--skip-audit", "-p"}
	sb.RunCLIInDir(project, args...).AssertSuccess(t)
	pin := readLock(t, project).Extras["rules"]
	sb.RunCLIInDir(project, "extras", "rules", "--add-target", ".cursor/rules", "-p").AssertSuccess(t)
	sb.RunCLIInDir(project, "sync", "extras", "-p").AssertSuccess(t)
	body, err := os.ReadFile(filepath.Join(project, ".cursor", "rules", "SKILL.md"))
	if err != nil || !strings.Contains(string(body), "# v1") {
		t.Fatalf("sync: %s %v", body, err)
	}
	for _, command := range [][]string{
		{"install", remote, "--skip-audit", "-p"},
		{"update", "demo", "--skip-audit", "-p"},
		{"move", "demo", "group", "-p"},
		{"uninstall", "group/demo", "--force", "-p"},
	} {
		if command[0] == "update" {
			bump("v2")
		}
		sb.RunCLIInDir(project, command...).AssertSuccess(t)
		lock := readLock(t, project)
		if lock.Version != 1 || lock.Extras["rules"] != pin {
			t.Fatalf("%v changed extra pin: %+v", command, lock)
		}
	}
}

func TestInstallExtra_LinkSentinelNeverCopied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges vary")
	}
	for _, targetKind := range []string{"absolute", "escaping", "directory", "dangling"} {
		t.Run(targetKind, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			remote := extraFixture(t, sb)
			repo := strings.TrimPrefix(remote, "file://")
			sentinel := filepath.Join(sb.Root, "sentinel")
			if err := os.WriteFile(sentinel, []byte("SYNTHETIC_SENTINEL_BYTES"), 0644); err != nil {
				t.Fatal(err)
			}
			target := sentinel
			switch targetKind {
			case "escaping":
				target = "../sentinel"
			case "directory":
				target = "subdir"
				if err := os.Mkdir(filepath.Join(repo, target), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(repo, target, "keep.md"), []byte("inside"), 0644); err != nil {
					t.Fatal(err)
				}
			case "dangling":
				target = "absent"
			}
			if err := os.Symlink(target, filepath.Join(repo, "unsafe")); err != nil {
				t.Fatal(err)
			}
			testutil.RunGit(t, repo, "add", ".")
			testutil.RunGit(t, repo, "commit", "-m", "synthetic unsafe link")
			before, err := os.ReadFile(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			result := sb.RunCLI("install", remote, "--kind", "extra", "--name", "rules", "--json", "-g")
			result.AssertFailure(t)
			result.AssertAnyOutputContains(t, "unsafe")
			after, err := os.ReadFile(sb.ConfigPath)
			if err != nil || string(before) != string(after) {
				t.Fatal("link refusal changed config")
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(sb.SourcePath), "extras")); !os.IsNotExist(err) {
				t.Fatalf("link refusal published: %v", err)
			}
			for path, data := range extraFiles(t, sb.Root) {
				if path != "sentinel" && strings.Contains(data, "SYNTHETIC_SENTINEL_BYTES") {
					t.Fatalf("sentinel copied to %s", path)
				}
			}
		})
	}
}

func TestInstallExtra_WebURLBranchAndExplicitOverride(t *testing.T) {
	for _, override := range []bool{false, true} {
		t.Run(map[bool]string{false: "parsed", true: "explicit"}[override], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			bare := testutil.SetupBareRemoteRepo(t, sb.Root)
			for _, branch := range []string{"main", "release"} {
				testutil.SeedRemoteBranch(t, sb.Root, bare, branch, map[string]string{"rules/rule.md": branch})
			}
			gitConfig := filepath.Join(sb.Root, "fixture.gitconfig")
			testutil.RunGit(t, sb.Root, "config", "--file", gitConfig, "url.file://"+filepath.ToSlash(bare)+".insteadOf", "https://github.com/example-org/ai-hub.git")
			sb.SetEnv("GIT_CONFIG_GLOBAL", gitConfig)
			sb.SetEnv("GIT_CONFIG_NOSYSTEM", "1")
			sb.SetEnv("GIT_ALLOW_PROTOCOL", "file")
			args := []string{"install", "https://github.com/example-org/ai-hub/tree/release/rules", "--kind", "extra", "--name", "rules", "--skip-audit", "--json", "-g"}
			want := "release"
			if override {
				args = append(args, "--branch", "main")
				want = "main"
			}
			result := sb.RunCLI(args...)
			result.AssertSuccess(t)
			var out struct{ Branch, Commit string }
			if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
				t.Fatal(err)
			}
			head := strings.TrimSpace(testutil.RunGit(t, filepath.Join(sb.Root, "seed-"+want), "rev-parse", "HEAD"))
			if out.Branch != want || out.Commit != head {
				t.Fatalf("effective branch: %+v want %s %s", out, want, head)
			}
			root := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
			store, _, err := install.ReadMetadata(root)
			if err != nil || store.GetExact("rules").Branch != want {
				t.Fatalf("metadata branch: %+v %v", store, err)
			}
			data, err := os.ReadFile(sb.ConfigPath)
			if err != nil || !strings.Contains(string(data), "branch: "+want) {
				t.Fatalf("config branch: %s %v", data, err)
			}
		})
	}
}
