//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func doctorCheckNamed(t *testing.T, out doctorJSON, name string) doctorJSONCheck {
	t.Helper()
	for _, c := range out.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("doctor has no %q check: %+v", name, out.Checks)
	return doctorJSONCheck{}
}

func TestDoctor_JSON_UnconfiguredResourcesAreInfo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	out := parseDoctorJSON(t, sb.RunCLI("doctor", "--json").Stdout)

	for _, name := range []string{"mcp", "hooks", "plugins"} {
		if c := doctorCheckNamed(t, out, name); c.Status != "info" {
			t.Errorf("%s: want info when nothing is configured, got %+v", name, c)
		}
	}
}

func TestDoctor_JSON_MCPMissingVariableIsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nmcp:\n  targets: [claude]\n  servers:\n    local:\n      command: sh\n      env:\n        API_KEY: {fromEnv: SKILLSHARE_DOCTOR_TOKEN}\n")

	out := parseDoctorJSON(t, sb.RunCLI("doctor", "--json").Stdout)

	c := doctorCheckNamed(t, out, "mcp")
	if c.Status != "error" || !strings.Contains(strings.Join(c.Details, "\n"), "local") {
		t.Fatalf("want an MCP error naming the server, got %+v", c)
	}
}

func TestDoctor_JSON_HooksPendingSyncIsWarning(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	sb.RunCLI("hooks", "add", "bash-log", "--file", writeHooksEntry(t, sb), "-g").AssertSuccess(t)

	out := parseDoctorJSON(t, sb.RunCLI("doctor", "--json").Stdout)

	c := doctorCheckNamed(t, out, "hooks")
	if c.Status != "warning" || !strings.Contains(strings.Join(c.Details, "\n"), "bash-log") {
		t.Fatalf("want a hooks warning naming the unsynced entry, got %+v", c)
	}
}

func TestDoctor_JSON_GitHooksWithoutGitAreInactive(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	entry := filepath.Join(sb.Root, "git-entry.yaml")
	sb.WriteFile(entry, "bindings:\n  git:\n    commands:\n      check:\n        events: [pre-commit]\n        command: \"true\"\n")
	sb.RunCLI("hooks", "add", "git-check", "--file", entry, "-g").AssertSuccess(t)

	// An empty PATH makes Git unavailable to the doctor run only.
	out := parseDoctorJSON(t, sb.RunCLIEnv(map[string]string{"PATH": ""}, "doctor", "--json").Stdout)

	details := strings.Join(doctorCheckNamed(t, out, "hooks").Details, "\n")
	if !strings.Contains(details, "inactive: git not found") || strings.Contains(details, "not synced") {
		t.Fatalf("want Git reported as missing, not as an unsynced change, got %q", details)
	}
}

func TestDoctor_JSON_PluginMissingCLIIsWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  codex-x: {agent: codex, config_dir: ~/.codex-x, cli: no-such-codex-cli, skills: {enabled: false}}\nplugins:\n  packages:\n    demo:\n      bindings:\n        codex-x:\n          id: demo@team\n")

	out := parseDoctorJSON(t, sb.RunCLI("doctor", "--json").Stdout)

	c := doctorCheckNamed(t, out, "plugins")
	if c.Status != "warning" || !strings.Contains(strings.Join(c.Details, "\n"), "demo") {
		t.Fatalf("want a plugins warning naming the blocked package, got %+v", c)
	}
}

func writeDoctorExtra(t *testing.T, sb *testutil.Sandbox, targetYAML string) (source, target string) {
	t.Helper()
	source = filepath.Join(filepath.Dir(sb.SourcePath), "extras", "rules")
	target = filepath.Join(sb.Home, ".claude", "rules")
	for _, dir := range []string{source, target} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "coding.md"), []byte("# Coding"), 0644); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nextras:\n  - name: rules\n    targets:\n      - path: " + target + "\n" + targetYAML)
	return source, target
}

func TestDoctor_JSON_ExtrasNotSyncedIsWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	writeDoctorExtra(t, sb, "")

	out := parseDoctorJSON(t, sb.RunCLI("doctor", "--json").Stdout)

	c := doctorCheckNamed(t, out, "extras")
	if c.Status != "warning" || !strings.Contains(strings.Join(c.Details, "\n"), "out of sync") {
		t.Fatalf("want an extras drift warning, got %+v", c)
	}
}

func TestDoctor_JSON_ExtrasSyncedPasses(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	writeDoctorExtra(t, sb, "")
	sb.RunCLI("sync", "extras").AssertSuccess(t)

	out := parseDoctorJSON(t, sb.RunCLI("doctor", "--json").Stdout)

	if c := doctorCheckNamed(t, out, "extras"); c.Status != "pass" {
		t.Fatalf("want synced extras to pass, got %+v", c)
	}
}

func TestDoctor_JSON_ExtrasBrokenSymlinkIsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, target := writeDoctorExtra(t, sb, "")
	sb.RunCLI("sync", "extras").AssertSuccess(t)
	if err := os.Symlink(filepath.Join(source, "deleted.md"), filepath.Join(target, "deleted.md")); err != nil {
		t.Fatal(err)
	}

	out := parseDoctorJSON(t, sb.RunCLI("doctor", "--json").Stdout)

	c := doctorCheckNamed(t, out, "extras")
	if c.Status != "error" || !strings.Contains(strings.Join(c.Details, "\n"), "deleted.md") {
		t.Fatalf("want an extras error naming the broken link, got %+v", c)
	}
}

func TestDoctor_JSON_ExtrasInvalidModeIsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	writeDoctorExtra(t, sb, "        mode: mirror\n")

	out := parseDoctorJSON(t, sb.RunCLI("doctor", "--json").Stdout)

	c := doctorCheckNamed(t, out, "extras")
	if c.Status != "error" || !strings.Contains(strings.Join(c.Details, "\n"), "mirror") {
		t.Fatalf("want an extras error naming the invalid mode, got %+v", c)
	}
}

func TestDoctor_JSON_ProjectModeChecksMCP(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.WriteFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"), "targets:\n  - claude\nmcp:\n  targets: [claude]\n  servers:\n    local:\n      command: no-such-mcp-binary\n")

	out := parseDoctorJSON(t, sb.RunCLIInDir(projectRoot, "doctor", "--json").Stdout)

	c := doctorCheckNamed(t, out, "mcp")
	if c.Status != "error" || !strings.Contains(strings.Join(c.Details, "\n"), "local") {
		t.Fatalf("want a project MCP error naming the server, got %+v", c)
	}
}
