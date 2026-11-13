//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"skillshare/internal/testutil"
	"strings"
	"testing"
)

func TestExtrasFileRejectsConflictingCLIChanges(t *testing.T) {
	for _, operation := range []string{"mode", "add", "import", "sync"} {
		t.Run(operation, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			setupSingleFileExtra(t, sb, "first", "AGENTS.md", "first")
			setupSingleFileExtra(t, sb, "second", "AGENTS.md", "second")
			dir := filepath.Join(sb.Home, ".codex")
			mode := "import"
			if operation == "add" || operation == "import" {
				mode = "copy"
			}
			second := "      - path: " + dir + "\n        mode: import\n"
			if operation == "add" || operation == "import" {
				second = "      []\n"
			}
			// Use a custom import-capable file for the mixed-mode case.
			cfg := singleFileConfig(sb, "  - name: first\n    file: AGENTS.md\n    targets:\n      - path: "+dir+"\n        mode: "+mode+"\n  - name: second\n    file: AGENTS.md\n    targets:"+"\n"+second)
			if operation == "mode" || operation == "sync" {
				dir = filepath.Join(sb.Home, "custom")
				cfg = strings.ReplaceAll(cfg, filepath.Join(sb.Home, ".codex"), dir)
			}
			if operation == "sync" {
				cfg = strings.Replace(cfg, "mode: import", "mode: copy", 1)
			}
			sb.WriteConfig(cfg)
			sb.RunCLI("extras", "list", "--json").AssertSuccess(t) // Normalize legacy config before checking the rejected mutation.
			before := sb.ReadFile(sb.ConfigPath)
			var args []string
			switch operation {
			case "mode":
				args = []string{"extras", "second", "--mode", "copy", "--target", dir}
			case "add":
				args = []string{"extras", "second", "--add-target", dir}
			case "import":
				args = []string{"extras", "first", "--mode", "import", "--target", dir}
			case "sync":
				args = []string{"sync", "extras"}
			}
			result := sb.RunCLI(args...)
			result.AssertFailure(t)
			if operation == "import" {
				result.AssertOutputContains(t, "does not read @import lines")
			} else {
				result.AssertOutputContains(t, "first")
			}
			if sb.ReadFile(sb.ConfigPath) != before {
				t.Fatal("invalid change persisted")
			}
		})
	}
}

func TestExtrasFileSyncRejectsEscapingAs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupSingleFileExtra(t, sb, "shared", "AGENTS.md", "shared")
	sb.WriteConfig(singleFileConfig(sb, "  - name: shared\n    file: AGENTS.md\n    targets:\n      - path: "+filepath.Join(sb.Home, ".codex")+"\n        as: ../escaped.md\n"))
	result := sb.RunCLI("sync", "extras")
	result.AssertFailure(t)
	result.AssertOutputContains(t, "must be a plain filename")
	if sb.FileExists(filepath.Join(sb.Home, "escaped.md")) {
		t.Fatal("escaped target created")
	}
}

func TestExtrasFileProjectImportSurvivesMove(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	root := filepath.Join(sb.Home, "project")
	sb.WriteFile(filepath.Join(root, ".skillshare", "config.yaml"), "targets:\n  - claude\nextras:\n  - name: shared\n    file: AGENTS.md\n    targets:\n      - path: .\n        as: CLAUDE.md\n        mode: import\n")
	sb.WriteFile(filepath.Join(root, ".skillshare", "extras", "shared", "AGENTS.md"), "shared")
	sb.WriteFile(filepath.Join(root, "CLAUDE.md"), "mine\n")
	sb.RunCLIInDir(root, "sync", "extras", "-p").AssertSuccess(t)
	if got := sb.ReadFile(filepath.Join(root, "CLAUDE.md")); !strings.Contains(got, "@.skillshare/extras/shared/AGENTS.md") || strings.Contains(got, sb.Home) {
		t.Fatalf("import=%q", got)
	}
	moved := root + "-moved"
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	sb.RunCLIInDir(moved, "sync", "extras", "-p").AssertSuccess(t)
	if got := sb.ReadFile(filepath.Join(moved, "CLAUDE.md")); strings.Count(got, "@") != 1 {
		t.Fatalf("import=%q", got)
	}
	sb.RunCLIInDir(moved, "extras", "remove", "shared", "-p", "--force").AssertSuccess(t)
	if got := sb.ReadFile(filepath.Join(moved, "CLAUDE.md")); got != "mine\n" {
		t.Fatalf("restored=%q", got)
	}
}

func TestExtrasFileDryRunAndInvalidStatus(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupSingleFileExtra(t, sb, "shared", "AGENTS.md", "shared")
	dir := filepath.Join(sb.Home, ".codex")
	cfg := singleFileConfig(sb, "  - name: shared\n    file: AGENTS.md\n    targets:\n      - path: "+dir+"\n        mode: symlink\n")
	sb.WriteConfig(cfg)
	sb.RunCLI("sync", "extras").AssertSuccess(t)
	sb.WriteConfig(strings.Replace(cfg, "symlink", "hardlink", 1))
	sb.RunCLI("extras", "list", "--no-tui").AssertOutputContains(t, "invalid mode")
	sb.RunCLI("status").AssertOutputContains(t, "invalid mode")
	if got := extrasListStatuses(t, sb)["shared"]; got != "invalid mode" {
		t.Errorf("status=%q", got)
	}
	sb.WriteConfig(cfg)
	os.Remove(filepath.Join(dir, "AGENTS.md"))
	sb.WriteFile(filepath.Join(dir, "AGENTS.md"), "edited")
	result := sb.RunCLI("sync", "extras", "--dry-run")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "would back up")
}

func TestExtrasFileSyncRefusesOtherSharedSourceLink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	src := setupSingleFileExtra(t, sb, "personal", "AGENTS.md", "personal")
	setupSingleFileExtra(t, sb, "other", "AGENTS.md", "other")
	dir := filepath.Join(sb.Home, ".claude")
	if err := os.Symlink(src, filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig(singleFileConfig(sb, "  - name: personal\n    file: AGENTS.md\n  - name: other\n    file: AGENTS.md\n    targets:\n      - path: "+dir+"\n        as: CLAUDE.md\n        mode: import\n"))
	result := sb.RunCLI("sync", "extras")
	result.AssertFailure(t)
	result.AssertOutputContains(t, "personal")
	if got := sb.ReadFile(src); got != "personal" {
		t.Fatalf("source corrupted: %q", got)
	}
}

func TestExtrasFileRemoveKeepsConfigOnRestoreFailure(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			dir := filepath.Join(sb.Home, ".codex")
			setupSingleFileExtra(t, sb, "shared", "AGENTS.md", "shared")
			cfg := singleFileConfig(sb, "  - name: shared\n    file: AGENTS.md\n    targets:\n      - path: "+dir+"\n        mode: copy\n")
			cwd := sb.Home
			args := []string{"sync", "extras"}
			remove := []string{"extras", "remove", "shared", "--force"}
			configPath := sb.ConfigPath
			if project {
				cwd = filepath.Join(sb.Home, "project")
				configPath = filepath.Join(cwd, ".skillshare", "config.yaml")
				sb.WriteFile(filepath.Join(cwd, ".skillshare", "extras", "shared", "AGENTS.md"), "shared")
				cfg = "targets:\n  - claude\nextras:\n  - name: shared\n    file: AGENTS.md\n    targets:\n      - path: " + dir + "\n        mode: copy\n"
				args = append(args, "-p")
				remove = append(remove, "-p")
			}
			sb.WriteFile(configPath, cfg)
			sb.WriteFile(filepath.Join(dir, "AGENTS.md"), "original")
			sb.RunCLIInDir(cwd, args...).AssertSuccess(t)
			backupRoot := filepath.Join(sb.Home, ".local", "state", "skillshare", "extras", "backups")
			entries, err := os.ReadDir(backupRoot)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				path := filepath.Join(backupRoot, e.Name(), "restore")
				if _, err := os.Stat(path); err == nil {
					os.Remove(path)
					os.Mkdir(path, 0700)
				}
			}
			before := sb.ReadFile(configPath)
			sb.RunCLIInDir(cwd, remove...).AssertFailure(t)
			if got := sb.ReadFile(configPath); got != before {
				t.Fatalf("failed restore removed config: %s", got)
			}
		})
	}
}

func TestExtrasFileProjectListRelativeLink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	root := filepath.Join(sb.Home, "project")
	sb.WriteFile(filepath.Join(root, ".skillshare", "config.yaml"), "targets:\n  - claude\nextras:\n  - name: shared\n    file: AGENTS.md\n    targets:\n      - path: sub\n")
	sb.WriteFile(filepath.Join(root, ".skillshare", "extras", "shared", "AGENTS.md"), "shared")
	sb.RunCLIInDir(root, "sync", "extras", "-p").AssertSuccess(t)
	res := sb.RunCLIInDir(root, "extras", "list", "-p", "--json")
	res.AssertSuccess(t)
	res.AssertOutputContains(t, `"status": "synced"`)
}

func TestExtrasFileJSONErrorExit(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupSingleFileExtra(t, sb, "shared", "AGENTS.md", "shared")
	sb.WriteConfig(singleFileConfig(sb, "  - name: shared\n    file: AGENTS.md\n    targets:\n      - path: "+filepath.Join(sb.Home, ".codex")+"\n"))
	os.Remove(filepath.Join(filepath.Dir(sb.SourcePath), "extras", "shared", "AGENTS.md"))
	res := sb.RunCLI("sync", "extras", "--json")
	res.AssertFailure(t)
	var out map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v: %s", err, res.Stdout)
	}
	res.AssertOutputContains(t, "extras source file does not exist")
}

func TestExtrasFileHelpAndAddAs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupSingleFileExtra(t, sb, "shared", "AGENTS.md", "shared")
	sb.WriteConfig(singleFileConfig(sb, "  - name: shared\n    file: AGENTS.md\n"))
	for _, args := range [][]string{{"extras", "shared"}, {"extras", "shared", "--help"}} {
		res := sb.RunCLI(args...)
		res.AssertSuccess(t)
		res.AssertOutputContains(t, "--add-target")
		res.AssertOutputContains(t, "--as")
	}
	dir := filepath.Join(sb.Home, ".gemini")
	sb.RunCLI("extras", "shared", "--add-target", dir, "--as", "GEMINI.md").AssertSuccess(t)
	sb.RunCLI("sync", "extras").AssertSuccess(t)
	if got := sb.ReadFile(filepath.Join(dir, "GEMINI.md")); got != "shared" {
		t.Fatalf("target=%q", got)
	}
	res := sb.RunCLI("extras", "shared", "--add-target", dir+"-bad", "--as", "../escape")
	res.AssertFailure(t)
	res.AssertOutputContains(t, "must be a plain filename")
}

func TestExtrasFileDetachMessageIsAccurate(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupSingleFileExtra(t, sb, "shared", "AGENTS.md", "shared")
	dir := filepath.Join(sb.Home, ".codex")
	sb.WriteConfig(singleFileConfig(sb, "  - name: shared\n    file: AGENTS.md\n    targets:\n      - path: "+dir+"\n      - path: "+filepath.Join(sb.Home, "other")+"\n"))
	sb.RunCLI("sync", "extras").AssertSuccess(t)
	res := sb.RunCLI("extras", "shared", "--remove-target", dir)
	res.AssertSuccess(t)
	res.AssertOutputContains(t, "left in place and no longer managed")
	if strings.Contains(res.Output(), "clean up orphaned") {
		t.Fatal(res.Output())
	}
	if got := sb.ReadFile(filepath.Join(dir, "AGENTS.md")); got != "shared" {
		t.Fatalf("target=%q", got)
	}
	help := sb.RunCLI("extras", "remove", "--help")
	help.AssertSuccess(t)
	help.AssertOutputContains(t, "single-file targets are restored to their pre-attach state")
	help.AssertOutputContains(t, "Remote extras delete an unedited fetched copy; edited copies stay")
}
