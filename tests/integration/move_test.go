//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

type moveJSON struct {
	Moved []struct {
		Name   string `json:"name"`
		From   string `json:"from"`
		To     string `json:"to"`
		Record bool   `json:"record"`
		Skills int    `json:"skills"`
	} `json:"moved"`
	Failed []struct {
		Name string `json:"name"`
		Code string `json:"code"`
	} `json:"failed"`
	Skipped  int      `json:"skipped"`
	Warnings []string `json:"warnings"`
	DryRun   bool     `json:"dry_run"`
}

func parseMoveJSON(t *testing.T, r *testutil.Result) moveJSON {
	t.Helper()
	var out moveJSON
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
		t.Fatalf("stdout is not the move JSON: %v\n%s", err, r.Stdout)
	}
	return out
}

// movedSandbox holds an installed skill "demo" (with a record and an accepted
// finding) next to a plain local skill, and one target.
func movedSandbox(t *testing.T) (*testutil.Sandbox, string) {
	t.Helper()
	sb := testutil.NewSandbox(t)
	target := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + target + "\n")
	sb.CreateSkill("demo", map[string]string{"SKILL.md": "---\nname: demo\n---\n# Demo"})
	sb.CreateSkill("mine", map[string]string{"SKILL.md": "---\nname: mine\n---\n# Mine"})
	store := install.NewMetadataStore()
	store.Set("demo", &install.MetadataEntry{Source: "github.com/user/repo/demo"})
	store.AuditAccepted = map[string][]string{"demo": {"accepted-key"}}
	if err := store.Save(sb.SourcePath); err != nil {
		t.Fatal(err)
	}
	return sb, target
}

func TestMove_MovesSkillKeepsRecordAndSuggestsSync(t *testing.T) {
	sb, target := movedSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("move", "demo", "grp")

	result.AssertSuccess(t)
	result.AssertRowContains(t, "demo", "→ grp/demo")
	result.AssertAnyOutputContains(t, "skillshare sync")
	if !sb.FileExists(filepath.Join(sb.SourcePath, "grp", "demo", "SKILL.md")) || sb.FileExists(filepath.Join(sb.SourcePath, "demo")) {
		t.Fatal("skill did not move on disk")
	}
	store, err := install.LoadMetadata(sb.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := store.Get("grp/demo"); got == nil || got.Source != "github.com/user/repo/demo" || got.Group != "grp" || store.Has("demo") {
		t.Errorf("store keys = %v, want the record under grp/demo", store.List())
	}
	if len(store.AuditAccepted["grp/demo"]) != 1 {
		t.Errorf("accepted findings = %v, want them carried", store.AuditAccepted)
	}

	// move never syncs; after sync the target holds the new flat name only.
	sb.RunCLI("sync").AssertSuccess(t)
	if !sb.FileExists(filepath.Join(target, "grp__demo")) || sb.FileExists(filepath.Join(target, "demo")) {
		t.Errorf("target entries = %v, want grp__demo and no demo", sb.ListDir(target))
	}
}

func TestMove_DryRunChangesNothing(t *testing.T) {
	sb, _ := movedSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("move", "demo", "grp", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "would move to grp/demo")
	result.AssertAnyOutputContains(t, "Dry run")
	if !sb.FileExists(filepath.Join(sb.SourcePath, "demo", "SKILL.md")) || sb.FileExists(filepath.Join(sb.SourcePath, "grp")) {
		t.Error("dry run changed the source")
	}
	if store, _ := install.LoadMetadata(sb.SourcePath); !store.Has("demo") {
		t.Error("dry run re-keyed the store")
	}
}

func TestMove_JSON(t *testing.T) {
	sb, _ := movedSandbox(t)
	defer sb.Cleanup()
	sb.CreateNestedSkill("frontend/a", map[string]string{"SKILL.md": "---\nname: a\n---\n"})
	sb.CreateNestedSkill("frontend/b", map[string]string{"SKILL.md": "---\nname: b\n---\n"})

	result := sb.RunCLI("move", "demo", "frontend", "ghost", "archive", "--json")

	result.AssertFailure(t) // one name failed
	out := parseMoveJSON(t, result)
	if len(out.Moved) != 2 || out.Moved[0].To != "archive/demo" || !out.Moved[0].Record || out.Moved[0].Skills != 1 {
		t.Errorf("moved = %+v, want demo with its record", out.Moved)
	}
	if out.Moved[1].To != "archive/frontend" || out.Moved[1].Skills != 2 || out.Moved[1].Record {
		t.Errorf("moved folder = %+v, want 2 skills and no record", out.Moved[1])
	}
	if len(out.Failed) != 1 || out.Failed[0].Name != "ghost" || out.Failed[0].Code != "skill_not_found" {
		t.Errorf("failed = %+v, want ghost: skill_not_found", out.Failed)
	}
}

func TestMove_RefusesWithoutChangingAnything(t *testing.T) {
	sb, _ := movedSandbox(t)
	defer sb.Cleanup()
	sb.CreateNestedSkill("grp/demo", map[string]string{"SKILL.md": "---\nname: demo\n---\n"})

	result := sb.RunCLI("move", "demo", "grp", "--json")

	result.AssertFailure(t)
	out := parseMoveJSON(t, result)
	if len(out.Moved) != 0 || len(out.Failed) != 1 || out.Failed[0].Code != "dest_exists" {
		t.Fatalf("output = %+v, want one dest_exists failure", out)
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "demo", "SKILL.md")) {
		t.Error("a refused move touched the skill")
	}
}

func TestMove_FolderWithTrackedRepoIsRefusedWhole(t *testing.T) {
	sb, _ := movedSandbox(t)
	defer sb.Cleanup()
	sb.CreateNestedSkill("frontend/a", map[string]string{"SKILL.md": "---\nname: a\n---\n"})
	if err := os.MkdirAll(filepath.Join(sb.SourcePath, "frontend", "_vendor", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := sb.RunCLI("move", "frontend", "archive", "--json")

	result.AssertFailure(t)
	out := parseMoveJSON(t, result)
	if len(out.Failed) != 1 || out.Failed[0].Code != "inside_tracked_repo" {
		t.Fatalf("failed = %+v, want inside_tracked_repo", out.Failed)
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "frontend", "a", "SKILL.md")) || sb.FileExists(filepath.Join(sb.SourcePath, "archive")) {
		t.Error("part of the refused folder moved")
	}
}

func TestMove_RequiresNameAndDestination(t *testing.T) {
	sb, _ := movedSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("move", "demo")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "skillshare move <skill|folder>... <dest-folder>")
}

func TestMoveProject_CarriesPinGroupAndGitignore(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	root := sb.SetupProjectDir("claude")
	const pinned = "0123456789abcdef0123456789abcdef01234567"
	sb.WriteProjectConfig(root, "targets:\n  - claude\nskills:\n  - name: demo\n    source: github.com/user/repo/demo\n")
	sb.CreateProjectSkill(root, "demo", map[string]string{"SKILL.md": "---\nname: demo\n---\n# Demo"})
	skills := filepath.Join(root, ".skillshare", "skills")
	store := install.NewMetadataStore()
	store.Set("demo", &install.MetadataEntry{Source: "github.com/user/repo/demo", Commit: strings.Repeat("b", 40)})
	if err := store.Save(skills); err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(filepath.Join(root, ".skillshare", "skills.lock.json"),
		`{"version":1,"skills":{"demo":{"source":"github.com/user/repo/demo","commit":"`+pinned+`"}}}`)
	sb.WriteFile(filepath.Join(root, ".skillshare", ".gitignore"),
		"# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\nskills/demo/\n# END SKILLSHARE MANAGED\n")

	result := sb.RunCLIInDir(root, "move", "demo", "grp", "-p", "--json")

	result.AssertSuccess(t)
	out := parseMoveJSON(t, result)
	if len(out.Moved) != 1 || out.Moved[0].To != "grp/demo" || !out.Moved[0].Record {
		t.Fatalf("moved = %+v", out.Moved)
	}
	lock := sb.ReadFile(filepath.Join(root, ".skillshare", "skills.lock.json"))
	if !strings.Contains(lock, `"grp/demo"`) || !strings.Contains(lock, pinned) || strings.Contains(lock, `"demo": {`) {
		t.Errorf("skills.lock.json = %s, want the older pin under grp/demo only", lock)
	}
	cfg := sb.ReadFile(filepath.Join(root, ".skillshare", "config.yaml"))
	if !strings.Contains(cfg, "group: grp") {
		t.Errorf("config.yaml = %s, want the skill in group grp", cfg)
	}
	ignore := sb.ReadFile(filepath.Join(root, ".skillshare", ".gitignore"))
	if strings.Contains(ignore, "skills/demo/") || !strings.Contains(ignore, "skills/grp/demo/") {
		t.Errorf(".gitignore = %s, want the entry moved", ignore)
	}
}

// install --into shares move's destination preflight: it must not put a skill
// inside another skill.
func TestInstallInto_RefusesFolderInsideSkill(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	sb.CreateSkill("holder", map[string]string{"SKILL.md": "---\nname: holder\n---\n# Holder"})
	src := filepath.Join(sb.Root, "incoming", "new-skill")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: new-skill\n---\n# New"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := sb.RunCLI("install", src, "--into", "holder/sub")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "dest_is_skill")
	if sb.FileExists(filepath.Join(sb.SourcePath, "holder", "sub")) {
		t.Error("a refused install created the folder")
	}
}

// An unreadable .metadata.json stops the move before anything is renamed:
// saving an empty store over it would lose every record.
func TestMove_RefusesWhenRecordsCannotBeRead(t *testing.T) {
	sb, _ := movedSandbox(t)
	defer sb.Cleanup()
	meta := filepath.Join(sb.SourcePath, install.MetadataFileName)
	sb.WriteFile(meta, "{ not json")

	result := sb.RunCLI("move", "demo", "grp")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "cannot read install records")
	if !sb.FileExists(filepath.Join(sb.SourcePath, "demo", "SKILL.md")) {
		t.Error("the skill moved although its records could not be read")
	}
	if got := sb.ReadFile(meta); got != "{ not json" {
		t.Errorf(".metadata.json was rewritten: %q", got)
	}
}

func TestMove_AlreadyInPlaceIsNotAFailure(t *testing.T) {
	sb, _ := movedSandbox(t)
	defer sb.Cleanup()
	sb.CreateNestedSkill("grp/stay", map[string]string{"SKILL.md": "---\nname: stay\n---\n"})

	result := sb.RunCLI("move", "grp/stay", "grp")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Already in place")
	result.AssertOutputNotContains(t, "Nothing moved")
}

// install --track shares move's destination preflight, like a plain install.
func TestInstallTrackInto_RefusesFolderInsideSkill(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	sb.CreateSkill("holder", map[string]string{"SKILL.md": "---\nname: holder\n---\n# Holder"})
	repo := filepath.Join(sb.Root, "incoming", "team-repo")
	sb.WriteFile(filepath.Join(repo, "pdf", "SKILL.md"), "---\nname: pdf\n---\n# Pdf")
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=t", "-c", "user.email=t@e.c", "-c", "commit.gpgsign=false", "commit", "-qm", "seed"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	result := sb.RunCLI("install", repo, "--track", "--into", "holder/sub")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "dest_is_skill")
	if sb.FileExists(filepath.Join(sb.SourcePath, "holder", "sub")) {
		t.Error("a refused tracked install created the folder")
	}
}
