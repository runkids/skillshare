//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestPluginRexLifecycle(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	root := filepath.Join(sb.Home, "fixture")
	bin := filepath.Join(sb.Home, "bin")
	for path, content := range map[string]string{
		filepath.Join(root, "rex-plugin.toml"): "id = \"acme.test\"\nname = \"Test\"\nversion = \"1\"\nconfig = \"rex/config.lua\"\n",
		filepath.Join(root, "rex/config.lua"):  "-- test",
		filepath.Join(bin, "rex"):              "#!/bin/sh\nif [ \"$1\" = version ]; then echo '{\"rex_version\":\"1\"}'; else echo '{\"diagnostics\":[]}'; fi\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}
	}
	init := filepath.Join(sb.Home, "rex", "init.lua")
	env := map[string]string{"PATH": bin + ":" + os.Getenv("PATH"), "REX_CONFIG": init, "XDG_CONFIG_HOME": filepath.Join(sb.Home, ".config")}
	run := func(args ...string) { t.Helper(); sb.RunCLIEnv(env, args...).AssertSuccess(t) }
	run("plugin", "add", root, "--target", "rex", "--no-tui", "--json", "-g")
	data, err := os.ReadFile(init)
	if err != nil || !strings.Contains(string(data), "skillshare_rex") {
		t.Fatalf("init = %q, %v", data, err)
	}
	run("plugin", "check", "acme.test", "--json", "-g")
	if err := os.WriteFile(filepath.Join(root, "rex/config.lua"), []byte("-- updated"), 0644); err != nil {
		t.Fatal(err)
	}
	check := sb.RunCLIEnv(env, "plugin", "check", "acme.test", "--json", "-g")
	check.AssertSuccess(t)
	check.AssertOutputContains(t, "update-available")
	run("plugin", "update", "acme.test", "--no-tui", "--json", "-g")
	installed, err := filepath.Glob(filepath.Join(sb.Home, ".local", "state", "skillshare", "plugins", "*", "rex", "acme.test", "rex", "config.lua"))
	if err != nil || len(installed) != 1 {
		t.Fatalf("installed bundle = %v, %v", installed, err)
	}
	if content, err := os.ReadFile(installed[0]); err != nil || string(content) != "-- updated" {
		t.Fatalf("updated bundle = %q, %v", content, err)
	}

	run("plugin", "remove", "acme.test", "--no-tui", "--json", "-g")
	if _, err := os.Stat(init); !os.IsNotExist(err) {
		t.Fatalf("init remains: %v", err)
	}
}
