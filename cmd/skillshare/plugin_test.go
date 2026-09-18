package main

import "testing"

func TestPluginOptions(t *testing.T) {
	o, err := parsePluginOptions([]string{"add", "owner/repo", "--plugin", "demo", "--target", "claude", "--target", "codex", "--json"})
	if err != nil || o.request.Source != "owner/repo" || len(o.request.Targets) != 2 || !o.json {
		t.Fatalf("%+v %v", o, err)
	}
	for _, args := range [][]string{{"unknown"}, {"add", "source", "--target"}, {"remove", "a", "b"}, {"list", "--oops"}} {
		if _, err := parsePluginOptions(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestPluginAntigravityAlias(t *testing.T) {
	o, err := parsePluginOptions([]string{"add", "./demo", "--target", "agy", "--no-tui"})
	if err != nil || o.request.Targets[0] != "antigravity" {
		t.Fatalf("alias: %+v %v", o, err)
	}
}
