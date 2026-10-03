//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

const (
	extrasImportBegin = "<!-- skillshare:instructions:begin -->"
	extrasImportEnd   = "<!-- skillshare:instructions:end -->"
)

// setupSingleFileExtra creates extras/<name>/<file> plus an unrelated file in
// the same source directory, and returns the source file path.
func setupSingleFileExtra(t *testing.T, sb *testutil.Sandbox, name, file, content string) string {
	t.Helper()
	dir := filepath.Join(filepath.Dir(sb.SourcePath), "extras", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(filepath.Join(dir, file), content)
	sb.WriteFile(filepath.Join(dir, "notes.md"), "not part of the extra")
	return filepath.Join(dir, file)
}

func singleFileConfig(sb *testutil.Sandbox, extras string) string {
	return "source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + sb.CreateTarget("claude") + "\nextras:\n" + extras
}

func extrasListStatuses(t *testing.T, sb *testutil.Sandbox) map[string]string {
	t.Helper()
	result := sb.RunCLI("extras", "list", "--json")
	result.AssertSuccess(t)
	var entries []struct {
		Name    string `json:"name"`
		Targets []struct {
			Status string `json:"status"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &entries); err != nil {
		t.Fatalf("parse extras list: %v\n%s", err, result.Stdout)
	}
	statuses := map[string]string{}
	for _, e := range entries {
		for _, tg := range e.Targets {
			statuses[e.Name] = tg.Status
		}
	}
	return statuses
}

func TestExtrasFile_SymlinkWithAsAndCopy(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("placeholder", map[string]string{"SKILL.md": "# P"})
	src := setupSingleFileExtra(t, sb, "instructions", "AGENTS.md", "# shared")
	claudeDir := filepath.Join(sb.Home, ".claude")
	codexDir := filepath.Join(sb.Home, ".codex")
	sb.WriteConfig(singleFileConfig(sb, `  - name: instructions
    file: AGENTS.md
    targets:
      - path: `+claudeDir+`
        mode: symlink
        as: CLAUDE.md
      - path: `+codexDir+`
        mode: copy
`))

	sb.RunCLI("sync", "extras").AssertSuccess(t)

	if got := sb.SymlinkTarget(filepath.Join(claudeDir, "CLAUDE.md")); got != src {
		t.Errorf("CLAUDE.md link = %q, want %q", got, src)
	}
	if got := sb.ReadFile(filepath.Join(codexDir, "AGENTS.md")); got != "# shared" {
		t.Errorf("codex copy = %q", got)
	}
	if sb.FileExists(filepath.Join(claudeDir, "notes.md")) {
		t.Error("only the extra's file should be synced")
	}
	if got := extrasListStatuses(t, sb)["instructions"]; got != "synced" {
		t.Errorf("list status = %q, want synced", got)
	}
	diff := sb.RunCLI("diff", "--json")
	diff.AssertSuccess(t)
	if strings.Contains(diff.Stdout, "source directory not found") {
		t.Errorf("diff should understand single-file extras:\n%s", diff.Stdout)
	}
	sb.RunCLI("status").AssertSuccess(t)
}

func TestExtrasFile_ImportIdempotentAndRemoveKeepsOthers(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("placeholder", map[string]string{"SKILL.md": "# P"})
	srcA := setupSingleFileExtra(t, sb, "instructions", "AGENTS.md", "# shared")
	srcB := setupSingleFileExtra(t, sb, "team", "TEAM.md", "# team")
	claudeDir := filepath.Join(sb.Home, ".claude")
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
	sb.WriteFile(claudeMD, "# My notes\n")
	sb.WriteConfig(singleFileConfig(sb, `  - name: instructions
    file: AGENTS.md
    targets:
      - path: `+claudeDir+`
        mode: import
        as: CLAUDE.md
  - name: team
    file: TEAM.md
    targets:
      - path: `+claudeDir+`
        mode: import
        as: CLAUDE.md
`))

	sb.RunCLI("sync", "extras").AssertSuccess(t)
	sb.RunCLI("sync", "extras").AssertSuccess(t)

	want := extrasImportBegin + "\n@" + srcA + "\n@" + srcB + "\n" + extrasImportEnd + "\n\n# My notes\n"
	if got := sb.ReadFile(claudeMD); got != want {
		t.Fatalf("CLAUDE.md =\n%s\nwant\n%s", got, want)
	}
	if got := extrasListStatuses(t, sb)["instructions"]; got != "synced" {
		t.Errorf("list status = %q, want synced", got)
	}

	sb.RunCLI("extras", "remove", "instructions", "--force").AssertSuccess(t)

	want = extrasImportBegin + "\n@" + srcB + "\n" + extrasImportEnd + "\n\n# My notes\n"
	if got := sb.ReadFile(claudeMD); got != want {
		t.Fatalf("after remove CLAUDE.md =\n%s\nwant\n%s", got, want)
	}
}

func TestExtrasFile_BackupThenReplaceAndRestore(t *testing.T) {
	for _, prior := range []bool{true, false} {
		name := "without prior file"
		if prior {
			name = "with prior file"
		}
		t.Run(name, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.CreateSkill("placeholder", map[string]string{"SKILL.md": "# P"})
			setupSingleFileExtra(t, sb, "instructions", "AGENTS.md", "# shared")
			claudeDir := filepath.Join(sb.Home, ".claude")
			claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
			if prior {
				sb.WriteFile(claudeMD, "# original")
			}
			sb.WriteConfig(singleFileConfig(sb, `  - name: instructions
    file: AGENTS.md
    targets:
      - path: `+claudeDir+`
        as: CLAUDE.md
`))

			sb.RunCLI("sync", "extras").AssertSuccess(t)
			if !sb.IsSymlink(claudeMD) {
				t.Fatal("CLAUDE.md should be replaced by a symlink without --force")
			}

			sb.RunCLI("extras", "remove", "instructions", "--force").AssertSuccess(t)

			if prior {
				if sb.IsSymlink(claudeMD) || sb.ReadFile(claudeMD) != "# original" {
					t.Fatal("remove should restore the backed-up file")
				}
			} else if _, err := os.Lstat(claudeMD); !os.IsNotExist(err) {
				t.Fatal("remove should delete only the symlink skillshare created")
			}
		})
	}
}

func TestExtrasFile_ReattachAfterConfigOnlyRemoveTargetRestoresNewFile(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("placeholder", map[string]string{"SKILL.md": "# P"})
	setupSingleFileExtra(t, sb, "instructions", "AGENTS.md", "# shared")
	claudeDir := filepath.Join(sb.Home, ".claude")
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
	otherDir := filepath.Join(sb.Home, ".other")
	sb.WriteFile(claudeMD, "v1")
	cfg := singleFileConfig(sb, `  - name: instructions
    file: AGENTS.md
    targets:
      - path: `+otherDir+`
      - path: `+claudeDir+`
        as: CLAUDE.md
`)
	sb.WriteConfig(cfg)
	sb.RunCLI("sync", "extras").AssertSuccess(t)

	sb.RunCLI("extras", "instructions", "--remove-target", claudeDir, "-g").AssertSuccess(t)
	os.Remove(claudeMD)
	sb.WriteFile(claudeMD, "v2")
	sb.WriteConfig(cfg)
	sb.RunCLI("sync", "extras").AssertSuccess(t)
	sb.RunCLI("extras", "remove", "instructions", "--force").AssertSuccess(t)

	if got := sb.ReadFile(claudeMD); got != "v2" {
		t.Errorf("CLAUDE.md = %q, want the file present at the second attach", got)
	}
}

func TestExtrasFile_ModifiedStatus(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("placeholder", map[string]string{"SKILL.md": "# P"})
	setupSingleFileExtra(t, sb, "instructions", "AGENTS.md", "# shared")
	claudeDir := filepath.Join(sb.Home, ".claude")
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
	sb.WriteConfig(singleFileConfig(sb, `  - name: instructions
    file: AGENTS.md
    targets:
      - path: `+claudeDir+`
        mode: symlink
        as: CLAUDE.md
`))
	sb.RunCLI("sync", "extras").AssertSuccess(t)
	os.Remove(claudeMD)
	sb.WriteFile(claudeMD, "# edited in the tool")

	if got := extrasListStatuses(t, sb)["instructions"]; got != "modified" {
		t.Errorf("list status = %q, want modified", got)
	}
}

func TestExtrasFile_ImportRequiresFile(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("placeholder", map[string]string{"SKILL.md": "# P"})
	rules := filepath.Join(sb.Home, ".claude", "rules")
	sb.WriteConfig(singleFileConfig(sb, `  - name: rules
    targets:
      - path: `+rules+`
        mode: import
`))

	result := sb.RunCLI("sync")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "import mode requires file")
}

func TestExtrasFile_SkillsTargetRejectsImport(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("placeholder", map[string]string{"SKILL.md": "# P"})
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + sb.CreateTarget("claude") + "\n    mode: import\n")

	result := sb.RunCLI("sync")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid sync mode")
}

func TestExtrasFile_InitRejectsImport(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("placeholder", map[string]string{"SKILL.md": "# P"})
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + sb.CreateTarget("claude") + "\n")

	result := sb.RunCLI("extras", "init", "rules", "--target", filepath.Join(sb.Home, ".claude", "rules"), "--mode", "import")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "import mode requires a single-file extra")
}

func TestExtrasFile_InitFileAsThenSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("placeholder", map[string]string{"SKILL.md": "# P"})
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + sb.CreateTarget("claude") + "\n")
	prompts := filepath.Join(sb.Home, "dotfiles", "prompts")
	piDir := filepath.Join(sb.Home, ".pi", "agent")

	result := sb.RunCLI("extras", "init", "pi-prompt", "--source", prompts, "--file", "system.md", "--target", piDir, "--as", "APPEND_SYSTEM.md")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Created extra pi-prompt (single file)")
	result.AssertOutputContains(t, "Source    ~/dotfiles/prompts/system.md · not found")
	result.AssertOutputContains(t, "Target    ~/.pi/agent/APPEND_SYSTEM.md · merge")
	result.AssertOutputContains(t, "after creating the source file")
	if sb.FileExists(filepath.Join(prompts, "system.md")) {
		t.Error("init created the source file")
	}

	sb.WriteFile(filepath.Join(prompts, "system.md"), "# system")
	sb.RunCLI("sync", "extras").AssertSuccess(t)
	if got := sb.SymlinkTarget(filepath.Join(piDir, "APPEND_SYSTEM.md")); got != filepath.Join(prompts, "system.md") {
		t.Errorf("APPEND_SYSTEM.md link = %q, want the source file", got)
	}

	list := sb.RunCLI("extras", "list", "--no-tui")
	list.AssertSuccess(t)
	list.AssertOutputContains(t, "~/dotfiles/prompts/system.md")
	list.AssertOutputContains(t, "~/.pi/agent/APPEND_SYSTEM.md")
}

func TestExtrasFile_InitSharedSourceDirSyncsOwnFileOnly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("placeholder", map[string]string{"SKILL.md": "# P"})
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + sb.CreateTarget("claude") + "\n")
	prompts := filepath.Join(sb.Home, "dotfiles", "prompts")
	sb.WriteFile(filepath.Join(prompts, "a.md"), "a")
	sb.WriteFile(filepath.Join(prompts, "b.md"), "b")
	aDir, bDir := filepath.Join(sb.Home, "out-a"), filepath.Join(sb.Home, "out-b")

	sb.RunCLI("extras", "init", "a", "--source", prompts, "--file", "a.md", "--target", aDir).AssertSuccess(t)
	result := sb.RunCLI("extras", "init", "b", "--source", prompts, "--file", "b.md", "--target", bDir, "--mode", "copy")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "skillshare sync extras")
	sb.RunCLI("sync", "extras").AssertSuccess(t)

	if got := sb.ListDir(aDir); len(got) != 1 || got[0] != "a.md" {
		t.Errorf("out-a = %v, want [a.md]", got)
	}
	if got := sb.ListDir(bDir); len(got) != 1 || got[0] != "b.md" || sb.IsSymlink(filepath.Join(bDir, "b.md")) {
		t.Errorf("out-b = %v, want a copied b.md only", got)
	}
}

func TestExtrasFile_InitRejectsFlattenAndDirectorySource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("placeholder", map[string]string{"SKILL.md": "# P"})
	cfg := "source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + sb.CreateTarget("claude") + "\n"
	sb.WriteConfig(cfg)
	prompts := filepath.Join(sb.Home, "prompts")
	if err := os.MkdirAll(filepath.Join(prompts, "system.md"), 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(sb.Home, "out")

	result := sb.RunCLI("extras", "init", "p", "--file", "system.md", "--target", target, "--flatten")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "flatten cannot be used with a single-file extra")
	result = sb.RunCLI("extras", "init", "p", "--source", prompts, "--file", "system.md", "--target", target)
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "is a directory")
	if got := sb.ReadFile(sb.ConfigPath); strings.Contains(got, "name: p") {
		t.Errorf("config changed after rejected init:\n%s", got)
	}
}

// Issue #300: several project single-file extras share one source folder.
func TestExtrasFile_ProjectSharedSourceFolder(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	prompts := filepath.Join(projectRoot, ".skillshare", "extras", "prompts")
	if err := os.MkdirAll(prompts, 0755); err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(filepath.Join(prompts, "a.md"), "# a")
	sb.WriteFile(filepath.Join(prompts, "b.md"), "# b")

	for _, name := range []string{"a", "b"} {
		sb.RunCLIInDir(projectRoot, "extras", "init", name, "-p",
			"--source", ".skillshare/extras/prompts", "--file", name+".md",
			"--target", ".claude/commands", "--mode", "copy").AssertSuccess(t)
	}
	if got := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml")); !strings.Contains(got, "source: .skillshare/extras/prompts") {
		t.Errorf("project config does not keep the relative source:\n%s", got)
	}

	sb.RunCLIInDir(projectRoot, "sync", "extras", "-p").AssertSuccess(t)

	out := filepath.Join(projectRoot, ".claude", "commands")
	for _, name := range []string{"a", "b"} {
		if got := sb.ReadFile(filepath.Join(out, name+".md")); got != "# "+name {
			t.Errorf("%s.md = %q, want %q", name, got, "# "+name)
		}
	}

	// Removing one extra keeps the folder and the other extra's file.
	sb.RunCLIInDir(projectRoot, "extras", "remove", "a", "-p", "--force").AssertSuccess(t)
	if got := sb.ReadFile(filepath.Join(prompts, "b.md")); got != "# b" {
		t.Errorf("b.md after removing a = %q, want %q", got, "# b")
	}
}
