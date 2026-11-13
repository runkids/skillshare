//go:build !online

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/gitenv"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/testutil"
	"skillshare/internal/utils"
)

// Refusal may create only the permanent administrative root lock, never records.
func assertExtraAdministrativeRootOnly(t *testing.T, root string) {
	t.Helper()
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || utils.IsLinkMode(root, info.Mode()) {
		t.Fatalf("administrative root is not a no-follow directory: %v %v", info, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != utils.ExtrasRootLockFileName {
		t.Fatalf("refusal must leave only administrative lock, no managed tree or metadata: %+v %v", entries, err)
	}
	lockPath := filepath.Join(root, utils.ExtrasRootLockFileName)
	info, err = os.Lstat(lockPath)
	if err != nil || !info.Mode().IsRegular() || utils.IsLinkMode(lockPath, info.Mode()) || info.Size() != 0 {
		t.Fatalf("administrative lock is not an empty no-follow regular file: %v %v", info, err)
	}
}

func extraRefusalRecords(t *testing.T, configPath, cwd string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, path := range []string{
		filepath.Join(filepath.Dir(configPath), install.LockFileName),
		filepath.Join(filepath.Dir(configPath), ".gitignore"),
		filepath.Join(cwd, ".gitignore"),
	} {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		files[path] = string(data)
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
	// Make the real previously recorded revision unavailable at its disposable
	// origin, preserving the exact producer-written pin and installed tree.
	remote := strings.TrimSuffix(strings.TrimPrefix(source, "file://"), "//skills/demo")
	seed := filepath.Join(sb.Root, "seed-main")
	testutil.RunGit(t, seed, "checkout", "--orphan", "replacement-history")
	testutil.RunGit(t, seed, "commit", "--allow-empty", "-m", "replace disposable history")
	testutil.RunGit(t, seed, "push", "--force", "origin", "HEAD:main")
	testutil.RunGit(t, remote, "reflog", "expire", "--expire=now", "--all")
	testutil.RunGit(t, remote, "gc", "--prune=now")
	if err := gitenv.Command("-C", remote, "cat-file", "-e", first.Commit+"^{commit}").Run(); err == nil {
		t.Fatal("fixture still exposes the producer's pinned revision")
	}
	before := projectTree(t, filepath.Join(other, ".skillshare", "extras"))
	result := sb.RunCLIInDir(other, args...)
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, first.Commit[:7])
	if after := projectTree(t, filepath.Join(other, ".skillshare", "extras")); !reflect.DeepEqual(before, after) {
		t.Fatal("unreachable pin changed extras")
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
			recordsBefore := extraRefusalRecords(t, configPath, cwd)
			args := []string{"install", remote, "--kind", "extra", "--name", "rules", "--json", mode}
			result := sb.RunCLIInDir(cwd, args...)
			result.AssertFailure(t)
			assertExtraAdministrativeRootOnly(t, root)
			// First-failure logging may add only its existing logs/ ignore rule.
			if projectMode {
				ignorePath := filepath.Join(filepath.Dir(configPath), ".gitignore")
				if data, ok := recordsBefore[ignorePath]; !ok || data != "" {
					t.Fatal("fixture must start with an empty project ignore file")
				}
				recordsBefore[ignorePath] = "# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\nlogs/\n# END SKILLSHARE MANAGED\n"
			}
			if !reflect.DeepEqual(recordsBefore, extraRefusalRecords(t, configPath, cwd)) {
				t.Fatalf("audit refusal changed records beyond failure logging: want=%#v got=%#v", recordsBefore, extraRefusalRecords(t, configPath, cwd))
			}
			entries, err := oplog.Read(configPath, oplog.OpsFile, 0)
			if err != nil || len(entries) != 1 {
				t.Fatalf("failed install oplog: %+v %v", entries, err)
			}
			entry := entries[0]
			if entry.Command != "install" || entry.Status != "error" || entry.Message == "" || entry.Args["kind"] != "extra" || entry.Args["name"] != "rules" || entry.Args["source"] != remote || entry.Args["mode"] != map[bool]string{false: "global", true: "project"}[projectMode] {
				t.Fatalf("failed install oplog: %+v", entry)
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
			before := projectTree(t, sb.Root)
			result := sb.RunCLIInDir(cwd, "install", remote, "--kind", "extra", "--name", "rules", "--dry-run", "--json", mode)
			result.AssertSuccess(t)
			var out struct{ Action string }
			if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil || out.Action != "would install" {
				t.Fatalf("dry result %s %v", result.Stdout, err)
			}
			if after := projectTree(t, sb.Root); !reflect.DeepEqual(before, after) {
				t.Fatal("dry run changed sandbox files or directories")
			}
			for _, source := range []string{"https://git.example.invalid/org/repo.git//outside", "https://git.example.invalid/org/repo.git/%2Foutside"} {
				result := sb.RunCLIInDir(cwd, "install", source, "--kind", "extra", "--name", "rules", "--dry-run", "--json", mode)
				result.AssertFailure(t)
				var out struct{ Action, Error string }
				if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil || out.Error == "" || out.Action == "would install" {
					t.Errorf("unsafe directory dry-run result: %s %v", result.Stdout, err)
				}
				if after := projectTree(t, sb.Root); !reflect.DeepEqual(before, after) {
					t.Fatal("unsafe directory dry run changed sandbox files or directories")
				}
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
			recordsBefore := extraRefusalRecords(t, sb.ConfigPath, sb.Root)
			result := sb.RunCLI("install", remote, "--kind", "extra", "--name", "rules", "--json", "-g")
			result.AssertFailure(t)
			result.AssertAnyOutputContains(t, "unsafe")
			after, err := os.ReadFile(sb.ConfigPath)
			if err != nil || string(before) != string(after) {
				t.Fatal("link refusal changed config")
			}
			assertExtraAdministrativeRootOnly(t, filepath.Join(filepath.Dir(sb.SourcePath), "extras"))
			if !reflect.DeepEqual(recordsBefore, extraRefusalRecords(t, sb.ConfigPath, sb.Root)) {
				t.Fatal("link refusal changed pin or ignore records")
			}
			for path, data := range projectTree(t, sb.Root) {
				if path != "sentinel" && strings.Contains(data, "SYNTHETIC_SENTINEL_BYTES") {
					t.Fatalf("sentinel copied to %s", path)
				}
			}
		})
	}
}

func TestInstallExtra_BranchSwitchAndOmittedBranch(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		for _, projectMode := range []bool{false, true} {
			for _, missing := range []bool{false, true} {
				t.Run(fmt.Sprintf("project=%t/missing=%t", projectMode, missing), func(t *testing.T) {
					sb := testutil.NewSandbox(t)
					defer sb.Cleanup()
					sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
					bare := testutil.SetupBareRemoteRepo(t, sb.Root)
					for _, branch := range []string{"main", "release"} {
						testutil.SeedRemoteBranch(t, sb.Root, bare, branch, map[string]string{"rules.md": branch})
					}
					cwd, mode := sb.Root, "-g"
					root := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
					if projectMode {
						cwd, mode = sb.SetupProjectDir("claude"), "-p"
						root = filepath.Join(cwd, ".skillshare", "extras")
					}
					args := []string{"install", "file://" + bare, "--kind", "extra", "--name", "rules", "--skip-audit", "--json", mode}
					run := func(branch string) (commit string) {
						t.Helper()
						command := append([]string(nil), args...)
						if branch != "" {
							command = append(command, "--branch", branch)
						}
						result := sb.RunCLIInDir(cwd, command...)
						result.AssertSuccess(t)
						var out struct{ Branch, Commit string }
						if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
							t.Fatal(err)
						}
						want := branch
						if want == "" {
							want = "release"
						}
						if out.Branch != want {
							t.Fatalf("branch %q want %q", out.Branch, want)
						}
						store, _, err := install.ReadMetadata(root)
						if err != nil || store.GetExact("rules").Branch != want || store.GetExact("rules").Commit != out.Commit {
							t.Fatalf("metadata: %+v %v", store, err)
						}
						return out.Commit
					}
					main := run("main")
					if missing {
						if err := os.RemoveAll(filepath.Join(root, "rules")); err != nil {
							t.Fatal(err)
						}
						if err := os.Remove(filepath.Join(root, install.MetadataFileName)); err != nil {
							t.Fatal(err)
						}
					}
					release := run("release")
					if release == main {
						t.Fatal("branch switch reused main pin")
					}
					body, err := os.ReadFile(filepath.Join(root, "rules", "rules.md"))
					if err != nil || string(body) != "release" {
						t.Fatalf("branch content: %q %v", body, err)
					}
					seed := filepath.Join(sb.Root, "seed-release")
					if err := os.WriteFile(filepath.Join(seed, "rules.md"), []byte("release advanced"), 0644); err != nil {
						t.Fatal(err)
					}
					testutil.RunGit(t, seed, "commit", "-am", "advance release")
					testutil.RunGit(t, seed, "push", "origin", "HEAD:release")
					if got := run("release"); got != release {
						t.Fatalf("same branch moved pin: %s", got)
					}
					if got := run(""); got != release {
						t.Fatalf("omitted branch moved pin: %s", got)
					}
				})
			}
		}
	})
	t.Run("TestInstallExtra_BranchAndGlobalPin", func(t *testing.T) {
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
	})
	t.Run("TestInstallExtra_WebURLBranchAndExplicitOverride", func(t *testing.T) {
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
	})
	t.Run("TestInstallExtra_WebURLDefaultDoesNotResetDeclaredBranch", func(t *testing.T) {
		for _, project := range []bool{false, true} {
			for _, missing := range []bool{false, true} {
				t.Run(fmt.Sprintf("project=%t/missing=%t", project, missing), func(t *testing.T) {
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
					cwd, mode := sb.Root, "-g"
					root, configPath := filepath.Join(filepath.Dir(sb.SourcePath), "extras"), sb.ConfigPath
					if project {
						cwd, mode = sb.SetupProjectDir("claude"), "-p"
						root, configPath = filepath.Join(cwd, ".skillshare", "extras"), filepath.Join(cwd, ".skillshare", "config.yaml")
					}
					args := []string{"install", "https://github.com/example-org/ai-hub/tree/release/rules", "--kind", "extra", "--name", "rules", "--skip-audit", "--json", mode}
					first := sb.RunCLIInDir(cwd, append(append([]string{}, args...), "--branch", "main")...)
					first.AssertSuccess(t)
					var original struct{ Branch, Commit string }
					if err := json.Unmarshal([]byte(first.Stdout), &original); err != nil || original.Branch != "main" {
						t.Fatalf("initial override: %s %v", first.Stdout, err)
					}
					if missing {
						if err := os.RemoveAll(filepath.Join(root, "rules")); err != nil {
							t.Fatal(err)
						}
						if err := os.Remove(filepath.Join(root, install.MetadataFileName)); err != nil {
							t.Fatal(err)
						}
					}
					result := sb.RunCLIInDir(cwd, args...)
					result.AssertSuccess(t)
					var got struct{ Branch, Commit string }
					if err := json.Unmarshal([]byte(result.Stdout), &got); err != nil || got != original {
						t.Fatalf("omitted flag reset URL override: %s want %+v %v", result.Stdout, original, err)
					}
					store, _, err := install.ReadMetadata(root)
					if err != nil || store.GetExact("rules").Branch != "main" || store.GetExact("rules").Commit != original.Commit {
						t.Fatalf("metadata: %+v %v", store, err)
					}
					data, err := os.ReadFile(configPath)
					if err != nil || !strings.Contains(string(data), "branch: main") {
						t.Fatalf("config: %s %v", data, err)
					}
					body, err := os.ReadFile(filepath.Join(root, "rules", "rule.md"))
					if err != nil || string(body) != "main" {
						t.Fatalf("content: %q %v", body, err)
					}
				})
			}
		}
	})
	t.Run("TestInstallExtra_DeclaredURLBranchTracksResolvedRef", func(t *testing.T) {
		for _, project := range []bool{false, true} {
			for _, explicit := range []bool{false, true} {
				t.Run(fmt.Sprintf("project=%t/explicit=%t", project, explicit), func(t *testing.T) {
					sb := testutil.NewSandbox(t)
					defer sb.Cleanup()
					bare := testutil.SetupBareRemoteRepo(t, sb.Root)
					testutil.SeedRemoteBranch(t, sb.Root, bare, "feature/x", map[string]string{
						"rules/rule.md": "correct", "x/rules/rule.md": "WRONG",
					})
					gitConfig := filepath.Join(sb.Root, "fixture.gitconfig")
					testutil.RunGit(t, sb.Root, "config", "--file", gitConfig, "url.file://"+filepath.ToSlash(bare)+".insteadOf", "https://github.com/example-org/ai-hub.git")
					sb.SetEnv("GIT_CONFIG_GLOBAL", gitConfig)
					sb.SetEnv("GIT_CONFIG_NOSYSTEM", "1")
					sb.SetEnv("GIT_ALLOW_PROTOCOL", "file")
					raw := "https://github.com/example-org/ai-hub/tree/feature/x/rules"
					// Supported raw-editor declarations can select a branch before
					// content/metadata exists; this value needs URL-ref resolution.
					sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nextras:\n  - name: rules\n    remote: " + raw + "\n    branch: feature\n    targets: []\n")
					cwd, mode, cfgPath := sb.Root, "-g", sb.ConfigPath
					root := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
					if project {
						cwd, mode = sb.SetupProjectDir("claude"), "-p"
						cfg, err := config.LoadProjectReadOnly(cwd)
						if err != nil {
							t.Fatal(err)
						}
						cfg.Extras = []config.ExtraConfig{{Name: "rules", Remote: raw, Branch: "feature", Targets: []config.ExtraTargetConfig{}}}
						if err := cfg.Save(cwd); err != nil {
							t.Fatal(err)
						}
						cfgPath = config.ProjectConfigPath(cwd)
						root = cfg.EffectiveExtrasSource(cwd)
					}
					args := []string{"install", raw, "--kind", "extra", "--name", "rules", "--skip-audit", "--json", mode}
					if explicit {
						args = append(args, "--branch", "feature/x")
					}
					result := sb.RunCLIInDir(cwd, args...)
					result.AssertSuccess(t)
					var out struct{ Branch, Commit string }
					if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil || out.Branch != "feature/x" {
						t.Fatalf("effective ref: %+v %v", out, err)
					}
					data, err := os.ReadFile(cfgPath)
					if err != nil {
						t.Fatal(err)
					}
					entries, err := config.RawExtras(data)
					if err != nil || len(entries) != 1 || entries[0].Branch != out.Branch {
						t.Errorf("saved branch disagrees with resolved install: %+v result=%+v error=%v", entries, out, err)
					}
					store, _, err := install.ReadMetadata(root)
					if err != nil || store.GetExact("rules") == nil || store.GetExact("rules").Branch != out.Branch {
						t.Errorf("metadata branch disagrees: %+v %v", store, err)
					}
					data, err = os.ReadFile(filepath.Join(root, "rules", "rule.md"))
					if err != nil || strings.TrimSpace(string(data)) != "correct" {
						t.Errorf("resolved directory disagrees: %s %v", data, err)
					}
				})
			}
		}
	})
	t.Run("TestInstallExtra_WebSlashRefResolvesDirectoryAndBranch", func(t *testing.T) {
		providers := map[string]string{
			"github.com":    "tree/",
			"gitlab.com":    "-/tree/",
			"bitbucket.org": "src/",
		}
		for host, marker := range providers {
			for _, project := range []bool{false, true} {
				for _, explicit := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/project=%t/explicit=%t", host, project, explicit), func(t *testing.T) {
						sb := testutil.NewSandbox(t)
						defer sb.Cleanup()
						sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
						bare := testutil.SetupBareRemoteRepo(t, sb.Root)
						files := map[string]string{"rules/rule.md": "correct slash-ref directory", "x/rules/rule.md": "WRONG directory from first-segment ref split"}
						testutil.SeedRemoteBranch(t, sb.Root, bare, "feature/x", files)
						head := strings.TrimSpace(testutil.RunGit(t, bare, "rev-parse", "refs/heads/feature/x"))
						gitConfig := filepath.Join(sb.Root, "fixture.gitconfig")
						repoURL := "https://" + host + "/example-org/ai-hub"
						testutil.RunGit(t, sb.Root, "config", "--file", gitConfig, "url.file://"+filepath.ToSlash(bare)+".insteadOf", repoURL+".git")
						sb.SetEnv("GIT_CONFIG_GLOBAL", gitConfig)
						sb.SetEnv("GIT_CONFIG_NOSYSTEM", "1")
						sb.SetEnv("GIT_ALLOW_PROTOCOL", "file")
						cwd, mode := sb.Root, "-g"
						root := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
						if project {
							cwd, mode = sb.SetupProjectDir("claude"), "-p"
							root = filepath.Join(cwd, ".skillshare", "extras")
						}
						raw := repoURL + "/" + marker + "feature/x/rules"
						args := []string{"install", raw, "--kind", "extra", "--name", "rules", "--skip-audit", "--json", mode}
						if explicit {
							args = append(args, "--branch", "feature/x")
						}
						result := sb.RunCLIInDir(cwd, args...)
						result.AssertSuccess(t)
						var out struct{ Branch, Commit string }
						if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
							t.Fatal(err)
						}
						if out.Branch != "feature/x" || out.Commit != head {
							t.Errorf("effective ref lost: %+v want feature/x at%s", out, head)
						}
						if data, err := os.ReadFile(filepath.Join(root, "rules", "rule.md")); err != nil || string(data) != files["rules/rule.md"] {
							t.Errorf("wrong selected directory: %s %v", data, err)
						}
						store, _, err := install.ReadMetadata(root)
						if err != nil {
							t.Fatal(err)
						}
						entry := store.GetExact("rules")
						if entry == nil || entry.Branch != "feature/x" || entry.Commit != head || entry.Source != raw {
							t.Errorf("metadata ref/identity lost: %+v", entry)
						}
						var extras []config.ExtraConfig
						if project {
							cfg, err := config.LoadProjectReadOnly(cwd)
							if err != nil {
								t.Fatal(err)
							}
							extras = cfg.Extras
							if pin := readLock(t, cwd).Extras["rules"]; pin.Source != raw || pin.Commit != head {
								t.Errorf("project pin lost: %+v", pin)
							}
						} else {
							data, err := os.ReadFile(sb.ConfigPath)
							if err != nil {
								t.Fatal(err)
							}
							extras, err = config.RawExtras(data)
							if err != nil {
								t.Fatal(err)
							}
						}
						if len(extras) != 1 || extras[0].Branch != "feature/x" || extras[0].Remote != raw {
							t.Errorf("saved ref/identity lost: %+v", extras)
						}
						// Omitted branch on a later install must preserve the resolved
						// declaration and the same exact pin, even when the URL is ambiguous.
						sb.RunCLIInDir(cwd, "install", raw, "--kind", "extra", "--name", "rules", "--skip-audit", "--json", mode).AssertSuccess(t)
						if data, err := os.ReadFile(filepath.Join(root, "rules", "rule.md")); err != nil || string(data) != files["rules/rule.md"] {
							t.Errorf("reinstall changed selected directory: %s %v", data, err)
						}
					})
				}
			}
		}
	})
}
