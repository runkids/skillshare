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

const ompHookCode = "export default function (pi) {\n  pi.on(\"tool_call\", async () => {});\n}\n"

// sameCode compares written code with the entry; the YAML round trip through the
// config file may drop the final newline, as for every code Agent.
func sameCode(got string) bool { return strings.TrimSpace(got) == strings.TrimSpace(ompHookCode) }

func newOMPHooksSandbox(t *testing.T) *testutil.Sandbox {
	t.Helper()
	sb := newHooksSandbox(t)
	sb.SetEnv("PI_CODING_AGENT_DIR", "")
	return sb
}

func writeOMPEntry(t *testing.T, sb *testutil.Sandbox, targets ...string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("bindings:\n")
	for _, target := range targets {
		b.WriteString("  " + target + ":\n    code: |\n")
		for _, line := range strings.Split(strings.TrimSuffix(ompHookCode, "\n"), "\n") {
			b.WriteString("      " + line + "\n")
		}
	}
	path := filepath.Join(sb.Root, "omp-entry.yaml")
	sb.WriteFile(path, b.String())
	return path
}

func TestHooksOMP_GlobalLifecycle(t *testing.T) {
	sb := newOMPHooksSandbox(t)
	defer sb.Cleanup()
	agent := filepath.Join(sb.Home, ".omp", "agent")
	mine := filepath.Join(agent, "extensions", "mine.ts")
	guard := filepath.Join(agent, "hooks", "pre", "guard.ts")
	sb.WriteFile(mine, "export default function () {}\n")
	sb.WriteFile(guard, "export default function () {}\n")
	out := filepath.Join(agent, "extensions", "skillshare-tool-log.ts")
	entry := writeOMPEntry(t, sb, "omp")

	sb.RunCLI("hooks", "add", "tool-log", "--file", entry, "--dry-run", "-g").AssertSuccess(t)
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote the extension")
	}
	sb.RunCLI("hooks", "add", "tool-log", "--file", entry, "--sync", "-g").AssertSuccess(t)
	if !sameCode(sb.ReadFile(out)) {
		t.Fatalf("extension not written verbatim:\n%s", sb.ReadFile(out))
	}
	if _, err := os.Stat(filepath.Join(sb.Home, ".pi")); !os.IsNotExist(err) {
		t.Fatal("OMP sync wrote into the Pi home")
	}

	list := sb.RunCLI("hooks", "list", "-g")
	list.AssertSuccess(t)
	list.AssertOutputContains(t, "tool-log")
	list.AssertOutputContains(t, "mine.ts")
	list.AssertOutputContains(t, "guard.ts")

	imported := sb.RunCLI("hooks", "import", "--from", "omp", "--json", "-g")
	imported.AssertSuccess(t)
	imported.AssertOutputContains(t, `"mine"`)
	imported.AssertOutputNotContains(t, "guard")

	// An edit backs the previous extension up; restore shows it without writing.
	sb.WriteFile(entry, strings.Replace(sb.ReadFile(entry), "tool_call", "tool_result", 1))
	sb.RunCLI("hooks", "edit", "tool-log", "--file", entry, "-g").AssertSuccess(t)
	r := sb.RunCLI("hooks", "sync", "--json", "-g")
	r.AssertSuccess(t)
	var synced struct {
		BackupIDs []string `json:"backupIds"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &synced); err != nil || len(synced.BackupIDs) == 0 {
		t.Fatalf("sync over an existing extension made no backup: %v\n%s", err, r.Stdout)
	}
	if !strings.Contains(sb.ReadFile(out), "tool_result") {
		t.Fatal("edit --sync did not rewrite the extension")
	}
	sb.RunCLI("hooks", "restore", synced.BackupIDs[0], "--dry-run", "-g").AssertSuccess(t)
	if !strings.Contains(sb.ReadFile(out), "tool_result") {
		t.Fatal("restore --dry-run changed the extension")
	}

	sb.RunCLI("hooks", "sync", "--revision", "stale", "-g").AssertFailure(t)

	sb.RunCLI("hooks", "disable", "tool-log", "--sync", "-g").AssertSuccess(t)
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("disable --sync kept the extension")
	}
	sb.RunCLI("hooks", "enable", "tool-log", "--sync", "-g").AssertSuccess(t)
	if _, err := os.Stat(out); err != nil {
		t.Fatal("enable --sync did not republish the extension")
	}
	sb.RunCLI("hooks", "remove", "tool-log", "--sync", "-g").AssertSuccess(t)
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("remove --sync kept the extension")
	}
	if sb.ReadFile(mine) != "export default function () {}\n" || sb.ReadFile(guard) != "export default function () {}\n" {
		t.Fatal("sync touched the user's own extension or hook factory")
	}
}

func TestHooksOMP_ProjectAndConflict(t *testing.T) {
	sb := newOMPHooksSandbox(t)
	defer sb.Cleanup()
	project := sb.SetupProjectDir()
	out := filepath.Join(project, ".omp", "extensions", "skillshare-tool-log.ts")
	sb.WriteFile(out, "// mine\n")
	entry := writeOMPEntry(t, sb, "omp")

	sb.RunCLIInDir(project, "hooks", "add", "tool-log", "--file", entry, "-p").AssertSuccess(t)
	r := sb.RunCLIInDir(project, "hooks", "sync", "-p")
	r.AssertFailure(t)
	r.AssertAnyOutputContains(t, "conflict")
	if sb.ReadFile(out) != "// mine\n" {
		t.Fatal("conflict overwrote the user's file")
	}
	sb.RunCLIInDir(project, "hooks", "sync", "tool-log", "--replace", "-p").AssertSuccess(t)
	if !sameCode(sb.ReadFile(out)) {
		t.Fatal("--replace did not take the file over")
	}
	if _, err := os.Stat(filepath.Join(sb.Home, ".omp")); !os.IsNotExist(err) {
		t.Fatal("project sync wrote the global OMP home")
	}
}

func TestHooksOMP_AccountAndSharedAgentDir(t *testing.T) {
	sb := newOMPHooksSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets:\n  omp-work: {agent: omp, config_dir: ~/.omp-work, skills: {enabled: false}}\n")
	if err := os.MkdirAll(filepath.Join(sb.Home, ".omp-work"), 0755); err != nil {
		t.Fatal(err)
	}
	sb.RunCLI("hooks", "add", "tool-log", "--file", writeOMPEntry(t, sb, "omp-work"), "--sync", "-g").AssertSuccess(t)
	if !sameCode(sb.ReadFile(filepath.Join(sb.Home, ".omp-work", "extensions", "skillshare-tool-log.ts"))) {
		t.Fatal("account extension missing")
	}
	if _, err := os.Stat(filepath.Join(sb.Home, ".omp")); !os.IsNotExist(err) {
		t.Fatal("account sync wrote the default OMP home")
	}
	// PI_CODING_AGENT_DIR points Pi and OMP at the same directory, so a hook bound to both must stop before writing.
	shared := filepath.Join(sb.Home, "shared-agent")
	sb.SetEnv("PI_CODING_AGENT_DIR", shared)
	r := sb.RunCLI("hooks", "add", "both", "--file", writeOMPEntry(t, sb, "pi", "omp"), "--sync", "-g")
	r.AssertFailure(t)
	r.AssertAnyOutputContains(t, "both write")
	if _, err := os.Stat(shared); !os.IsNotExist(err) {
		t.Fatal("blocked sync created the shared agent directory")
	}
}
