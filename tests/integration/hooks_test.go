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

const hooksEntryYAML = `description: Log Bash calls
bindings:
  claude:
    events:
      PreToolUse:
        - matcher: Bash
          hooks:
            - type: command
              command: echo skillshare-hooks-test
`

func newHooksSandbox(t *testing.T) *testutil.Sandbox {
	t.Helper()
	sb := testutil.NewSandbox(t)
	sb.SetEnv("CLAUDE_CONFIG_DIR", "")
	sb.WriteConfig("targets: {}\n")
	return sb
}

func writeHooksEntry(t *testing.T, sb *testutil.Sandbox) string {
	t.Helper()
	path := filepath.Join(sb.Root, "entry.yaml")
	sb.WriteFile(path, hooksEntryYAML)
	return path
}

func TestHooksLifecycle_Claude(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	settings := filepath.Join(sb.Home, ".claude", "settings.json")
	sb.WriteFile(settings, `{"theme":"dark"}`)
	entry := writeHooksEntry(t, sb)

	sb.RunCLI("hooks", "add", "bash-log", "--file", entry, "-g").AssertSuccess(t)
	if strings.Contains(sb.ReadFile(settings), "skillshare-hooks-test") {
		t.Fatal("add without --sync must only save the source")
	}
	sb.RunCLI("hooks", "sync", "--dry-run", "-g").AssertSuccess(t)
	if strings.Contains(sb.ReadFile(settings), "skillshare-hooks-test") {
		t.Fatal("dry-run wrote native configuration")
	}

	r := sb.RunCLI("hooks", "sync", "--json", "-g")
	r.AssertSuccess(t)
	var synced struct {
		BackupIDs []string `json:"backupIds"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &synced); err != nil {
		t.Fatalf("sync --json is not JSON: %v\n%s", err, r.Stdout)
	}
	written := sb.ReadFile(settings)
	if !strings.Contains(written, "skillshare-hooks-test") || !strings.Contains(written, `"theme"`) {
		t.Fatalf("sync did not merge the hook into settings.json:\n%s", written)
	}
	sb.RunCLI("hooks", "sync", "-g").AssertSuccess(t)
	if sb.ReadFile(settings) != written {
		t.Fatal("repeated sync changed settings.json")
	}
	if len(synced.BackupIDs) == 0 {
		t.Fatal("sync of an existing settings.json made no backup")
	}
	restore := sb.RunCLI("hooks", "restore", synced.BackupIDs[0], "--dry-run", "-g")
	restore.AssertSuccess(t)
	restore.AssertOutputContains(t, "Hooks source:")
	if sb.ReadFile(settings) != written {
		t.Fatal("restore --dry-run changed settings.json")
	}

	sb.RunCLI("hooks", "disable", "bash-log", "--sync", "-g").AssertSuccess(t)
	if strings.Contains(sb.ReadFile(settings), "skillshare-hooks-test") {
		t.Fatal("disable --sync kept the native hook")
	}
	list := sb.RunCLI("hooks", "list", "-g")
	list.AssertSuccess(t)
	list.AssertOutputContains(t, "disabled")

	sb.RunCLI("hooks", "enable", "bash-log", "--sync", "-g").AssertSuccess(t)
	if !strings.Contains(sb.ReadFile(settings), "skillshare-hooks-test") {
		t.Fatal("enable --sync did not republish the hook")
	}

	sb.RunCLI("hooks", "remove", "bash-log", "--sync", "-g").AssertSuccess(t)
	after := sb.ReadFile(settings)
	if strings.Contains(after, "skillshare-hooks-test") || !strings.Contains(after, `"theme"`) {
		t.Fatalf("remove --sync did not prune only the owned hook:\n%s", after)
	}
}

func TestHooksAddAndEditGuardExistence(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	entry := writeHooksEntry(t, sb)

	sb.RunCLI("hooks", "edit", "bash-log", "--file", entry, "-g").AssertFailure(t)
	sb.RunCLI("hooks", "add", "bash-log", "--file", entry, "-g").AssertSuccess(t)
	again := sb.RunCLI("hooks", "add", "bash-log", "--file", entry, "-g")
	again.AssertFailure(t)
	again.AssertAnyOutputContains(t, "already exists")
	sb.RunCLI("hooks", "edit", "bash-log", "--file", entry, "-g").AssertSuccess(t)
	sb.RunCLI("hooks", "enable", "missing", "-g").AssertFailure(t)
}

func TestHooksJSONOutputIsMachineReadable(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	entry := writeHooksEntry(t, sb)

	preview := sb.RunCLI("hooks", "add", "bash-log", "--file", entry, "--dry-run", "--json", "-g")
	preview.AssertSuccess(t)
	var plan struct {
		Revision string `json:"revision"`
	}
	if err := json.Unmarshal([]byte(preview.Stdout), &plan); err != nil || plan.Revision == "" {
		t.Fatalf("dry-run JSON has no revision: %v\n%s", err, preview.Stdout)
	}
	if strings.Contains(sb.ReadFile(sb.ConfigPath), "bash-log") {
		t.Fatal("dry-run saved the source")
	}

	sb.RunCLI("hooks", "add", "bash-log", "--file", entry, "-g").AssertSuccess(t)
	list := sb.RunCLI("hooks", "--json", "-g")
	list.AssertSuccess(t)
	var inventory struct {
		Source struct {
			Entries map[string]json.RawMessage `json:"entries"`
		} `json:"source"`
	}
	if err := json.Unmarshal([]byte(list.Stdout), &inventory); err != nil {
		t.Fatalf("list --json is not JSON: %v\n%s", err, list.Stdout)
	}
	if _, ok := inventory.Source.Entries["bash-log"]; !ok {
		t.Fatalf("list --json omitted the entry:\n%s", list.Stdout)
	}

	failed := sb.RunCLI("hooks", "enable", "missing", "--json", "-g")
	failed.AssertFailure(t)
	var errOut struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(failed.Stdout), &errOut); err != nil || errOut.Error == "" {
		t.Fatalf("JSON failure is not a JSON error: %v\n%s", err, failed.Stdout)
	}
}

func TestHooksRevisionMustMatchPreview(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	entry := writeHooksEntry(t, sb)
	sb.RunCLI("hooks", "add", "bash-log", "--file", entry, "-g").AssertSuccess(t)

	r := sb.RunCLI("hooks", "sync", "--revision", "stale", "-g")
	r.AssertFailure(t)
	r.AssertAnyOutputContains(t, "changed since preview")
	if _, err := os.Stat(filepath.Join(sb.Home, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("stale revision wrote native configuration")
	}
}

func TestHooksRejectsInapplicableArguments(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	for _, args := range [][]string{
		{"hooks", "list", "--sync"},
		{"hooks", "sync", "bash-log"},
		{"hooks", "restore", "id", "--replace"},
		{"hooks", "add", "bash-log"},
		{"hooks", "import"},
		{"hooks", "restore"},
		{"hooks", "bogus"},
		{"hooks", "list", "--unknown"},
	} {
		sb.RunCLI(append(args, "-g")...).AssertFailure(t)
	}
}

func TestHooksImportDoesNotExecute(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	marker := filepath.Join(sb.Root, "executed")
	settings := filepath.Join(sb.Home, ".claude", "settings.json")
	sb.WriteFile(settings, `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"touch `+marker+`"}]}]}}`)

	r := sb.RunCLI("hooks", "import", "--from", "claude", "--json", "-g")
	r.AssertSuccess(t)
	var candidates []struct {
		Name     string   `json:"name"`
		Problems []string `json:"problems"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &candidates); err != nil || len(candidates) == 0 {
		t.Fatalf("import produced no candidates: %v\n%s", err, r.Stdout)
	}
	name := candidates[0].Name
	sb.RunCLI("hooks", "import", "--from", "claude", name, "--dry-run", "-g").AssertSuccess(t)
	if strings.Contains(sb.ReadFile(sb.ConfigPath), "touch") {
		t.Fatal("import --dry-run saved the source")
	}
	sb.RunCLI("hooks", "import", "--from", "claude", name, "-g").AssertSuccess(t)
	sb.RunCLI("hooks", "list", "-g").AssertOutputContains(t, name)
	// Importing takes the registration over, so sync needs no --replace.
	sb.RunCLI("hooks", "sync", "-g").AssertSuccess(t)
	if n := strings.Count(sb.ReadFile(settings), "touch "); n != 1 {
		t.Fatalf("taking over the imported hook left %d registrations, want 1", n)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("import executed a hook command")
	}
}

func TestHooksProjectModeWritesProjectFiles(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	project := sb.SetupProjectDir()
	entry := writeHooksEntry(t, sb)

	sb.RunCLIInDir(project, "hooks", "add", "bash-log", "--file", entry, "--sync", "-p").AssertSuccess(t)
	if !strings.Contains(sb.ReadFile(filepath.Join(project, ".claude", "settings.json")), "skillshare-hooks-test") {
		t.Fatal("project sync did not write .claude/settings.json in the project")
	}
	if _, err := os.Stat(filepath.Join(sb.Home, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("project sync wrote the global Claude settings")
	}
	if strings.Contains(sb.ReadFile(sb.ConfigPath), "bash-log") {
		t.Fatal("project add saved to the global config")
	}
}

func TestSyncHooksAlias(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	settings := filepath.Join(sb.Home, ".claude", "settings.json")
	sb.RunCLI("hooks", "add", "bash-log", "--file", writeHooksEntry(t, sb), "-g").AssertSuccess(t)

	sb.RunCLI("sync", "hooks", "--dry-run", "-g").AssertSuccess(t)
	if sb.FileExists(settings) {
		t.Fatal("sync hooks --dry-run wrote settings.json")
	}
	sb.RunCLI("sync", "hooks", "bash-log", "--replace", "-g").AssertFailure(t)
	sb.RunCLI("sync", "-g", "hooks").AssertSuccess(t)
	if !strings.Contains(sb.ReadFile(settings), "skillshare-hooks-test") {
		t.Fatal("sync hooks did not write settings.json")
	}
}

func TestSyncWithoutAllLeavesHooks(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	sb.RunCLI("hooks", "add", "bash-log", "--file", writeHooksEntry(t, sb), "-g").AssertSuccess(t)

	sb.RunCLI("sync", "-g").AssertSuccess(t)
	if sb.FileExists(filepath.Join(sb.Home, ".claude", "settings.json")) {
		t.Fatal("plain sync applied hooks")
	}
}

func TestSyncAllIncludesHooks(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	settings := filepath.Join(sb.Home, ".claude", "settings.json")
	sb.RunCLI("hooks", "add", "bash-log", "--file", writeHooksEntry(t, sb), "-g").AssertSuccess(t)

	preview := sb.RunCLI("sync", "--all", "--dry-run", "--json", "-g")
	preview.AssertSuccess(t)
	var output struct {
		Hooks struct {
			Plan struct {
				Changes []struct {
					Name string `json:"name"`
				} `json:"changes"`
			} `json:"plan"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(preview.Stdout), &output); err != nil || len(output.Hooks.Plan.Changes) == 0 {
		t.Fatalf("sync --all --dry-run omitted hooks: %v\n%s", err, preview.Stdout)
	}
	if sb.FileExists(settings) {
		t.Fatal("sync --all --dry-run wrote settings.json")
	}
	sb.RunCLI("sync", "--all", "-g").AssertSuccess(t)
	if !strings.Contains(sb.ReadFile(settings), "skillshare-hooks-test") {
		t.Fatal("sync --all did not apply hooks")
	}
}

func TestSyncAllHooksConflictStopsEveryResource(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\nmcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n")
	sb.RunCLI("hooks", "add", "bash-log", "--file", writeHooksEntry(t, sb), "-g").AssertSuccess(t)
	sb.WriteFile(filepath.Join(sb.Home, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo skillshare-hooks-test"}]}]}}`)

	r := sb.RunCLI("sync", "--all", "--json", "-g")
	r.AssertFailure(t)
	var output struct {
		Error string `json:"error"`
		Hooks struct {
			Plan struct {
				Blocked bool `json:"blocked"`
			} `json:"plan"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &output); err != nil || !strings.Contains(output.Error, "hooks conflicts") || !output.Hooks.Plan.Blocked {
		t.Fatalf("conflict not reported as JSON: %v\n%s", err, r.Stdout)
	}
	if sb.FileExists(filepath.Join(sb.Home, ".claude.json")) {
		t.Fatal("a hooks conflict still let MCP sync")
	}
}

func TestHooksImport_TakesOverAndPlansShowEventDetail(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	settings := filepath.Join(sb.Home, ".claude", "settings.json")
	sb.WriteFile(settings, `{"model":"opus","hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo stop","timeout":5}]}],"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo guard"}]}]}}`)

	sb.RunCLI("hooks", "import", "--from", "claude", "claude-stop", "-g").AssertSuccess(t)
	r := sb.RunCLI("hooks", "sync", "--dry-run", "-g")
	r.AssertSuccess(t)
	r.AssertOutputContains(t, "adopt")
	r.AssertOutputNotContains(t, "conflict")

	both := filepath.Join(sb.Root, "both.yaml")
	sb.WriteFile(both, "bindings:\n  claude:\n    events:\n      Stop: [{hooks: [{type: command, command: echo stop, timeout: 5}]}]\n      Stopp: [{hooks: [{type: command, command: echo typo}]}]\n")
	sb.RunCLI("hooks", "edit", "claude-stop", "--file", both, "--sync", "-g").AssertSuccess(t)
	narrowed := filepath.Join(sb.Root, "narrowed.yaml")
	sb.WriteFile(narrowed, "bindings:\n  claude:\n    events:\n      Stop: [{hooks: [{type: command, command: echo stop, timeout: 5}]}]\n")
	r = sb.RunCLI("hooks", "edit", "claude-stop", "--file", both, "--dry-run", "-g")
	r.AssertSuccess(t)
	r.AssertOutputContains(t, `does not document the event "Stopp"`)
	r = sb.RunCLI("hooks", "edit", "claude-stop", "--file", narrowed, "--dry-run", "-g")
	r.AssertSuccess(t)
	r.AssertOutputContains(t, "~/.claude/settings.json  − Stopp")
	r.AssertOutputNotContains(t, "remove")
}
