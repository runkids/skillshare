//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/hooks"
)

func TestGitHooksCLILifecycle(t *testing.T) {
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	sb.SetEnv("GIT_CONFIG_GLOBAL", filepath.Join(sb.Home, "custom.gitconfig"))
	sb.SetEnv("GIT_CONFIG_SYSTEM", filepath.Join(sb.Home, "system.gitconfig"))
	sb.SetEnv("GIT_CONFIG_NOSYSTEM", "1")
	path := filepath.Join(sb.Root, "git-entry.yaml")
	sb.WriteFile(path, "bindings:\n  git:\n    commands:\n      check:\n        events: [pre-commit]\n        command: '{files}/check.sh'\n    files:\n      check.sh: |\n        #!/bin/sh\n        exit 0\n")
	file := filepath.Join(sb.Home, ".config", "git", "skillshare", "hooks.gitconfig")
	sb.RunCLI("hooks", "add", "guard", "--file", path, "-g").AssertSuccess(t)
	sb.RunCLI("hooks", "sync", "--dry-run", "-g").AssertSuccess(t)
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("source-only add or preview published hooks")
	}
	sb.RunCLI("hooks", "sync", "-g").AssertSuccess(t)
	list := sb.RunCLI("hooks", "list", "--json", "-g")
	list.AssertSuccess(t)
	var inv hooks.Inventory
	if err := json.Unmarshal([]byte(list.Stdout), &inv); err != nil {
		t.Fatal(err)
	}
	if inv.Git == nil || !inv.Git.Include.Owned || inv.Git.Include.Target != filepath.Join(sb.Home, "custom.gitconfig") {
		t.Fatalf("CLI inventory: %+v", inv.Git)
	}
	sb.RunCLI("hooks", "disable", "guard", "--sync", "-g").AssertSuccess(t)
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("disable did not remove owned hooks file")
	}
	sb.RunCLI("hooks", "enable", "guard", "--sync", "-g").AssertSuccess(t)
	r := sb.RunCLI("hooks", "remove", "guard", "--keep-files", "-g")
	r.AssertFailure(t)
	r.AssertAnyOutputContains(t, "cannot keep files")
	r = sb.RunCLI("hooks", "import", "--from", "git", "-g")
	r.AssertFailure(t)
	r.AssertAnyOutputContains(t, "not supported yet")
	sb.RunCLI("hooks", "remove", "guard", "--sync", "-g").AssertSuccess(t)
	// Project mode publishes only in common-dir config. Repository initialization
	// runs before any hooks are present and never creates a commit.
	root := filepath.Join(sb.Root, "repo")
	sb.WriteFile(filepath.Join(root, ".skillshare", "config.yaml"), "targets: []\n")
	cmd := exec.Command("git", "init", "--quiet", root)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+filepath.Join(sb.Home, "custom.gitconfig"), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init: %v %s", err, out)
	}
	sb.RunCLIInDir(root, "hooks", "add", "guard", "--file", path, "--sync", "-p").AssertSuccess(t)
	if !strings.Contains(sb.ReadFile(filepath.Join(root, ".git", "config")), "skillshare/hooks.gitconfig") {
		t.Fatal("project include missing")
	}
	sb.RunCLIInDir(root, "hooks", "remove", "guard", "--sync", "-p").AssertSuccess(t)
}
