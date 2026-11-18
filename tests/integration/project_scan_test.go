//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

// scanFixture builds a workspace of checkouts, a linked worktree, a plain
// folder and folders the scan must ignore.
func scanFixture(t *testing.T) (*testutil.Sandbox, string) {
	t.Helper()
	sb, _ := projectFixture(t)
	suite := filepath.Join(sb.Home, "suite")
	for _, dir := range []string{"app-a", "app-b", "node_modules/pkg", "nested/deep/lib"} {
		mkCheckout(t, filepath.Join(suite, dir))
	}
	mkWorktree(t, filepath.Join(suite, ".worktrees", "app-a-fix"))
	if err := os.MkdirAll(filepath.Join(suite, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	return sb, suite
}

func mkCheckout(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
}

// A linked worktree has a .git file instead of a directory.
func mkWorktree(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /elsewhere\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func scanRows(t *testing.T, payload map[string]any) map[string]map[string]any {
	t.Helper()
	rows := map[string]map[string]any{}
	list, _ := payload["scan"].([]any)
	for _, v := range list {
		row := v.(map[string]any)
		rows[row["root"].(string)] = row
	}
	return rows
}

func loadProjects(t *testing.T, sb *testutil.Sandbox) map[string]config.ManagedProject {
	t.Helper()
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Projects
}

func TestProjectScan_RegistersCheckouts(t *testing.T) {
	sb, suite := scanFixture(t)
	r := sb.RunCLI("project", "add", suite, "--scan", ".", "--scan", ".worktrees", "--target", "codex", "--skills-include", "team-*", "--json")
	r.AssertSuccess(t)
	rows := scanRows(t, projectJSON(t, r))

	projects := loadProjects(t, sb)
	for _, rel := range []string{"", "app-a", "app-b", ".worktrees/app-a-fix", "nested/deep/lib"} {
		root := filepath.Join(suite, rel)
		p, ok := projects[root]
		if !ok {
			t.Fatalf("%s not registered; projects: %v", root, projects)
		}
		if p.Name != "" || p.Skills == nil || len(p.Skills.Include) != 1 {
			t.Fatalf("%s registered with %+v", root, p)
		}
		if rows[root]["result"] != "added" {
			t.Fatalf("%s row: %v", root, rows[root])
		}
	}
	for _, rel := range []string{"docs", "node_modules/pkg"} {
		if _, ok := projects[filepath.Join(suite, rel)]; ok {
			t.Fatalf("%s must not be registered", rel)
		}
	}
}

func TestProjectScan_DryRunWritesNothing(t *testing.T) {
	sb, suite := scanFixture(t)
	before := sb.ReadFile(sb.ConfigPath)
	r := sb.RunCLI("project", "add", suite, "--scan", ".", "--target", "codex", "-n", "--json")
	r.AssertSuccess(t)
	payload := projectJSON(t, r)
	if payload["applied"] != false || scanRows(t, payload)[filepath.Join(suite, "app-a")]["result"] != "added" {
		t.Fatalf("preview: %s", r.Stdout)
	}
	if sb.ReadFile(sb.ConfigPath) != before {
		t.Fatal("dry run wrote config.yaml")
	}
}

func TestProjectScan_RescanOnlyAdds(t *testing.T) {
	sb, suite := scanFixture(t)
	args := []string{"project", "add", suite, "--scan", ".", "--target", "codex", "--json"}
	sb.RunCLI(args...).AssertSuccess(t)
	sb.RunCLI("project", "edit", filepath.Join(suite, "app-b"), "--target", "codex,claude").AssertSuccess(t)
	mkCheckout(t, filepath.Join(suite, "tools"))
	sb.WriteFile(filepath.Join(suite, "tools", ".skillshare", "config.yaml"), "targets: {}\n")
	mkCheckout(t, filepath.Join(suite, "app-c"))

	r := sb.RunCLI(args...)
	r.AssertSuccess(t)
	rows := scanRows(t, projectJSON(t, r))
	want := map[string]string{"": "kept", "app-a": "kept", "app-b": "skipped", "tools": "skipped", "app-c": "added"}
	for rel, result := range want {
		if row := rows[filepath.Join(suite, rel)]; row["result"] != result {
			t.Fatalf("%q: got %v, want %s", rel, row, result)
		}
	}
	if reason, _ := rows[filepath.Join(suite, "app-b")]["reason"].(string); !strings.Contains(reason, "skillshare project edit app-b") {
		t.Fatalf("app-b reason: %q", reason)
	}
	if reason, _ := rows[filepath.Join(suite, "tools")]["reason"].(string); !strings.Contains(reason, ".skillshare") {
		t.Fatalf("tools reason: %q", reason)
	}
	projects := loadProjects(t, sb)
	if got := projects[filepath.Join(suite, "app-b")].Targets; len(got) != 2 {
		t.Fatalf("rescan changed app-b's targets: %v", got)
	}
	if _, ok := projects[filepath.Join(suite, "tools")]; ok {
		t.Fatal("tools has its own config and must not be registered")
	}

	before := sb.ReadFile(sb.ConfigPath)
	sb.RunCLI(args...).AssertSuccess(t)
	if sb.ReadFile(sb.ConfigPath) != before {
		t.Fatal("a rescan with nothing new rewrote config.yaml")
	}
}

func TestProjectScan_LeavesRegisteredRootAlone(t *testing.T) {
	sb, suite := scanFixture(t)
	sb.RunCLI("project", "add", suite, "--target", "claude").AssertSuccess(t)
	r := sb.RunCLI("project", "add", suite, "--scan", ".", "--target", "codex", "--json")
	r.AssertSuccess(t)
	payload := projectJSON(t, r)
	if row := scanRows(t, payload)[suite]; row["result"] != "skipped" {
		t.Fatalf("root row: %v", row)
	}
	if project, _ := payload["project"].(map[string]any); project["root"] != suite {
		t.Fatalf("project must be the root: %v", payload["project"])
	}
	projects := loadProjects(t, sb)
	if got := projects[suite].Targets; len(got) != 1 || got[0] != "claude" {
		t.Fatalf("root targets changed: %v", got)
	}
	if got := projects[filepath.Join(suite, "app-a")].Targets; len(got) != 1 || got[0] != "codex" {
		t.Fatalf("app-a targets: %v", got)
	}
}

func TestProjectScan_MachineLocal(t *testing.T) {
	sb, suite := scanFixture(t)
	before := sb.ReadFile(sb.ConfigPath)
	sb.RunCLI("project", "add", suite, "--scan", ".", "--target", "codex", "--machine-local").AssertSuccess(t)
	if sb.ReadFile(sb.ConfigPath) != before {
		t.Fatal("--machine-local changed config.yaml")
	}
	if local := localRegistry(t, sb.ConfigPath); !strings.Contains(local, filepath.Join(suite, "app-a")) {
		t.Fatalf("local registry lacks app-a:\n%s", local)
	}
}

func TestProjectScan_SyncsEachCheckout(t *testing.T) {
	sb, suite := scanFixture(t)
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
	r := sb.RunCLI("project", "add", suite, "--scan", ".", "--target", "claude", "--sync")
	r.AssertSuccess(t)
	for _, rel := range []string{"", "app-a", "app-b"} {
		if _, err := os.Stat(filepath.Join(suite, rel, ".claude", "skills", "alpha", "SKILL.md")); err != nil {
			t.Fatalf("%q not synced: %v\n%s", rel, err, r.Stdout)
		}
	}
}

func TestProjectScan_SyncFailureStopsNoOtherCheckout(t *testing.T) {
	sb, suite := scanFixture(t)
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
	// A file where app-a's skills folder belongs makes that target invalid.
	sb.WriteFile(filepath.Join(suite, "app-a", ".claude", "skills"), "blocked")
	args := []string{"project", "add", suite, "--scan", ".", "--target", "claude", "--sync", "--json"}

	before := sb.ReadFile(sb.ConfigPath)
	preview := sb.RunCLI(append(args, "-n")...)
	if preview.ExitCode == 0 || sb.ReadFile(sb.ConfigPath) != before {
		t.Fatalf("preview: exit %d, config changed %v: %s", preview.ExitCode, sb.ReadFile(sb.ConfigPath) != before, preview.Stdout)
	}
	if _, err := os.Stat(filepath.Join(suite, "app-b", ".claude")); err == nil {
		t.Fatal("preview synced app-b")
	}

	r := sb.RunCLI(args...)
	if r.ExitCode == 0 {
		t.Fatalf("a failed checkout sync must exit 1: %s", r.Stdout)
	}
	rows := scanRows(t, projectJSON(t, r))
	if rows[filepath.Join(suite, "app-a")]["error"] == nil || rows[filepath.Join(suite, "app-b")]["error"] != nil {
		t.Fatalf("row errors: %v", rows)
	}
	if _, err := os.Stat(filepath.Join(suite, "app-b", ".claude", "skills", "alpha", "SKILL.md")); err != nil {
		t.Fatalf("app-b not synced after app-a failed: %v", err)
	}
}

func TestProjectScan_Folders(t *testing.T) {
	sb, suite := scanFixture(t)
	for _, dir := range []string{"..", filepath.Join(sb.Home, "other")} {
		r := sb.RunCLI("project", "add", suite, "--scan", dir, "--target", "codex")
		if r.ExitCode == 0 {
			t.Fatalf("--scan %s accepted", dir)
		}
	}
	r := sb.RunCLI("project", "add", suite, "--scan", ".worktrees", "--scan", "later", "--target", "codex", "--json")
	r.AssertSuccess(t)
	if warnings, _ := projectJSON(t, r)["warnings"].([]any); len(warnings) != 1 || !strings.Contains(warnings[0].(string), "later") {
		t.Fatalf("missing folder warning: %s", r.Stdout)
	}
	if r := sb.RunCLI("project", "edit", suite, "--scan", "."); r.ExitCode == 0 {
		t.Fatal("--scan accepted by project edit")
	}
}
