//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/plugin"
	"skillshare/internal/testutil"
)

func TestPluginCommandsCLI(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR", "OMP_CONFIG_DIR"} {
		t.Setenv(key, "")
	}
	sb.WriteConfig("targets: {}\n")
	root := filepath.Join(sb.Home, "command-source")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	write := func(path, content string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("plugin.json", `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","version":"1.0.0","extensions":{"cc.runkids.skillshare":{"commands":{"demo-cli":"bin/demo"}}}}`, 0644)
	write("bin/demo", "#!/bin/sh\nprintf original\n", 0755)
	preview := sb.RunCLI("plugin", "add", root, "--dry-run", "--json", "-g")
	preview.AssertSuccess(t)
	var plan plugin.Plan
	if err := json.Unmarshal([]byte(preview.Stdout), &plan); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range plan.Changes {
		if c.Commands {
			found = strings.Contains(c.Message, filepath.Join(sb.Home, ".local", "bin"))
		}
	}
	if !found {
		t.Fatalf("preview lost scoped commands: %s", preview.Stdout)
	}
	global := filepath.Join(sb.Home, ".local", "bin", "demo-cli")
	if _, err := os.Lstat(global); !os.IsNotExist(err) {
		t.Fatal("dry-run published command")
	}
	sb.RunCLI("plugin", "add", root, "--target", "cursor", "--json", "-g").AssertSuccess(t)
	output := func(want string) {
		t.Helper()
		cmd := exec.Command("/bin/sh", "-c", "demo-cli")
		cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(global)+":/usr/bin:/bin")
		out, err := cmd.CombinedOutput()
		if err != nil || string(out) != want {
			t.Fatalf("command output %q, %v, want %s", out, err, want)
		}
	}
	output("original")
	write("bin/demo", "#!/bin/sh\nprintf updated\n", 0755)
	sb.RunCLI("plugin", "update", "demo", "--target", "cursor", "--json", "-g").AssertSuccess(t)
	output("updated")
	if err := os.Remove(global); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(global, []byte("operator"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, root+"-offline"); err != nil {
		t.Fatal(err)
	}
	sb.RunCLI("plugin", "remove", "demo", "--json", "-g").AssertSuccess(t)
	if data, err := os.ReadFile(global); err != nil || string(data) != "operator" {
		t.Fatalf("operator replacement lost: %q %v", data, err)
	}
	if err := os.Rename(root+"-offline", root); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(sb.Home, "project")
	if err := os.MkdirAll(filepath.Join(project, ".skillshare"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".skillshare", "config.yaml"), []byte("targets: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sb.RunCLIInDir(project, "plugin", "add", root, "--json", "-p").AssertSuccess(t)
	if _, err := os.Lstat(filepath.Join(project, ".skillshare", "bin", "demo-cli")); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(global); string(data) != "operator" {
		t.Fatal("project publication touched global bin")
	}
}
