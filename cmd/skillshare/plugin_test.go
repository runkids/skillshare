package main

import (
	"strings"
	"testing"
)

func TestPluginOptions(t *testing.T) {
	o, err := parsePluginOptions([]string{"add", "owner/repo", "--plugin", "demo", "--target", "claude", "--target", "codex", "--json"})
	if err != nil || o.request.Source != "owner/repo" || len(o.request.Targets) != 2 || !o.json {
		t.Fatalf("%+v %v", o, err)
	}
	for _, args := range [][]string{{"unknown"}, {"add", "source", "--target"}, {"remove", "a", "b"}, {"list", "--oops"}, {"update", "demo", "--all"}, {"update", "--name", "demo", "-a"}, {"update", "--all", "--source-ref", "main"}, {"sync", "--all"}} {
		if _, err := parsePluginOptions(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, flag := range []string{"--all", "-a"} {
		u, err := parsePluginOptions([]string{"update", flag, "--target", "claude"})
		if err != nil || !u.request.All || u.request.Name != "" || len(u.request.Targets) != 1 {
			t.Fatalf("update %s: %+v %v", flag, u, err)
		}
	}
}

func TestPluginCompletionsIncludeAllFlag(t *testing.T) {
	want := map[string]string{
		"bash":       "plugin)     COMPREPLY=($(compgen -W \"add discover import list inspect sync check update enable disable remove --target --from --plugin --name --source-ref --entry --revision --all -a",
		"zsh":        "'--all[Update all managed plugins (update only)]' \\\n                        '-a[Update all managed plugins (update only)]'",
		"fish":       "complete -c skillshare -n '__fish_skillshare_using_command plugin' -l all -s a",
		"powershell": "'plugin' = '--target', '--from', '--plugin', '--name', '--source-ref', '--entry', '--revision', '--all', '-a'",
		"nushell":    "--revision: string\n    --all(-a)",
	}
	for shell, snippet := range want {
		if !strings.Contains(shells[shell].script, snippet) {
			t.Fatalf("%s completion does not advertise plugin --all/-a", shell)
		}
	}
}

func TestPluginAntigravityAlias(t *testing.T) {
	o, err := parsePluginOptions([]string{"add", "./demo", "--target", "agy", "--no-tui"})
	if err != nil || o.request.Targets[0] != "antigravity" {
		t.Fatalf("alias: %+v %v", o, err)
	}
}
