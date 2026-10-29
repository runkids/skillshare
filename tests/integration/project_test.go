//go:build !online

package integration

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func projectFixture(t *testing.T) (*testutil.Sandbox, string) {
	t.Helper()
	sb := testutil.NewSandbox(t)
	t.Cleanup(sb.Cleanup)
	sb.SetEnv("XDG_CACHE_HOME", filepath.Join(sb.Home, ".cache"))
	root := filepath.Join(sb.Home, "app")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	return sb, root
}
func projectJSON(t *testing.T, r *testutil.Result) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &v); err != nil {
		t.Fatalf("invalid JSON (%v): stdout=%s stderr=%s", err, r.Stdout, r.Stderr)
	}
	if r.Stderr != "" {
		t.Fatalf("JSON stderr: %s", r.Stderr)
	}
	return v
}
func projectTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if e.IsDir() {
			tree[rel] = "dir"
			return nil
		}
		if e.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			tree[rel] = "link:" + link
			return err
		}
		b, err := os.ReadFile(path)
		tree[rel] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}
func TestProject_RoundTrip(t *testing.T) {
	sb, root := projectFixture(t)
	r := sb.RunCLI("project", "--json", "add", root, "--target", "claude,codex", "--name", "work", "--agents", "--skills-include", "one", "--skills-mode", "copy")
	r.AssertSuccess(t)
	v := projectJSON(t, r)
	if v["action"] != "add" || v["applied"] != true {
		t.Fatalf("add: %v", v)
	}
	p := v["project"].(map[string]any)
	if p["name"] != "work" || len(p["targets"].([]any)) != 2 {
		t.Fatalf("item: %v", p)
	}
	for _, token := range []string{"work", root, "~/app"} {
		r = sb.RunCLI("project", "show", token, "--json")
		r.AssertSuccess(t)
		if projectJSON(t, r)["project"].(map[string]any)["root"] != root {
			t.Fatal(r.Stdout)
		}
	}
	r = sb.RunCLI("project", "edit", "work", "--name=", "--skills-include=", "--no-agents", "--json")
	r.AssertSuccess(t)
	p = projectJSON(t, r)["project"].(map[string]any)
	if p["name"] != "app" || p["agents"] != nil || len(p["skills"].(map[string]any)["include"].([]any)) != 0 {
		t.Fatal(p)
	}
	r = sb.RunCLI("project", "list", "--json")
	r.AssertSuccess(t)
	if len(projectJSON(t, r)["projects"].([]any)) != 1 {
		t.Fatal(r.Stdout)
	}
	sb.RunCLI("project", "show", "app").AssertSuccess(t)
	before := projectTree(t, sb.Home)
	r = sb.RunCLI("project", "edit", "app", "--json")
	r.AssertSuccess(t)
	v = projectJSON(t, r)
	if v["applied"] != false || v["unchanged"] != true {
		t.Fatal(v)
	}
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("no-op changed files")
	}
	r = sb.RunCLI("project", "remove", "app", "--json", "--dry-run")
	r.AssertSuccess(t)
	if projectJSON(t, r)["applied"] != false {
		t.Fatal(r.Stdout)
	}
	r = sb.RunCLI("project", "remove", "app", "--json")
	r.AssertFailure(t)
	projectJSON(t, r)
	r = sb.RunCLI("project", "remove", "app", "--yes", "--json")
	r.AssertSuccess(t)
	if projectJSON(t, r)["registration"] != "removed" {
		t.Fatal(r.Stdout)
	}
	r = sb.RunCLI("project", "--json")
	r.AssertSuccess(t)
	if len(projectJSON(t, r)["projects"].([]any)) != 0 {
		t.Fatal(r.Stdout)
	}
}
func TestProject_GlobalModeAndParsing(t *testing.T) {
	for _, style := range []string{".skillshare", "skillshare"} {
		t.Run(style, func(t *testing.T) {
			sb, root := projectFixture(t)
			os.MkdirAll(filepath.Join(root, style), 0755)
			os.WriteFile(filepath.Join(root, style, "config.yaml"), []byte("targets: [claude]\n"), 0644)
			r := sb.RunCLIInDir(root, "project", "add", "--name", "-p", "--target=claude", "--json", "--", ".")
			r.AssertSuccess(t)
			if projectJSON(t, r)["project"].(map[string]any)["name"] != "-p" {
				t.Fatal(r.Stdout)
			}
			sb.RunCLIInDir(root, "project", "list", "-g").AssertSuccess(t)
			r = sb.RunCLIInDir(root, "project", "list", "-p", "--json")
			r.AssertFailure(t)
			if !strings.Contains(projectJSON(t, r)["error"].(string), "projects belong in the global config") {
				t.Fatal(r.Stdout)
			}
		})
	}
	for _, args := range [][]string{{"edit", "x", "--bad"}, {"show"}, {"add", "x", "--target"}, {"add", "x", "--skills", "--no-skills"}, {"wat"}, {"add", "~user", "--target", "claude"}} {
		sb, _ := projectFixture(t)
		r := sb.RunCLI(append([]string{"project", "--json"}, args...)...)
		r.AssertFailure(t)
		if projectJSON(t, r)["error"] == nil {
			t.Fatal(r.Stdout)
		}
	}
	sb, _ := projectFixture(t)
	os.WriteFile(sb.ConfigPath, []byte("[bad"), 0644)
	r := sb.RunCLI("project", "--json")
	r.AssertFailure(t)
	projectJSON(t, r)
}
func TestProject_Confirmation(t *testing.T) {
	for _, input := range []string{"", "n\n", "y\n"} {
		t.Run(strings.TrimSpace(input), func(t *testing.T) {
			sb, root := projectFixture(t)
			sb.RunCLI("project", "add", root, "--target", "claude", "--json").AssertSuccess(t)
			r := sb.RunCLIWithInput(input, "project", "remove", "app")
			r.AssertSuccess(t)
			r = sb.RunCLI("project", "list", "--json")
			r.AssertSuccess(t)
			count := len(projectJSON(t, r)["projects"].([]any))
			if (input == "y\n" && count != 0) || (input != "y\n" && count != 1) {
				t.Fatal(r.Stdout)
			}
		})
	}
}
func TestProject_ReadOnlyLegacyWholeInvocation(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "skillshare")
	cmd := exec.Command("go", "build", "-ldflags", "-X main.version=1.0.0", "-o", binary, "./cmd/skillshare")
	cmd.Dir = findProjectRoot(t)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, b)
	}
	// Text add, edit, sync and remove run startup maintenance like other commands.
	for _, tc := range []struct {
		args     []string
		readOnly bool
	}{
		{[]string{"list"}, true},
		{[]string{"show", "app"}, true},
		{[]string{"edit", "app", "--name", "preview", "--dry-run"}, true},
		{[]string{"list", "--json"}, true},
		{[]string{"edit", "app", "--json"}, true},
		{[]string{"edit", "app", "--skills-mode", "bad", "--json"}, true},
		{[]string{"edit", "app"}, false},
		{[]string{"edit", "app", "--skills-mode", "bad"}, false},
	} {
		args := tc.args
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			sb, root := projectFixture(t)
			sb.BinaryPath = binary
			os.RemoveAll(filepath.Join(sb.Home, ".local"))
			for _, dir := range []string{"backups", "trash", "logs"} {
				p := filepath.Join(filepath.Dir(sb.ConfigPath), dir)
				os.MkdirAll(p, 0755)
				os.WriteFile(filepath.Join(p, "old"), []byte("keep"), 0644)
			}
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nskills:\n  - name: legacy\n    source: github.com/a/b\nprojects:\n  " + root + ":\n    targets: [claude]\n    skills: {}\n")
			before := projectTree(t, sb.Home)
			r := sb.RunCLI(append([]string{"project"}, args...)...)
			if args[0] == "edit" && strings.Contains(strings.Join(args, " "), "bad") {
				r.AssertFailure(t)
			} else {
				r.AssertSuccess(t)
			}
			if strings.Contains(strings.Join(args, " "), "--json") {
				projectJSON(t, r)
			}
			legacy := filepath.Join(filepath.Dir(sb.ConfigPath), "backups", "old")
			if !tc.readOnly {
				if _, err := os.Stat(legacy); !os.IsNotExist(err) {
					t.Fatalf("text mutation skipped startup migration: %v %s", err, r.Output())
				}
				return
			}
			after := projectTree(t, sb.Home)
			// Actual mutations acquire a persistent flock in the state dir, even when
			// unchanged or invalid. Never unlink it: queued writers may still hold its inode.
			if args[0] == "edit" && !slices.Contains(args, "--dry-run") {
				for rel := range after {
					if _, ok := before[rel]; !ok && (rel == ".local" || strings.HasPrefix(rel, filepath.Join(".local", "state"))) {
						delete(after, rel)
					}
				}
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("read-only changed filesystem: %s", r.Output())
			}
		})
	}
}
func TestProject_LegacyMutationAndAnchors(t *testing.T) {
	sb, root := projectFixture(t)
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nskills:\n  - name: legacy\n    source: github.com/a/b\n    custom: keep\nprojects:\n  " + root + ": &shared\n    targets: [claude]\n    skills: {}\n  ~/other: *shared\n")
	r := sb.RunCLI("project", "edit", "app", "--skills-mode", "copy", "--json")
	r.AssertSuccess(t)
	if len(projectJSON(t, r)["affected"].([]any)) != 2 {
		t.Fatal(r.Stdout)
	}
	b, _ := os.ReadFile(sb.ConfigPath)
	if !strings.Contains(string(b), "*shared") || !strings.Contains(string(b), "custom: keep") {
		t.Fatal(string(b))
	}
	extra := filepath.Join(sb.Home, "extra")
	os.MkdirAll(extra, 0755)
	r = sb.RunCLI("project", "add", extra, "--target", "claude", "--json")
	r.AssertSuccess(t)
	projectJSON(t, r)
	b, _ = os.ReadFile(sb.ConfigPath)
	if !strings.Contains(string(b), "*shared") || !strings.Contains(string(b), "custom: keep") {
		t.Fatal(string(b))
	}
	before := projectTree(t, sb.Home)
	r = sb.RunCLI("project", "edit", "app", "--name", "duplicate", "--json")
	r.AssertFailure(t)
	projectJSON(t, r)
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("invalid alias candidate changed files")
	}
}

func projectLogs(t *testing.T, sb *testutil.Sandbox) []map[string]any {
	t.Helper()
	path := filepath.Join(sb.Home, ".local", "state", "skillshare", "logs", "operations.log")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	return entries
}
func TestProject_SyncScopePreviewAndRetry(t *testing.T) {
	sb, root := projectFixture(t)
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
	skillsSource := sb.SourcePath
	if runtime.GOOS != "windows" {
		skillsSource = filepath.Join(sb.Root, "skills-alias")
		sb.CreateSymlink(sb.SourcePath, skillsSource)
	}
	sb.WriteFile(filepath.Join(sb.SourcePath, ".skillignore"), "not-present\n")
	repoDir := filepath.Join(sb.SourcePath, "_team")
	gitInit(t, repoDir, false)
	sb.WriteFile(filepath.Join(repoDir, ".skillignore"), "not-present\n")
	sb.WriteConfig("source: " + skillsSource + "\ntargets: {}\n")
	other := filepath.Join(sb.Home, "other")
	os.MkdirAll(other, 0755)
	sb.RunCLI("project", "add", other, "--target", "claude", "--json").AssertSuccess(t)
	// A blocked second skills parent makes the saved entry a partial sync.
	os.WriteFile(filepath.Join(root, ".agents"), []byte("blocked"), 0644)
	r := sb.RunCLI("project", "add", root, "--target", "claude,codex", "--sync", "--json")
	r.AssertFailure(t)
	v := projectJSON(t, r)
	if v["applied"] != true || v["project"] == nil || v["sync"] == nil {
		t.Fatal(v)
	}
	sync := v["sync"].(map[string]any)
	if len(sync["failed"].([]any)) == 0 || len(sync["results"].([]any)) == 0 {
		t.Fatal(sync)
	}
	for _, key := range []string{"warnings", "unmatched", "folder_conflicts", "path_overlap", "ignored_count", "ignored_skills", "ignore_root", "ignore_repos"} {
		if _, ok := sync[key]; !ok {
			t.Fatalf("missing %s: %v", key, sync)
		}
	}
	canonicalSource, err := filepath.EvalSymlinks(sb.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(canonicalSource, ".skillignore"); sync["ignore_root"] != want {
		t.Fatalf("expected canonical skill ignore root %q, got %v", want, sync["ignore_root"])
	}
	ignoreRepos := sync["ignore_repos"].([]any)
	canonicalRepo, err := filepath.EvalSymlinks(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(canonicalRepo, ".skillignore"); len(ignoreRepos) != 1 || ignoreRepos[0] != want {
		t.Fatalf("expected canonical repo ignore path %q, got %v", want, ignoreRepos)
	}
	if sync["Skills"] != nil || sync["Results"] != nil {
		t.Fatal("internal fields leaked")
	}
	if _, err := os.Lstat(filepath.Join(other, ".claude", "skills", "alpha")); !os.IsNotExist(err) {
		t.Fatal("other project synced")
	}
	logs := projectLogs(t, sb)
	if len(logs) != 3 || logs[1]["cmd"] != "project" || logs[2]["cmd"] != "sync" || logs[2]["status"] != "partial" {
		t.Fatal(logs)
	}
	os.Remove(filepath.Join(root, ".agents"))
	before := projectTree(t, sb.Home)
	r = sb.RunCLI("project", "edit", "app", "--name", "renamed", "--target", "cursor", "--sync", "--dry-run", "--json")
	r.AssertSuccess(t)
	v = projectJSON(t, r)
	rows := v["sync"].(map[string]any)["results"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["target"] != "renamed@cursor" {
		t.Fatal(v)
	}
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("sync preview wrote")
	}
	r = sb.RunCLI("project", "sync", "app", "--json")
	r.AssertSuccess(t)
	v = projectJSON(t, r)
	if v["action"] != "sync" || v["applied"] != nil || len(v["sync"].(map[string]any)["failed"].([]any)) != 0 {
		t.Fatal(v)
	}
	if _, err := os.Lstat(filepath.Join(root, ".agents", "skills", "alpha")); err != nil {
		t.Fatal(err)
	}
	if len(projectLogs(t, sb)) != 4 {
		t.Fatal(projectLogs(t, sb))
	}
	sb.CreateSkill("beta", map[string]string{"SKILL.md": "---\nname: beta\n---\n# Beta\n"})
	before = projectTree(t, sb.Home)
	r = sb.RunCLI("project", "sync", "app", "--dry-run", "--json")
	r.AssertSuccess(t)
	projectJSON(t, r)
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("standalone sync preview wrote")
	}
}
func TestProject_RemoveDetachPartialAndWarnings(t *testing.T) {
	sb, root := projectFixture(t)
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
	r := sb.RunCLI("project", "add", root, "--target", "claude,codex", "--sync", "--json")
	r.AssertSuccess(t)
	v := projectJSON(t, r)
	groups := v["project"].(map[string]any)["groups"].([]any)
	var second string
	for _, g := range groups {
		group := g.(map[string]any)
		if group["target"] == "app@codex" {
			second = group["skillsPath"].(string)
		}
	}
	if second == "" {
		t.Fatal(groups)
	}
	// A nonempty directory where the second manifest belongs produces a real
	// later-target detach failure without relying on root-sensitive permissions.
	manifest := filepath.Join(second, ".skillshare-manifest.json")
	os.Remove(manifest)
	os.MkdirAll(manifest, 0755)
	os.WriteFile(filepath.Join(manifest, "blocker"), []byte("x"), 0644)
	before := projectTree(t, sb.Home)
	r = sb.RunCLI("project", "remove", "app", "--dry-run", "--json")
	r.AssertSuccess(t)
	v = projectJSON(t, r)
	if v["registration"] != "kept" || v["applied"] != false || v["detach"] == nil {
		t.Fatal(v)
	}
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("detach preview wrote")
	}
	r = sb.RunCLI("project", "remove", "app", "--yes", "--json")
	r.AssertFailure(t)
	v = projectJSON(t, r)
	if v["registration"] != "kept" || v["detach"] == nil || v["error"] == nil {
		t.Fatal(v)
	}
	if len(v["detach"].(map[string]any)["removed"].([]any)) == 0 {
		t.Fatal(v)
	}
	logs := projectLogs(t, sb)
	if len(logs) != 3 || logs[2]["status"] != "partial" || logs[2]["cmd"] != "project" {
		t.Fatal(logs)
	}
	info, err := os.Lstat(filepath.Join(root, ".claude", "skills", "alpha"))
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("first detach did not preserve copy: %v %v", info, err)
	}
	os.Remove(filepath.Join(manifest, "blocker"))
	os.Remove(manifest)
	// Out-of-scope registrations are retained and explicitly warned about.
	b, _ := os.ReadFile(sb.ConfigPath)
	os.WriteFile(sb.ConfigPath, append(b, []byte("\nmcp:\n  projects:\n    "+root+"/: {servers: {}}\nhooks:\n  projects:\n    "+root+"/.: {entries: {}}\n")...), 0644)
	r = sb.RunCLI("project", "remove", "app", "--yes", "--json")
	r.AssertSuccess(t)
	v = projectJSON(t, r)
	if v["registration"] != "removed" || len(v["warnings"].([]any)) != 2 {
		t.Fatal(v)
	}
}
func TestProject_AmbiguityAndValidation(t *testing.T) {
	sb, root := projectFixture(t)
	cwd := sb.Home
	if runtime.GOOS != "windows" {
		// Exercise an aliased cwd. A shell exports the alias as PWD; inherit
		// it so the CLI sees the same spelling a user would.
		cwd = filepath.Join(sb.Root, "home-alias")
		sb.CreateSymlink(sb.Home, cwd)
		sb.SetEnv("PWD", cwd)
	}
	collision := filepath.Join(cwd, "named")
	os.MkdirAll(collision, 0755)
	sb.RunCLI("project", "add", root, "--name", "named", "--target", "claude", "--json").AssertSuccess(t)
	sb.RunCLI("project", "add", collision, "--name", "other", "--target", "claude", "--json").AssertSuccess(t)
	r := sb.RunCLIInDir(cwd, "project", "show", "named", "--json")
	r.AssertFailure(t)
	v := projectJSON(t, r)
	if !strings.Contains(v["error"].(string), root) || !strings.Contains(v["error"].(string), collision) {
		t.Fatal(v)
	}
	sb.RunCLIInDir(cwd, "project", "show", "./named", "--json").AssertSuccess(t)
	os.MkdirAll(filepath.Join(sb.Home, "other-empty"), 0755)
	before := projectTree(t, sb.Home)
	for _, args := range [][]string{
		{"add", "~/app", "--target", "claude"},
		{"add", filepath.Join(sb.Home, "other-empty"), "--target="},
		{"add", filepath.Join(sb.Home, "missing"), "--target", "claude"},
		{"edit", root, "--target", "nonesuch"},
		{"edit", root, "--skills-mode", "bad"},
		{"edit", root, "--skills-naming", "bad"},
		{"edit", root, "--skills-include", "["},
		{"edit", root, "--agents-mode", "symlink", "--agents-include", "*"},
		{"edit", root, "--skills", "--no-skills"},
	} {
		r := sb.RunCLI(append(append([]string{"project"}, args...), "--json")...)
		r.AssertFailure(t)
		projectJSON(t, r)
	}
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("validation wrote")
	}
}

func TestProject_MissingRegisteredFolderAndStoredName(t *testing.T) {
	sb, root := projectFixture(t)
	// Absolute spelling with an internal '..' stays as typed. The displayed
	// default name must also be a supported lookup token from another cwd.
	os.MkdirAll(filepath.Join(root, "child"), 0755)
	spelling := root + string(filepath.Separator) + "child" + string(filepath.Separator) + ".."
	r := sb.RunCLI("project", "add", spelling, "--target", "claude", "--json")
	r.AssertSuccess(t)
	item := projectJSON(t, r)["project"].(map[string]any)
	name := item["name"].(string)
	r = sb.RunCLIInDir(sb.Home, "project", "show", name, "--json")
	r.AssertSuccess(t)
	if projectJSON(t, r)["project"].(map[string]any)["root"] != spelling {
		t.Fatal(r.Stdout)
	}
	os.Rename(root, root+"-moved")
	r = sb.RunCLI("project", "edit", spelling, "--name", "missing", "--json")
	r.AssertSuccess(t)
	r = sb.RunCLI("project", "edit", "missing", "--name", "still-missing", "--sync", "--json")
	r.AssertFailure(t)
	v := projectJSON(t, r)
	if v["applied"] != true || v["project"].(map[string]any)["name"] != "still-missing" || !strings.Contains(v["error"].(string), "folder not found") {
		t.Fatal(v)
	}
}
func TestProject_SyncAgentIgnoreStatistics(t *testing.T) {
	sb, root := projectFixture(t)
	agents := filepath.Join(sb.Home, "agents")
	os.MkdirAll(agents, 0755)
	os.WriteFile(filepath.Join(agents, "keep.md"), []byte("# Keep\n"), 0644)
	os.WriteFile(filepath.Join(agents, "ignored.md"), []byte("# Ignored\n"), 0644)
	ignore := filepath.Join(agents, ".agentignore")
	os.WriteFile(ignore, []byte("ignored.md\n"), 0644)
	agentsSource := agents
	if runtime.GOOS != "windows" {
		// Exercise a configured source alias while the reported ignore path stays canonical.
		agentsSource = filepath.Join(sb.Root, "agents-alias")
		sb.CreateSymlink(agents, agentsSource)
	}
	sb.WriteConfig("source: " + sb.SourcePath + "\nagents_source: " + agentsSource + "\ntargets: {}\n")
	r := sb.RunCLI("project", "add", root, "--target", "claude", "--no-skills", "--agents", "--sync", "--json")
	r.AssertSuccess(t)
	sync := projectJSON(t, r)["sync"].(map[string]any)
	canonicalIgnore, err := filepath.EvalSymlinks(ignore)
	if err != nil {
		t.Fatal(err)
	}
	if sync["agent_ignore_root"] != canonicalIgnore || sync["agent_ignored_count"] != float64(1) {
		t.Fatalf("agent statistics missing: %v", sync)
	}
	ignored, ok := sync["agent_ignored_skills"].([]any)
	if !ok || len(ignored) != 1 || ignored[0] != "ignored.md" {
		t.Fatal(sync)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude", "agents", "keep.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude", "agents", "ignored.md")); !os.IsNotExist(err) {
		t.Fatal("ignored agent synced")
	}
}

func TestProject_TextDefaultsAndRelativeKey(t *testing.T) {
	sb, _ := projectFixture(t)
	// A shell exports the lexical cwd; macOS temp homes sit behind /var symlinks.
	sb.SetEnv("PWD", sb.Home)
	r := sb.RunCLIInDir(sb.Home, "project", "add", "app", "--target", "claude", "--target", "codex")
	r.AssertSuccess(t)
	r.AssertRowContains(t, "Project", "app")
	r.AssertRowContains(t, "Root", "~/app")
	r = sb.RunCLI("project", "edit", "app", "--name", "text", "--agents", "--skills-exclude", "skip")
	r.AssertSuccess(t)
	r.AssertRowContains(t, "Project", "text")
	before := projectTree(t, sb.Home)
	r = sb.RunCLI("project", "sync", "text", "--dry-run")
	r.AssertSuccess(t)
	r.AssertOutputContains(t, "Dry run")
	if !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
		t.Fatal("text sync preview wrote")
	}
	r = sb.RunCLI("project", "remove", "text", "--yes")
	r.AssertSuccess(t)
	r.AssertRowContains(t, "Registration", "removed")
	sb.RunCLI("project").AssertOutputContains(t, "No projects registered")
	logs := projectLogs(t, sb)
	if len(logs) != 3 || logs[0]["args"].(map[string]any)["action"] != "add" || logs[1]["args"].(map[string]any)["action"] != "update" || logs[2]["args"].(map[string]any)["action"] != "remove" {
		t.Fatal(logs)
	}
	for _, entry := range logs {
		if entry["cmd"] != "project" || entry["args"].(map[string]any)["scope"] != "cli" {
			t.Fatal(logs)
		}
	}
}

func TestProject_TextUnmatchedSync(t *testing.T) {
	sb, root := projectFixture(t)
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
	for _, args := range [][]string{
		{"project", "add", root, "--target", "claude", "--skills-include", "typo", "--sync"},
		{"project", "sync", "app", "--dry-run"},
		{"project", "edit", "app", "--skills-include", "other-typo", "--sync", "--dry-run"},
	} {
		r := sb.RunCLI(args...)
		r.AssertSuccess(t)
		r.AssertOutputContains(t, "matches no skill in the source")
	}
}

func TestProject_EditKeepsStoredAgentSymlinkFilters(t *testing.T) {
	sb, root := projectFixture(t)
	// The dashboard saves symlink agents with filters; unrelated CLI edits must still work.
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nprojects:\n  " + root + ":\n    targets: [claude]\n    agents:\n      mode: symlink\n      include: [team-*]\n")
	sb.RunCLI("project", "edit", "app", "--name", "renamed", "--json").AssertSuccess(t)
	b, _ := os.ReadFile(sb.ConfigPath)
	if !strings.Contains(string(b), "name: renamed") || !strings.Contains(string(b), "team-*") {
		t.Fatalf("edit lost settings:\n%s", b)
	}
	r := sb.RunCLI("project", "edit", "renamed", "--agents-include", "other-*", "--json")
	r.AssertFailure(t)
	if !strings.Contains(projectJSON(t, r)["error"].(string), "symlink mode") {
		t.Fatal(r.Stdout)
	}
}

func TestProject_MissingRootsRemainInspectableAfterSync(t *testing.T) {
	sb, root := projectFixture(t)
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
	missing := []string{filepath.Join(sb.Home, "removed"), filepath.Join(sb.Home, "replaced-by-file")}
	sb.RunCLI("project", "add", root, "--target", "claude", "--json").AssertSuccess(t)
	for _, path := range missing {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
		sb.RunCLI("project", "add", path, "--target", "claude,codex", "--agents", "--json").AssertSuccess(t)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(missing[1], []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}
	r := sb.RunCLI("sync", "-g", "--json")
	r.AssertSuccess(t)
	projectJSON(t, r)
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "alpha", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(missing[0]); !os.IsNotExist(err) {
		t.Fatalf("absent project was created: %v", err)
	}
	if content, err := os.ReadFile(missing[1]); err != nil || string(content) != "keep me" {
		t.Fatalf("file root changed: %q, %v", content, err)
	}

	r = sb.RunCLI("sync", "-g")
	r.AssertSuccess(t)
	r.AssertOutputContains(t, "2 registered projects not found on this machine, skipped; run skillshare project list to inspect")
	for _, path := range missing {
		if strings.Contains(r.Stdout, path) {
			t.Fatalf("sync printed individual missing root: %s", r.Stdout)
		}
	}

	r = sb.RunCLI("project", "list", "--json")
	r.AssertSuccess(t)
	items := projectJSON(t, r)["projects"].([]any)
	got := map[string]bool{}
	for _, item := range items {
		p := item.(map[string]any)
		got[p["path"].(string)] = p["missing"].(bool)
	}
	if len(got) != 3 || got[root] || !got[missing[0]] || !got[missing[1]] {
		t.Fatalf("incorrect missing flags: %v", got)
	}
	r = sb.RunCLI("project", "list")
	r.AssertSuccess(t)
	for _, path := range missing {
		r.AssertOutputContains(t, "Project folder is missing: "+path)
	}
}
